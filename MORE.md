# More Info

## Migration files

A migration consists of a forward migration (.sql) file and optionally a backward
migration step in a separate file. The filenames have to have a specific format.

### Migration filename format

- The filename has to start with a positive integer number that can have an
  arbitrary number of leading zeros. This number is the numeric migration ID.

  `sql-migrate` uses integer comparison to order/sort the migrations instead of
  relying on alphabetical order. This means that leading zeros don't affect the
  order used by `sql-migrate`. However leading zeros are still useful because
  most tools/editors/IDEs sort files using alphabetical order.

  It is subjective but I recommend a zero padding that makes the migration ID
  at least 4 digits long.

- The numeric ID is followed by the description of the migration.
  The description can't start with a digit but the rest of description can
  contain any character. The description is optional, can be omitted.

  If a migration has both a forward and a backward .sql file then they have
  to have the same description in the filenames (besides the same migration ID).

- The description is followed by a few optional suffixes appended to the
  filename in any order. There are 3 suffixes and they can optionally be
  configured from the commandline:

  - The `-fwd` commandline parameter (default: "") sets the suffix that
    marks a file as a forward step.
  - The `-bwd` commandline parameter (default: ".back") sets the suffix that
    marks a file as a backward step.
  - The `-notx` commandline parameter (default: ".notx") sets the suffix that
    makes sure that the given .sql file is executed outside of transactions.

    Useful only with databases that allow DDL statements inside transactions
    (e.g.: postgres) because this tool automatically executes forward and backward
    migration steps in their own transactions when this filename suffix isn't used.

    This filename suffix is ignored (and therefore shouldn't be used) with
    databases that don't support DDL inside transactions (e.g.: mysql).

  By default the `-fwd` suffix is an empty string that means if you don't use
  the suffix specified with `-bwd` then the file is automatically a forward step.

  If both `-fwd` and `-bwd` are set to non-empty strings then you are allowed to
  use only one of them in a filename.

- After the suffixes the file has to have a ".sql" extension.
  Optionally you can configure the extension with the `-ext` commandline parameter.

### Migration filename examples

- Without description in the filenames:
  - `0001.sql`
  - `0001.back.sql`
  
- If you use a description in the migration filenames then the description part
  of the forward and backward filenames must have the same description
  (which is `"_my_description"` in this example):
  - `001_my_description.sql`
  - `001_my_description.back.sql`

- The forward step has to be executed outside of transactions:
  - `01.notx.sql`
  - `01.back.sql`

- Both the forward and backward steps have to be executed outside of transactions:
  - `1.notx.sql`
  - `1.notx.back.sql` OR `1.back.notx.sql`

- Customising the filename suffixes and the filename extension: assuming that the
  `sql-migrate` command is executed with the `-fwd .fw -bwd .bw -notx .nt -ext ""`
  parameters. No description, the backward step has to be outside transactions:
  - `0001.fw`
  - `0001.nt.bw` OR `0001.bw.nt`
