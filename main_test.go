// +build !integration

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseFilename(t *testing.T) {
	t.Run("both fwd nor bwd are non-empty", func(t *testing.T) {
		tests := []*struct {
			filename string
			parsed   ParsedFilename
		}{
			{
				filename: "1.fw.sql",
				parsed: ParsedFilename{
					ID:        1,
					IDStr:     "1",
					Direction: DirectionForward,
				},
			},
			{
				filename: "1.nt.fw.sql",
				parsed: ParsedFilename{
					ID:        1,
					IDStr:     "1",
					Direction: DirectionForward,
					NoTx:      true,
				},
			},
			{
				filename: "1.fw.nt.sql",
				parsed: ParsedFilename{
					ID:        1,
					IDStr:     "1",
					Direction: DirectionForward,
					NoTx:      true,
				},
			},
			{
				filename: "00001_my_description.fw.sql",
				parsed: ParsedFilename{
					ID:          1,
					IDStr:       "00001",
					Description: "_my_description",
					Direction:   DirectionForward,
				},
			},
			{
				filename: "001_my_description.bw.nt.sql",
				parsed: ParsedFilename{
					ID:          1,
					IDStr:       "001",
					Description: "_my_description",
					Direction:   DirectionBackward,
					NoTx:        true,
				},
			},
			{
				filename: "001_my_description.nt.bw.sql",
				parsed: ParsedFilename{
					ID:          1,
					IDStr:       "001",
					Description: "_my_description",
					Direction:   DirectionBackward,
					NoTx:        true,
				},
			},
		}

		for _, test := range tests {
			t.Run(test.filename, func(t *testing.T) {
				parsed, err := parseFilename(test.filename, ".fw", ".bw", ".nt", ".sql")
				require.NoError(t, err)
				assert.Equal(t, test.parsed, *parsed)
			})
		}
	})

	t.Run("fwd is empty", func(t *testing.T) {
		tests := []*struct {
			filename string
			parsed   ParsedFilename
		}{
			{
				filename: "1.sql",
				parsed: ParsedFilename{
					ID:        1,
					IDStr:     "1",
					Direction: DirectionForward,
				},
			},
			{
				filename: "1.nt.sql",
				parsed: ParsedFilename{
					ID:        1,
					IDStr:     "1",
					Direction: DirectionForward,
					NoTx:      true,
				},
			},
			{
				filename: "00020_my_description.sql",
				parsed: ParsedFilename{
					ID:          20,
					IDStr:       "00020",
					Description: "_my_description",
					Direction:   DirectionForward,
				},
			},
			{
				filename: "020_my_description.bw.nt.sql",
				parsed: ParsedFilename{
					ID:          20,
					IDStr:       "020",
					Description: "_my_description",
					Direction:   DirectionBackward,
					NoTx:        true,
				},
			},
			{
				filename: "020_my_description.nt.bw.sql",
				parsed: ParsedFilename{
					ID:          20,
					IDStr:       "020",
					Description: "_my_description",
					Direction:   DirectionBackward,
					NoTx:        true,
				},
			},
		}

		for _, test := range tests {
			t.Run(test.filename, func(t *testing.T) {
				parsed, err := parseFilename(test.filename, "", ".bw", ".nt", ".sql")
				require.NoError(t, err)
				assert.Equal(t, test.parsed, *parsed)
			})
		}
	})

	t.Run("bwd is empty", func(t *testing.T) {
		tests := []*struct {
			filename string
			parsed   ParsedFilename
		}{
			{
				filename: "1.fw.sql",
				parsed: ParsedFilename{
					ID:        1,
					IDStr:     "1",
					Direction: DirectionForward,
				},
			},
			{
				filename: "1.fw.nt.sql",
				parsed: ParsedFilename{
					ID:        1,
					IDStr:     "1",
					Direction: DirectionForward,
					NoTx:      true,
				},
			},
			{
				filename: "1.nt.fw.sql",
				parsed: ParsedFilename{
					ID:        1,
					IDStr:     "1",
					Direction: DirectionForward,
					NoTx:      true,
				},
			},
			{
				filename: "00020_my_description.sql",
				parsed: ParsedFilename{
					ID:          20,
					IDStr:       "00020",
					Description: "_my_description",
					Direction:   DirectionBackward,
				},
			},
			{
				filename: "020_my_description.nt.sql",
				parsed: ParsedFilename{
					ID:          20,
					IDStr:       "020",
					Description: "_my_description",
					Direction:   DirectionBackward,
					NoTx:        true,
				},
			},
		}

		for _, test := range tests {
			t.Run(test.filename, func(t *testing.T) {
				parsed, err := parseFilename(test.filename, ".fw", "", ".nt", ".sql")
				require.NoError(t, err)
				assert.Equal(t, test.parsed, *parsed)
			})
		}
	})

	t.Run("missing numeric ID prefix", func(t *testing.T) {
		_, err := parseFilename("woof.fw.sql", ".fw", ".bw", ".nt", ".sql")
		assert.EqualError(t, err, `missing numeric ID prefix`)
	})

	t.Run("required direction is missing from filename", func(t *testing.T) {
		_, err := parseFilename("1.sql", ".fw", ".bw", ".nt", ".sql")
		assert.EqualError(t, err, `exactly one of the ".fw" and ".bw" suffixes has to be used`)
	})

	t.Run("disabling .notx in filenames results in notx=false", func(t *testing.T) {
		parsed, err := parseFilename("013.sql", "", ".bw", "", ".sql")
		require.NoError(t, err)
		assert.Equal(t, ParsedFilename{
			ID:        13,
			IDStr:     "013",
			Direction: DirectionForward,
		}, *parsed)
	})

	t.Run("empty fwd and bwd results in forward direction", func(t *testing.T) {
		parsed, err := parseFilename("013.sql", "", "", "", ".sql")
		require.NoError(t, err)
		assert.Equal(t, ParsedFilename{
			ID:        13,
			IDStr:     "013",
			Direction: DirectionForward,
		}, *parsed)
	})

	t.Run("empty ext", func(t *testing.T) {
		parsed, err := parseFilename("013.bw.nt", ".fw", ".bw", ".nt", "")
		require.NoError(t, err)
		assert.Equal(t, ParsedFilename{
			ID:        13,
			IDStr:     "013",
			Direction: DirectionBackward,
			NoTx:      true,
		}, *parsed)
	})

	t.Run("missing ext", func(t *testing.T) {
		_, err := parseFilename("1.sql", ".fw", ".bw", ".nt", ".sqlx")
		assert.EqualError(t, err, `missing ".sqlx" extension`)
	})

	t.Run("multiple fwd and/or bwd suffixes", func(t *testing.T) {
		tests := []string{
			"1.nt.fw.fw.sql", "1.fw.nt.fw.sql", "1.fw.fw.nt.sql", "1.fw.fw.sql", "1.fw.fw.fw.sql",
			"1.nt.bw.bw.sql", "1.bw.nt.bw.sql", "1.bw.bw.nt.sql", "1.bw.bw.sql", "1.bw.bw.bw.sql",
			"1.nt.fw.bw.sql", "1.fw.nt.bw.sql", "1.fw.bw.nt.sql", "1.fw.bw.sql",
		}
		for _, s := range tests {
			t.Run(s, func(t *testing.T) {
				_, err := parseFilename(s, ".fw", ".bw", ".nt", ".sql")
				assert.EqualError(t, err, `multiple ".fw" and/or ".bw" suffixes`)
			})
		}
	})

	t.Run("multiple notx suffixes", func(t *testing.T) {
		tests := []string{
			"1.nt.nt.sql", "1.nt.nt.nt.sql",
			"1.fw.nt.nt.sql", "1.nt.fw.nt.sql", "1.nt.nt.fw.sql",
			"1.bw.nt.nt.sql", "1.nt.bw.nt.sql", "1.nt.nt.bw.sql",
		}
		for _, s := range tests {
			t.Run(s, func(t *testing.T) {
				_, err := parseFilename(s, ".fw", ".bw", ".nt", ".sql")
				assert.EqualError(t, err, `multiple ".nt" suffixes`)
			})
		}
	})
}

