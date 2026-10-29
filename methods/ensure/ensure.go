// Package ensure implements `yozora ensure [-f] [-p] [-d dir] [-s shell]`,
// modelled on `pipx ensurepath`: it makes sure the directory holding the
// yozora binary is on PATH, appending a small block to the current shell's
// config file if it isn't.
//
//	$ yozora ensure
//	Success! Added /home/me/go/bin to the PATH environment variable.
//	Open a new terminal or reload your shell config ('source ~/.bashrc') for the change to take effect.
//
// Running it again is a no-op. Only the config file is edited; the PATH of
// the running shell can't be changed from a child process.
package ensure

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/ShashankRaoCoding/yozora/methods"
)

func init() {
	methods.Methods["ensure"] = ensure
}

// Status says what Ensure found or did.
type Status int

const (
	AlreadyInPath     Status = iota // dir is on the current PATH; nothing changed
	AlreadyConfigured               // shell config already adds dir, but this shell hasn't reloaded it
	Added                           // shell config was edited
)

// Options are the inputs to Ensure. They are explicit rather than read from
// the environment so Ensure can be tested and used as a library.
type Options struct {
	Dir     string // directory that should be on PATH
	Home    string // home directory holding the shell config
	Shell   string // shell name or path, e.g. "bash" or "/bin/zsh"
	Path    string // current PATH value
	Force   bool   // edit the config even if dir is already on PATH
	Prepend bool   // put dir before existing PATH entries instead of after
}

// Result describes the outcome of Ensure.
type Result struct {
	Status Status
	File   string // shell config file examined or edited ("" for AlreadyInPath)
}

func ensure(args []string) {
	fl := flag.NewFlagSet("ensure", flag.ExitOnError)
	force := fl.Bool("f", false, "add to the shell config even if the directory is already on PATH")
	prepend := fl.Bool("p", false, "prepend to PATH instead of appending")
	dir := fl.String("d", "", "directory to add (default: the directory containing the yozora binary)")
	shell := fl.String("s", "", "shell to configure: bash|zsh|fish|sh (default: $SHELL)")
	fl.Parse(args)

	if fl.NArg() > 0 {
		fatal(fmt.Errorf("unexpected argument %q", fl.Arg(0)))
	}
	d, err := target(*dir)
	if err != nil {
		fatal(err)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		fatal(err)
	}
	sh := *shell
	if sh == "" {
		sh = os.Getenv("SHELL")
	}

	res, err := Ensure(Options{Dir: d, Home: home, Shell: sh, Path: os.Getenv("PATH"), Force: *force, Prepend: *prepend})
	if err != nil {
		fatal(err)
	}
	switch res.Status {
	case AlreadyInPath:
		fmt.Printf("%s is already in PATH.\n", d)
		fmt.Println("yozora is ready to go!")
	case AlreadyConfigured:
		fmt.Printf("%s is already added to PATH in %s, but this shell hasn't picked it up.\n", d, res.File)
		fmt.Println(reload(res.File))
	case Added:
		fmt.Printf("Success! Added %s to the PATH environment variable.\n", d)
		fmt.Println(reload(res.File))
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "Error:", err)
	os.Exit(1)
}

func reload(file string) string {
	return fmt.Sprintf("Open a new terminal or reload your shell config ('source %s') for the change to take effect.", file)
}

// target resolves the directory to put on PATH: the -d flag, else the
// directory of the running binary with symlinks resolved.
func target(flagDir string) (string, error) {
	d := flagDir
	if d == "" {
		exe, err := os.Executable()
		if err != nil {
			return "", err
		}
		if real, err := filepath.EvalSymlinks(exe); err == nil {
			exe = real
		}
		d = filepath.Dir(exe)
	}
	d, err := filepath.Abs(d)
	if err != nil {
		return "", err
	}
	if info, err := os.Stat(d); err != nil || !info.IsDir() {
		return "", fmt.Errorf("%s is not a directory", d)
	}
	return d, nil
}

// Ensure makes sure o.Dir is on PATH for new shells.
func Ensure(o Options) (Result, error) {
	if runtime.GOOS == "windows" {
		return Result{}, errors.New("ensure does not support Windows; add the directory to PATH in System Settings")
	}
	if !filepath.IsAbs(o.Dir) {
		return Result{}, fmt.Errorf("%q is not an absolute path", o.Dir)
	}
	if !o.Force && onPath(o.Path, o.Dir) {
		return Result{Status: AlreadyInPath}, nil
	}

	shell := filepath.Base(o.Shell)
	if o.Shell == "" {
		return Result{}, errors.New("can't tell which shell you use; set $SHELL or pass -s bash|zsh|fish|sh")
	}
	file := rcFile(shell, o.Home, runtime.GOOS)

	if configured(file, o.Dir) {
		return Result{Status: AlreadyConfigured, File: file}, nil
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return Result{}, err
	}
	f, err := os.OpenFile(file, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		return Result{}, err
	}
	if _, err := f.WriteString(block(shell, o.Dir, o.Prepend, needsNewline(f))); err != nil {
		f.Close()
		return Result{}, err
	}
	return Result{Status: Added, File: file}, f.Close()
}

// onPath reports whether dir is one of the entries of path.
func onPath(path, dir string) bool {
	for _, p := range filepath.SplitList(path) {
		if p != "" && filepath.Clean(p) == filepath.Clean(dir) {
			return true
		}
	}
	return false
}

// rcFile returns the config file yozora edits for shell.
func rcFile(shell, home, goos string) string {
	switch shell {
	case "bash":
		if goos == "darwin" {
			return filepath.Join(home, ".bash_profile") // macOS terminals start login shells
		}
		return filepath.Join(home, ".bashrc")
	case "zsh":
		return filepath.Join(home, ".zshrc")
	case "fish":
		return filepath.Join(home, ".config", "fish", "config.fish")
	}
	return filepath.Join(home, ".profile") // sh, dash, ksh and anything POSIX-like
}

const marker = "# Added by `yozora ensure`"

// configured reports whether file already has an uncommented line that
// mentions both PATH and dir (ours or the user's own).
func configured(file, dir string) bool {
	f, err := os.Open(file)
	if err != nil {
		return false
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "#") {
			continue
		}
		if strings.Contains(line, "PATH") && strings.Contains(line, dir) {
			return true
		}
	}
	return false
}

// needsNewline reports whether the (append-mode) file is non-empty and
// doesn't end in a newline, so the block doesn't join the last line.
func needsNewline(f *os.File) bool {
	info, err := f.Stat()
	if err != nil || info.Size() == 0 {
		return false
	}
	var last [1]byte
	if _, err := f.ReadAt(last[:], info.Size()-1); err != nil {
		return false
	}
	return last[0] != '\n'
}

// block returns the text appended to the shell config.
func block(shell, dir string, prepend, nl bool) string {
	var b strings.Builder
	if nl {
		b.WriteString("\n")
	}
	b.WriteString("\n" + marker + "\n")
	q := quote(dir)
	switch {
	case shell == "fish" && prepend:
		b.WriteString(`fish_add_path -g "` + q + `"` + "\n")
	case shell == "fish":
		b.WriteString(`fish_add_path -g -a "` + q + `"` + "\n")
	case prepend:
		b.WriteString(`export PATH="` + q + `:$PATH"` + "\n")
	default:
		b.WriteString(`export PATH="$PATH:` + q + `"` + "\n")
	}
	return b.String()
}

// quote escapes dir for use inside double quotes (POSIX shells and fish).
func quote(dir string) string {
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "$", `\$`, "`", "\\`")
	return r.Replace(dir)
}
