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

	t.Run("CreateMigrationsTable", func(t *testing.T) {
		tableExists := func(d Driver) bool {
			_, err := d.GetForwardMigratedNames()
			return err == nil
		}

		// This test has to be executed as the first test with an empty DB
		// because these tests are DB-independent and don't execute DB-specific
		// SQL to manipulate/clean the DB between tests.
		t.Run("table creation succeeds if not exists", func(t *testing.T) {
			driver, err := driverFactory(dsn, table)
			require.NoError(t, err)
			defer driver.Close()

			require.False(t, tableExists(driver), "this test must be run with an empty database")

			err = driver.CreateMigrationsTable()
			require.NoError(t, err)

			require.True(t, tableExists(driver))
		})

		t.Run("table creation succeeds if exists", func(t *testing.T) {
			driver, err := driverFactory(dsn, table)
			require.NoError(t, err)
			defer driver.Close()

			err = driver.CreateMigrationsTable()
			require.NoError(t, err)

			require.True(t, tableExists(driver))

			err = driver.CreateMigrationsTable()
			require.NoError(t, err)

			require.True(t, tableExists(driver))
		})
	})

	t.Run("GetForwardMigratedNames and SetMigrationState", func(t *testing.T) {
		driver, err := driverFactory(dsn, table)
		require.NoError(t, err)
		defer driver.Close()
		err = driver.CreateMigrationsTable()
		require.NoError(t, err)

		newTestStep := func(migrationName, filename string) *Step {
			parsed, err := parseFilename(filename, ".fw", ".bw", ".nt", ".sql")
			if err != nil {
				panic(err)
			}
			return &Step{
				Filename:       filename,
				MigrationName:  migrationName,
				ParsedFilename: parsed,
			}
		}

		nameSet := func(names ...string) map[string]struct{} {
			set := make(map[string]struct{}, len(names))
			for _, name := range names {
				set[name] = struct{}{}
			}
			return set
		}

		names, err := driver.GetForwardMigratedNames()
		require.NoError(t, err)
		assert.Equal(t, nameSet(), names)

		const noOpQuery = "SELECT 1;"

		err = driver.ExecuteStep(newTestStep("0001_initial", "0001_initial.fw.sql"), noOpQuery)
		require.NoError(t, err)
		names, err = driver.GetForwardMigratedNames()
		require.NoError(t, err)
		assert.Equal(t, nameSet("0001_initial"), names)

		err = driver.ExecuteStep(newTestStep("0002", "0002.fw.sql"), noOpQuery)
		require.NoError(t, err)
		names, err = driver.GetForwardMigratedNames()
		require.NoError(t, err)
		assert.Equal(t, nameSet("0001_initial", "0002"), names)

		err = driver.ExecuteStep(newTestStep("0003", "0003.fw.sql"), noOpQuery)
		require.NoError(t, err)
		names, err = driver.GetForwardMigratedNames()
		require.NoError(t, err)
		assert.Equal(t, nameSet("0001_initial", "0002", "0003"), names)

		err = driver.ExecuteStep(newTestStep("0002", "0002.bw.sql"), noOpQuery)
		require.NoError(t, err)
		names, err = driver.GetForwardMigratedNames()
		require.NoError(t, err)
		assert.Equal(t, nameSet("0001_initial", "0003"), names)
	})
}
