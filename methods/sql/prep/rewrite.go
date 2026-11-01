// Package prep turns yozora's simplified SQL into plain SQLite SQL and prepares
// CSV/TSV records for loading into an in-memory table. It is pure Go, with no
// database dependency, so it can be tested on its own.
//
// Two conveniences sit on top of ordinary SQL:
//
//   - the table name is implied: a select with no FROM reads from the input
//     (TableName), so `select name` means `select name from t`;
//   - `if` is accepted as a synonym for `where`.
//
// Everything else is passed through untouched.
package prep

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// TableName is the name of the implicit table the input is loaded into.
const TableName = "t"

type kind int

const (
	other  kind = iota // strings, quoted identifiers, numbers, operators, parameters
	word               // bare identifier or keyword; text is lower-cased
	lparen             // (
	rparen             // )
	semi               // ;
)

type token struct {
	kind       kind
	start, end int    // byte offsets into the query
	text       string // lower-cased; words only
}

func isWordStart(r rune) bool { return r == '_' || unicode.IsLetter(r) }
func isWordRune(r rune) bool  { return isWordStart(r) || unicode.IsDigit(r) || r == '$' }

// lex splits s into tokens, skipping whitespace and comments. Anything inside
// quotes or brackets becomes a single opaque token, so keywords inside strings
// and quoted identifiers are never rewritten. openComment reports that s ends
// inside a -- comment with no trailing newline.
func lex(s string) (toks []token, openComment bool, err error) {
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		switch {
		case unicode.IsSpace(r):
			i += size
		case strings.HasPrefix(s[i:], "--"):
			j := strings.IndexByte(s[i:], '\n')
			if j < 0 {
				return toks, true, nil
			}
			i += j + 1
		case strings.HasPrefix(s[i:], "/*"):
			j := strings.Index(s[i+2:], "*/")
			if j < 0 {
				return nil, false, fmt.Errorf("unterminated /* comment")
			}
			i += 2 + j + 2
		case r == '\'' || r == '"' || r == '`':
			end, err := skipQuoted(s, i, byte(r))
			if err != nil {
				return nil, false, err
			}
			toks = append(toks, token{kind: other, start: i, end: end})
			i = end
		case r == '[':
			j := strings.IndexByte(s[i:], ']')
			if j < 0 {
				return nil, false, fmt.Errorf("unterminated [ identifier")
			}
			toks = append(toks, token{kind: other, start: i, end: i + j + 1})
			i += j + 1
		case r == '(':
			toks = append(toks, token{kind: lparen, start: i, end: i + 1})
			i++
		case r == ')':
			toks = append(toks, token{kind: rparen, start: i, end: i + 1})
			i++
		case r == ';':
			toks = append(toks, token{kind: semi, start: i, end: i + 1})
			i++
		case r == ':' || r == '@' || r == '$' || r == '?': // bind parameter such as :if
			j := i + size
			for j < len(s) {
				r2, s2 := utf8.DecodeRuneInString(s[j:])
				if !isWordRune(r2) {
					break
				}
				j += s2
			}
			toks = append(toks, token{kind: other, start: i, end: j})
			i = j
		case unicode.IsDigit(r):
			j := i + size
			for j < len(s) {
				r2, s2 := utf8.DecodeRuneInString(s[j:])
				if !isWordRune(r2) && r2 != '.' {
					break
				}
				j += s2
			}
			toks = append(toks, token{kind: other, start: i, end: j})
			i = j
		case isWordStart(r):
			j := i + size
			for j < len(s) {
				r2, s2 := utf8.DecodeRuneInString(s[j:])
				if !isWordRune(r2) {
					break
				}
				j += s2
			}
			k := word
			if i > 0 && s[i-1] == '.' { // qualified name such as t.from: never a keyword
				k = other
			}
			toks = append(toks, token{kind: k, start: i, end: j, text: strings.ToLower(s[i:j])})
			i = j
		default:
			toks = append(toks, token{kind: other, start: i, end: i + size})
			i += size
		}
	}
	return toks, false, nil
}

