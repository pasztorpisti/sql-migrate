//go:generate mockgen -package main -destination mock_interfaces_test.go -source interfaces.go
//go:generate mockgen -package main -destination mock_sql_test.go database/sql Result

package main

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"os"
)

type Driver interface {
	ExecuteStep(st *Step, contents string) error
	CreateMigrationsTable() error
	GetForwardMigratedNames() (map[string]struct{}, error)
	Close()
}

// The DB and TX interfaces are used instead of *sql.DB and *sql.Tx
// in order to be able to use mock DB and TX implementations during testing.
type DB interface {
	Execer
	Querier
	Begin() (TX, error)
	Close() error
}

type TX interface {
	Execer
	Commit() error
	Rollback() error
}

type Execer interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
}

type Querier interface {
	Query(query string, args ...interface{}) (*sql.Rows, error)
}

type Printer interface {
	Print(string)
}

type FileReader interface {
	ReadFile(filename string) ([]byte, error)
}

type Exiter interface {
	Exit(int)
}

// dbWrapper implements the DB interface.
type dbWrapper struct {
	*sql.DB
}

func (o dbWrapper) Begin() (TX, error) {
	tx, err := o.DB.Begin()
	return tx, err
}

// ioutilFileReader implements the FileReader interface.
type ioutilFileReader struct{}

func (ioutilFileReader) ReadFile(filename string) ([]byte, error) {
	return ioutil.ReadFile(filename)
}

// stdoutPrinter implements the Printer interface.
type stdoutPrinter struct{}

func (stdoutPrinter) Print(s string) {
	fmt.Print(s)
}

// stderrPrinter implements the Printer interface.
type stderrPrinter struct{}

func (stderrPrinter) Print(s string) {
	fmt.Fprint(os.Stderr, s)
}

// osExiter implements the Exiter interface.
type osExiter struct{}

func (osExiter) Exit(code int) {
	os.Exit(code)
}