func newTestStep(filename string) *Step {
	if filename == "" {
		return nil
	}
	parsed, err := parseFilename(filename, ".fw", ".bw", ".nt", ".sql")
	if err != nil {
		panic(err)
	}
	return &Step{
		Filename:       filename,
		ParsedFilename: parsed,
	}
}

func newTestMigration(fwName, bwName string) *Migration {
	return &Migration{
		Forward:  newTestStep(fwName),
		Backward: newTestStep(bwName),
	}
}

func newTestMigrationWithName(fwName, bwName, name string) *Migration {
	m := newTestMigration(fwName, bwName)
	if m.Forward != nil {
		m.Forward.MigrationName = name
	}
	if m.Backward != nil {
		m.Backward.MigrationName = name
	}
	return m
}

func newTestIDMap(a []*Migration) map[int64]*Migration {
	idMap := make(map[int64]*Migration, len(a))
	for _, m := range a {
		if m.Forward != nil {
			idMap[m.Forward.ParsedFilename.ID] = m
		} else {
			idMap[m.Backward.ParsedFilename.ID] = m
		}
	}
	return idMap
}

func indexTestMigrations(a []*Migration) *Migrations {
	ms, err := sortAndIndexMigrations(newTestIDMap(a))
	if err != nil {
		panic(err)
	}
	return ms
}