// skipQuoted returns the offset just past the quote that closes the one at s[i].
// A doubled quote character inside escapes itself.
func skipQuoted(s string, i int, q byte) (int, error) {
	for j := i + 1; j < len(s); j++ {
		if s[j] != q {
			continue
		}
		if j+1 < len(s) && s[j+1] == q {
			j++
			continue
		}
		return j + 1, nil
	}
	rest := s[i:]
	if len(rest) > 24 {
		rest = rest[:24] + "..."
	}
	return 0, fmt.Errorf("unterminated %c quote starting at %s", q, rest)
}

type edit struct {
	pos, del int
	text     string
}

// scope tracks one parenthesis level while scanning.
type scope struct {
	sel   bool // a SELECT is open at this level
	from  bool // ... and it has a FROM (or one has been supplied)
	where bool // ... and it has a WHERE (or an `if` standing for one)
}

// Rewrite converts a simplified query into plain SQL: bare `if` becomes `where`,
// and every select without a FROM gains `from t`. Subqueries and compound
// selects (union, intersect, except) are handled independently, and text inside
// strings, quoted identifiers and comments is never touched.
func Rewrite(query string) (string, error) {
	toks, openComment, err := lex(query)
	if err != nil {
		return "", err
	}
	if openComment {
		query += "\n" // keep anything we append out of the trailing comment
	}

	stack := []scope{{}}
	var edits []edit
	lastWord := ""

	// pre is a space when the character before at is not whitespace.
	pre := func(at int) string {
		if at > 0 && !unicode.IsSpace(rune(query[at-1])) {
			return " "
		}
		return ""
	}
	// finish closes a select at offset at, supplying its FROM if it has none.
	finish := func(sc *scope, at int, followed bool) {
		if sc.sel && !sc.from {
			text := pre(at) + "from " + TableName
			if followed {
				text += " "
			}
			edits = append(edits, edit{at, 0, text})
		}
		*sc = scope{}
	}
	// clause supplies the FROM just before a clause keyword that needs one.
	clause := func(sc *scope, at int) {
		if sc.sel && !sc.from {
			edits = append(edits, edit{at, 0, pre(at) + "from " + TableName + " "})
			sc.from = true
		}
	}

	for _, t := range toks {
		sc := &stack[len(stack)-1]
		if t.kind != word {
			lastWord = ""
		}
		switch t.kind {
		case lparen:
			stack = append(stack, scope{})
		case rparen:
			if len(stack) == 1 {
				return "", fmt.Errorf("unexpected )")
			}
			finish(sc, t.start, false)
			stack = stack[:len(stack)-1]
		case semi:
			finish(sc, t.start, false)
		case word:
			switch t.text {
			case "select":
				*sc = scope{sel: true}
			case "from":
				// "is [not] distinct from" is an operator, not a FROM clause.
				if sc.sel && lastWord != "distinct" {
					sc.from = true
				}
			case "where":
				clause(sc, t.start)
				if sc.sel {
					sc.where = true
				}
			case "if":
				if sc.sel && !sc.where {
					text := "where"
					if !sc.from {
						text = pre(t.start) + "from " + TableName + " where"
						sc.from = true
					}
					edits = append(edits, edit{t.start, t.end - t.start, text})
					sc.where = true
				}
			case "group", "having", "order", "limit":
				clause(sc, t.start)
			case "union", "intersect", "except":
				finish(sc, t.start, true)
			}
			lastWord = t.text
		}
	}
	if len(stack) != 1 {
		return "", fmt.Errorf("missing closing parenthesis")
	}
	finish(&stack[0], len(query), false)

	var sb strings.Builder
	prev := 0
	for _, e := range edits {
		sb.WriteString(query[prev:e.pos])
		sb.WriteString(e.text)
		prev = e.pos + e.del
	}
	sb.WriteString(query[prev:])
	return sb.String(), nil
}
