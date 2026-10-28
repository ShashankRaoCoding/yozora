package table

import (
	"reflect"
	"strings"
	"testing"
)

var data = [][]string{
	{"name", "age", "city"},
	{"Alice", "30", "London"},
	{"Bob", "18", "Leeds"},
	{"Carol", "25", "Lisbon"},
	{"Dave", "21", "Oslo"},
	{"Eve", "9", "London"},
}

func run(t *testing.T, query string) [][]string {
	t.Helper()
	q, err := Parse(query)
	if err != nil {
		t.Fatalf("parse %q: %v", query, err)
	}
	res, err := q.Run(data)
	if err != nil {
		t.Fatalf("run %q: %v", query, err)
	}
	return res
}

func TestRun(t *testing.T) {
	tests := []struct {
		query string
		want  [][]string
	}{
		{"select name,age", [][]string{{"name", "age"}, {"Alice", "30"}, {"Bob", "18"}, {"Carol", "25"}, {"Dave", "21"}, {"Eve", "9"}}},
		{"SELECT name IF age >21", [][]string{{"name"}, {"Alice"}, {"Carol"}}}, // numeric, not lexical: 9 < 21
		{"select name where age >= 21 and city = 'London'", [][]string{{"name"}, {"Alice"}}},
		{"select name where age < 20 or city like 'os%'", [][]string{{"name"}, {"Bob"}, {"Dave"}, {"Eve"}}},
		{"select name where not (age > 20)", [][]string{{"name"}, {"Bob"}, {"Eve"}}},
		{"select name where city != 'London' order by age desc limit 2", [][]string{{"name"}, {"Carol"}, {"Dave"}}},
		{"select name order by age", [][]string{{"name"}, {"Eve"}, {"Bob"}, {"Dave"}, {"Carol"}, {"Alice"}}},
		{"select NAME where AGE > 100", [][]string{{"name"}}},
		{"select * where name = 'Eve'", [][]string{{"name", "age", "city"}, {"Eve", "9", "London"}}},
	}
	for _, tc := range tests {
		if got := run(t, tc.query); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%q:\n got %v\nwant %v", tc.query, got, tc.want)
		}
	}
}

func TestParseErrors(t *testing.T) {
	for _, q := range []string{
		"", "name,age", "select", "select name where", "select name where age >",
		"select name where (age > 1", "select name limit -1", "select name order age",
		"select name where age ! 3", "select name where x = 'oops", "select name junk",
	} {
		if _, err := Parse(q); err == nil {
			t.Errorf("Parse(%q): expected error", q)
		}
	}
}

func TestUnknownColumn(t *testing.T) {
	for _, query := range []string{"select nope", "select name where nope = 1", "select name order by nope"} {
		q, err := Parse(query)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := q.Run(data); err == nil || !strings.Contains(err.Error(), "nope") {
			t.Errorf("%q: expected unknown column error, got %v", query, err)
		}
	}
}

func TestQuotedIdentAndRaggedRows(t *testing.T) {
	rows := [][]string{{"first name", "n"}, {"Ann", "1"}, {"Bo"}}
	q, err := Parse(`select "first name" where n = ''`)
	if err != nil {
		t.Fatal(err)
	}
	res, err := q.Run(rows)
	if err != nil {
		t.Fatal(err)
	}
	if want := [][]string{{"first name"}, {"Bo"}}; !reflect.DeepEqual(res, want) {
		t.Errorf("got %v want %v", res, want)
	}
}

func TestLike(t *testing.T) {
	for _, c := range []struct {
		s, p string
		want bool
	}{
		{"Alice", "a%", true}, {"Alice", "%ice", true}, {"Alice", "A_ice", true},
		{"Alice", "a%e%", true}, {"Alice", "b%", false}, {"", "%", true}, {"a", "", false},
	} {
		if got := like(c.s, c.p); got != c.want {
			t.Errorf("like(%q,%q)=%v want %v", c.s, c.p, got, c.want)
		}
	}
}