func TestCreatePlan(t *testing.T) {
	t.Run("no migrations", func(t *testing.T) {
		tests := []*struct {
			target string
			error  string
		}{
			{"initial", ""},
			{"latest", ""},
			{"0", `invalid target migration - "0"`},
			{"023", `invalid target migration - "023"`},
			{"woof", `invalid target migration - "woof"`},
		}

		for _, test := range tests {
			t.Run(test.target, func(t *testing.T) {
				ms := indexTestMigrations(nil)
				forwardMigrated := map[string]struct{}{}
				steps, err := createPlan(test.target, ms, forwardMigrated)
				if test.error == "" {
					require.NoError(t, err)
					assert.Nil(t, steps)
				} else {
					require.EqualError(t, err, test.error)
				}
			})
		}
	})

	t.Run("all migrations have backward step", func(t *testing.T) {
		createTestMigrations := func() *Migrations {
			return indexTestMigrations([]*Migration{
				newTestMigration("00001_initial.fw.sql", "00001_initial.bw.sql"),
				newTestMigration("00002.fw.sql", "00002.bw.sql"),
				newTestMigration("00003_woof.fw.sql", "00003_woof.bw.sql"),
			})
		}

		ms := createTestMigrations().Sorted

		t.Run("0 applied migrations", func(t *testing.T) {
			tests := []*struct {
				target string
				steps  []*Step
			}{
				{
					target: "initial",
					steps:  nil,
				},
				{
					target: "1",
					steps:  []*Step{ms[0].Forward},
				},
				{
					target: "00001",
					steps:  []*Step{ms[0].Forward},
				},
				{
					target: "0001_initial",
					steps:  []*Step{ms[0].Forward},
				},
				{
					target: "00001_initial.fw.sql",
					steps:  []*Step{ms[0].Forward},
				},
				{
					target: "2",
					steps:  []*Step{ms[0].Forward, ms[1].Forward},
				},
				{
					target: "00003",
					steps:  []*Step{ms[0].Forward, ms[1].Forward, ms[2].Forward},
				},
				{
					target: "latest",
					steps:  []*Step{ms[0].Forward, ms[1].Forward, ms[2].Forward},
				},
			}

			for _, test := range tests {
				t.Run(test.target, func(t *testing.T) {
					forwardMigrated := map[string]struct{}{}
					steps, err := createPlan(test.target, createTestMigrations(), forwardMigrated)
					require.NoError(t, err)
					assert.Equal(t, test.steps, steps)
				})
			}
		})

		t.Run("1 applied migration", func(t *testing.T) {
			tests := []*struct {
				target string
				steps  []*Step
			}{
				{
					target: "initial",
					steps:  []*Step{ms[0].Backward},
				},
				{
					target: "1",
					steps:  nil,
				},
				{
					target: "00001",
					steps:  nil,
				},
				{
					target: ms[0].Forward.MigrationName,
					steps:  nil,
				},
				{
					target: "00001_initial.fw.sql",
					steps:  nil,
				},
				{
					target: "2",
					steps:  []*Step{ms[1].Forward},
				},
				{
					target: "00003",
					steps:  []*Step{ms[1].Forward, ms[2].Forward},
				},
				{
					target: "latest",
					steps:  []*Step{ms[1].Forward, ms[2].Forward},
				},
			}

			for _, test := range tests {
				t.Run(test.target, func(t *testing.T) {
					forwardMigrated := map[string]struct{}{
						ms[0].Forward.MigrationName: struct{}{},
					}
					steps, err := createPlan(test.target, createTestMigrations(), forwardMigrated)
					require.NoError(t, err)
					assert.Equal(t, test.steps, steps)
				})
			}
		})

		t.Run("2 applied migrations", func(t *testing.T) {
			tests := []*struct {
				target string
				steps  []*Step
			}{
				{
					target: "initial",
					steps:  []*Step{ms[1].Backward, ms[0].Backward},
				},
				{
					target: "1",
					steps:  []*Step{ms[1].Backward},
				},
				{
					target: "00001",
					steps:  []*Step{ms[1].Backward},
				},
				{
					target: ms[0].Forward.MigrationName,
					steps:  []*Step{ms[1].Backward},
				},
				{
					target: "00001_initial.fw.sql",
					steps:  []*Step{ms[1].Backward},
				},
				{
					target: "2",
					steps:  nil,
				},
				{
					target: "00003",
					steps:  []*Step{ms[2].Forward},
				},
				{
					target: "latest",
					steps:  []*Step{ms[2].Forward},
				},
			}

			for _, test := range tests {
				t.Run(test.target, func(t *testing.T) {
					forwardMigrated := map[string]struct{}{
						ms[0].Forward.MigrationName: struct{}{},
						ms[1].Forward.MigrationName: struct{}{},
					}
					steps, err := createPlan(test.target, createTestMigrations(), forwardMigrated)
					require.NoError(t, err)
					assert.Equal(t, test.steps, steps)
				})
			}
		})

		t.Run("3 applied migrations", func(t *testing.T) {
			tests := []*struct {
				target string
				steps  []*Step
			}{
				{
					target: "initial",
					steps:  []*Step{ms[2].Backward, ms[1].Backward, ms[0].Backward},
				},
				{
					target: "1",
					steps:  []*Step{ms[2].Backward, ms[1].Backward},
				},
				{
					target: "00001",
					steps:  []*Step{ms[2].Backward, ms[1].Backward},
				},
				{
					target: ms[0].Forward.MigrationName,
					steps:  []*Step{ms[2].Backward, ms[1].Backward},
				},
				{
					target: "00001_initial.fw.sql",
					steps:  []*Step{ms[2].Backward, ms[1].Backward},
				},
				{
					target: "2",
					steps:  []*Step{ms[2].Backward},
				},
				{
					target: "00003",
					steps:  nil,
				},
				{
					target: "latest",
					steps:  nil,
				},
			}

			for _, test := range tests {
				t.Run(test.target, func(t *testing.T) {
					forwardMigrated := map[string]struct{}{
						ms[0].Forward.MigrationName: struct{}{},
						ms[1].Forward.MigrationName: struct{}{},
						ms[2].Forward.MigrationName: struct{}{},
					}
					steps, err := createPlan(test.target, createTestMigrations(), forwardMigrated)
					require.NoError(t, err)
					assert.Equal(t, test.steps, steps)
				})
			}
		})
	})

	t.Run("some migrations have no backward step", func(t *testing.T) {
		createTestMigrations := func() *Migrations {
			return indexTestMigrations([]*Migration{
				newTestMigration("001_initial.fw.sql", "001_initial.bw.sql"),
				newTestMigration("002.fw.sql", ""),
				newTestMigration("003.fw.sql", ""),
				newTestMigration("004_woof.fw.sql", "004_woof.bw.sql"),
			})
		}

		ms := createTestMigrations().Sorted

		t.Run("0 applied migrations", func(t *testing.T) {
			tests := []*struct {
				target string
				steps  []*Step
			}{
				{
					target: "initial",
					steps:  nil,
				},
				{
					target: "1",
					steps:  []*Step{ms[0].Forward},
				},
				{
					target: "2",
					steps:  []*Step{ms[0].Forward, ms[1].Forward},
				},
				{
					target: "003",
					steps:  []*Step{ms[0].Forward, ms[1].Forward, ms[2].Forward},
				},
				{
					target: "004",
					steps:  []*Step{ms[0].Forward, ms[1].Forward, ms[2].Forward, ms[3].Forward},
				},
				{
					target: "latest",
					steps:  []*Step{ms[0].Forward, ms[1].Forward, ms[2].Forward, ms[3].Forward},
				},
			}

			for _, test := range tests {
				t.Run(test.target, func(t *testing.T) {
					forwardMigrated := map[string]struct{}{}
					steps, err := createPlan(test.target, createTestMigrations(), forwardMigrated)
					require.NoError(t, err)
					assert.Equal(t, test.steps, steps)
				})
			}
		})

		t.Run("1 applied migration", func(t *testing.T) {
			tests := []*struct {
				target string
				steps  []*Step
			}{
				{
					target: "initial",
					steps:  []*Step{ms[0].Backward},
				},
				{
					target: "1",
					steps:  nil,
				},
				{
					target: "2",
					steps:  []*Step{ms[1].Forward},
				},
				{
					target: "003",
					steps:  []*Step{ms[1].Forward, ms[2].Forward},
				},
				{
					target: "004",
					steps:  []*Step{ms[1].Forward, ms[2].Forward, ms[3].Forward},
				},
				{
					target: "latest",
					steps:  []*Step{ms[1].Forward, ms[2].Forward, ms[3].Forward},
				},
			}

			for _, test := range tests {
				t.Run(test.target, func(t *testing.T) {
					forwardMigrated := map[string]struct{}{
						ms[0].Forward.MigrationName: struct{}{},
					}
					steps, err := createPlan(test.target, createTestMigrations(), forwardMigrated)
					require.NoError(t, err)
					assert.Equal(t, test.steps, steps)
				})
			}
		})

		t.Run("2 applied migrations", func(t *testing.T) {
			tests := []*struct {
				target string
				steps  []*Step
				error  bool
			}{
				{
					target: "initial",
					error:  true,
				},
				{
					target: "1",
					error:  true,
				},
				{
					target: "2",
					steps:  nil,
				},
				{
					target: "003",
					steps:  []*Step{ms[2].Forward},
				},
				{
					target: "004",
					steps:  []*Step{ms[2].Forward, ms[3].Forward},
				},
				{
					target: "latest",
					steps:  []*Step{ms[2].Forward, ms[3].Forward},
				},
			}

			for _, test := range tests {
				t.Run(test.target, func(t *testing.T) {
					forwardMigrated := map[string]struct{}{
						ms[0].Forward.MigrationName: struct{}{},
						ms[1].Forward.MigrationName: struct{}{},
					}
					steps, err := createPlan(test.target, createTestMigrations(), forwardMigrated)
					if test.error {
						require.Error(t, err)
					} else {
						require.NoError(t, err)
						assert.Equal(t, test.steps, steps)
					}
				})
			}
		})

		t.Run("3 applied migrations", func(t *testing.T) {
			tests := []*struct {
				target string
				steps  []*Step
				error  bool
			}{
				{
					target: "initial",
					error:  true,
				},
				{
					target: "1",
					error:  true,
				},
				{
					target: "2",
					error:  true,
				},
				{
					target: "003",
					steps:  nil,
				},
				{
					target: "004",
					steps:  []*Step{ms[3].Forward},
				},
				{
					target: "latest",
					steps:  []*Step{ms[3].Forward},
				},
			}

			for _, test := range tests {
				t.Run(test.target, func(t *testing.T) {
					forwardMigrated := map[string]struct{}{
						ms[0].Forward.MigrationName: struct{}{},
						ms[1].Forward.MigrationName: struct{}{},
						ms[2].Forward.MigrationName: struct{}{},
					}
					steps, err := createPlan(test.target, createTestMigrations(), forwardMigrated)
					if test.error {
						require.Error(t, err)
					} else {
						require.NoError(t, err)
						assert.Equal(t, test.steps, steps)
					}
				})
			}
		})

		t.Run("4 applied migrations", func(t *testing.T) {
			tests := []*struct {
				target string
				steps  []*Step
				error  bool
			}{
				{
					target: "initial",
					error:  true,
				},
				{
					target: "1",
					error:  true,
				},
				{
					target: "2",
					error:  true,
				},
				{
					target: "003",
					steps:  []*Step{ms[3].Backward},
				},
				{
					target: "004",
					steps:  nil,
				},
				{
					target: "latest",
					steps:  nil,
				},
			}

			for _, test := range tests {
				t.Run(test.target, func(t *testing.T) {
					forwardMigrated := map[string]struct{}{
						ms[0].Forward.MigrationName: struct{}{},
						ms[1].Forward.MigrationName: struct{}{},
						ms[2].Forward.MigrationName: struct{}{},
						ms[3].Forward.MigrationName: struct{}{},
					}
					steps, err := createPlan(test.target, createTestMigrations(), forwardMigrated)
					if test.error {
						require.Error(t, err)
					} else {
						require.NoError(t, err)
						assert.Equal(t, test.steps, steps)
					}
				})
			}
		})
	})

	t.Run("unapplied migration gap", func(t *testing.T) {
		createTestMigrations := func() *Migrations {
			return indexTestMigrations([]*Migration{
				newTestMigration("002.fw.sql", "002.bw.sql"),
				newTestMigration("001.fw.sql", "001.bw.sql"),
				newTestMigration("004.fw.sql", "004.bw.sql"),
				newTestMigration("003.fw.sql", "003.bw.sql"),
			})
		}

		ms := createTestMigrations().Sorted

		for _, target := range []string{"initial", "1", "2", "3", "4", "latest"} {
			t.Run(target, func(t *testing.T) {
				forwardMigrated := map[string]struct{}{
					ms[0].Forward.MigrationName: struct{}{},
					// gap: index 1 hasn't been applied
					ms[2].Forward.MigrationName: struct{}{},
					ms[3].Forward.MigrationName: struct{}{},
				}
				_, err := createPlan(target, createTestMigrations(), forwardMigrated)
				require.EqualError(t, err, `there is at least one unapplied migration before applied migration "003.fw.sql" (examine it with the status command and fix it manually)`)
			})
		}
	})

	t.Run("invalid target migration", func(t *testing.T) {
		createTestMigrations := func() *Migrations {
			return indexTestMigrations([]*Migration{
				newTestMigration("002.fw.sql", "002.bw.sql"),
				newTestMigration("001.fw.sql", "001.bw.sql"),
				newTestMigration("004.fw.sql", "004.bw.sql"),
				newTestMigration("003.fw.sql", "003.bw.sql"),
			})
		}

		forwardMigrated := map[string]struct{}{}
		_, err := createPlan("9", createTestMigrations(), forwardMigrated)
		require.EqualError(t, err, `invalid target migration - "9"`)
	})

	t.Run("missing forward migration file", func(t *testing.T) {
		createTestMigrations := func() *Migrations {
			return indexTestMigrations([]*Migration{
				newTestMigration("001.fw.sql", "001.bw.sql"),
			})
		}

		ms := createTestMigrations().Sorted

		for _, target := range []string{"initial", "1", "2", "latest"} {
			t.Run(target, func(t *testing.T) {
				forwardMigrated := map[string]struct{}{
					ms[0].Forward.MigrationName: struct{}{},
					"0002_missing":              struct{}{},
				}
				_, err := createPlan(target, createTestMigrations(), forwardMigrated)
				require.EqualError(t, err, `there is at least one entry in the migrations table without an existing migration file (examine it with the status command and fix it manually) - entry="0002_missing"`)
			})
		}
	})

	t.Run("can't backward migrate when there is no backward migration file", func(t *testing.T) {
		createTestMigrations := func() *Migrations {
			return indexTestMigrations([]*Migration{
				newTestMigration("001.fw.sql", ""),
			})
		}

		ms := createTestMigrations().Sorted

		forwardMigrated := map[string]struct{}{
			ms[0].Forward.MigrationName: struct{}{},
		}
		_, err := createPlan("initial", createTestMigrations(), forwardMigrated)
		require.EqualError(t, err, `migration "001.fw.sql" doesn't have a backward step`)
	})
}

