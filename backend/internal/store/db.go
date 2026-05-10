package store

import (
	"database/sql"

	"quizsystem/internal/store/ent"

	"encore.dev/storage/sqldb"
	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
)

var DB = sqldb.NewDatabase("quiz_db", sqldb.DatabaseConfig{
	Migrations: "migrations",
})

var EntClient *ent.Client

func InitEnt(db *sql.DB) {
	drv := entsql.OpenDB(dialect.Postgres, db)
	EntClient = ent.NewClient(ent.Driver(drv))
}
