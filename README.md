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
| `table -i csv\|tsv -o csv\|tsv "<query>"` | Run a SQL-style query over CSV/TSV from stdin. |
| `print table` | Render CSV from stdin as an ASCII table. |
| `print image [file]` | Draw a PNG, JPEG or GIF in the terminal (kitty graphics, sixel, or an ASCII fallback). |
| `convert -i <fmt> -o <fmt>` | Convert between file formats, stdin to stdout. Currently `mrc` to `png`. |
| `ensure [-f] [-p] [-d dir] [-s shell]` | Add the binary's directory to your shell `PATH`, once. |

## Examples

```bash
yzr ls -d 2                                   # tree, two levels deep
yzr create -n 3 notes/2026/draft.txt          # makes notes/2026/ and a 3-line file

yzr table "select name,age where age > 21 order by age desc limit 10" < people.csv
yzr table -i tsv -o csv "select *" < data.tsv > data.csv

yzr table "select name,age" < people.csv | yzr print table
yzr print image figure.png
yzr convert -i mrc -o png < micrograph.mrc > micrograph.png
```

Query columns are matched exactly, then case-insensitively. Use "double quotes" for column
names with spaces and 'single quotes' for string values. The first input row is the header.

## Notes

- `ensure` edits only your shell config file (a short marked block), is safe to run twice, and
  does not support Windows.
- The PyPI package is built and uploaded with [kyoyu](https://github.com/ShashankRaoCoding/kyoyu).
  Its only job is to make `yzr` installable with pip; the program itself is this repository.

## Licence

GNU General Public License v3.0. See [LICENSE](LICENSE).
