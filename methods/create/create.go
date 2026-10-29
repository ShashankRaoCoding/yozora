// Package create implements `yozora create [-n lines] <path>`: it makes any
// missing parent directories and then the file itself.
//
//	yozora create a/b/c.txt        # a/b/ and an empty c.txt
//	yozora create -n 3 a/b/c.txt   # c.txt holding 3 empty lines
//
// An existing file is never modified. Without -n that is a no-op; with -n it
// is an error, because the file can't be created with the requested lines.
package create

import (
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/ShashankRaoCoding/yozora/methods"
)

func init() {
	methods.Methods["create"] = create
}

func create(args []string) {
	fl := flag.NewFlagSet("create", flag.ExitOnError)
	n := fl.Int("n", 0, "number of (empty) lines to write into the new file")
	fl.Parse(args)

	if fl.NArg() != 1 {
		fatal(fmt.Errorf("usage: yozora create [-n lines] <path>"))
	}
	path := fl.Arg(0)

	created, err := Create(path, *n)
	switch {
	case errors.Is(err, fs.ErrExist) && *n == 0:
		fmt.Println("exists:", path) // plain create on an existing file is a no-op
	case err != nil:
		fatal(err)
	case created:
		fmt.Println("created:", path)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "Error:", err)
	os.Exit(1)
}

// Create makes path's parent directories, then path itself holding lines
// newline-terminated empty lines. It never touches an existing file: that
// returns an error wrapping fs.ErrExist.
func Create(path string, lines int) (created bool, err error) {
	if lines < 0 {
		return false, fmt.Errorf("-n must be non-negative, got %d", lines)
	}
	if path == "" || strings.HasSuffix(path, "/") || strings.HasSuffix(path, string(filepath.Separator)) {
		return false, fmt.Errorf("%q is not a file path", path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, err
	}
	// O_EXCL makes "create if not exists" atomic: no check-then-create race.
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return false, err
	}
	if _, err := f.WriteString(strings.Repeat("\n", lines)); err != nil {
		f.Close()
		return false, err
	}
	return true, f.Close()
}
