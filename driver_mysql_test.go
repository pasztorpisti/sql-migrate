// +build !integration

package main

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMySQLDriver(t *testing.T) {
	const tableName = "migrations"

	newDriver := func(ctrl *gomock.Controller) (*mySQLDriver, *MockDB) {
		db := NewMockDB(ctrl)
		return &mySQLDriver{
			db:        db,
			tableName: tableName,
		}, db
	}

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

	t.Run("ExecuteStep", func(t *testing.T) {
		t.Run("success", func(t *testing.T) {
			ctrl := gomock.NewController(t)
			driver, db := newDriver(ctrl)
			res := NewMockResult(ctrl)

			const migrationName = "0001"
			const query = "SELECT 1;"

			gomock.InOrder(
				db.EXPECT().Exec(query),
				// mysqlDriver.SetMigrationState
				db.EXPECT().Exec(gomock.Any(), migrationName).Return(res, nil),
				res.EXPECT().RowsAffected().Return(int64(1), nil),
			)

			err := driver.ExecuteStep(newTestStep(migrationName, "1.fw.sql"), query)
			require.NoError(t, err)
			ctrl.Finish()
		})

		t.Run("error", func(t *testing.T) {
			ctrl := gomock.NewController(t)
			driver, db := newDriver(ctrl)

			const migrationName = "0001"
			const query = "SELECT 1;"

			gomock.InOrder(
				db.EXPECT().Exec(query).Return(nil, assert.AnError),
			)

			err := driver.ExecuteStep(newTestStep(migrationName, "1.fw.sql"), query)
			require.Error(t, err)
			ctrl.Finish()
		})
	})

	t.Run("Close", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		driver, db := newDriver(ctrl)

		db.EXPECT().Close()

		driver.Close()

		ctrl.Finish()
	})
}
