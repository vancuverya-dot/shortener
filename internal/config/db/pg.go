package db

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DB инкапсулирует пул соединений к PostgreSQL.
// Создаётся через New и закрывается методом Close при завершении программы.
type DB struct {
	pool *pgxpool.Pool
}

// New создаёт новый пул соединений к PostgreSQL по указанному DSN.
func New(ctx context.Context, dsn string) (*DB, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, err
	}
	return &DB{pool: pool}, nil
}

// Pool возвращает пул соединений для передачи в storage.Init.
func (d *DB) Pool() *pgxpool.Pool {
	return d.pool
}

// Close закрывает пул соединений. Должен вызываться при завершении программы.
func (d *DB) Close() {
	if d.pool != nil {
		d.pool.Close()
	}
}