func TestLoadMigrationsDir(t *testing.T) {
	const testMigrationsDir = "my/dir"

	newDirLister := func(ms []*Migration) (_ listDirFunc, called *bool) {
		var a []string
		for _, m := range ms {
			if m.Forward != nil {
				a = append(a, m.Forward.Filename)
			}
			if m.Backward != nil {
				a = append(a, m.Backward.Filename)
			}
		}

		listDirCalled := false
		return func(dir string) []string {
			listDirCalled = true
			assert.Equal(t, testMigrationsDir, dir)
			return a
		}, &listDirCalled
	}

	assertErrorWithPrefix := func(t *testing.T, e error, prefix string) {
		if assert.Error(t, e) {
			if !strings.HasPrefix(e.Error(), prefix) {
				t.Errorf("error message prefix mismatch - error=%q prefix=%q", e.Error(), prefix)
			}
		}
	}

	t.Run("success", func(t *testing.T) {
		t.Run("no migrations", func(t *testing.T) {
			listDir, called := newDirLister(nil)
			ms, err := loadMigrationsDir(testMigrationsDir, ".fw", ".bw", ".nt", ".sql", listDir)
			require.NoError(t, err)
			assert.Equal(t, indexTestMigrations(nil), ms)
			assert.True(t, *called)
		})

		t.Run("have migrations", func(t *testing.T) {
			migrationList := []*Migration{
				newTestMigration("002.fw.sql", ""),
				newTestMigration("004_woof.fw.sql", "004_woof.bw.sql"),
				newTestMigration("003.fw.sql", ""),
				newTestMigration("001_initial.fw.sql", "001_initial.bw.sql"),
			}
			listDir, called := newDirLister(migrationList)
			createTestMigrations := func() *Migrations {
				return indexTestMigrations(migrationList)
			}

			ms, err := loadMigrationsDir(testMigrationsDir, ".fw", ".bw", ".nt", ".sql", listDir)
			require.NoError(t, err)
			assert.Equal(t, createTestMigrations(), ms)
			assert.True(t, *called)
		})
	})

	t.Run("duplicate forward migration", func(t *testing.T) {
		migrationList := []*Migration{
			newTestMigration("002.fw.sql", ""),
			newTestMigration("003.fw.sql", ""),
			newTestMigration("001_initial.fw.sql", "001_initial.bw.sql"),
			newTestMigration("1_meow.fw.sql", ""),
		}
		listDir, _ := newDirLister(migrationList)

		_, err := loadMigrationsDir(testMigrationsDir, ".fw", ".bw", ".nt", ".sql", listDir)
		assertErrorWithPrefix(t, err, "duplicate forward migration for ID 1:")
	})

	t.Run("duplicate backward migration", func(t *testing.T) {
		migrationList := []*Migration{
			newTestMigration("002.fw.sql", ""),
			newTestMigration("003.fw.sql", ""),
			newTestMigration("001_initial.fw.sql", "001_initial.bw.sql"),
			newTestMigration("1_meow.bw.nt.sql", ""),
		}
		listDir, _ := newDirLister(migrationList)

		_, err := loadMigrationsDir(testMigrationsDir, ".fw", ".bw", ".nt", ".sql", listDir)
		assertErrorWithPrefix(t, err, "duplicate backward migration for ID 1:")
	})
}

