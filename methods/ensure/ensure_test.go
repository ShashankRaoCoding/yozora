package ensure

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func opts(t *testing.T) Options {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("ensure is unsupported on Windows")
	}
	return Options{Dir: "/opt/yozora/bin", Home: t.TempDir(), Shell: "/bin/bash", Path: "/usr/bin:/bin"}
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestAddsOnceThenIsIdempotent(t *testing.T) {
	o := opts(t)
	res, err := Ensure(o)
	if err != nil || res.Status != Added {
		t.Fatalf("first run: %+v %v", res, err)
	}
	if want := filepath.Join(o.Home, ".bashrc"); res.File != want && runtime.GOOS != "darwin" {
		t.Errorf("file = %s, want %s", res.File, want)
	}
	want := "\n" + marker + "\nexport PATH=\"$PATH:/opt/yozora/bin\"\n"
	if got := read(t, res.File); got != want {
		t.Errorf("file contents:\n got %q\nwant %q", got, want)
	}

	res, err = Ensure(o)
	if err != nil || res.Status != AlreadyConfigured {
		t.Fatalf("second run: %+v %v", res, err)
	}
	if got := read(t, res.File); got != want {
		t.Errorf("second run changed the file: %q", got)
	}
}

func TestAlreadyInPathTouchesNothing(t *testing.T) {
	o := opts(t)
	o.Path = "/usr/bin:/opt/yozora/bin/:/bin" // trailing slash must still match
	res, err := Ensure(o)
	if err != nil || res.Status != AlreadyInPath {
		t.Fatalf("%+v %v", res, err)
	}
	if entries, _ := os.ReadDir(o.Home); len(entries) != 0 {
		t.Errorf("home was modified: %v", entries)
	}
}

func TestForceEditsEvenWhenOnPath(t *testing.T) {
	o := opts(t)
	o.Path += ":" + o.Dir
	o.Force = true
	if res, err := Ensure(o); err != nil || res.Status != Added {
		t.Fatalf("%+v %v", res, err)
	}
}

func TestShells(t *testing.T) {
	for _, tc := range []struct {
		shell, file, line string
		prepend           bool
	}{
		{"zsh", ".zshrc", `export PATH="$PATH:/opt/yozora/bin"`, false},
		{"/usr/bin/zsh", ".zshrc", `export PATH="/opt/yozora/bin:$PATH"`, true},
		{"fish", ".config/fish/config.fish", `fish_add_path -g -a "/opt/yozora/bin"`, false},
		{"fish", ".config/fish/config.fish", `fish_add_path -g "/opt/yozora/bin"`, true},
		{"dash", ".profile", `export PATH="$PATH:/opt/yozora/bin"`, false},
	} {
		o := opts(t)
		o.Shell, o.Prepend = tc.shell, tc.prepend
		res, err := Ensure(o)
		if err != nil || res.Status != Added {
			t.Fatalf("%s: %+v %v", tc.shell, res, err)
		}
		if want := filepath.Join(o.Home, filepath.FromSlash(tc.file)); res.File != want {
			t.Errorf("%s: file %s, want %s", tc.shell, res.File, want)
		}
		if got := read(t, res.File); !strings.Contains(got, tc.line+"\n") {
			t.Errorf("%s: missing %q in %q", tc.shell, tc.line, got)
		}
	}
}

func TestPreservesExistingContent(t *testing.T) {
	o := opts(t)
	rc := filepath.Join(o.Home, ".bashrc")
	if runtime.GOOS == "darwin" {
		rc = filepath.Join(o.Home, ".bash_profile")
	}
	if err := os.WriteFile(rc, []byte("alias a=b"), 0o644); err != nil { // no trailing newline
		t.Fatal(err)
	}
	if _, err := Ensure(o); err != nil {
		t.Fatal(err)
	}
	got := read(t, rc)
	if !strings.HasPrefix(got, "alias a=b\n\n"+marker+"\n") {
		t.Errorf("existing content mangled: %q", got)
	}
}

func TestRecognisesUsersOwnPathLine(t *testing.T) {
	o := opts(t)
	rc := filepath.Join(o.Home, ".bashrc")
	if runtime.GOOS == "darwin" {
		rc = filepath.Join(o.Home, ".bash_profile")
	}
	os.WriteFile(rc, []byte("# export PATH=$PATH:/opt/yozora/bin\n"), 0o644)
	if res, _ := Ensure(o); res.Status != Added { // commented-out line doesn't count
		t.Errorf("commented line treated as configured: %+v", res)
	}
	o2 := opts(t)
	rc2 := filepath.Join(o2.Home, filepath.Base(rc))
	os.WriteFile(rc2, []byte("export PATH=\"$HOME/x:/opt/yozora/bin:$PATH\"\n"), 0o644)
	if res, _ := Ensure(o2); res.Status != AlreadyConfigured {
		t.Errorf("user's own line not recognised: %+v", res)
	}
}

func TestQuoting(t *testing.T) {
	o := opts(t)
	o.Dir = `/opt/my "dir"/$HOME`
	res, err := Ensure(o)
	if err != nil {
		t.Fatal(err)
	}
	if want := `export PATH="$PATH:/opt/my \"dir\"/\$HOME"`; !strings.Contains(read(t, res.File), want) {
		t.Errorf("bad quoting: %q", read(t, res.File))
	}
}

func TestErrors(t *testing.T) {
	o := opts(t)
	o.Shell = ""
	if _, err := Ensure(o); err == nil || !strings.Contains(err.Error(), "which shell") {
		t.Errorf("empty shell: %v", err)
	}
	o = opts(t)
	o.Dir = "relative/bin"
	if _, err := Ensure(o); err == nil || !strings.Contains(err.Error(), "absolute") {
		t.Errorf("relative dir: %v", err)
	}
}

func TestTarget(t *testing.T) {
	d := t.TempDir()
	if got, err := target(d); err != nil || got != d {
		t.Errorf("target(%s) = %q, %v", d, got, err)
	}
	if _, err := target(filepath.Join(d, "missing")); err == nil {
		t.Error("expected error for missing directory")
	}
	if got, err := target(""); err != nil || !filepath.IsAbs(got) {
		t.Errorf("default target = %q, %v", got, err)
	}
}
