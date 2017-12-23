package main

import (
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

const usage = `Usage: sql-migrate <command> [command_options...]

Commands:
  init      Create the migrations table in the DB if not exists
  status    Show info about the current state of the migrations
  plan      Show the plan that would be executed by a goto command
  goto      Migrate to a specific version of the DB schema
  version   Show version info

Run 'sql-migrate <command> -help' for more info.
`

var commands = map[string]func(args []string){
	"init":    cmdInit,
	"status":  cmdStatus,
	"plan":    cmdPlan,
	"goto":    cmdGoto,
	"version": cmdVersion,
}

func main() {
	log.SetFlags(0)

	flag.Usage = func() {
		log.Print(usage)
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(1)
	}

	cmdFunc, ok := commands[flag.Arg(0)]
	if !ok {
		log.Printf("Invalid command: %s", flag.Arg(0))
		flag.Usage()
		os.Exit(1)
	}

	cmdFunc(flag.Args()[1:])
}

const initUsage = `Usage: sql-migrate init <options...>

Creates the migration table if it hasn't yet been created.
Issuing an init command on an already initialised DB is a harmless no-op.

Options:
`

func cmdInit(args []string) {
	fs := newFlagSet("init", initUsage)
	driverName, dsn, table := addDriverFlags(fs)
	fs.Parse(args)
	expectNoArgs(fs)
	driver, db := processDriverFlags(fs, driverName, dsn, table)
	defer db.Close()
	if err := driver.CreateMigrationsTable(db); err != nil {
		log.Print(err)
		os.Exit(1)
	}
	fmt.Println("Init success.")
}

const statusUsage = `Usage: sql-migrate status <options...>

Print the status of the migrations.

Options:
`

func cmdStatus(args []string) {
	fs := newFlagSet("status", statusUsage)
	driverName, dsn, table := addDriverFlags(fs)
	dir, fwd, bwd, notx, ext := addDirFlags(fs)
	fs.Parse(args)

	expectNoArgs(fs)
	migrations := processDirFlag(dir, fwd, bwd, notx, ext)
	driver, db := processDriverFlags(fs, driverName, dsn, table)
	defer db.Close()

	forwardMigrated, err := driver.GetForwardMigratedNames(db)
	if err != nil {
		log.Printf("Error loading migration status from the migrations table: %s", err)
		os.Exit(1)
	}

	checkbox := func(checked bool) string {
		if checked {
			return "[X]"
		}
		return "[ ]"
	}
	for _, m := range migrations.Sorted {
		_, applied := forwardMigrated[m.Forward.MigrationName]
		s := checkbox(applied) + " " + m.Forward.Filename
		if m.Forward.ParsedFilename.NoTx {
			s += " [no-forward-transaction]"
		}
		if m.Backward == nil {
			s += " [no-backward-migration]"
		} else if m.Backward.ParsedFilename.NoTx {
			s += " [no-backward-transaction]"
		}
		fmt.Println(s)
	}

	allSet := make(map[string]struct{}, len(migrations.Sorted))
	for _, m := range migrations.Sorted {
		allSet[m.Forward.MigrationName] = struct{}{}
	}
	var invalidForwardMigrated []string
	for name := range forwardMigrated {
		if _, ok := allSet[name]; !ok {
			invalidForwardMigrated = append(invalidForwardMigrated, name)
		}
	}
	sort.Strings(invalidForwardMigrated)
	for _, migrationName := range invalidForwardMigrated {
		fmt.Println(" !  Entry in the migration table without migration files: " + migrationName)
	}

	if len(migrations.Sorted) == 0 && len(invalidForwardMigrated) == 0 {
		fmt.Println("There are no migrations.")
	}
}

const planUsage = `Usage: sql-migrate plan <options...>

Show a plan without modifying the database.

This command has the same options as the goto command.
The plan lists the steps that would be performed
by a goto with the same commandline parameters.

Options:
`

func cmdPlan(args []string) {
	fs := newFlagSet("plan", planUsage)
	driverName, dsn, table := addDriverFlags(fs)
	dir, fwd, bwd, notx, ext := addDirFlags(fs)
	target := addTargetFlag(fs)
	fs.Parse(args)

	expectNoArgs(fs)
	processTargetFlag(target)
	migrations := processDirFlag(dir, fwd, bwd, notx, ext)
	driver, db := processDriverFlags(fs, driverName, dsn, table)
	defer db.Close()

	steps := loadStateAndCreatePlan(*target, migrations, driver, db)
	for _, st := range steps {
		fmt.Println(st)
	}
	if len(steps) == 0 {
		fmt.Println("Nothing to migrate.")
	}
}

const gotoUsage = `Usage: sql-migrate goto <options...>

Go to the specified version of the database schema by going to the
migration specified by the -target parameter.

- Migrations that are newer the target and have been forward migrated
  are reverted by executing their backward steps in descending order.
- The target migration along with the older migrations are forward
  migrated in ascending order by executing the forward steps of those
  that haven't yet been forward migrated.

Options:
`

func cmdGoto(args []string) {
	fs := newFlagSet("goto", gotoUsage)
	driverName, dsn, table := addDriverFlags(fs)
	dir, fwd, bwd, notx, ext := addDirFlags(fs)
	target := addTargetFlag(fs)
	fs.Parse(args)

	expectNoArgs(fs)
	processTargetFlag(target)
	migrations := processDirFlag(dir, fwd, bwd, notx, ext)
	driver, db := processDriverFlags(fs, driverName, dsn, table)
	defer db.Close()

	steps := loadStateAndCreatePlan(*target, migrations, driver, db)
	for _, st := range steps {
		if err := st.ExecuteAndLog(*dir, driver, db, ioutilFileReader{}, fmtPrinter{}); err != nil {
			log.Print(err)
			os.Exit(1)
		}
	}
	if len(steps) == 0 {
		fmt.Println("Nothing to migrate.")
	}
}

type ioutilFileReader struct{}

func (ioutilFileReader) ReadFile(filename string) ([]byte, error) {
	return ioutil.ReadFile(filename)
}

type fmtPrinter struct{}

func (fmtPrinter) Print(s string) {
	fmt.Print(s)
}

const versionUsage = `Usage: sql-migrate version

Show version and build info.
`

// You can set the below variables at compile time with
// `go build -ldflags "-X main.variableName=value"`.

var version string
var gitHash string
var buildDate string

func cmdVersion(args []string) {
	fs := newFlagSet("version", versionUsage)
	fs.Parse(args)
	expectNoArgs(fs)

	if version == "" {
		version = "dev"
	}
	fmt.Printf("version     : %s\n", version)
	if buildDate != "" {
		fmt.Printf("build date  : %s\n", buildDate)
	}
	if gitHash != "" {
		fmt.Printf("git hash    : %s\n", gitHash)
	}
	fmt.Printf("go version  : %s\n", runtime.Version())
	fmt.Printf("go compiler : %s\n", runtime.Compiler)
	fmt.Printf("platform    : %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Printf("db drivers  : %s\n", strings.Join(driverNames(), ", "))
}

func newFlagSet(name, usage string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ExitOnError)
	fs.Usage = func() {
		log.Print(usage)
		fs.PrintDefaults()
	}
	return fs
}

func addDriverFlags(fs *flag.FlagSet) (driverName, dsn, table *string) {
	driverName = fs.String("driver", "", "Driver name. Valid values: "+strings.Join(driverNames(), ", "))
	dsn = fs.String("dsn", "", "Driver specific data source name.")
	table = fs.String("migrations_table", "migrations", "The name of the table that stores the migration state.")
	return
}

func processDriverFlags(fs *flag.FlagSet, driverName, dsn, table *string) (Driver, DB) {
	if *table == "" {
		log.Print("The -migrations_table option can't be an empty string.")
		os.Exit(1)
	}
	if *driverName == "" || *dsn == "" {
		log.Print("Missing -driver or -dsn parameter.")
		fs.Usage()
		os.Exit(1)
	}

	driverFactory, ok := drivers[*driverName]
	if !ok {
		log.Print("Invalid driver: " + *driverName)
		os.Exit(1)
	}
	driver := driverFactory(*table)

	db, err := driver.Open(*dsn)
	if err != nil {
		log.Printf("Error connecting to DB %q: %s", *dsn, err)
		os.Exit(1)
	}
	return driver, dbWrapper{db}
}

func addDirFlags(fs *flag.FlagSet) (dir, fwd, bwd, notx, ext *string) {
	dir = fs.String("dir", "", "The directory containing the migration files.")
	fwd = fs.String("fwd", "", "The filename suffix that marks the file as a forward migration.")
	bwd = fs.String("bwd", ".back", "The filename suffix that marks the file as a backward migration.")
	notx = fs.String("notx", ".notx", "The filename suffix that doesn't allow the execution of the migration step in a transaction.")
	ext = fs.String("ext", ".sql", "The expected extension of migration files.")
	return
}

func processDirFlag(dir, fwd, bwd, notx, ext *string) *Migrations {
	if *dir == "" {
		log.Print("The -dir option can't be an empty string.")
		os.Exit(1)
	}
	if *fwd != "" && *fwd == *bwd {
		log.Print("The -fwd and -bwd options can't have the same non-empty value.")
		os.Exit(1)
	}
	if *notx == "" {
		log.Print("The -notx option can't be an empty string.")
		os.Exit(1)
	}

	ms, err := loadMigrationsDir(*dir, *fwd, *bwd, *notx, *ext, func(dir string) []string {
		entries, err := ioutil.ReadDir(dir)
		if err != nil {
			log.Printf("Error loading migrations dir %q: %s", dir, err)
			os.Exit(1)
		}
		a := make([]string, 0, len(entries))
		for _, fi := range entries {
			if !fi.IsDir() {
				a = append(a, fi.Name())
			}
		}
		return a
	})
	if err != nil {
		log.Print(err)
		os.Exit(1)
	}
	return ms
}

func addTargetFlag(fs *flag.FlagSet) *string {
	return fs.String("target", "", `The numeric ID or name of the target migration file. It can also be one of the "initial" and "latest" constants.`)
}

func processTargetFlag(target *string) {
	if *target == "" {
		log.Print("The -target option can't be an empty string.")
		os.Exit(1)
	}
}

func expectNoArgs(fs *flag.FlagSet) {
	if fs.NArg() != 0 {
		log.Printf("Redundant args: %s", strings.Join(fs.Args(), ", "))
		fs.Usage()
		os.Exit(1)
	}
}

var drivers = map[string]func(tableName string) Driver{}

func driverNames() []string {
	names := make([]string, 0, len(drivers))
	for name := range drivers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

type Driver interface {
	Open(dsn string) (*sql.DB, error)
	CreateMigrationsTable(Execer) error
	SetMigrationState(e Execer, migrationName string, forwardMigrated bool) error
	GetForwardMigratedNames(Querier) (map[string]struct{}, error)
}

type Execer interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
}

type Querier interface {
	Query(query string, args ...interface{}) (*sql.Rows, error)
}

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

// dbWrapper implements the DB interface
type dbWrapper struct {
	*sql.DB
}

func (o dbWrapper) Begin() (TX, error) {
	tx, err := o.DB.Begin()
	return tx, err
}

type Migrations struct {
	Sorted []*Migration
	Names  map[string]int
}

type Migration struct {
	Forward  *Step
	Backward *Step
}

type Printer interface {
	Print(string)
}

type FileReader interface {
	ReadFile(filename string) ([]byte, error)
}

type Step struct {
	Filename       string
	MigrationName  string
	ParsedFilename *ParsedFilename
}

func (o *Step) ExecuteAndLog(dir string, d Driver, db DB, r FileReader, p Printer) error {
	p.Print(o.String() + " ... ")
	if err := o.Execute(dir, d, db, r); err != nil {
		p.Print("FAILED\n")
		return err
	}
	p.Print("OK\n")
	return nil
}

func (o *Step) Execute(dir string, d Driver, db DB, r FileReader) error {
	performStep := func(e Execer) error {
		query, err := r.ReadFile(filepath.Join(dir, o.Filename))
		if err != nil {
			return err
		}
		if _, err := e.Exec(string(query)); err != nil {
			return err
		}
		return d.SetMigrationState(e, o.MigrationName, o.ParsedFilename.Direction == DirectionForward)
	}

	if o.ParsedFilename.NoTx {
		return performStep(db)
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	if err := performStep(tx); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (o *Step) String() string {
	s := "forward-migrate "
	if o.ParsedFilename.Direction == DirectionBackward {
		s = "backward-migrate "
	}
	s += o.Filename
	if o.ParsedFilename.NoTx {
		s += " [no-transaction]"
	}
	return s
}

type listDirFunc func(dir string) []string

func loadMigrationsDir(migrationsDir, fwd, bwd, notx, ext string, f listDirFunc) (*Migrations, error) {
	entries := f(migrationsDir)
	idMap := make(map[int64]*Migration, len(entries))
	for _, name := range entries {
		parsed, err := parseFilename(name, fwd, bwd, notx, ext)
		if err != nil {
			return nil, fmt.Errorf("error parsing filename %q: %s", name, err)
		}

		m, ok := idMap[parsed.ID]
		if !ok {
			m = &Migration{}
			idMap[parsed.ID] = m
		}
		p := &m.Forward
		if parsed.Direction == DirectionBackward {
			p = &m.Backward
		}
		if *p != nil {
			return nil, fmt.Errorf("duplicate %s migration for ID %v: %q and %q", parsed.Direction, parsed.ID, (*p).Filename, name)
		}
		*p = &Step{
			Filename:       name,
			ParsedFilename: parsed,
		}
	}

	return sortAndIndexMigrations(idMap)
}

func sortAndIndexMigrations(idMap map[int64]*Migration) (*Migrations, error) {
	ms := &Migrations{
		Sorted: make([]*Migration, 0, len(idMap)),
		Names:  make(map[string]int, len(idMap)*4),
	}

	for _, m := range idMap {
		if m.Forward == nil {
			return nil, fmt.Errorf("migration without forward step - %q", m.Backward.Filename)
		}
		ms.Sorted = append(ms.Sorted, m)

		name := fmt.Sprintf("%04d%s", m.Forward.ParsedFilename.ID, m.Forward.ParsedFilename.Description)
		m.Forward.MigrationName = name
		if m.Backward != nil {
			m.Backward.MigrationName = name
			if m.Forward.ParsedFilename.Description != m.Backward.ParsedFilename.Description {
				return nil, fmt.Errorf("forward and backward migrations (%q and %q) have different description (%q and %q)",
					m.Forward.Filename, m.Backward.Filename, m.Forward.ParsedFilename.Description, m.Backward.ParsedFilename.Description)
			}
		}
	}
	sort.Slice(ms.Sorted, func(i, j int) bool {
		return ms.Sorted[i].Forward.ParsedFilename.ID < ms.Sorted[j].Forward.ParsedFilename.ID
	})

	for i, m := range ms.Sorted {
		ms.Names[strconv.FormatInt(m.Forward.ParsedFilename.ID, 10)] = i
		ms.Names[m.Forward.ParsedFilename.IDStr] = i
		ms.Names[m.Forward.MigrationName] = i
		ms.Names[m.Forward.Filename] = i
	}

	if len(ms.Sorted) == 0 {
		return ms, nil
	}
	if ms.Sorted[0].Forward.ParsedFilename.ID != 1 {
		return nil, fmt.Errorf("the first migration ID must be 1 but it is %v", ms.Sorted[0].Forward.ParsedFilename.ID)
	}
	for i, m := range ms.Sorted[1:] {
		if m.Forward.ParsedFilename.ID != ms.Sorted[i].Forward.ParsedFilename.ID+1 {
			return nil, fmt.Errorf("missing migration ID (gap): %v", ms.Sorted[i].Forward.ParsedFilename.ID+1)
		}
	}
	return ms, nil
}

type Direction int

const (
	DirectionUndefined Direction = iota
	DirectionForward
	DirectionBackward
)

func (o Direction) String() string {
	switch o {
	case DirectionForward:
		return "forward"
	case DirectionBackward:
		return "backward"
	default:
		return fmt.Sprintf("Direction(%v)", int(o))
	}
}

type ParsedFilename struct {
	ID          int64
	IDStr       string // IDStr retains leading zeros (if any)
	Description string
	Direction   Direction
	NoTx        bool
}

func parseFilename(fn, fwd, bwd, notx, ext string) (*ParsedFilename, error) {
	var parsed ParsedFilename

	i := strings.IndexFunc(fn, func(c rune) bool {
		return c < '0' || c > '9'
	})

	switch {
	case fn == "" || i == 0:
		return nil, errors.New("missing numeric ID prefix")
	case i < 0:
		parsed.IDStr, fn = fn, ""
	default:
		parsed.IDStr, fn = fn[:i], fn[i:]
	}

	id, err := strconv.ParseInt(parsed.IDStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid ID: %s", err)
	}
	parsed.ID = id

	if !strings.HasSuffix(fn, ext) {
		return nil, fmt.Errorf("missing %q extension", ext)
	}
	fn = strings.TrimSuffix(fn, ext)

loop:
	for {
		switch {
		case fwd != "" && strings.HasSuffix(fn, fwd):
			fn = strings.TrimSuffix(fn, fwd)
			if parsed.Direction != DirectionUndefined {
				return nil, fmt.Errorf("multiple %q and/or %q suffixes", fwd, bwd)
			}
			parsed.Direction = DirectionForward
		case bwd != "" && strings.HasSuffix(fn, bwd):
			fn = strings.TrimSuffix(fn, bwd)
			if parsed.Direction != DirectionUndefined {
				return nil, fmt.Errorf("multiple %q and/or %q suffixes", fwd, bwd)
			}
			parsed.Direction = DirectionBackward
		case notx != "" && strings.HasSuffix(fn, notx):
			fn = strings.TrimSuffix(fn, notx)
			if parsed.NoTx {
				return nil, fmt.Errorf("multiple %q suffixes", notx)
			}
			parsed.NoTx = true
		default:
			break loop
		}
	}

	parsed.Description = fn

	if parsed.Direction != DirectionUndefined {
		return &parsed, nil
	}

	switch {
	case fwd != "" && bwd != "":
		return nil, fmt.Errorf("exactly one of the %q and %q suffixes has to be used", fwd, bwd)
	case fwd == "":
		parsed.Direction = DirectionForward
	case bwd == "":
		parsed.Direction = DirectionBackward
	}

	return &parsed, nil
}

func loadStateAndCreatePlan(target string, ms *Migrations, d Driver, db DB) []*Step {
	forwardMigrated, err := d.GetForwardMigratedNames(db)
	if err != nil {
		log.Printf("Error loading migration status from the migrations table: %s", err)
		os.Exit(1)
	}
	steps, err := createPlan(target, ms, forwardMigrated)
	if err != nil {
		log.Print(err)
		os.Exit(1)
	}
	return steps
}

func createPlan(target string, ms *Migrations, forwardMigrated map[string]struct{}) ([]*Step, error) {
	allSet := make(map[string]struct{}, len(ms.Sorted))
	seenUnapplied := false
	for _, m := range ms.Sorted {
		allSet[m.Forward.MigrationName] = struct{}{}
		_, applied := forwardMigrated[m.Forward.MigrationName]
		if applied && seenUnapplied {
			return nil, fmt.Errorf("there is at least one unapplied migration before applied migration %q (examine it with the status command and fix it manually)", m.Forward.Filename)
		}
		seenUnapplied = seenUnapplied || !applied
	}
	for entry := range forwardMigrated {
		if _, ok := allSet[entry]; !ok {
			return nil, fmt.Errorf("there is at least one entry in the migrations table without an existing migration file (examine it with the status command and fix it manually) - entry=%q", entry)
		}
	}

	targetIdx := -1
	switch target {
	case "initial":
	case "latest":
		targetIdx = len(ms.Sorted) - 1
	default:
		if idx, ok := ms.Names[target]; ok {
			targetIdx = idx
		} else {
			return nil, fmt.Errorf("invalid target migration - %q", target)
		}
	}

	var steps []*Step
	for i := len(ms.Sorted) - 1; i > targetIdx; i-- {
		if _, ok := forwardMigrated[ms.Sorted[i].Forward.MigrationName]; ok {
			st := ms.Sorted[i].Backward
			if st == nil {
				return nil, fmt.Errorf("migration %q doesn't have a backward step", ms.Sorted[i].Forward.Filename)
			}
			steps = append(steps, st)
		}
	}
	for i := 0; i <= targetIdx; i++ {
		if _, ok := forwardMigrated[ms.Sorted[i].Forward.MigrationName]; !ok {
			steps = append(steps, ms.Sorted[i].Forward)
		}
	}
	return steps, nil
}
