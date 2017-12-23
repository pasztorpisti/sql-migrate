// +build integration

package main

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The TestDrivers test has to be executed with
// an empty DB that doesn't have a migrations table.
// It is recommended to start up the DBs in docker
// and delete the containers after test completion.
func TestDrivers(t *testing.T) {
	for name := range drivers {
		t.Run(name, func(t *testing.T) {
			testDriver(t, name)
		})
	}
}

func testDriver(t *testing.T, driverName string) {
	driverFactory := drivers[driverName]
	driverNameUpper := strings.ToUpper(driverName)

	mustGetEnv := func(key string) string {
		v := os.Getenv(key)
		if v == "" {
			t.Fatal("Environment variable not set:", key)
		}
		return v
	}

	getEnvDefault := func(key, defaultVal string) string {
		if v := os.Getenv(key); v != "" {
			return v
		}
		return defaultVal
	}

	dsn := mustGetEnv(driverNameUpper + "_DSN")
	table := getEnvDefault(driverNameUpper+"MIGRATIONS_TABLE", "migrations")

	t.Run("Open", func(t *testing.T) {
		driver := driverFactory(table)
		db, err := driver.Open(dsn)
		require.NoError(t, err)
		defer db.Close()
		err = db.Ping()
		assert.NoError(t, err)
	})

	t.Run("CreateMigrationsTable", func(t *testing.T) {
		tableExists := func(d Driver, q Querier) bool {
			_, err := d.GetForwardMigratedNames(q)
			return err == nil
		}

		// This test has to be executed as the first test with an empty DB
		// because these tests are DB-independent and don't execute DB-specific
		// SQL to manipulate/clean the DB between tests.
		t.Run("table creation succeeds if not exists", func(t *testing.T) {
			driver := driverFactory(table)
			db, err := driver.Open(dsn)
			require.NoError(t, err)
			defer db.Close()

			require.False(t, tableExists(driver, db))

			err = driver.CreateMigrationsTable(db)
			require.NoError(t, err)

			require.True(t, tableExists(driver, db))
		})

		t.Run("table creation succeeds if exists", func(t *testing.T) {
			driver := driverFactory(table)
			db, err := driver.Open(dsn)
			require.NoError(t, err)
			defer db.Close()

			err = driver.CreateMigrationsTable(db)
			require.NoError(t, err)

			require.True(t, tableExists(driver, db))

			err = driver.CreateMigrationsTable(db)
			require.NoError(t, err)

			require.True(t, tableExists(driver, db))
		})
	})

	t.Run("GetForwardMigratedNames and SetMigrationState", func(t *testing.T) {
		driver := driverFactory(table)
		db, err := driver.Open(dsn)
		require.NoError(t, err)
		defer db.Close()
		err = driver.CreateMigrationsTable(db)
		require.NoError(t, err)

		nameSet := func(names ...string) map[string]struct{} {
			set := make(map[string]struct{}, len(names))
			for _, name := range names {
				set[name] = struct{}{}
			}
			return set
		}

		names, err := driver.GetForwardMigratedNames(db)
		require.NoError(t, err)
		assert.Equal(t, nameSet(), names)

		driver.SetMigrationState(db, "0001_initial", true)
		names, err = driver.GetForwardMigratedNames(db)
		require.NoError(t, err)
		assert.Equal(t, nameSet("0001_initial"), names)

		driver.SetMigrationState(db, "0002", true)
		names, err = driver.GetForwardMigratedNames(db)
		require.NoError(t, err)
		assert.Equal(t, nameSet("0001_initial", "0002"), names)

		driver.SetMigrationState(db, "0003", true)
		names, err = driver.GetForwardMigratedNames(db)
		require.NoError(t, err)
		assert.Equal(t, nameSet("0001_initial", "0002", "0003"), names)

		driver.SetMigrationState(db, "0002", false)
		names, err = driver.GetForwardMigratedNames(db)
		require.NoError(t, err)
		assert.Equal(t, nameSet("0001_initial", "0003"), names)
	})
}
