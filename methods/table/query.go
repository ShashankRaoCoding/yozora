package table

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// Query is a parsed SQL-style query over a single, implicit table.
type Query struct {
	Cols    []string // nil means *
	Where   expr     // nil means every row
	OrderBy string
	Desc    bool
	Limit   int // -1 means no limit
}

// ---- lexer ----

type tokKind int

const (
	tEOF tokKind = iota
	tIdent
	tQIdent // "quoted identifier"
	tString // 'string literal'
	tNumber
	tOp
	tPunct // , ( ) *
)

type token struct {
	kind tokKind
	text string
}

func lex(s string) ([]token, error) {
	var toks []token
	rs := []rune(s)
	for i := 0; i < len(rs); {
		c := rs[i]
		switch {
		case unicode.IsSpace(c):
			i++
		case c == ',' || c == '(' || c == ')' || c == '*':
			toks = append(toks, token{tPunct, string(c)})
			i++
		case c == '\'' || c == '"':
			j := i + 1
			var sb strings.Builder
			for ; j < len(rs); j++ {
				if rs[j] == c {
					if j+1 < len(rs) && rs[j+1] == c { // doubled quote escapes itself
						sb.WriteRune(c)
						j++
						continue
					}
					break
				}
				sb.WriteRune(rs[j])
			}
			if j >= len(rs) {
				return nil, fmt.Errorf("unterminated quote starting at %q", string(rs[i:]))
			}
			k := tString
			if c == '"' {
				k = tQIdent
			}
			toks = append(toks, token{k, sb.String()})
			i = j + 1
		case strings.ContainsRune("=!<>", c):
			j := i + 1
			if j < len(rs) && (rs[j] == '=' || (c == '<' && rs[j] == '>')) {
				j++
			}
			op := string(rs[i:j])
			if op == "!" {
				return nil, fmt.Errorf("unexpected %q (did you mean !=?)", op)
			}
			toks = append(toks, token{tOp, op})
			i = j
		case unicode.IsDigit(c) || (c == '-' || c == '.') && i+1 < len(rs) && (unicode.IsDigit(rs[i+1]) || rs[i+1] == '.'):
			j := i + 1
			for j < len(rs) && (unicode.IsDigit(rs[j]) || rs[j] == '.' || rs[j] == 'e' || rs[j] == 'E') {
				j++
			}
			toks = append(toks, token{tNumber, string(rs[i:j])})
			i = j
		case unicode.IsLetter(c) || c == '_':
			j := i + 1
			for j < len(rs) && (unicode.IsLetter(rs[j]) || unicode.IsDigit(rs[j]) || rs[j] == '_' || rs[j] == '.') {
				j++
			}
			toks = append(toks, token{tIdent, string(rs[i:j])})
			i = j
		default:
			return nil, fmt.Errorf("unexpected character %q", string(c))
		}
	}
	return append(toks, token{kind: tEOF}), nil
}

// ---- parser ----

type parser struct {
	toks []token
	pos  int
}

func (p *parser) peek() token { return p.toks[p.pos] }
func (p *parser) next() token {
	t := p.toks[p.pos]
	if t.kind != tEOF {
		p.pos++
	}
	return t
}

// kw reports whether the next token is the keyword k (case-insensitive).
func (p *parser) kw(k string) bool {
	t := p.peek()
	return t.kind == tIdent && strings.EqualFold(t.text, k)
}

func (p *parser) eatKw(k string) bool {
	if p.kw(k) {
		p.pos++
		return true
	}
	return false
}

func (p *parser) punct(s string) bool {
	t := p.peek()
	return t.kind == tPunct && t.text == s
}

// Parse parses: select <cols|*> [where|if <cond>] [order by <col> [asc|desc]] [limit <n>]
func Parse(s string) (*Query, error) {
	toks, err := lex(s)
	if err != nil {
		return nil, err
	}
	p := &parser{toks: toks}
	q := &Query{Limit: -1}

	if !p.eatKw("select") {
		return nil, fmt.Errorf("query must start with select")
	}
	if p.punct("*") {
		p.next()
	} else {
		for {
			t := p.next()
			if t.kind != tIdent && t.kind != tQIdent {
				return nil, fmt.Errorf("expected column name in select list, got %q", t.text)
			}
			q.Cols = append(q.Cols, t.text)
			if !p.punct(",") {
				break
			}
			p.next()
		}
	}

	if p.eatKw("where") || p.eatKw("if") {
		if q.Where, err = p.parseOr(); err != nil {
			return nil, err
		}
	}
	if p.eatKw("order") {
		if !p.eatKw("by") {
			return nil, fmt.Errorf("expected by after order")
		}
		t := p.next()
		if t.kind != tIdent && t.kind != tQIdent {
			return nil, fmt.Errorf("expected column name after order by, got %q", t.text)
		}
		q.OrderBy = t.text
		if p.eatKw("desc") {
			q.Desc = true
		} else {
			p.eatKw("asc")
		}
	}
	if p.eatKw("limit") {
		t := p.next()
		n, err := strconv.Atoi(t.text)
		if t.kind != tNumber || err != nil || n < 0 {
			return nil, fmt.Errorf("limit needs a non-negative integer, got %q", t.text)
		}
		q.Limit = n
	}
	if t := p.peek(); t.kind != tEOF {
		return nil, fmt.Errorf("unexpected %q", t.text)
	}
	return q, nil
}

