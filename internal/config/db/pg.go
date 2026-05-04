package db

import (
	"context"

	"github.com/jackc/pgx/v5"
)

var dbConn *pgx.Conn

func InitDB(conn string) error {
	var err error
	dbConn, err = pgx.Connect(context.Background(), conn)
	if err != nil {
		return err
	}

	return nil
}

func CloseDB() {
	if dbConn != nil {
		dbConn.Close(context.Background())
	}
}

func PingDB(ctx context.Context) error {
	if dbConn == nil {
		return pgx.ErrNoRows
	}
	return dbConn.Ping(ctx)
}

func InsertURL(ctx context.Context, shortURL string, originalURL string) error {
	_, err := dbConn.Exec(ctx,
		"INSERT INTO public.urls (urls_short_url, urls_original_url) VALUES ($1, $2)",
		shortURL, originalURL,
	)
	if err != nil {
		return err
	}
	return nil
}

func GetOriginalURL(ctx context.Context, shortURL string) (string, error) {
	var originalURL string
	err := dbConn.QueryRow(ctx,
		"SELECT urls_original_url FROM public.urls WHERE urls_short_url = $1",
		shortURL,
	).Scan(&originalURL)
	if err != nil {
		return "", err
	}
	return originalURL, nil
}
