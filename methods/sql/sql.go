// Package sql implements `yozora sql -i <fmt> -o <fmt> "<query>"`: CSV/TSV on
// stdin is loaded into an in-memory SQLite database (modernc.org/sqlite, pure
// Go) as a single table, the query runs as ordinary SQL, and the result goes to
// stdout as CSV/TSV.
//
//	cat data.csv | yozora sql -i csv -o tsv "select * if name = 'Shanks'"
//
// The table name is implied (see the prep package) and `if` stands for `where`;
// the rest is full SQLite SQL: joins against subqueries, group by, window
// functions, CTEs, and so on. The first input row is the header.
package sql

import (
	"database/sql"
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	sqlite "modernc.org/sqlite" // also registers the "sqlite" database/sql driver

	"github.com/ShashankRaoCoding/yozora/methods"
	"github.com/ShashankRaoCoding/yozora/methods/sql/prep"
)

func init() {
	methods.Methods["sql"] = run
}

var delims = map[string]rune{"csv": ',', "tsv": '\t'}

func run(args []string) {
	fs := flag.NewFlagSet("sql", flag.ExitOnError)
	in := fs.String("i", "csv", "input format: csv|tsv")
	out := fs.String("o", "csv", "output format: csv|tsv")
	fs.Parse(args)

	inD, ok := delims[*in]
	if !ok {
		fatal(fmt.Errorf("unknown input format %q; available: csv, tsv", *in))
	}
	outD, ok := delims[*out]
	if !ok {
		fatal(fmt.Errorf("unknown output format %q; available: csv, tsv", *out))
	}
	if fs.NArg() < 1 {
		fatal(fmt.Errorf(`no query given, e.g. "select name,age if age > 21"`))
	}
	if err := execute(os.Stdin, os.Stdout, strings.Join(fs.Args(), " "), inD, outD); err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "Error:", err)
	os.Exit(1)
}

// execute loads r into a table, runs query against it and writes the result to w.
func execute(r io.Reader, w io.Writer, query string, inD, outD rune) error {
	sqlText, err := prep.Rewrite(query)
	if err != nil {
		return fmt.Errorf("query: %w", err)
	}

	cr := csv.NewReader(r)
	cr.Comma = inD
	cr.FieldsPerRecord = -1
	cr.LazyQuotes = inD == '\t'
	rows, err := cr.ReadAll()
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil
	}
	header, data := rows[0], rows[1:]
	create, err := prep.CreateTable(header)
	if err != nil {
		return err
	}

	// Registered before the first connection opens, so the table can use it.
	if err := sqlite.RegisterCollationUtf8(prep.Collation, prep.Compare); err != nil {
		return err
	}
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		return err
	}
	defer db.Close()
	db.SetMaxOpenConns(1) // every :memory: connection is its own database

	if _, err := db.Exec(create); err != nil {
		return err
	}
	if err := load(db, len(header), data); err != nil {
		return err
	}

	rs, err := db.Query(sqlText)
	if err != nil {
		return fmt.Errorf("sql: %w", err)
	}
	defer rs.Close()
	cols, err := rs.Columns()
	if err != nil {
		return err
	}

	cw := csv.NewWriter(w)
	cw.Comma = outD
	if len(cols) > 0 {
		if err := cw.Write(cols); err != nil {
			return err
		}
	}
	vals := make([]any, len(cols))
	ptrs := make([]any, len(cols))
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	rec := make([]string, len(cols))
	for rs.Next() {
		if err := rs.Scan(ptrs...); err != nil {
			return err
		}
		for i, v := range vals {
			rec[i] = format(v)
		}
		if err := cw.Write(rec); err != nil {
			return err
		}
	}
	if err := rs.Err(); err != nil {
		return fmt.Errorf("sql: %w", err)
	}
	cw.Flush()
	return cw.Error()
}

// load inserts every record into the input table inside one transaction.
func load(db *sql.DB, n int, data [][]string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() // no-op once committed
	stmt, err := tx.Prepare(prep.InsertStmt(n))
	if err != nil {
		return err
	}
	defer stmt.Close()
	args := make([]any, n)
	for _, rec := range data {
		prep.Row(rec, args)
		if _, err := stmt.Exec(args...); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// format renders one result value as a cell. NULL is an empty cell.
func format(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case []byte:
		return string(x)
	case int64:
		return strconv.FormatInt(x, 10)
	case float64:
		return prep.FormatFloat(x)
	case bool:
		if x {
			return "1"
		}
		return "0"
	}
	return fmt.Sprint(v)
}
