package storage

import (
	"context"
	"errors"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var _dbConn *pgxpool.Pool

func Init(dbConn *pgxpool.Pool) {
	_dbConn = dbConn
}

func PingDB(ctx context.Context) error {
	if _dbConn == nil {
		return pgx.ErrNoRows
	}
	return _dbConn.Ping(ctx)
}

var ErrConflict = errors.New("original URL already exists")

func InsertURL(ctx context.Context, shortURL string, originalURL string, userID string) (string, error) {

	var existingShort string
	err := _dbConn.QueryRow(ctx,
		`UPDATE public.urls SET is_deleted = FALSE, user_id = $1 WHERE urls_original_url = $2 AND is_deleted = true RETURNING urls_short_url`,
		userID, originalURL,
	).Scan(&existingShort)

	if err == nil {
		return existingShort, nil
	}

	_, err = _dbConn.Exec(ctx, "INSERT INTO public.urls (urls_short_url, urls_original_url, user_id) VALUES ($1, $2, $3)",
		shortURL, originalURL, userID,
	)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
			existing, getErr := GetShortURL(ctx, originalURL)
			if getErr != nil {
				return "", getErr
			}
			return existing, ErrConflict
		}
		return "", err
	}

	return shortURL, nil
}

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

func InsertURLBatch(ctx context.Context, shortURLs []string, originalURLs []string) ([]string, error) {
	batch := &pgx.Batch{}

	for i := range shortURLs {
		batch.Queue(
			`INSERT INTO public.urls (urls_short_url, urls_original_url, user_id) 
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
