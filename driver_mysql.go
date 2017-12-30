// +build !custom custom,mysql

package main

import (
	"database/sql"
	"fmt"
	"log"
	"strings"

	"github.com/go-sql-driver/mysql"
)

func init() {
	drivers["mysql"] = newMySQLDriver
}

func newMySQLDriver(dsn, tableName string) (Driver, error) {
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		return nil, err
	}
	cfg.MultiStatements = true
	cfg.ParseTime = true
	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		return nil, err
	}

	return &mySQLDriver{
		db:        dbWrapper{db},
		tableName: quoteMySQLIdentifier(tableName),
	}, nil
}

func quoteMySQLIdentifier(s string) string {
	return "`" + strings.Replace(s, "`", "``", -1) + "`"
}

type mySQLDriver struct {
	db        DB
	tableName string
}

func (o *mySQLDriver) ExecuteStep(st *Step, contents string) error {
	// MySQL doesn't support DDL statements inside transactions.
	// For this reason we don't even try to open a transaction.
	if _, err := o.db.Exec(contents); err != nil {
		return err
	}
	return o.SetMigrationState(o.db, st.MigrationName, st.ParsedFilename.Direction == DirectionForward)
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

func (o *mySQLDriver) CreateMigrationsTable() error {
	query := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			%s VARCHAR(255) PRIMARY KEY,
			%s DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);`,
		o.tableName, "`name`", "`time`",
	)
	_, err := o.db.Exec(query)
	return err
}

func (o *mySQLDriver) GetForwardMigratedNames() (map[string]struct{}, error) {
	rows, err := o.db.Query("SELECT `name` FROM " + o.tableName)
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

func (o *mySQLDriver) Close() {
	if err := o.db.Close(); err != nil {
		log.Print(err)
	}
}
