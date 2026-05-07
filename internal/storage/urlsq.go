package storage

import (
	"context"
	"errors"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var _dbConn *pgx.Conn

func Init(dbConn *pgx.Conn) {
	_dbConn = dbConn
}

func PingDB(ctx context.Context) error {
	if _dbConn == nil {
		return pgx.ErrNoRows
	}
	return _dbConn.Ping(ctx)
}

var ErrConflict = errors.New("original URL already exists")

func InsertURL(ctx context.Context, shortURL string, originalURL string) (string, error) {
	_, err := _dbConn.Exec(ctx, "INSERT INTO public.urls (urls_short_url, urls_original_url) VALUES ($1, $2)",
		shortURL, originalURL,
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

func GetOriginalURL(ctx context.Context, shortURL string) (string, error) {
	var originalURL string
	err := _dbConn.QueryRow(ctx,
		"SELECT urls_original_url FROM public.urls WHERE urls_short_url = $1",
		shortURL,
	).Scan(&originalURL)
	if err != nil {
		return "", err
	}
	return originalURL, nil
}

func GetShortURL(ctx context.Context, shortURL string) (string, error) {
	var originalURL string
	err := _dbConn.QueryRow(ctx,
		"SELECT urls_short_url FROM public.urls WHERE urls_original_url = $1",
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
			"INSERT INTO public.urls (urls_short_url, urls_original_url) VALUES ($1, $2) ON CONFLICT (urls_original_url) DO NOTHING",
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