func TestSortAndIndexMigrations(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		t.Run("no migrations", func(t *testing.T) {
			a := []*Migration{}
			ms, err := sortAndIndexMigrations(newTestIDMap(a))
			require.NoError(t, err)
			assert.Equal(t, &Migrations{
				Sorted: []*Migration{},
				Names:  map[string]int{},
			}, ms)
		})

		t.Run("have migrations", func(t *testing.T) {
			a := []*Migration{
				newTestMigration("001.fw.sql", ""),
				newTestMigration("00003.fw.sql", "00003.bw.sql"),
				newTestMigration("2_woof.fw.sql", "2_woof.bw.sql"),
			}
			ms, err := sortAndIndexMigrations(newTestIDMap(a))
			require.NoError(t, err)
			assert.Equal(t, &Migrations{
				Sorted: []*Migration{
					newTestMigrationWithName("001.fw.sql", "", "0001"),
					newTestMigrationWithName("2_woof.fw.sql", "2_woof.bw.sql", "0002_woof"),
					newTestMigrationWithName("00003.fw.sql", "00003.bw.sql", "0003"),
				},
				Names: map[string]int{
					"0001":          0, // migration name
					"1":             0, // ID without zero prefix
					"001":           0, // original zero prefixed ID
					"001.fw.sql":    0, // forward filename
					"0002_woof":     1, // migration name
					"2":             1, // ID without zero prefix
					"2_woof.fw.sql": 1, // forward filename
					"0003":          2, // migration name
					"3":             2, // ID without zero prefix
					"00003":         2, // original zero prefixed ID
					"00003.fw.sql":  2, // forward filename
				},
			}, ms)
		})
	})

	t.Run("backward migration without a forward step", func(t *testing.T) {
		a := []*Migration{
			newTestMigration("001.fw.sql", "001.bw.sql"),
			newTestMigration("", "2_meow.bw.nt.sql"),
		}
		_, err := sortAndIndexMigrations(newTestIDMap(a))
		require.EqualError(t, err, `migration without forward step - "2_meow.bw.nt.sql"`)
	})

	t.Run("forward and backward file descriptions differ", func(t *testing.T) {
		a := []*Migration{
			newTestMigration("001_woof.fw.sql", "001_meow.bw.sql"),
		}
		_, err := sortAndIndexMigrations(newTestIDMap(a))
		require.EqualError(t, err, `forward and backward migrations ("001_woof.fw.sql" and "001_meow.bw.sql") have different description ("_woof" and "_meow")`)
	})

	t.Run("the first migration ID isn't 1", func(t *testing.T) {
		t.Run("0", func(t *testing.T) {
			a := []*Migration{
				newTestMigration("000.fw.sql", ""),
				newTestMigration("001.fw.sql", ""),
				newTestMigration("2_woof.fw.sql", "2_woof.bw.sql"),
			}
			_, err := sortAndIndexMigrations(newTestIDMap(a))
			require.EqualError(t, err, `the first migration ID must be 1 but it is 0`)
		})

		t.Run("2", func(t *testing.T) {
			a := []*Migration{
				newTestMigration("002.fw.sql", ""),
				newTestMigration("003.fw.sql", ""),
				newTestMigration("4_woof.fw.sql", "4_woof.bw.sql"),
			}
			_, err := sortAndIndexMigrations(newTestIDMap(a))
			require.EqualError(t, err, `the first migration ID must be 1 but it is 2`)
		})
	})

	t.Run("migration ID gap", func(t *testing.T) {
		a := []*Migration{
			newTestMigration("001.fw.sql", ""),
			newTestMigration("003.fw.sql", ""),
			newTestMigration("4_woof.fw.sql", "4_woof.bw.sql"),
		}
		_, err := sortAndIndexMigrations(newTestIDMap(a))
		require.EqualError(t, err, `missing migration ID (gap): 2`)
	})
}

