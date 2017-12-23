// +build !custom,integration custom,mysql,integration

package integrationtest

import (
	"database/sql"
	"strings"
	"testing"

	"github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/require"
)

func TestMySQL(t *testing.T) {
	cfg, err := mysql.ParseDSN(mustGetEnv(t, "MYSQL_DSN"))
	require.NoError(t, err)
	cfg.MultiStatements = true
	cfg.ParseTime = true

	sdb := &mysqlSharedTestDB{
		driver: "mysql",
		dsn:    cfg.FormatDSN(),
	}

	sharedTest(t, sdb)
}

// mysqlSharedTestDB implements the sharedTestDB interface.
type mysqlSharedTestDB struct {
	driver string
	dsn    string
}

func (o *mysqlSharedTestDB) DriverName() string {
	return o.driver
}

func (o *mysqlSharedTestDB) DSN() string {
	return o.dsn
}

func (o *mysqlSharedTestDB) ConnectAndResetDB(t *testing.T) *sql.DB {
	db, err := sql.Open("mysql", o.dsn)
	require.NoError(t, err)
	defer func() {
		if t.Failed() {
			db.Close()
		}
	}()

	_, err = db.Exec(`
		DROP TABLE IF EXISTS ` + quoteMySQLIdentifier(migrationsTable) + `;
		CREATE TABLE IF NOT EXISTS test_events (
			id INT AUTO_INCREMENT PRIMARY KEY,
			event_name VARCHAR(255) NOT NULL
		);
		TRUNCATE TABLE test_events;`,
	)
	require.NoError(t, err)
	runSQLMigrate(t, true, nil, nil, "init", "-driver", "mysql", "-dsn", o.dsn, "-migrations_table", migrationsTable)
	return db
}

func (o *mysqlSharedTestDB) GetTestEvents(t *testing.T, db *sql.DB) []string {
	var events []string
	rows, err := db.Query("SELECT event_name FROM test_events ORDER BY id")
	require.NoError(t, err)
	defer rows.Close()

	for rows.Next() {
		var e string
		err := rows.Scan(&e)
		require.NoError(t, err, "error scanning test_event result set")
		events = append(events, e)
	}
	require.NoError(t, rows.Err())
	return events
}

func (o *mysqlSharedTestDB) GetForwardMigrated(t *testing.T, db *sql.DB) []string {
	rows, err := db.Query("SELECT name FROM " + quoteMySQLIdentifier(migrationsTable) + " ORDER BY name")
	require.NoError(t, err)
	defer rows.Close()

	var migrationNames []string
	for rows.Next() {
		var name string
		err := rows.Scan(&name)
		require.NoError(t, err, "error scanning migration result set")
		migrationNames = append(migrationNames, name)
	}
	require.NoError(t, rows.Err())
	return migrationNames
}

func quoteMySQLIdentifier(s string) string {
	return "`" + strings.Replace(s, "`", "``", -1) + "`"
}
