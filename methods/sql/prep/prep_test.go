package prep

import (
	"reflect"
	"strings"
	"testing"
)

func TestRewrite(t *testing.T) {
	tests := []struct{ in, want string }{
		// the headline cases: implied table, `if` for `where`
		{"select * if name = 'Shanks'", "select * from t where name = 'Shanks'"},
		{"select name", "select name from t"},
		{"select name where age > 21", "select name from t where age > 21"},
		{"SELECT name IF age > 21 ORDER BY age DESC LIMIT 2", "SELECT name from t where age > 21 ORDER BY age DESC LIMIT 2"},
		{"select name order by age", "select name from t order by age"},
		{"select name group by city having count(*) > 1", "select name from t group by city having count(*) > 1"},
		{"select count(*)", "select count(*) from t"},
		{"select name limit 3", "select name from t limit 3"},
		{"select name if (age > 1 or city = 'Oslo')", "select name from t where (age > 1 or city = 'Oslo')"},

		// already-explicit queries are left alone, except `if` still means `where`
		{"select name from t where age > 1", "select name from t where age > 1"},
		{"select name from t if age > 1", "select name from t where age > 1"},
		{"select a from t group by a order by a", "select a from t group by a order by a"},

		// subqueries and compound selects are handled one by one
		{"select name if age > (select avg(age))", "select name from t where age > (select avg(age) from t)"},
		{"select a from (select a if a > 1) if a < 5", "select a from (select a from t where a > 1) where a < 5"},
		{"select a union select b if b > 1", "select a from t union select b from t where b > 1"},
		{"select a union all select b", "select a from t union all select b from t"},
		{"select a if a in (select b)", "select a from t where a in (select b from t)"},

		// keywords in strings, quoted identifiers, comments and qualified names are not keywords
		{"select 'if from where' as x if a = 1", "select 'if from where' as x from t where a = 1"},
		{`select "if", "from" if "if" = 1`, `select "if", "from" from t where "if" = 1`},
		{"select a -- if x\nif a = 1", "select a -- if x\nfrom t where a = 1"},
		{"select a /* from */ if a = 1", "select a /* from */ from t where a = 1"},
		{"select a -- trailing", "select a -- trailing\nfrom t"},
		{"select t.if if t.if = 1", "select t.if from t where t.if = 1"},
		{"select [if], `from` if [if] = 1", "select [if], `from` from t where [if] = 1"},
		{"select a if a = :if", "select a from t where a = :if"},
		{"select 'it''s' if a = 'x''if'", "select 'it''s' from t where a = 'x''if'"},

		// FROM that is not a FROM clause
		{"select a if a is distinct from b", "select a from t where a is distinct from b"},
		{"select a if a is not distinct from b", "select a from t where a is not distinct from b"},
		{"select substr(a from 1 for 2) if b = 1", "select substr(a from 1 for 2) from t where b = 1"},
		{"select group_concat(a) if b = 1", "select group_concat(a) from t where b = 1"},
		{"select iif(a > 1, 'y', 'n') if b = 1", "select iif(a > 1, 'y', 'n') from t where b = 1"},
		{"select row_number() over (order by a)", "select row_number() over (order by a) from t"},

		// statement ends and spacing
		{"select a;", "select a from t;"},
		{"select a if a = 1;", "select a from t where a = 1;"},
		{"select a  \n", "select a  \nfrom t"},
		{"  select\ta\tif\ta=1  ", "  select\ta\tfrom t where\ta=1  "},

		// not selects: nothing to do
		{"drop table if exists x", "drop table if exists x"},
	}
	for _, tc := range tests {
		got, err := Rewrite(tc.in)
		if err != nil {
			t.Errorf("Rewrite(%q): %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("Rewrite(%q)\n got %q\nwant %q", tc.in, got, tc.want)
		}
	}
}

func TestRewriteIdempotent(t *testing.T) {
	for _, q := range []string{
		"select * if name = 'Shanks'",
		"select a union select b order by 1",
		"select a if a > (select avg(a)) limit 2",
	} {
		once, err := Rewrite(q)
		if err != nil {
			t.Fatal(err)
		}
		twice, err := Rewrite(once)
		if err != nil {
			t.Fatal(err)
		}
		if once != twice {
			t.Errorf("not idempotent for %q:\n once %q\ntwice %q", q, once, twice)
		}
	}
}

func TestRewriteErrors(t *testing.T) {
	for _, q := range []string{
		"select 'oops", `select "oops`, "select `oops", "select [oops", "select /* oops",
		"select (a", "select a)",
	} {
		if _, err := Rewrite(q); err == nil {
			t.Errorf("Rewrite(%q): expected error", q)
		}
	}
}

func TestCompare(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want int
	}{
		{"9", "21", -1}, {"21", "9", 1}, {"3", "3", 0},
		{"1", "1.0", 0}, {" 5", "5", 0}, {"-2", "1", -1}, {"1e3", "999", 1}, {".5", "0.25", 1},
		{"a", "b", -1}, {"b", "a", 1}, {"Shanks", "Shanks", 0}, {"Shanks", "shanks", -1},
		{"10", "NA", -1}, {"NA", "10", 1}, // numbers sort before text
		{"Nan", "NaN", 1}, {"inf", "1", 1}, // not numbers
		{"0x10", "9", 1}, // not a plain decimal
	} {
		if got := Compare(c.a, c.b); got != c.want {
			t.Errorf("Compare(%q,%q)=%d want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestCompareIsTotalOrder(t *testing.T) {
	vals := []string{"2", "10", "1a", "NA", "9", "1.5", "-3", "b", "a", "1", "1.0", "Nan", "0x10"}
	for _, a := range vals {
		for _, b := range vals {
			if Compare(a, b) != -Compare(b, a) {
				t.Errorf("not antisymmetric: %q,%q", a, b)
			}
			for _, c := range vals {
				if Compare(a, b) <= 0 && Compare(b, c) <= 0 && Compare(a, c) > 0 {
					t.Errorf("not transitive: %q <= %q <= %q but %q > %q", a, b, c, a, c)
				}
			}
		}
	}
}

func TestCreateTable(t *testing.T) {
	got, err := CreateTable([]string{"name", "first name", `q"uote`})
	if err != nil {
		t.Fatal(err)
	}
	want := `CREATE TABLE "t" ("name" TEXT COLLATE yozora_num, "first name" TEXT COLLATE yozora_num, "q""uote" TEXT COLLATE yozora_num)`
	if got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
	for _, h := range [][]string{{"a", "A"}, {"x", "y", "x"}, {"", ""}} {
		_, err := CreateTable(h)
		if err == nil || !strings.Contains(err.Error(), "duplicate") {
			t.Errorf("CreateTable(%q): expected duplicate error, got %v", h, err)
		}
	}
	if _, err := CreateTable([]string{"é", "É"}); err != nil { // SQLite folds only ASCII
		t.Errorf("non-ASCII case variants should be distinct: %v", err)
	}
}

func TestInsertStmt(t *testing.T) {
	if got, want := InsertStmt(3), `INSERT INTO "t" VALUES (?,?,?)`; got != want {
		t.Errorf("got %s want %s", got, want)
	}
	if got, want := InsertStmt(1), `INSERT INTO "t" VALUES (?)`; got != want {
		t.Errorf("got %s want %s", got, want)
	}
}

func TestRow(t *testing.T) {
	dst := make([]any, 3)
	Row([]string{"Ann", "", "x"}, dst)
	if want := []any{"Ann", nil, "x"}; !reflect.DeepEqual(dst, want) {
		t.Errorf("empty cell: got %v want %v", dst, want)
	}
	Row([]string{"Bo"}, dst)
	if want := []any{"Bo", nil, nil}; !reflect.DeepEqual(dst, want) {
		t.Errorf("short record: got %v want %v", dst, want)
	}
	Row([]string{"a", "b", "c", "d"}, dst)
	if want := []any{"a", "b", "c"}; !reflect.DeepEqual(dst, want) {
		t.Errorf("long record: got %v want %v", dst, want)
	}
}

func TestFormatFloat(t *testing.T) {
	for _, c := range []struct {
		f    float64
		want string
	}{
		{3.5, "3.5"}, {30, "30"}, {0, "0"}, {-2.25, "-2.25"}, {12345678.5, "12345678.5"},
		{1.2e-05, "0.000012"}, {1e-7, "1e-07"}, {1e21, "1e+21"}, {0.30000000000000004, "0.30000000000000004"},
	} {
		if got := FormatFloat(c.f); got != c.want {
			t.Errorf("FormatFloat(%v)=%q want %q", c.f, got, c.want)
		}
	}
}
