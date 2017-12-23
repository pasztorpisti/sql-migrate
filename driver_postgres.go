// +build !custom custom,postgres

package main

import (
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/lib/pq"
)

func init() {
	drivers["postgres"] = newPostgresDriver
}

func newPostgresDriver(tableName string) Driver {
	return &postgresDriver{
		tableName: quotePostgresIdentifier(tableName),
	}
}

func quotePostgresIdentifier(s string) string {
	return `"` + strings.Replace(s, `"`, `""`, -1) + `"`
}

type postgresDriver struct {
	tableName string
}

func (o *postgresDriver) Open(dsn string) (*sql.DB, error) {
	return sql.Open("postgres", dsn)
}

func (o *postgresDriver) CreateMigrationsTable(e Execer) error {
	_, err := e.Exec(`
		CREATE TABLE IF NOT EXISTS ` + o.tableName + `(
			"name" TEXT PRIMARY KEY,
			"time" TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT (now() AT TIME ZONE 'UTC')
		);`,
	)
	return err
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

func (o *postgresDriver) GetForwardMigratedNames(q Querier) (map[string]struct{}, error) {
	rows, err := q.Query(`SELECT "name" FROM ` + o.tableName)
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
