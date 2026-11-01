package prep

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Collation is the name of the collating sequence every input column uses.
// The command registers Compare under this name.
const Collation = "yozora_num"

// Compare orders two cell values the way people expect from a spreadsheet:
// values that look like numbers compare numerically ("9" < "21"), numbers sort
// before anything else, and everything else compares as plain strings. It is a
// total order, as SQLite requires of a collation.
//
// Columns are stored as text with this collation, so comparisons, ORDER BY,
// min/max and DISTINCT all treat numeric-looking values numerically while the
// stored text, and therefore the output, stays exactly as it was in the input.
func Compare(a, b string) int {
	fa, oka := parseNum(a)
	fb, okb := parseNum(b)
	switch {
	case oka && okb:
		switch {
		case fa < fb:
			return -1
		case fa > fb:
			return 1
		}
		return 0
	case oka:
		return -1
	case okb:
		return 1
	}
	return strings.Compare(a, b)
}

// parseNum reports whether s looks like a plain decimal number. Words that
// strconv would accept, such as "nan" or "inf", are not numbers here.
func parseNum(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	switch c := s[0]; {
	case c >= '0' && c <= '9', c == '-', c == '+', c == '.':
	default:
		return 0, false
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
		return 0, false
	}
	return f, true
}

// QuoteIdent quotes s as an SQL identifier.
func QuoteIdent(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

// CreateTable returns the statement that creates the input table for header.
// Every column is TEXT with the Collation. SQL column names are
// case-insensitive, so headers that differ only by ASCII case are an error.
func CreateTable(header []string) (string, error) {
	seen := make(map[string]string, len(header))
	cols := make([]string, len(header))
	for i, h := range header {
		key := asciiLower(h)
		if prev, dup := seen[key]; dup {
			return "", fmt.Errorf("duplicate column name %q (same as %q; SQL column names are case-insensitive)", h, prev)
		}
		seen[key] = h
		cols[i] = QuoteIdent(h) + " TEXT COLLATE " + Collation
	}
	return "CREATE TABLE " + QuoteIdent(TableName) + " (" + strings.Join(cols, ", ") + ")", nil
}

// InsertStmt returns the statement that inserts one row of n values.
func InsertStmt(n int) string {
	return "INSERT INTO " + QuoteIdent(TableName) + " VALUES (" + strings.TrimSuffix(strings.Repeat("?,", n), ",") + ")"
}

// Row fills dst (length n) from one input record. Empty cells become NULL;
// missing trailing cells read as empty and surplus cells are ignored, so
// ragged records load without error.
func Row(rec []string, dst []any) {
	for i := range dst {
		if i < len(rec) && rec[i] != "" {
			dst[i] = rec[i]
		} else {
			dst[i] = nil
		}
	}
}

// FormatFloat renders a computed REAL (avg, division, ...) without an exponent
// for everyday magnitudes.
func FormatFloat(f float64) string {
	switch {
	case math.IsNaN(f):
		return "NaN"
	case math.IsInf(f, 1):
		return "Inf"
	case math.IsInf(f, -1):
		return "-Inf"
	}
	if a := math.Abs(f); a != 0 && (a < 1e-6 || a >= 1e21) {
		return strconv.FormatFloat(f, 'g', -1, 64)
	}
	return strconv.FormatFloat(f, 'f', -1, 64)
}

func asciiLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 'a' - 'A'
		}
	}
	return string(b)
}
