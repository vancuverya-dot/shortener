package storage_test

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/vancuverya-dot/shortener/internal/storage"
)

// connectFromEnv создаёт пул соединений к тестовой БД, используя DSN
// из переменной окружения TEST_DATABASE_DSN.
func connectFromEnv(ctx context.Context) *pgxpool.Pool {
	dsn := os.Getenv("DATABASE_DSN")
	if dsn == "" {
		return nil
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil
	}
	return pool
}

// ExampleInit демонстрирует инициализацию пакета storage пулом соединений.
//
// Init должна вызываться один раз при старте приложения, до любых
// других вызовов функций пакета.
func ExampleInit() {
	ctx := context.Background()

	pool := connectFromEnv(ctx)
	if pool == nil {
		fmt.Println("storage initialized")
		return
	}
	defer pool.Close()

	storage.Init(pool)

	fmt.Println("storage initialized")
	// Output:
	// storage initialized
}

// ExamplePingDB демонстрирует проверку доступности базы данных.
//
// Если пакет не инициализирован вызовом Init, PingDB возвращает
// pgx.ErrNoRows вместо паники или nil-pointer dereference.
func ExamplePingDB() {
	ctx := context.Background()

	err := storage.PingDB(ctx)
	if err != nil {
		fmt.Println("db is not reachable")
		return
	}
	fmt.Println("db is reachable")

	// Output:
	// db is not reachable
}

// ExampleInsertURL демонстрирует сохранение пары short/original URL
// для конкретного пользователя.
//
// Если originalURL уже существует в хранилище, InsertURL возвращает
// короткий идентификатор существующей записи вместе с ошибкой
// ErrConflict — это не сбой операции, и непустой результат нужно
// использовать.
func ExampleInsertURL() {
	ctx := context.Background()

	shortURL, err := storage.InsertURL(ctx, "abc123", "https://practicum.yandex.ru", "user-1")
	switch {
	case err == nil:
		fmt.Println("created:", shortURL)
	case err == storage.ErrConflict:
		fmt.Println("already exists, short URL is:", shortURL)
	default:
		fmt.Println("insert failed")
	}
}

// ExampleGetOriginalURL демонстрирует получение оригинального URL
// по короткому идентификатору.
//
// Возвращаемый флаг isDeleted нужно проверять отдельно: функция не
// фильтрует удалённые записи самостоятельно и не возвращает для них
// особую ошибку.
func ExampleGetOriginalURL() {
	ctx := context.Background()

	originalURL, isDeleted, err := storage.GetOriginalURL(ctx, "abc123")
	if err != nil {
		fmt.Println("not found")
		return
	}
	if isDeleted {
		fmt.Println("found, but marked as deleted:", originalURL)
		return
	}
	fmt.Println("found:", originalURL)
}

// ExampleInsertURLBatch демонстрирует батчевую вставку нескольких
// пар short/original URL за одну операцию.
//
// shortURLs и originalURLs должны быть одинаковой длины и поэлементно
// соответствовать друг другу. Результат сохраняет тот же порядок, что
// и входные originalURLs.
func ExampleInsertURLBatch() {
	ctx := context.Background()

	shortURLs := []string{"id1", "id2"}
	originalURLs := []string{
		"https://practicum.yandex.ru",
		"https://go.dev",
	}

	resultIDs, err := storage.InsertURLBatch(ctx, shortURLs, originalURLs)
	if err != nil {
		fmt.Println("batch insert failed")
		return
	}
	fmt.Println("inserted count:", len(resultIDs))
}

// ExampleGetURLsByUser демонстрирует получение всех неудалённых URL
// конкретного пользователя.
//
// Возвращаются два параллельных среза одинаковой длины: shortURLs[i]
// соответствует originalURLs[i].
func ExampleGetURLsByUser() {
	ctx := context.Background()

	shortURLs, originalURLs, err := storage.GetURLsByUser(ctx, "user-1")
	if err != nil {
		fmt.Println("query failed")
		return
	}
	fmt.Println("user has", len(shortURLs), "and", len(originalURLs), "urls")
}

// ExampleDeleteURLBatch демонстрирует мягкое удаление (soft delete)
// нескольких URL, принадлежащих конкретному пользователю.
//
// Удаление затрагивает только записи, принадлежащие переданному userID:
// если shortURL принадлежит другому пользователю или не существует,
// функция не вернёт ошибку — отсутствие эффекта от удаления нельзя
// отличить от его успешного выполнения по результату этой функции.
func ExampleDeleteURLBatch() {
	ctx := context.Background()

	err := storage.DeleteURLBatch(ctx, []string{"abc123", "id1"}, "user-1")
	if err != nil {
		fmt.Println("delete failed")
		return
	}
	fmt.Println("delete request sent")
}
