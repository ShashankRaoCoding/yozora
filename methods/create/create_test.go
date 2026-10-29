package create

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreate(t *testing.T) {
	root := t.TempDir()
	tests := []struct {
		name  string
		path  string
		lines int
		want  string
	}{
		{"empty file", "a.txt", 0, ""},
		{"parents created", "x/y/z/b.txt", 0, ""},
		{"n lines", "c.txt", 3, "\n\n\n"},
		{"n lines with parents", "p/q/d.txt", 2, "\n\n"},
	}
	for _, tc := range tests {
		p := filepath.Join(root, tc.path)
		created, err := Create(p, tc.lines)
		if err != nil || !created {
			t.Fatalf("%s: created=%v err=%v", tc.name, created, err)
		}
		got, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if string(got) != tc.want {
			t.Errorf("%s: got %q want %q", tc.name, got, tc.want)
		}
	}
}

func TestCreateExistingUntouched(t *testing.T) {
	p := filepath.Join(t.TempDir(), "keep.txt")
	if err := os.WriteFile(p, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, n := range []int{0, 5} {
		created, err := Create(p, n)
		if created || !errors.Is(err, fs.ErrExist) {
			t.Errorf("n=%d: want created=false and ErrExist, got created=%v err=%v", n, created, err)
		}
	}
	if got, _ := os.ReadFile(p); string(got) != "data" {
		t.Errorf("existing file modified: %q", got)
	}
}

func TestCreateErrors(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "f")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	for name, tc := range map[string]struct {
		path  string
		lines int
		want  string
	}{
		"negative n":     {filepath.Join(root, "n"), -1, "non-negative"},
		"empty path":     {"", 0, "not a file path"},
		"trailing slash": {filepath.Join(root, "d") + "/", 0, "not a file path"},
		"parent is file": {filepath.Join(file, "child"), 0, ""},
	} {
		if _, err := Create(tc.path, tc.lines); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: expected error containing %q, got %v", name, tc.want, err)
		}
	}
	if _, err := Create(root, 0); !errors.Is(err, fs.ErrExist) {
		t.Errorf("existing directory: want ErrExist, got %v", err)
	}
}
