# yozora

A small command-line toolbox of everyday file and data utilities, written in Go and
shipped as a single self-contained binary. The command is `yzr`.

```
yzr <method> [flags] [args]
```

## Install

From PyPI (a prebuilt `yzr` binary is bundled in the wheel for Linux, macOS and Windows):

```bash
pip install yozora
yzr ls
```

From source (needs Go):

```bash
go install github.com/ShashankRaoCoding/yozora@latest
```

The Go install puts the binary in `~/go/bin` and names it `yozora`; the PyPI package installs
it as `yzr`. To put the binary's directory on your shell `PATH`, run `yzr ensure` (or
`python -m yozora ensure` right after a pip install).

## Methods

| Method | What it does |
| --- | --- |
| `ls [-a] [-d depth] [-D] [path]` | Print a directory as a file tree. Symlinks are shown, never followed. |
| `create [-n lines] <path>` | Create a file and any missing parent directories. Never modifies an existing file. |
| `sql [-i csv\|tsv] [-o csv\|tsv] "<query>"` | Run SQL (SQLite) over CSV/TSV from stdin. The table name is implied and `if` works as `where`. |
| `print table` | Render CSV from stdin as an ASCII table. |
| `print image [file]` | Draw a PNG, JPEG or GIF in the terminal (kitty graphics, sixel, or an ASCII fallback). |
| `convert -i <fmt> -o <fmt>` | Convert between file formats, stdin to stdout. Currently `mrc` to `png`. |
| `ensure [-f] [-p] [-d dir] [-s shell]` | Add the binary's directory to your shell `PATH`, once. |

## Examples

```bash
yzr ls -d 2                                   # tree, two levels deep
yzr create -n 3 notes/2026/draft.txt          # makes notes/2026/ and a 3-line file

cat data.csv | yzr sql -i csv -o tsv "select * if name = 'Shanks'"
yzr sql "select name,age where age > 21 order by age desc limit 10" < people.csv
yzr sql -i tsv -o csv "select *" < data.tsv > data.csv
yzr sql "select city, count(*) as n group by city order by n desc" < people.csv

yzr sql "select name,age" < people.csv | yzr print table
yzr print image figure.png
yzr convert -i mrc -o png < micrograph.mrc > micrograph.png
```

## SQL

`yzr sql` loads stdin into an in-memory SQLite database
([modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite), pure Go, no cgo) as one table and
runs your query against it, so everything SQLite supports works: `group by`, joins against
subqueries, CTEs, window functions, `union`, and so on. The first input row is the header. Two
shortcuts sit on top of plain SQL:

- **The table name is implied.** A `select` with no `from` reads from the input, as if you had
  written `from t`. This applies to every select in the query, subqueries and `union` arms
  included. Writing `from t` yourself is fine too.
- **`if` means `where`.**

Things worth knowing:

- Column names are case-insensitive. Use "double quotes" for names with spaces and 'single
  quotes' for string values. Two headers that differ only by case are an error.
- Values stay text, exactly as in the input, but numeric-looking values compare and sort
  numerically: `age > 21` treats 9 as less than 21, and `max(age)` is the numeric maximum.
  Numbers sort before non-numeric text.
- Empty cells are NULL: use `age is null`, not `age = ''`. NULL is written back out as an empty
  cell.
- A `select` without `from` always reads the input, so `select 1+1` returns one row per input
  row. Add `limit 1` if you only want one.

## Notes

- `ensure` edits only your shell config file (a short marked block), is safe to run twice, and
  does not support Windows.
- The PyPI package is built and uploaded with [kyoyu](https://github.com/ShashankRaoCoding/kyoyu).
  Its only job is to make `yzr` installable with pip; the program itself is this repository.

## Licence

GNU General Public License v3.0. See [LICENSE](LICENSE).
