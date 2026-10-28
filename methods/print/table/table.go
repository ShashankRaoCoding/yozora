// Package table renders CSV from stdin as an ASCII table.
package table

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/ShashankRaoCoding/yozora/methods/print"
)

func init() {
	print.Methods["table"] = table
}

func table(_ []string) {
	r := csv.NewReader(os.Stdin)
	r.FieldsPerRecord = -1
	rows, err := r.ReadAll()
	if err != nil && err != io.EOF {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
	if len(rows) == 0 {
		return
	}
	fmt.Print(render(rows))
}

func render(rows [][]string) string {
	cols := 0
	for _, row := range rows {
		if len(row) > cols {
			cols = len(row)
		}
	}
	widths := make([]int, cols)
	for _, row := range rows {
		for i, c := range row {
			if w := utf8.RuneCountInString(c); w > widths[i] {
				widths[i] = w
			}
		}
	}
	var sep strings.Builder
	sep.WriteByte('+')
	for _, w := range widths {
		sep.WriteString(strings.Repeat("-", w+2))
		sep.WriteByte('+')
	}
	sep.WriteByte('\n')

	var b strings.Builder
	b.WriteString(sep.String())
	for ri, row := range rows {
		b.WriteByte('|')
		for i := 0; i < cols; i++ {
			c := ""
			if i < len(row) {
				c = row[i]
			}
			b.WriteString(" " + c + strings.Repeat(" ", widths[i]-utf8.RuneCountInString(c)) + " |")
		}
		b.WriteByte('\n')
		if ri == 0 {
			b.WriteString(sep.String())
		}
	}
	b.WriteString(sep.String())
	return b.String()
}
