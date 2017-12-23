// +build !custom,integration custom,postgres,integration

package integrationtest

import (
	"database/sql"
	"testing"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func TestPostgres(t *testing.T) {
	sdb := &postgresSharedTestDB{
		driver: "postgres",
		dsn:    mustGetEnv(t, "POSTGRES_DSN"),
	}

	sharedTest(t, sdb)
}

// postgresSharedTestDB implements the sharedTestDB interface.
type postgresSharedTestDB struct {
	driver string
	dsn    string
}

func (o *postgresSharedTestDB) DriverName() string {
	return o.driver
}

func (o *postgresSharedTestDB) DSN() string {
	return o.dsn
}

func (o *postgresSharedTestDB) ConnectAndResetDB(t *testing.T) *sql.DB {
	db, err := sql.Open("postgres", o.dsn)
	require.NoError(t, err)
	defer func() {
		if t.Failed() {
			db.Close()
		}
	}()

	_, err = db.Exec(`
		DROP TABLE IF EXISTS "` + migrationsTable + `";
		CREATE TABLE IF NOT EXISTS test_events (
			id SERIAL,
			event_name TEXT NOT NULL
		);
		TRUNCATE TABLE test_events;`,
	)
	require.NoError(t, err)
	runSQLMigrate(t, true, nil, nil, "init", "-driver", "postgres", "-dsn", o.dsn, "-migrations_table", migrationsTable)
	return db
}

func (o *postgresSharedTestDB) GetTestEvents(t *testing.T, db *sql.DB) []string {
	var events []string
	rows, err := db.Query(`SELECT event_name FROM test_events ORDER BY id`)
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

func (o *postgresSharedTestDB) GetForwardMigrated(t *testing.T, db *sql.DB) []string {
	var migrationNames []string
	rows, err := db.Query(`SELECT name FROM "` + migrationsTable + `" ORDER BY name`)
	require.NoError(t, err)
	defer rows.Close()

	for rows.Next() {
		var name string
		err := rows.Scan(&name)
		require.NoError(t, err, "error scanning migration result set")
		migrationNames = append(migrationNames, name)
	}
	require.NoError(t, rows.Err())
	return migrationNames
}