func TestStep(t *testing.T) {
	t.Run("ExecuteAndLog", func(t *testing.T) {
		t.Run("success", func(t *testing.T) {
			ctrl := gomock.NewController(t)
			driver := NewMockDriver(ctrl)
			printer := NewMockPrinter(ctrl)
			fileReader := NewMockFileReader(ctrl)

			const query = "my sql query"
			const dir = "my/dir"
			const filename = "1_initial_migration.fw.nt.sql"
			const migrationName = "0001_initial_migration"

			step := newTestStep(filename)
			step.MigrationName = migrationName

			path := filepath.Join(dir, filename)

			gomock.InOrder(
				printer.EXPECT().Print(step.String()+" ... "),
				fileReader.EXPECT().ReadFile(path).Return([]byte(query), nil),
				driver.EXPECT().ExecuteStep(step, query),
				printer.EXPECT().Print("OK\n"),
			)

			err := step.ExecuteAndLog(dir, driver, fileReader, printer)
			require.NoError(t, err)
			ctrl.Finish()
		})

		t.Run("ReadFile error", func(t *testing.T) {
			ctrl := gomock.NewController(t)
			driver := NewMockDriver(ctrl)
			printer := NewMockPrinter(ctrl)
			fileReader := NewMockFileReader(ctrl)

			const dir = "my/dir"
			const filename = "1_initial_migration.fw.nt.sql"
			const migrationName = "0001_initial_migration"

			step := newTestStep(filename)
			step.MigrationName = migrationName

			path := filepath.Join(dir, filename)

			gomock.InOrder(
				printer.EXPECT().Print(step.String()+" ... "),
				fileReader.EXPECT().ReadFile(path).Return(nil, assert.AnError),
				printer.EXPECT().Print("FAILED\n"),
			)

			err := step.ExecuteAndLog(dir, driver, fileReader, printer)
			require.Error(t, err)
			ctrl.Finish()
		})

		t.Run("ExecuteStep error", func(t *testing.T) {
			ctrl := gomock.NewController(t)
			driver := NewMockDriver(ctrl)
			printer := NewMockPrinter(ctrl)
			fileReader := NewMockFileReader(ctrl)

			const query = "my sql query"
			const dir = "my/dir"
			const filename = "1_initial_migration.fw.nt.sql"
			const migrationName = "0001_initial_migration"

			step := newTestStep(filename)
			step.MigrationName = migrationName

			path := filepath.Join(dir, filename)

			gomock.InOrder(
				printer.EXPECT().Print(step.String()+" ... "),
				fileReader.EXPECT().ReadFile(path).Return([]byte(query), nil),
				driver.EXPECT().ExecuteStep(step, query).Return(assert.AnError),
				printer.EXPECT().Print("FAILED\n"),
			)

			err := step.ExecuteAndLog(dir, driver, fileReader, printer)
			require.Error(t, err)
			ctrl.Finish()
		})
	})
}

