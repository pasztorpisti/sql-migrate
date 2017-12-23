// +build !custom custom,mysql

package main

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/go-sql-driver/mysql"
)

func init() {
	drivers["mysql"] = newMySQLDriver
}

func newMySQLDriver(tableName string) Driver {
	return &mySQLDriver{
		tableName: quoteMySQLIdentifier(tableName),
	}
}

func quoteMySQLIdentifier(s string) string {
	return "`" + strings.Replace(s, "`", "``", -1) + "`"
}

type mySQLDriver struct {
	tableName string
}

func (o *mySQLDriver) Open(dsn string) (*sql.DB, error) {
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		return nil, err
	}
	cfg.MultiStatements = true
	cfg.ParseTime = true
	return sql.Open("mysql", cfg.FormatDSN())
}

func (o *mySQLDriver) CreateMigrationsTable(e Execer) error {
	query := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			%s VARCHAR(255) PRIMARY KEY,
			%s DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);`,
		o.tableName, "`name`", "`time`",
	)
	_, err := e.Exec(query)
	return err
}

func (o *mySQLDriver) SetMigrationState(e Execer, migrationName string, forwardMigrated bool) error {
	query := "INSERT INTO " + o.tableName + " (`name`) VALUES (?)"
	if !forwardMigrated {
		query = "DELETE FROM " + o.tableName + " WHERE `name`=?"
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

func (o *mySQLDriver) GetForwardMigratedNames(q Querier) (map[string]struct{}, error) {
	rows, err := q.Query("SELECT `name` FROM " + o.tableName)
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
