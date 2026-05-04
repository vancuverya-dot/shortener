package main

import (
	"embed"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"

	// _ "github.com/golang-migrate/migrate/v4/source/file"

	"github.com/golang-migrate/migrate/v4/source/iofs"
)

// инструкция компилятору для встраивания файлов из папки migrations в исполняемый файл
//
//go:embed migrations/*.sql
var migrationsFS embed.FS

func migration() error {

	src, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return err
	}

	m, err := migrate.NewWithSourceInstance("iofs", src, "postgres://postgres:1@localhost:5432/ygo?sslmode=disable")
	if err != nil {
		return err
	}
	defer m.Close()

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return err
	}
	return nil
}
