package main

import (
	"database/sql"
	"github.com/Mightyfin/party-unification-platform/internal/config"
	"github.com/Mightyfin/party-unification-platform/internal/database"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"log"
)

func main() {
	c, e := config.Load()
	if e != nil {
		log.Fatal(e)
	}
	db, e := sql.Open("pgx", c.DatabaseURL)
	if e != nil {
		log.Fatal(e)
	}
	defer db.Close()
	goose.SetBaseFS(database.Migrations)
	if e = goose.SetDialect("postgres"); e != nil {
		log.Fatal(e)
	}
	if e = goose.Up(db, "migrations"); e != nil {
		log.Fatal(e)
	}
}