// ---- expressions ----

// row gives expressions access to column values by name.
type row interface {
	get(col string) (string, error)
}

type expr interface {
	eval(r row) (bool, error)
}

type orExpr struct{ l, r expr }
type andExpr struct{ l, r expr }
type notExpr struct{ e expr }
type cmpExpr struct {
	op   string
	l, r operand
}

type operand struct {
	col   string // set for column references
	lit   string // set for literals
	isCol bool
	isNum bool
}

func (o operand) value(r row) (string, error) {
	if o.isCol {
		return r.get(o.col)
	}
	return o.lit, nil
}

func (e orExpr) eval(r row) (bool, error) {
	l, err := e.l.eval(r)
	if err != nil || l {
		return l, err
	}
	return e.r.eval(r)
}

func (e andExpr) eval(r row) (bool, error) {
	l, err := e.l.eval(r)
	if err != nil || !l {
		return l, err
	}
	return e.r.eval(r)
}

func (e notExpr) eval(r row) (bool, error) {
	v, err := e.e.eval(r)
	return !v, err
}

func (e cmpExpr) eval(r row) (bool, error) {
	a, err := e.l.value(r)
	if err != nil {
		return false, err
	}
	b, err := e.r.value(r)
	if err != nil {
		return false, err
	}
	if e.op == "like" {
		return like(a, b), nil
	}
	c := compare(a, b)
	switch e.op {
	case "=", "==":
		return c == 0, nil
	case "!=", "<>":
		return c != 0, nil
	case "<":
		return c < 0, nil
	case "<=":
		return c <= 0, nil
	case ">":
		return c > 0, nil
	case ">=":
		return c >= 0, nil
	}
	return false, fmt.Errorf("unknown operator %q", e.op)
}

// compare orders a and b numerically when both parse as numbers, else as strings.
func compare(a, b string) int {
	fa, ea := strconv.ParseFloat(strings.TrimSpace(a), 64)
	fb, eb := strconv.ParseFloat(strings.TrimSpace(b), 64)
	if ea == nil && eb == nil {
		switch {
		case fa < fb:
			return -1
		case fa > fb:
			return 1
		}
		return 0
	}
	return strings.Compare(a, b)
}

// like implements SQL LIKE (case-insensitive): % matches any run, _ any one rune.
func like(s, pat string) bool {
	return likeRunes([]rune(strings.ToLower(s)), []rune(strings.ToLower(pat)))
}

func likeRunes(s, p []rune) bool {
	for len(p) > 0 {
		switch p[0] {
		case '%':
			for len(p) > 0 && p[0] == '%' {
				p = p[1:]
			}
			if len(p) == 0 {
				return true
			}
			for i := 0; i <= len(s); i++ {
				if likeRunes(s[i:], p) {
					return true
				}
			}
			return false
		case '_':
			if len(s) == 0 {
				return false
			}
		default:
			if len(s) == 0 || s[0] != p[0] {
				return false
			}
		}
		s, p = s[1:], p[1:]
	}
	return len(s) == 0
}

func (p *parser) parseOr() (expr, error) {
	l, err := p.parseAnd()
	if err != nil {
		return nil, err
	}
	for p.eatKw("or") {
		r, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		l = orExpr{l, r}
	}
	return l, nil
}

func (p *parser) parseAnd() (expr, error) {
	l, err := p.parseNot()
	if err != nil {
		return nil, err
	}
	for p.eatKw("and") {
		r, err := p.parseNot()
		if err != nil {
			return nil, err
		}
		l = andExpr{l, r}
	}
	return l, nil
}

func (p *parser) parseNot() (expr, error) {
	if p.eatKw("not") {
		e, err := p.parseNot()
		return notExpr{e}, err
	}
	if p.punct("(") {
		p.next()
		e, err := p.parseOr()
		if err != nil {
			return nil, err
		}
		if !p.punct(")") {
			return nil, fmt.Errorf("missing closing parenthesis")
		}
		p.next()
		return e, nil
	}
	return p.parseCmp()
}

func (p *parser) parseCmp() (expr, error) {
	l, err := p.parseOperand()
	if err != nil {
		return nil, err
	}
	t := p.next()
	var op string
	switch {
	case t.kind == tOp:
		op = t.text
	case t.kind == tIdent && strings.EqualFold(t.text, "like"):
		op = "like"
	default:
		return nil, fmt.Errorf("expected comparison operator, got %q", t.text)
	}
	r, err := p.parseOperand()
	if err != nil {
		return nil, err
	}
	return cmpExpr{op, l, r}, nil
}

func (p *parser) parseOperand() (operand, error) {
	t := p.next()
	switch t.kind {
	case tIdent, tQIdent:
		return operand{col: t.text, isCol: true}, nil
	case tString:
		return operand{lit: t.text}, nil
	case tNumber:
		return operand{lit: t.text, isNum: true}, nil
	}
	return operand{}, fmt.Errorf("expected column or value, got %q", t.text)
}
