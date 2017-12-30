// +build !custom custom,postgres

package main

import (
	"database/sql"
	"fmt"
	"log"
	"strings"

	_ "github.com/lib/pq"
)

func init() {
	drivers["postgres"] = newPostgresDriver
}

func newPostgresDriver(dsn, tableName string) (Driver, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	return &postgresDriver{
		db:        dbWrapper{db},
		tableName: quotePostgresIdentifier(tableName),
	}, nil
}

func quotePostgresIdentifier(s string) string {
	return `"` + strings.Replace(s, `"`, `""`, -1) + `"`
}

type postgresDriver struct {
	db        DB
	tableName string
}

func (o *postgresDriver) ExecuteStep(st *Step, contents string) error {
	performStep := func(e Execer) error {
		if _, err := e.Exec(contents); err != nil {
			return err
		}
		return o.SetMigrationState(e, st.MigrationName, st.ParsedFilename.Direction == DirectionForward)
	}

	if st.ParsedFilename.NoTx {
		return performStep(o.db)
	}

	tx, err := o.db.Begin()
	if err != nil {
		return err
	}
	if err := performStep(tx); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (o *postgresDriver) SetMigrationState(e Execer, migrationName string, forwardMigrated bool) error {
	query := `INSERT INTO ` + o.tableName + ` ("name") VALUES ($1)`
	if !forwardMigrated {
		query = `DELETE FROM ` + o.tableName + ` WHERE "name"=$1`
	}
	res, err := e.Exec(query, migrationName)
	if err != nil {
		return err
	}
	ra, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if ra != 1 {
		return fmt.Errorf("error setting state for migration %q in the migrations table (examine it with the status command and fix it manually)", migrationName)
	}
	return nil
}

func (o *postgresDriver) CreateMigrationsTable() error {
	_, err := o.db.Exec(`
		CREATE TABLE IF NOT EXISTS ` + o.tableName + `(
			"name" TEXT PRIMARY KEY,
			"time" TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT (now() AT TIME ZONE 'UTC')
		);`,
	)
	return err
}

func (o *postgresDriver) GetForwardMigratedNames() (map[string]struct{}, error) {
	rows, err := o.db.Query(`SELECT "name" FROM ` + o.tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	names := make(map[string]struct{})
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("error scanning forward migration result set: %s", err)
		}
		names[name] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row error: %s", err)
	}
	return names, nil
}

func (o *postgresDriver) Close() {
	if err := o.db.Close(); err != nil {
		log.Print(err)
	}
}
