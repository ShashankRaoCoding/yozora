// Package table implements `yozora table -i <fmt> -o <fmt> "<query>"`,
// running a SQL-style query over CSV/TSV from stdin and writing CSV/TSV to stdout.
//
// Queries look like:
//
//	select name,age
//	select * where age > 21 and name like 'A%'
//	select name if age > 21 order by age desc limit 10
//
// The first input row is the header; column names are matched exactly, then
// case-insensitively. Use "double quotes" for column names containing spaces
// and 'single quotes' for string values.
package table

import (
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/ShashankRaoCoding/yozora/methods"
)

func init() {
	methods.Methods["table"] = table
}

var delims = map[string]rune{"csv": ',', "tsv": '\t'}

func table(args []string) {
	fs := flag.NewFlagSet("table", flag.ExitOnError)
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
	q, err := Parse(strings.Join(fs.Args(), " "))
	if err != nil {
		fatal(fmt.Errorf("query: %w", err))
	}

	r := csv.NewReader(os.Stdin)
	r.Comma = inD
	r.FieldsPerRecord = -1
	r.LazyQuotes = inD == '\t'
	rows, err := r.ReadAll()
	if err != nil && err != io.EOF {
		fatal(err)
	}

	res, err := q.Run(rows)
	if err != nil {
		fatal(err)
	}
	w := csv.NewWriter(os.Stdout)
	w.Comma = outD
	if err := w.WriteAll(res); err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "Error:", err)
	os.Exit(1)
}

// tableRow adapts one record to the row interface used by expressions.
type tableRow struct {
	idx map[string]int
	rec []string
}

func (t tableRow) get(col string) (string, error) {
	i, ok := lookup(t.idx, col)
	if !ok {
		return "", fmt.Errorf("unknown column %q", col)
	}
	if i >= len(t.rec) {
		return "", nil // ragged row: missing trailing fields read as empty
	}
	return t.rec[i], nil
}

// lookup finds col in idx, exactly first and then case-insensitively.
func lookup(idx map[string]int, col string) (int, bool) {
	if i, ok := idx[col]; ok {
		return i, true
	}
	for name, i := range idx {
		if strings.EqualFold(name, col) {
			return i, true
		}
	}
	return 0, false
}

// Run applies the query to rows (header first) and returns the result, header first.
func (q *Query) Run(rows [][]string) ([][]string, error) {
	if len(rows) == 0 {
		return nil, nil
	}
	header, data := rows[0], rows[1:]
	idx := make(map[string]int, len(header))
	for i, h := range header {
		if _, dup := idx[h]; !dup {
			idx[h] = i
		}
	}

	var outIdx []int
	var outHeader []string
	if q.Cols == nil {
		for i, h := range header {
			outIdx = append(outIdx, i)
			outHeader = append(outHeader, h)
		}
	} else {
		for _, c := range q.Cols {
			i, ok := lookup(idx, c)
			if !ok {
				return nil, fmt.Errorf("unknown column %q; available: %v", c, header)
			}
			outIdx = append(outIdx, i)
			outHeader = append(outHeader, header[i])
		}
	}
	orderIdx := -1
	if q.OrderBy != "" {
		i, ok := lookup(idx, q.OrderBy)
		if !ok {
			return nil, fmt.Errorf("unknown column %q in order by; available: %v", q.OrderBy, header)
		}
		orderIdx = i
	}

	var kept [][]string
	for _, rec := range data {
		if q.Where != nil {
			ok, err := q.Where.eval(tableRow{idx, rec})
			if err != nil {
				return nil, fmt.Errorf("%w (available: %v)", err, header)
			}
			if !ok {
				continue
			}
		}
		kept = append(kept, rec)
	}

	if orderIdx >= 0 {
		field := func(rec []string) string {
			if orderIdx < len(rec) {
				return rec[orderIdx]
			}
			return ""
		}
		sort.SliceStable(kept, func(a, b int) bool {
			c := compare(field(kept[a]), field(kept[b]))
			if q.Desc {
				return c > 0
			}
			return c < 0
		})
	}
	if q.Limit >= 0 && len(kept) > q.Limit {
		kept = kept[:q.Limit]
	}

	res := [][]string{outHeader}
	for _, rec := range kept {
		o := make([]string, len(outIdx))
		for k, i := range outIdx {
			if i < len(rec) {
				o[k] = rec[i]
			}
		}
		res = append(res, o)
	}
	return res, nil
}
