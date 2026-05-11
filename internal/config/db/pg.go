package db

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

var dbConn *pgxpool.Pool

func InitDB(conn string) (*pgxpool.Pool, error) {

	var err error
	dbConn, err = pgxpool.New(context.Background(), conn)
	if err != nil {
		return nil, err
	}
	return dbConn, nil
}

func CloseDB() {
	if dbConn != nil {
		dbConn.Close()
	}
}
