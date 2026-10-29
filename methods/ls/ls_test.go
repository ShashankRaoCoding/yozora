package ls

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fixture builds:
//
//	root/
//	  .hidden
//	  a.txt
//	  sub/
//	    b.txt
//	    deep/
//	      c.txt
//	  empty/
//	  link -> a.txt
func fixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, d := range []string{"sub/deep", "empty"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range []string{".hidden", "a.txt", "sub/b.txt", "sub/deep/c.txt"} {
		if err := os.WriteFile(filepath.Join(root, f), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink("a.txt", filepath.Join(root, "link")); err != nil {
		t.Skip("symlinks unsupported:", err)
	}
	return root
}

func TestTree(t *testing.T) {
	root := fixture(t)
	tests := []struct {
		name string
		opt  Options
		want string // body after the root line
	}{
		{"default", Options{}, `├── a.txt
├── empty/
├── link -> a.txt
└── sub/
    ├── b.txt
    └── deep/
        └── c.txt

3 directories, 4 files
`},
		{"all", Options{All: true}, `├── .hidden
├── a.txt
├── empty/
├── link -> a.txt
└── sub/
    ├── b.txt
    └── deep/
        └── c.txt

3 directories, 5 files
`},
		{"depth 1", Options{MaxDepth: 1}, `├── a.txt
├── empty/
├── link -> a.txt
└── sub/

2 directories, 2 files
`},
		{"dirs only", Options{DirsOnly: true}, `├── empty/
└── sub/
    └── deep/

3 directories
`},
	}
	for _, tc := range tests {
		got, err := Tree(root, tc.opt)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if want := root + "\n" + tc.want; got != want {
			t.Errorf("%s:\n got:\n%s\nwant:\n%s", tc.name, got, want)
		}
	}
}

func TestTreeEmptyDir(t *testing.T) {
	root := t.TempDir()
	got, err := Tree(root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if want := root + "\n\n0 directories, 0 files\n"; got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestTreeSingular(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "d"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "f"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Tree(root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(got, "1 directory, 1 file\n") {
		t.Errorf("expected singular summary, got %q", got)
	}
}

func TestTreeErrors(t *testing.T) {
	root := fixture(t)
	for name, tc := range map[string]struct {
		path string
		opt  Options
		want string
	}{
		"missing":        {filepath.Join(root, "nope"), Options{}, "nope"},
		"file not dir":   {filepath.Join(root, "a.txt"), Options{}, "not a directory"},
		"negative depth": {root, Options{MaxDepth: -1}, "depth"},
	} {
		if _, err := Tree(tc.path, tc.opt); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: expected error containing %q, got %v", name, tc.want, err)
		}
	}
}

// A symlink to a parent directory must not be followed (no infinite loop).
func TestTreeSymlinkCycle(t *testing.T) {
	root := t.TempDir()
	if err := os.Symlink(".", filepath.Join(root, "loop")); err != nil {
		t.Skip("symlinks unsupported:", err)
	}
	got, err := Tree(root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if want := root + "\n└── loop -> .\n\n0 directories, 1 file\n"; got != want {
		t.Errorf("got %q want %q", got, want)
	}
}
