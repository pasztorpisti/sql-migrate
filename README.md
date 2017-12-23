# sql-migrate [![build-status](https://travis-ci.org/pasztorpisti/sql-migrate.svg?branch=master)](https://travis-ci.org/pasztorpisti/sql-migrate)

A powerfully simple cross-platform command-line (cli) SQL schema migration tool.

Supported databases:

- postgres
- mysql

Features:

- `sql-migrate goto` migrates to a specific version of the database schema by
                     executing the necessary forward or backward steps
- `sql-migrate plan` shows what `goto` would do without modifying the DB
                     (`goto` with dry-run)
- `sql-migrate status` shows the status of the migrations
- Plain SQL migration files without DSL.
- Receives all parameters from the commandline. No config files.

If you want a per-project config file then create a shell script in your project
and invoke `sql-migrate` through that script. This minimalist technique can
solve problems that are usually handled with config files:

- Specifying (default or fixed) parameters in the shell script (config) instead
  of passing them always as commandline parameters.
- Being able to specify and/or override parameters using env vars.
- Keeping config in one place for different databases located in different
  development environments (dev, staging, production, ...).

[Here is a shell script template: `sql-migrate.sh.template`](/sql-migrate.sh.template)

## Installation

### 1. Stable binary release (recommended)

[Download](https://github.com/pasztorpisti/sql-migrate/releases)
and run `sql-migrate -help` for commandline options.

### 2. Unstable dev version

```bash
go get -u github.com/pasztorpisti/sql-migrate
sql-migrate -help
```

## Usage

### 1. Setup

- Create a directory for your migration .sql files.
- Setup the database. With the credentials create a postgres or mysql driver
  specific "data source name" that you have to pass to `sql-migrate` in the
  `-dsn` parameter.
  
  - PostgreSQL DSN format: https://godoc.org/github.com/lib/pq
  - MySQL DSN format: https://github.com/go-sql-driver/mysql#dsn-data-source-name

- If you want a config file instead of passing commandline args all the time
  then copy [`sql-migrate.sh.template`](/sql-migrate.sh.template), rename it
  and tailor it's contents for your project.

### 2. Initialise migrations

Initialise the database by creating the migrations table:

```bash
sql-migrate init -driver <driver> -dsn <dsn>
```

This has to be performed only once for a given DB. Executing it again is a no-op.

### 3. Create migration file(s)

A migration consists of a forward migration (.sql) file and optionally a backward
migration in a separate file. The filenames have to conform to some rules that
are discussed in [`MORE.md`](/MORE.md).

In my example I create only one migration with a forward and a backward step
for a postgres database:

**0001_initial.sql:**
```sql
CREATE TABLE "test" (
  "id" SERIAL PRIMARY KEY,
  "str" TEXT NOT NULL
);

CREATE TABLE "test2" (
  "id2" SERIAL PRIMARY KEY,
  "str2" TEXT NOT NULL
);
```

**0001_initial.back.sql:**
```sql
DROP TABLE "test2";
DROP TABLE "test";
```

Every migration filename has to start with an integer ID. The first ID must be 1.
Subsequent IDs always increase by 1 without leaving any gaps.

### 4. Forward migrate

After creating one or more migration files you can apply them to the DB:

```bash
# It is optional but recommended to dry-run the migration without modifying
# the database in order to check what a goto command would do the the DB:
sql-migrate plan -target latest -dir <migrations_dir> -driver <driver> -dsn <dsn>

# Applying the migrations to the DB:
sql-migrate goto -target latest -dir <migrations_dir> -driver <driver> -dsn <dsn>

# Checking the status of the migrations:
sql-migrate status -dir migrations -driver <driver> -dsn <dsn>
```

If you copy the [`sql-migrate.sh.template`](/sql-migrate.sh.template) to your project
(and edit it, rename it to `sql-migrate.sh` and set the execute bit on the file)
then instead of the above commands you can use the following much simpler ones:

```bash
./sql-migrate.sh plan latest
./sql-migrate.sh goto latest
./sql-migrate.sh status
```

## TL;DR

I tried to keep this `README.md` as short as possible adding only:

- A list of features.
- The shortest possible guide to bootstrap and use `sql-migrate` in your project.

If you are interested in the details then read the [`MORE.md`](/MORE.md) file.
