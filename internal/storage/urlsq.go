// Package storage предоставляет доступ к хранилищу сокращённых URL
// в PostgreSQL через пул соединений pgx.
//
// Пакет инкапсулирует CRUD-операции над таблицей public.urls: создание
// коротких URL (одиночное и батчевое), получение оригинального URL по
// короткому идентификатору, получение всех URL пользователя и мягкое
// удаление (soft delete) через флаг is_deleted.
//
// Перед использованием пакета необходимо вызвать Init с готовым пулом
// соединений pgxpool.Pool — функции пакета обращаются к нему через
// внутреннюю пакетную переменную.
package storage

import (
	"context"
	"errors"
	"sync"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// _dbConn — пул соединений с базой данных, используемый всеми функциями
// пакета. Устанавливается вызовом Init и не защищён мьютексом: предполагается,
// что Init вызывается один раз при старте приложения, до начала конкурентного
// доступа к остальным функциям пакета.
var (
	_dbConn  *pgxpool.Pool
	initOnce sync.Once
)

// Init инициализирует пакет переданным пулом соединений к PostgreSQL.
//
// Должна быть вызвана один раз при старте приложения, до любых других
// вызовов функций пакета storage. Повторный вызов перезапишет текущий
// пул соединений.
func Init(dbConn *pgxpool.Pool) {
	initOnce.Do(func() {
		_dbConn = dbConn
	})
}

// PingDB проверяет доступность базы данных.
//
// Возвращает pgx.ErrNoRows, если пакет не был инициализирован вызовом Init.
// В остальных случаях возвращает результат Ping у пула соединений pgxpool.
func PingDB(ctx context.Context) error {
	if _dbConn == nil {
		return pgx.ErrNoRows
	}
	return _dbConn.Ping(ctx)
}

// ErrConflict возвращается, когда оригинальный URL уже существует
// в хранилище. В этом случае InsertURL и InsertURLBatch возвращают
// идентификатор уже существующей записи вместе с этой ошибкой,
// поэтому вызывающий код может использовать результат даже при ошибке.
var ErrConflict = errors.New("original URL already exists")

// InsertURL сохраняет пару short/original URL для указанного пользователя.
//
// Если originalURL уже существует в хранилище, новая запись не создаётся.
// Вместо этого:
//   - если существующая запись была помечена как удалённая (is_deleted),
//     она восстанавливается (is_deleted = FALSE) и привязывается к userID;
//
// В обоих случаях для существующей записи возвращается её текущий
// shortURL и ошибка ErrConflict — вызывающий код должен трактовать
// непустой результат при ErrConflict как корректный short URL, а не
// как признак сбоя операции.
//
// При успешной вставке новой записи возвращает shortURL и nil.
func InsertURL(ctx context.Context, shortURL string, originalURL string, userID string) (string, error) {

	var resultShort string
	var inserted bool

	err := _dbConn.QueryRow(ctx,
		`INSERT INTO public.urls (urls_short_url, urls_original_url, user_id)
         VALUES ($1, $2, $3)
         ON CONFLICT (urls_original_url) DO UPDATE
             SET is_deleted = FALSE,
                 user_id    = CASE WHEN public.urls.is_deleted THEN $3 ELSE public.urls.user_id END
         RETURNING urls_short_url, (xmax = 0) AS inserted`,
		shortURL, originalURL, userID,
	).Scan(&resultShort, &inserted)

	if err != nil {
		return "", err
	}

	if !inserted {
		return resultShort, ErrConflict
	}

	return resultShort, nil
}

// GetOriginalURL возвращает оригинальный URL по его короткому идентификатору.
//
// # Возвращаемое значение isDeleted указывает, помечена ли запись как удалённая
//
// Если запись с указанным shortURL не найдена, возвращает ошибку
// (pgx.ErrNoRows либо иную ошибку драйвера).
func GetOriginalURL(ctx context.Context, shortURL string) (string, bool, error) {
	var originalURL string
	var isDeleted bool
	err := _dbConn.QueryRow(ctx,
		"SELECT urls_original_url, is_deleted FROM public.urls WHERE urls_short_url = $1",
		shortURL,
	).Scan(&originalURL, &isDeleted)
	if err != nil {
		return "", false, err
	}
	return originalURL, isDeleted, nil
}

// GetShortURL возвращает идентификатор существующей неудалённой записи
// по значению, переданному в shortURL.
//
// Если подходящая запись не найдена, возвращает ошибку (pgx.ErrNoRows
// либо иную ошибку драйвера).
func GetShortURL(ctx context.Context, shortURL string) (string, error) {
	var originalURL string
	err := _dbConn.QueryRow(ctx,
		"SELECT urls_short_url FROM public.urls WHERE urls_original_url = $1 and is_deleted = false",
		shortURL,
	).Scan(&originalURL)
	if err != nil {
		return "", err
	}
	return originalURL, nil
}

// InsertURLBatch сохраняет несколько пар short/original URL за одну
// батчевую операцию (pgx.Batch), без привязки к конкретному пользователю.
//
// Если очередной originalURL уже существует в хранилище (конфликт
// unique-индекса, pgerrcode.UniqueViolation), функция не прерывает всю
// операцию: она восстанавливает существующую запись (is_deleted = FALSE),
// находит её текущий короткий идентификатор через GetShortURL и
// подставляет его в результат вместо переданного shortURLs[i].
//
// Возвращает срез коротких идентификаторов в том же порядке, что и
// входные originalURLs: для новых URL — переданный shortURLs[i], для
// уже существующих — фактический идентификатор из хранилища.
//
// При любой иной ошибке (не UniqueViolation) выполнение прерывается,
// и функция возвращает nil и эту ошибку.
func InsertURLBatch(ctx context.Context, shortURLs []string, originalURLs []string) ([]string, error) {
	batch := &pgx.Batch{}

	for i := range shortURLs {
		batch.Queue(
			`INSERT INTO public.urls (urls_short_url, urls_original_url) 
             VALUES ($1, $2) 
             ON CONFLICT (urls_original_url) DO UPDATE 
             SET is_deleted = FALSE
             RETURNING urls_short_url`,
			shortURLs[i], originalURLs[i],
		)
	}

	results := _dbConn.SendBatch(ctx, batch)
	defer results.Close()

	resultIDs := make([]string, 0, len(shortURLs))

	for i := range shortURLs {
		_, err := results.Exec()
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
				existing, getErr := GetShortURL(ctx, originalURLs[i])
				if getErr != nil {
					return nil, getErr
				}
				resultIDs = append(resultIDs, existing)
				continue
			}
			return nil, err
		}
		resultIDs = append(resultIDs, shortURLs[i])
	}

	return resultIDs, nil
}