func TestInterruptDetector(t *testing.T) {
	newDetector := func(t *testing.T) (_ *gomock.Controller, _ *MockExiter, _ *MockPrinter, _ *interruptDetector, cancel func(), ch chan<- os.Signal) {
		ctrl := gomock.NewController(t)
		exiter := NewMockExiter(ctrl)
		printer := NewMockPrinter(ctrl)
		id, idCancel, ch := newInternalInterruptDetector(exiter, printer)
		return ctrl, exiter, printer, id, idCancel, ch
	}

	t.Run("no signal", func(t *testing.T) {
		ctrl, _, _, id, idCancel, _ := newDetector(t)
		defer idCancel()

		id.ExitIfInterrupted()

		ctrl.Finish()
	})

	for _, sig := range []os.Signal{syscall.SIGINT, syscall.SIGTERM} {
		t.Run("signal:"+sig.String(), func(t *testing.T) {
			ctrl, exiter, printer, id, idCancel, ch := newDetector(t)
			defer idCancel()

			gomock.InOrder(
				printer.EXPECT().Print(fmt.Sprintf("\nsignal: %v\n", sig)),
				exiter.EXPECT().Exit(1),
			)

			ch <- sig
			id.waitForSignal()
			id.ExitIfInterrupted()

			ctrl.Finish()
		})
	}

	t.Run("cancel", func(t *testing.T) {
		ctrl, _, _, id, idCancel, _ := newDetector(t)

		idCancel()
		id.waitForSignal()

		ctrl.Finish()
	})
}
