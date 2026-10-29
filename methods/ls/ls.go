// Package ls implements `yozora ls [-a] [-d <depth>] [-D] [path]`, printing
// a directory as a file tree instead of a flat listing:
//
//	.
//	├── main.go
//	└── methods/
//	    └── methods.go
//
//	1 directory, 2 files
//
// Entries are sorted by name. Directories get a trailing "/" and symlinks are
// shown as "name -> target" and never followed, so link cycles can't loop.
package ls

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ShashankRaoCoding/yozora/methods"
)

func init() {
	methods.Methods["ls"] = ls
}

// Options controls which entries Tree includes.
type Options struct {
	All      bool // include dotfiles
	MaxDepth int  // levels below root to show; 0 means unlimited
	DirsOnly bool // omit files
}

func ls(args []string) {
	fs := flag.NewFlagSet("ls", flag.ExitOnError)
	all := fs.Bool("a", false, "include hidden (dot) files")
	depth := fs.Int("d", 0, "max depth below the root (0 = unlimited)")
	dirs := fs.Bool("D", false, "directories only")
	fs.Parse(args)

	root := "."
	if fs.NArg() > 0 {
		root = fs.Arg(0)
	}
	out, err := Tree(root, Options{All: *all, MaxDepth: *depth, DirsOnly: *dirs})
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
	fmt.Print(out)
}

// Tree renders root as a tree followed by a "N directories, M files" summary.
func Tree(root string, opt Options) (string, error) {
	info, err := os.Stat(root)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s: not a directory", root)
	}
	if opt.MaxDepth < 0 {
		return "", fmt.Errorf("depth must be non-negative, got %d", opt.MaxDepth)
	}

	var b strings.Builder
	b.WriteString(root + "\n")
	var nDirs, nFiles int
	walk(&b, root, "", 1, opt, &nDirs, &nFiles)

	b.WriteString("\n" + plural(nDirs, "directory", "directories"))
	if !opt.DirsOnly {
		b.WriteString(", " + plural(nFiles, "file", "files"))
	}
	b.WriteString("\n")
	return b.String(), nil
}

// walk writes the children of dir, indented by prefix. level is the depth of
// those children below the root (1 for the root's direct children).
func walk(b *strings.Builder, dir, prefix string, level int, opt Options, nDirs, nFiles *int) {
	entries, err := os.ReadDir(dir) // sorted by filename
	if err != nil {
		b.WriteString(prefix + "└── [error: " + reason(err) + "]\n")
		return
	}

	var kept []os.DirEntry
	for _, e := range entries {
		if !opt.All && strings.HasPrefix(e.Name(), ".") {
			continue
		}
		if opt.DirsOnly && !e.IsDir() {
			continue
		}
		kept = append(kept, e)
	}

	for i, e := range kept {
		last := i == len(kept)-1
		branch, next := "├── ", "│   "
		if last {
			branch, next = "└── ", "    "
		}
		b.WriteString(prefix + branch + label(dir, e) + "\n")

		if e.IsDir() { // IsDir is false for symlinks, so links are never followed
			*nDirs++
			if opt.MaxDepth == 0 || level < opt.MaxDepth {
				walk(b, filepath.Join(dir, e.Name()), prefix+next, level+1, opt, nDirs, nFiles)
			}
		} else {
			*nFiles++
		}
	}
}

// label returns the display name: "dir/", "link -> target" or "file".
func label(dir string, e os.DirEntry) string {
	switch {
	case e.IsDir():
		return e.Name() + "/"
	case e.Type()&os.ModeSymlink != 0:
		if target, err := os.Readlink(filepath.Join(dir, e.Name())); err == nil {
			return e.Name() + " -> " + target
		}
	}
	return e.Name()
}

// reason shortens an *PathError to just its cause, e.g. "permission denied".
func reason(err error) string {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err.Error()
	}
	return err.Error()
}

func plural(n int, one, many string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, one)
	}
	return fmt.Sprintf("%d %s", n, many)
}