// GetURLsByUser возвращает все неудалённые URL, принадлежащие пользователю
// userID.
//
// Возвращает два параллельных среза одинаковой длины: shortURLs[i]
// соответствует originalURLs[i]. Если у пользователя нет записей,
// возвращает два пустых (nil) среза без ошибки.
func GetURLsByUser(ctx context.Context, userID string) ([]string, []string, error) {
	rows, err := _dbConn.Query(ctx,
		"SELECT urls_short_url, urls_original_url FROM public.urls WHERE user_id = $1 and is_deleted = false",
		userID,
	)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var shortURLs, originalURLs []string
	for rows.Next() {
		var short, original string
		if err := rows.Scan(&short, &original); err != nil {
			return nil, nil, err
		}
		shortURLs = append(shortURLs, short)
		originalURLs = append(originalURLs, original)
	}
	return shortURLs, originalURLs, nil
}

// DeleteURLBatch помечает несколько URL пользователя userID как удалённые
// (soft delete), выставляя is_deleted = TRUE, за одну батчевую операцию.
//
// Удаление применяется к записям, принадлежащим userID: попытка
// удалить shortURL, принадлежащий другому пользователю
//
// Возвращает ошибку при сбое выполнения батча;
func DeleteURLBatch(ctx context.Context, shortURLs []string, userID string) error {
	batch := &pgx.Batch{}

	for _, shortURL := range shortURLs {
		batch.Queue(
			"UPDATE public.urls SET is_deleted = TRUE WHERE urls_short_url = $1 AND user_id = $2",
			shortURL, userID,
		)
	}

	results := _dbConn.SendBatch(ctx, batch)
	defer results.Close()

	for range shortURLs {
		if _, err := results.Exec(); err != nil {
			return err
		}
	}

	return nil
}

// GetStats возвращает количество сокращённых URL и количество
// пользователей в сервисе. Удалённые URL не учитываются.
func GetStats(ctx context.Context) (int, int, error) {
	if _dbConn == nil {
		return 0, 0, pgx.ErrNoRows
	}

	var urls, users int
	err := _dbConn.QueryRow(ctx,
		`SELECT COUNT(*), COUNT(DISTINCT user_id) FROM public.urls WHERE is_deleted = false`,
	).Scan(&urls, &users)
	if err != nil {
		return 0, 0, err
	}
	return urls, users, nil
}
