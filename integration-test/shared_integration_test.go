// +build integration

package integrationtest

import (
	"bytes"
	"database/sql"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// migrationsTable is used to avoid a conflict with the migrations table
// that is used by driver_integration_test.go.
// Using the default migrations table is a problem only when these tests
// are executed before driver_integration_test.go because those tests
// expect the DB to be empty without a migrations table and it doesn't try
// to cleanup the DB before running its DB-independent tests.
const migrationsTable = "test_migrations"

type sharedTestDB interface {
	DriverName() string
	DSN() string
	ConnectAndResetDB(t *testing.T) *sql.DB
	GetTestEvents(t *testing.T, db *sql.DB) []string
	GetForwardMigrated(t *testing.T, db *sql.DB) []string
}

func sharedTest(t *testing.T, sdb sharedTestDB) {
	t.Run("success", func(t *testing.T) {
		db := sdb.ConnectAndResetDB(t)
		defer db.Close()

		fp := &fixedParams{
			sdb:           sdb,
			t:             t,
			db:            db,
			migrationsDir: "testdata/success",
		}

		status(fp, &statusParams{
			output: outputLinesMatcher(
				"[ ] 0001_initial.notx.sql [no-forward-transaction]",
				"[ ] 0002.sql [no-backward-transaction]",
				"[ ] 0003.sql [no-backward-transaction]",
			),
		})
		migrate("goto", fp, &migrateParams{
			target: "initial",
			output: outputLinesMatcher(`Nothing to migrate.`),
		})
		status(fp, &statusParams{
			output: outputLinesMatcher(
				"[ ] 0001_initial.notx.sql [no-forward-transaction]",
				"[ ] 0002.sql [no-backward-transaction]",
				"[ ] 0003.sql [no-backward-transaction]",
			),
		})
		migrate("goto", fp, &migrateParams{
			target:          "1",
			forwardMigrated: []string{"0001_initial"},
			testEvents:      []string{"1"},
			output:          outputLinesMatcher(`forward-migrate 0001_initial.notx.sql [no-transaction] ... OK`),
		})
		status(fp, &statusParams{
			forwardMigrated: []string{"0001_initial"},
			testEvents:      []string{"1"},
			output: outputLinesMatcher(
				"[X] 0001_initial.notx.sql [no-forward-transaction]",
				"[ ] 0002.sql [no-backward-transaction]",
				"[ ] 0003.sql [no-backward-transaction]",
			),
		})
		migrate("goto", fp, &migrateParams{
			target:     "initial",
			testEvents: []string{"1", "1.back"},
			output:     outputLinesMatcher(`backward-migrate 0001_initial.back.sql ... OK`),
		})
		status(fp, &statusParams{
			testEvents: []string{"1", "1.back"},
			output: outputLinesMatcher(
				"[ ] 0001_initial.notx.sql [no-forward-transaction]",
				"[ ] 0002.sql [no-backward-transaction]",
				"[ ] 0003.sql [no-backward-transaction]",
			),
		})
		migrate("goto", fp, &migrateParams{
			target:     "initial",
			testEvents: []string{"1", "1.back"},
			output:     outputLinesMatcher(`Nothing to migrate.`),
		})
		status(fp, &statusParams{
			testEvents: []string{"1", "1.back"},
			output: outputLinesMatcher(
				"[ ] 0001_initial.notx.sql [no-forward-transaction]",
				"[ ] 0002.sql [no-backward-transaction]",
				"[ ] 0003.sql [no-backward-transaction]",
			),
		})
		migrate("goto", fp, &migrateParams{
			target:          "2",
			forwardMigrated: []string{"0001_initial", "0002"},
			testEvents:      []string{"1", "1.back", "1", "2"},
			output: outputLinesMatcher(
				`forward-migrate 0001_initial.notx.sql [no-transaction] ... OK`,
				`forward-migrate 0002.sql ... OK`,
			),
		})
		status(fp, &statusParams{
			forwardMigrated: []string{"0001_initial", "0002"},
			testEvents:      []string{"1", "1.back", "1", "2"},
			output: outputLinesMatcher(
				"[X] 0001_initial.notx.sql [no-forward-transaction]",
				"[X] 0002.sql [no-backward-transaction]",
				"[ ] 0003.sql [no-backward-transaction]",
			),
		})
		migrate("goto", fp, &migrateParams{
			target:          "invalid-target",
			expectError:     true,
			forwardMigrated: []string{"0001_initial", "0002"},
			testEvents:      []string{"1", "1.back", "1", "2"},
			output:          outputLinesMatcher(`invalid target migration - "invalid-target"`),
		})
		status(fp, &statusParams{
			forwardMigrated: []string{"0001_initial", "0002"},
			testEvents:      []string{"1", "1.back", "1", "2"},
			output: outputLinesMatcher(
				"[X] 0001_initial.notx.sql [no-forward-transaction]",
				"[X] 0002.sql [no-backward-transaction]",
				"[ ] 0003.sql [no-backward-transaction]",
			),
		})
		migrate("plan", fp, &migrateParams{
			target:          "invalid-target",
			expectError:     true,
			output:          outputLinesMatcher(`invalid target migration - "invalid-target"`),
			forwardMigrated: []string{"0001_initial", "0002"},
			testEvents:      []string{"1", "1.back", "1", "2"},
		})
		migrate("goto", fp, &migrateParams{
			target:          "1",
			forwardMigrated: []string{"0001_initial"},
			testEvents:      []string{"1", "1.back", "1", "2", "2.back"},
			output:          outputLinesMatcher(`backward-migrate 0002.back.notx.sql [no-transaction] ... OK`),
		})
		status(fp, &statusParams{
			forwardMigrated: []string{"0001_initial"},
			testEvents:      []string{"1", "1.back", "1", "2", "2.back"},
			output: outputLinesMatcher(
				"[X] 0001_initial.notx.sql [no-forward-transaction]",
				"[ ] 0002.sql [no-backward-transaction]",
				"[ ] 0003.sql [no-backward-transaction]",
			),
		})
		migrate("plan", fp, &migrateParams{
			target:          "latest",
			forwardMigrated: []string{"0001_initial"},
			testEvents:      []string{"1", "1.back", "1", "2", "2.back"},
			output: outputLinesMatcher(
				`forward-migrate 0002.sql`,
				`forward-migrate 0003.sql`,
			),
		})
		migrate("goto", fp, &migrateParams{
			target:          "latest",
			forwardMigrated: []string{"0001_initial", "0002", "0003"},
			testEvents:      []string{"1", "1.back", "1", "2", "2.back", "2", "3"},
			output: outputLinesMatcher(
				`forward-migrate 0002.sql ... OK`,
				`forward-migrate 0003.sql ... OK`,
			),
		})
		status(fp, &statusParams{
			forwardMigrated: []string{"0001_initial", "0002", "0003"},
			testEvents:      []string{"1", "1.back", "1", "2", "2.back", "2", "3"},
			output: outputLinesMatcher(
				"[X] 0001_initial.notx.sql [no-forward-transaction]",
				"[X] 0002.sql [no-backward-transaction]",
				"[X] 0003.sql [no-backward-transaction]",
			),
		})
		migrate("goto", fp, &migrateParams{
			target:          "3",
			forwardMigrated: []string{"0001_initial", "0002", "0003"},
			testEvents:      []string{"1", "1.back", "1", "2", "2.back", "2", "3"},
			output:          outputLinesMatcher(`Nothing to migrate.`),
		})
		migrate("goto", fp, &migrateParams{
			target:     "initial",
			testEvents: []string{"1", "1.back", "1", "2", "2.back", "2", "3", "3.back", "2.back", "1.back"},
			output: outputLinesMatcher(
				`backward-migrate 0003.notx.back.sql [no-transaction] ... OK`,
				`backward-migrate 0002.back.notx.sql [no-transaction] ... OK`,
				`backward-migrate 0001_initial.back.sql ... OK`,
			),
		})
		status(fp, &statusParams{
			testEvents: []string{"1", "1.back", "1", "2", "2.back", "2", "3", "3.back", "2.back", "1.back"},
			output: outputLinesMatcher(
				"[ ] 0001_initial.notx.sql [no-forward-transaction]",
				"[ ] 0002.sql [no-backward-transaction]",
				"[ ] 0003.sql [no-backward-transaction]",
			),
		})
		migrate("plan", fp, &migrateParams{
			target:     "latest",
			testEvents: []string{"1", "1.back", "1", "2", "2.back", "2", "3", "3.back", "2.back", "1.back"},
			output: outputLinesMatcher(
				`forward-migrate 0001_initial.notx.sql [no-transaction]`,
				`forward-migrate 0002.sql`,
				`forward-migrate 0003.sql`,
			),
		})
	})

	t.Run("custom extensions", func(t *testing.T) {
		db := sdb.ConnectAndResetDB(t)
		defer db.Close()

		fp := &fixedParams{
			sdb:           sdb,
			t:             t,
			db:            db,
			migrationsDir: "testdata/custom-extensions",
		}

		extraArgs := []string{"-fwd", ".fw", "-bwd", ".bw", "-notx", ".nt", "-ext", ""}

		status(fp, &statusParams{
			extraArgs: extraArgs,
			output: outputLinesMatcher(
				"[ ] 0001.fw.nt [no-forward-transaction] [no-backward-migration]",
				"[ ] 0002_description.nt.fw [no-forward-transaction] [no-backward-transaction]",
			),
		})
		migrate("goto", fp, &migrateParams{
			target:    "initial",
			extraArgs: extraArgs,
			output:    outputLinesMatcher(`Nothing to migrate.`),
		})
		migrate("goto", fp, &migrateParams{
			target:          "1",
			extraArgs:       extraArgs,
			forwardMigrated: []string{"0001"},
			testEvents:      []string{"1"},
			output:          outputLinesMatcher(`forward-migrate 0001.fw.nt [no-transaction] ... OK`),
		})
		status(fp, &statusParams{
			extraArgs:       extraArgs,
			forwardMigrated: []string{"0001"},
			testEvents:      []string{"1"},
			output: outputLinesMatcher(
				"[X] 0001.fw.nt [no-forward-transaction] [no-backward-migration]",
				"[ ] 0002_description.nt.fw [no-forward-transaction] [no-backward-transaction]",
			),
		})
		migrate("plan", fp, &migrateParams{
			target:          "latest",
			extraArgs:       extraArgs,
			forwardMigrated: []string{"0001"},
			testEvents:      []string{"1"},
			output:          outputLinesMatcher("forward-migrate 0002_description.nt.fw [no-transaction]"),
		})
		migrate("goto", fp, &migrateParams{
			target:          "latest",
			extraArgs:       extraArgs,
			forwardMigrated: []string{"0001", "0002_description"},
			testEvents:      []string{"1", "2"},
			output:          outputLinesMatcher(`forward-migrate 0002_description.nt.fw [no-transaction] ... OK`),
		})
		status(fp, &statusParams{
			extraArgs:       extraArgs,
			forwardMigrated: []string{"0001", "0002_description"},
			testEvents:      []string{"1", "2"},
			output: outputLinesMatcher(
				"[X] 0001.fw.nt [no-forward-transaction] [no-backward-migration]",
				"[X] 0002_description.nt.fw [no-forward-transaction] [no-backward-transaction]",
			),
		})
	})

	t.Run("empty migrations directory", func(t *testing.T) {
		migrationsDir, err := ioutil.TempDir("", "sql-migrate_shared-test_empty-migrations-dir")
		require.NoError(t, err)
		defer os.RemoveAll(migrationsDir)

		db := sdb.ConnectAndResetDB(t)
		defer db.Close()

		fp := &fixedParams{
			sdb:           sdb,
			t:             t,
			db:            db,
			migrationsDir: migrationsDir,
		}

		status(fp, &statusParams{
			output: outputLinesMatcher("There are no migrations."),
		})
		migrate("goto", fp, &migrateParams{
			target: "initial",
			output: outputLinesMatcher(`Nothing to migrate.`),
		})
		migrate("plan", fp, &migrateParams{
			target: "initial",
			output: outputLinesMatcher(`Nothing to migrate.`),
		})
		migrate("goto", fp, &migrateParams{
			target: "latest",
			output: outputLinesMatcher(`Nothing to migrate.`),
		})
		migrate("plan", fp, &migrateParams{
			target: "latest",
			output: outputLinesMatcher(`Nothing to migrate.`),
		})
		status(fp, &statusParams{
			output: outputLinesMatcher("There are no migrations."),
		})
	})

	t.Run("different traget formats", func(t *testing.T) {
		db := sdb.ConnectAndResetDB(t)
		defer db.Close()

		fp := &fixedParams{
			sdb:           sdb,
			t:             t,
			db:            db,
			migrationsDir: "testdata/success",
		}

		// target: parsed integer ID converted back to string without leading zeros
		migrate("goto", fp, &migrateParams{
			target:          "1",
			forwardMigrated: []string{"0001_initial"},
			testEvents:      []string{"1"},
			output:          outputLinesMatcher(`forward-migrate 0001_initial.notx.sql [no-transaction] ... OK`),
		})
		migrate("goto", fp, &migrateParams{
			target:     "initial",
			testEvents: []string{"1", "1.back"},
			output:     outputLinesMatcher(`backward-migrate 0001_initial.back.sql ... OK`),
		})

		// target: ID with leading zeros as specified in the filename
		migrate("goto", fp, &migrateParams{
			target:          "0001",
			forwardMigrated: []string{"0001_initial"},
			testEvents:      []string{"1", "1.back", "1"},
			output:          outputLinesMatcher(`forward-migrate 0001_initial.notx.sql [no-transaction] ... OK`),
		})
		migrate("goto", fp, &migrateParams{
			target:     "initial",
			testEvents: []string{"1", "1.back", "1", "1.back"},
			output:     outputLinesMatcher(`backward-migrate 0001_initial.back.sql ... OK`),
		})

		// target: migration name as present in the migrations table - this includes
		// the integer with leading zeros to make the integer width at least 4 digits
		// plus the description of the file without the .fwd/.bwd/.notx/.sql flags and extension.
		migrate("goto", fp, &migrateParams{
			target:          "0001_initial",
			forwardMigrated: []string{"0001_initial"},
			testEvents:      []string{"1", "1.back", "1", "1.back", "1"},
			output:          outputLinesMatcher(`forward-migrate 0001_initial.notx.sql [no-transaction] ... OK`),
		})
		migrate("goto", fp, &migrateParams{
			target:     "initial",
			testEvents: []string{"1", "1.back", "1", "1.back", "1", "1.back"},
			output:     outputLinesMatcher(`backward-migrate 0001_initial.back.sql ... OK`),
		})

		// target: full name of the *forward* migration file
		migrate("goto", fp, &migrateParams{
			target:          "0001_initial.notx.sql",
			forwardMigrated: []string{"0001_initial"},
			testEvents:      []string{"1", "1.back", "1", "1.back", "1", "1.back", "1"},
			output:          outputLinesMatcher(`forward-migrate 0001_initial.notx.sql [no-transaction] ... OK`),
		})
	})

	t.Run("no backward migration", func(t *testing.T) {
		db := sdb.ConnectAndResetDB(t)
		defer db.Close()

		fp := &fixedParams{
			sdb:           sdb,
			t:             t,
			db:            db,
			migrationsDir: "testdata/no-backward",
		}

		migrate("goto", fp, &migrateParams{
			target:          "1",
			forwardMigrated: []string{"0001_initial"},
			testEvents:      []string{"1"},
		})
		migrate("goto", fp, &migrateParams{
			target:          "1",
			forwardMigrated: []string{"0001_initial"},
			testEvents:      []string{"1"},
		})
		migrate("goto", fp, &migrateParams{
			target:          "latest",
			forwardMigrated: []string{"0001_initial"},
			testEvents:      []string{"1"},
		})
		migrate("goto", fp, &migrateParams{
			target:          "initial",
			forwardMigrated: []string{"0001_initial"},
			testEvents:      []string{"1"},
			expectError:     true,
			output:          outputLinesMatcher(`migration "0001_initial.sql" doesn't have a backward step`),
		})
		migrate("plan", fp, &migrateParams{
			target:          "initial",
			forwardMigrated: []string{"0001_initial"},
			testEvents:      []string{"1"},
			expectError:     true,
			output:          outputLinesMatcher(`migration "0001_initial.sql" doesn't have a backward step`),
		})
		status(fp, &statusParams{
			forwardMigrated: []string{"0001_initial"},
			testEvents:      []string{"1"},
			output:          outputLinesMatcher("[X] 0001_initial.sql [no-backward-migration]"),
		})
	})

	t.Run("no forward migration", func(t *testing.T) {
		db := sdb.ConnectAndResetDB(t)
		defer db.Close()

		fp := &fixedParams{
			sdb:           sdb,
			t:             t,
			db:            db,
			migrationsDir: "testdata/no-forward",
		}

		migrate("goto", fp, &migrateParams{
			target:      "latest",
			expectError: true,
			output:      outputLinesMatcher(`migration without forward step - "1.back.sql"`),
		})
		migrate("plan", fp, &migrateParams{
			target:      "latest",
			expectError: true,
			output:      outputLinesMatcher(`migration without forward step - "1.back.sql"`),
		})
		status(fp, &statusParams{
			expectError: true,
			output:      outputLinesMatcher(`migration without forward step - "1.back.sql"`),
		})
	})

	t.Run("first ID isn't 1", func(t *testing.T) {
		db := sdb.ConnectAndResetDB(t)
		defer db.Close()

		fp := &fixedParams{
			sdb:           sdb,
			t:             t,
			db:            db,
			migrationsDir: "testdata/first-id-is-not-1",
		}

		migrate("goto", fp, &migrateParams{
			target:      "latest",
			expectError: true,
			output:      outputLinesMatcher(`the first migration ID must be 1 but it is 2`),
		})
		migrate("plan", fp, &migrateParams{
			target:      "latest",
			expectError: true,
			output:      outputLinesMatcher(`the first migration ID must be 1 but it is 2`),
		})
		status(fp, &statusParams{
			expectError: true,
			output:      outputLinesMatcher(`the first migration ID must be 1 but it is 2`),
		})
	})

	t.Run("id-gap", func(t *testing.T) {
		db := sdb.ConnectAndResetDB(t)
		defer db.Close()

		fp := &fixedParams{
			sdb:           sdb,
			t:             t,
			db:            db,
			migrationsDir: "testdata/id-gap",
		}

		migrate("goto", fp, &migrateParams{
			target:      "latest",
			expectError: true,
			output:      outputLinesMatcher(`missing migration ID (gap): 2`),
		})
		migrate("plan", fp, &migrateParams{
			target:      "latest",
			expectError: true,
			output:      outputLinesMatcher(`missing migration ID (gap): 2`),
		})
		status(fp, &statusParams{
			expectError: true,
			output:      outputLinesMatcher(`missing migration ID (gap): 2`),
		})
	})

	t.Run("duplicate forward id", func(t *testing.T) {
		db := sdb.ConnectAndResetDB(t)
		defer db.Close()

		fp := &fixedParams{
			sdb:           sdb,
			t:             t,
			db:            db,
			migrationsDir: "testdata/duplicate-forward-id",
		}

		migrate("goto", fp, &migrateParams{
			target:      "latest",
			expectError: true,
			output:      outputHasPrefix(`duplicate forward migration for ID 1:`),
		})
		migrate("plan", fp, &migrateParams{
			target:      "latest",
			expectError: true,
			output:      outputHasPrefix(`duplicate forward migration for ID 1:`),
		})
		status(fp, &statusParams{
			expectError: true,
			output:      outputHasPrefix(`duplicate forward migration for ID 1:`),
		})
	})

	t.Run("duplicate backward id", func(t *testing.T) {
		db := sdb.ConnectAndResetDB(t)
		defer db.Close()

		fp := &fixedParams{
			sdb:           sdb,
			t:             t,
			db:            db,
			migrationsDir: "testdata/duplicate-backward-id",
		}

		migrate("goto", fp, &migrateParams{
			target:      "latest",
			expectError: true,
			output:      outputHasPrefix(`duplicate backward migration for ID 1:`),
		})
		migrate("plan", fp, &migrateParams{
			target:      "latest",
			expectError: true,
			output:      outputHasPrefix(`duplicate backward migration for ID 1:`),
		})
		status(fp, &statusParams{
			expectError: true,
			output:      outputHasPrefix(`duplicate backward migration for ID 1:`),
		})
	})

	t.Run("forward-backward filename description mismatch", func(t *testing.T) {
		db := sdb.ConnectAndResetDB(t)
		defer db.Close()

		fp := &fixedParams{
			sdb:           sdb,
			t:             t,
			db:            db,
			migrationsDir: "testdata/description-mismatch",
		}

		migrate("goto", fp, &migrateParams{
			target:      "latest",
			expectError: true,
			output:      outputLinesMatcher(`forward and backward migrations ("1_desc1.sql" and "1_desc2.back.sql") have different description ("_desc1" and "_desc2")`),
		})
		migrate("plan", fp, &migrateParams{
			target:      "latest",
			expectError: true,
			output:      outputLinesMatcher(`forward and backward migrations ("1_desc1.sql" and "1_desc2.back.sql") have different description ("_desc1" and "_desc2")`),
		})
		status(fp, &statusParams{
			expectError: true,
			output:      outputLinesMatcher(`forward and backward migrations ("1_desc1.sql" and "1_desc2.back.sql") have different description ("_desc1" and "_desc2")`),
		})
	})
}

func outputLinesMatcher(lines ...string) outputMatcher {
	return outputEquals(strings.Join(append(lines, ""), "\n"))
}

type outputEquals string

func (o outputEquals) Match(output string) bool {
	return string(o) == output
}

func (o outputEquals) String() string {
	return string(o)
}

type outputHasPrefix string

func (o outputHasPrefix) Match(output string) bool {
	return strings.HasPrefix(output, string(o))
}

func (o outputHasPrefix) String() string {
	return fmt.Sprintf("HasPrefix(%q)", string(o))
}

type outputMatcher interface {
	fmt.Stringer
	Match(output string) bool
}

type fixedParams struct {
	sdb           sharedTestDB
	t             *testing.T
	db            *sql.DB
	migrationsDir string
}

type migrateParams struct {
	target          string
	extraArgs       []string
	forwardMigrated []string
	testEvents      []string
	expectError     bool
	output          outputMatcher
}

// migrate: command has to be "goto" or "plan".
func migrate(command string, fp *fixedParams, p *migrateParams) {
	runSQLMigrate(fp.t, !p.expectError, p.output, p.extraArgs, command,
		"-driver", fp.sdb.DriverName(), "-dsn", fp.sdb.DSN(),
		"-dir", fp.migrationsDir, "-target", p.target, "-migrations_table", migrationsTable)
	requireForwardMigrations(fp, p.forwardMigrated)
	requireEvents(fp, p.testEvents)
}

type statusParams struct {
	extraArgs       []string
	forwardMigrated []string
	testEvents      []string
	expectError     bool
	output          outputMatcher
}

func status(fp *fixedParams, p *statusParams) {
	runSQLMigrate(fp.t, !p.expectError, p.output, p.extraArgs, "status",
		"-driver", fp.sdb.DriverName(), "-dsn", fp.sdb.DSN(),
		"-dir", fp.migrationsDir, "-migrations_table", migrationsTable)
	requireForwardMigrations(fp, p.forwardMigrated)
	requireEvents(fp, p.testEvents)
}

func runSQLMigrate(t *testing.T, expectSuccess bool, om outputMatcher, extraArgs []string, args ...string) {
	binary := mustGetEnv(t, "SQL_MIGRATE_BINARY")
	cmd := exec.Command(binary, append(args, extraArgs...)...)

	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output

	cmdErr := cmd.Run()

	switch {
	case expectSuccess && cmdErr != nil:
		require.FailNow(t, "unexpected failure", "%s", strings.Join([]string{
			fmt.Sprintf("command: %s %s", binary, strings.Join(args, " ")),
			"----- OUTPUT BEGIN -----",
			strings.TrimSuffix(output.String(), "\n"),
			"----- OUTPUT END -----",
			"error: " + cmdErr.Error(),
		}, "\n"))
	case !expectSuccess && cmdErr == nil:
		require.FailNow(t, "unexpected success", "%s", strings.Join([]string{
			fmt.Sprintf("command: %s %s", binary, strings.Join(args, " ")),
			"----- OUTPUT BEGIN -----",
			strings.TrimSuffix(output.String(), "\n"),
			"----- OUTPUT END -----",
		}, "\n"))
	}

	if om != nil && !om.Match(output.String()) {
		require.FailNow(t, "unexpected output", "%s", strings.Join([]string{
			fmt.Sprintf("command: %s %s", binary, strings.Join(args, " ")),
			"----- OUTPUT BEGIN -----",
			strings.TrimSuffix(output.String(), "\n"),
			"----- OUTPUT END -----",
			"----- EXPECTED BEGIN -----",
			strings.TrimSuffix(om.String(), "\n"),
			"----- EXPECTED END -----",
		}, "\n"))
	}
}

func requireForwardMigrations(fp *fixedParams, forwardMigrated []string) {
	if len(forwardMigrated) == 0 {
		forwardMigrated = nil
	}

	migrationNames := fp.sdb.GetForwardMigrated(fp.t, fp.db)
	if len(migrationNames) == 0 {
		migrationNames = nil
	}

	require.Equal(fp.t, forwardMigrated, migrationNames)
}

func requireEvents(fp *fixedParams, expectedEvents []string) {
	if len(expectedEvents) == 0 {
		expectedEvents = nil
	}

	events := fp.sdb.GetTestEvents(fp.t, fp.db)
	if len(events) == 0 {
		events = nil
	}

	require.Equal(fp.t, expectedEvents, events)
}

func mustGetEnv(t *testing.T, key string) string {
	v := os.Getenv(key)
	if v == "" {
		require.FailNow(t, "Environment variable not set", "variable: %s", key)
	}
	return v
}
