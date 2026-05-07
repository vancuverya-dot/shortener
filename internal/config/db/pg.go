package db

import (
	"context"

	"github.com/jackc/pgx/v5"
)

var dbConn *pgx.Conn

func InitDB(conn string) (*pgx.Conn, error) {
	var err error
	dbConn, err = pgx.Connect(context.Background(), conn)
	if err != nil {
		return nil, err
	}

	return dbConn, nil
}

func CloseDB() {
	if dbConn != nil {
		dbConn.Close(context.Background())
	}
}
