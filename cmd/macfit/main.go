// macfit keeps Mac config files in one encrypted store and restores them on
// any Mac. See README.md in this directory.
package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/queone/gkit/internal/color"
	"github.com/queone/gkit/internal/lockbox"
	"golang.org/x/term"
)

const programVersion = "1.5.0"

// storeSource names where the store path came from.
type storeSource string

const (
	sourceFlag    storeSource = "flag"
	sourceEnv     storeSource = "env"
	sourcePointer storeSource = "pointer"
	sourceDefault storeSource = "default"
)

// storeRef is the resolved store path and how it was chosen.
type storeRef struct {
	path   string
	source storeSource
}

// app carries the process-level dependencies so tests can swap them for fakes.
type app struct {
	stdout     io.Writer
	stderr     io.Writer
	stdin      io.Reader
	keys       lockbox.KeyStore
	env        lockbox.Env
	host       string
	kdf        lockbox.KDF
	goos       string
	storeEnv   string
	isTerminal func() bool
	readSecret func(prompt string) ([]byte, error)
	readLine   func(prompt string) (string, error)
}

func newApp() *app {
	a := &app{
		stdout:     os.Stdout,
		stderr:     os.Stderr,
		stdin:      os.Stdin,
		keys:       lockbox.SecurityKeyStore{},
		env:        lockbox.EnvFromOS(),
		host:       lockbox.Hostname(lockbox.DefaultExecutor, os.Hostname),
		kdf:        lockbox.DefaultKDF,
		goos:       runtime.GOOS,
		storeEnv:   os.Getenv("MACFIT_STORE"),
		isTerminal: func() bool { return term.IsTerminal(int(os.Stdin.Fd())) },
		readSecret: terminalSecret,
	}
	a.readLine = a.terminalLine
	return a
}

// terminalSecret prompts on stderr and reads one line with echo off.
func terminalSecret(prompt string) ([]byte, error) {
	fmt.Fprint(os.Stderr, prompt)
	b, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	return b, err
}

// terminalLine prompts on stderr and reads one visible line.
func (a *app) terminalLine(prompt string) (string, error) {
	fmt.Fprint(a.stderr, prompt)
	line, err := bufio.NewReader(a.stdin).ReadString('\n')
	if err != nil && line == "" {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

func main() {
	os.Exit(newApp().run(os.Args[1:]))
}

func isVersionArg(a string) bool { return a == "-v" || a == "--version" || a == "v" || a == "version" }
func isHelpArg(a string) bool {
	return a == "-h" || a == "-?" || a == "--help" || a == "h" || a == "help"
}

// heading renders a help heading in bold white, like the name on line one.
func heading(s string) string { return color.Bold(color.Gra10(s)) }

func usage() string {
	return heading("macfit") + " v" + programVersion + "\n" +
		color.Gra5("Keep Mac config files in one encrypted store and restore them on any Mac.") + "\n\n" +
		heading("Overview") + "\n" +
		"  The store is a single sealed file. Keep it in a synced folder and every Mac\n" +
		"  that sees the folder can open it. push sends live files into the store,\n" +
		"  pull restores them from the store, and diff shows what differs. The key\n" +
		"  lives in the login keychain, with a passphrase-wrapped copy in the store\n" +
		"  for other Macs.\n\n" +
		heading("Usage") + "\n" +
		"  macfit [st]                         status of the store, key, and drift; no command means st\n" +
		"  macfit init [-N]                    unlock an existing store, or create one with -N\n" +
		"  macfit add PATH... [-H HOST|-g]     register live files for this Mac and capture them\n" +
		"  macfit set TARGET [flags]           change an entry's Mac binding or mode (-H, -g, -m, -F)\n" +
		"  macfit rm TARGET [-H HOST]          forget a file and its stored versions\n" +
		"  macfit ls                           list entries\n" +
		"  macfit push [TARGET...]             send changed live files into the store\n" +
		"  macfit pull [TARGET...] [-f]        plan the restore, or write it with -f\n" +
		"  macfit diff [TARGET...] [-V]        show drift between the store and this Mac\n" +
		"  macfit render [-o DIR] [-a] [-f]    write the stored files into a browsable directory\n" +
		"  macfit cat TARGET [-H HOST]         print one stored file\n" +
		"  macfit key show                     store path, key id, keychain and store state\n" +
		"  macfit key restore                  put the key back in the keychain with the passphrase\n" +
		"  macfit key rm [-f]                  delete the keychain item after a prompt\n" +
		"  macfit key passphrase               change the recovery passphrase\n\n" +
		heading("Options") + "\n" +
		"  -s, --store PATH   Store file for this command; init -s also remembers it\n" +
		"  -N, --new          Create a new store (init)\n" +
		"  -H, --host NAME    Bind the entry to another Mac (add, set, rm, cat)\n" +
		"  -g, --global       Make the entry apply on every Mac (add, set)\n" +
		"  -m, --mode MODE    Store a new mode, three or four octal digits (set)\n" +
		"  -F, --from WHICH   Pick the entry to change by its binding: a host name or global (set)\n" +
		"  -l, --literal      Keep the path under ~ instead of an XDG variable (add)\n" +
		"  -n, --dry-run      Print the pull plan; the default, kept for scripts\n" +
		"  -f, --force        Write the pull plan, overwriting live files that differ; skip the key rm prompt\n" +
		"  -V, --verbose      Add a unified diff to diff output\n" +
		"  -o, --out DIR      Render into DIR instead of a fresh private temp directory (render)\n" +
		"  -a, --all          Render every stored version too, under versions/ (render)\n" +
		"  -v, --version      Print macfit v" + programVersion + " and exit\n" +
		"  -h, -?, --help     Show this help message and exit\n\n" +
		heading("Notes") + "\n" +
		"  Store path order: -s, then MACFIT_STORE, then the path init -s remembered in\n" +
		"  $XDG_CONFIG_HOME/macfit/store, then $XDG_DATA_HOME/macfit/macfit.store.\n" +
		"  A TARGET is the template ls shows ($XDG_CONFIG_HOME/git/config) or the live path.\n" +
		"  add binds a file to this Mac unless -g; on a Mac, its own entry wins over a global one.\n" +
		"  init needs a terminal for the passphrase prompt and creates only the default folder.\n" +
		"  render writes plaintext copies of the store; delete the directory when done.\n" +
		"  Files only: no directories, globs, or symlinks. macOS defaults settings are a planned addition.\n"
}

func (a *app) errorf(format string, args ...any) {
	fmt.Fprintf(a.stderr, "macfit: "+format+"\n", args...)
}

// run dispatches macfit's subcommands and returns the process exit code.
func (a *app) run(args []string) int {
	if len(args) == 1 && isVersionArg(args[0]) {
		fmt.Fprintf(a.stdout, "macfit v%s\n", programVersion)
		return 0
	}
	if len(args) == 1 && isHelpArg(args[0]) {
		fmt.Fprint(a.stdout, usage())
		return 0
	}
	storeFlag, rest, err := splitStoreFlag(args)
	if err != nil {
		a.errorf("%s; run `macfit help`", err)
		return 2
	}
	if len(rest) == 0 {
		rest = []string{"st"}
	}
	if isHelpArg(rest[0]) {
		fmt.Fprint(a.stdout, usage())
		return 0
	}
	if a.goos != "darwin" {
		a.errorf("macfit supports macOS only")
		return 1
	}
	ref, err := a.resolveStore(storeFlag)
	if err != nil {
		a.errorf("resolve store path: %s", err)
		return 1
	}
	verb, vargs := rest[0], rest[1:]
	switch verb {
	case "init":
		return a.cmdInit(ref, vargs)
	case "st":
		return a.cmdSt(ref, vargs)
	case "add":
		return a.cmdAdd(ref, vargs)
	case "rm":
		return a.cmdRm(ref, vargs)
	case "ls":
		return a.cmdLs(ref, vargs)
	case "push":
		return a.cmdPush(ref, vargs)
	case "pull":
		return a.cmdPull(ref, vargs)
	case "diff":
		return a.cmdDiff(ref, vargs)
	case "key":
		return a.cmdKey(ref, vargs)
	case "set":
		return a.cmdSet(ref, vargs)
	case "render":
		return a.cmdRender(ref, vargs)
	case "cat":
		return a.cmdCat(ref, vargs)
	default:
		a.errorf("unknown command %q; run `macfit help`", verb)
		return 2
	}
}

// splitStoreFlag pulls -s/--store out of args wherever it appears.
func splitStoreFlag(args []string) (string, []string, error) {
	var path string
	var rest []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-s" || arg == "--store":
			if i+1 >= len(args) {
				return "", nil, fmt.Errorf("%s needs a PATH", arg)
			}
			path = args[i+1]
			i++
		case strings.HasPrefix(arg, "--store="):
			path = strings.TrimPrefix(arg, "--store=")
		default:
			rest = append(rest, arg)
		}
	}
	return path, rest, nil
}

// pointerFile is where init -s remembers the store path on this Mac.
func (a *app) pointerFile() string { return filepath.Join(a.env.ConfigHome, "macfit", "store") }

// defaultStore is the store path when nothing else names one.
func (a *app) defaultStore() string { return filepath.Join(a.env.DataHome, "macfit", "macfit.store") }

func absPath(p string) (string, error) {
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	return filepath.Clean(abs), nil
}

// resolveStore applies the order: -s, MACFIT_STORE, the pointer file, the default.
func (a *app) resolveStore(flag string) (storeRef, error) {
	if flag != "" {
		abs, err := absPath(flag)
		return storeRef{abs, sourceFlag}, err
	}
	if a.storeEnv != "" {
		abs, err := absPath(a.storeEnv)
		return storeRef{abs, sourceEnv}, err
	}
	if b, err := os.ReadFile(a.pointerFile()); err == nil {
		if p := strings.TrimSpace(string(b)); p != "" {
			abs, err := absPath(p)
			return storeRef{abs, sourcePointer}, err
		}
	}
	return storeRef{a.defaultStore(), sourceDefault}, nil
}

// writePointer records path so later commands find the store without -s.
func (a *app) writePointer(path string) error {
	if err := os.MkdirAll(filepath.Dir(a.pointerFile()), 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(a.pointerFile(), []byte(path+"\n"), 0o600); err != nil {
		return err
	}
	return os.Chmod(a.pointerFile(), 0o600)
}

// rememberIfFlagged writes the pointer when the store came from -s.
func (a *app) rememberIfFlagged(ref storeRef) int {
	if ref.source != sourceFlag {
		return 0
	}
	if err := a.writePointer(ref.path); err != nil {
		a.errorf("remember store path: %s", err)
		return 1
	}
	fmt.Fprintf(a.stdout, "store path remembered in %s\n", a.pointerFile())
	return 0
}

type flagSpec struct {
	short string
	long  string
	value bool
}

// parseArgs separates known flags from positionals. A boolean flag maps to
// "true"; a value flag maps to its value, given as the next argument or
// after "=".
func parseArgs(args []string, specs []flagSpec) (map[string]string, []string, error) {
	flags := map[string]string{}
	var pos []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			pos = append(pos, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			pos = append(pos, arg)
			continue
		}
		name, inline, hasInline := strings.Cut(arg, "=")
		var spec *flagSpec
		for k := range specs {
			if name == specs[k].short || name == specs[k].long {
				spec = &specs[k]
				break
			}
		}
		if spec == nil {
			return nil, nil, fmt.Errorf("unknown flag %s", name)
		}
		if !spec.value {
			if hasInline {
				return nil, nil, fmt.Errorf("%s takes no value", name)
			}
			flags[spec.long] = "true"
			continue
		}
		if hasInline {
			flags[spec.long] = inline
			continue
		}
		if i+1 >= len(args) {
			return nil, nil, fmt.Errorf("%s needs a value", name)
		}
		flags[spec.long] = args[i+1]
		i++
	}
	return flags, pos, nil
}

// readStoreHeader reads the store file at path and parses its header.
func readStoreHeader(path string) ([]byte, lockbox.Header, error) {
	file, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, lockbox.Header{}, fmt.Errorf("no store at %s; run `macfit init -N` to create one, or pass -s PATH to point at an existing store", path)
	}
	if err != nil {
		return nil, lockbox.Header{}, err
	}
	hdr, err := lockbox.ParseHeader(file)
	if err != nil {
		return nil, lockbox.Header{}, fmt.Errorf("%s: %w", path, err)
	}
	return file, hdr, nil
}

// openStore loads the store with the key from the keychain.
func (a *app) openStore(path string) (*lockbox.Store, error) {
	_, hdr, err := readStoreHeader(path)
	if err != nil {
		return nil, err
	}
	key, err := a.keys.Get(lockbox.KeyIDString(hdr.KeyID))
	if errors.Is(err, lockbox.ErrKeyNotFound) {
		return nil, fmt.Errorf("no key for %s in the login keychain; run `macfit init` and enter the recovery passphrase", path)
	}
	if err != nil {
		return nil, err
	}
	st, err := lockbox.Load(path, key)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	if copies := lockbox.ConflictCopies(path); len(copies) > 0 {
		a.errorf("warning: sync conflict copies beside the store: %s", strings.Join(copies, ", "))
	}
	return st, nil
}

func (a *app) cmdInit(ref storeRef, args []string) int {
	flags, pos, err := parseArgs(args, []flagSpec{{"-N", "--new", false}})
	if err != nil || len(pos) > 0 {
		a.errorf("init: usage: macfit init [-N]")
		return 2
	}
	if flags["--new"] == "true" {
		return a.initNew(ref)
	}
	return a.initUnlock(ref)
}

// initUnlock puts an existing store's key into this Mac's keychain.
func (a *app) initUnlock(ref storeRef) int {
	_, hdr, err := readStoreHeader(ref.path)
	if err != nil {
		a.errorf("init: %s", err)
		return 1
	}
	keyID := lockbox.KeyIDString(hdr.KeyID)
	_, err = a.keys.Get(keyID)
	switch {
	case err == nil:
		fmt.Fprintf(a.stdout, "store at %s is already unlocked on this Mac\n", ref.path)
		return a.rememberIfFlagged(ref)
	case !errors.Is(err, lockbox.ErrKeyNotFound):
		a.errorf("init: %s", err)
		return 1
	}
	if code := a.restoreKey("init", ref.path, hdr); code != 0 {
		return code
	}
	return a.rememberIfFlagged(ref)
}

// restoreKey asks for the recovery passphrase, unwraps the key, and saves it.
func (a *app) restoreKey(verb, path string, hdr lockbox.Header) int {
	if !a.isTerminal() {
		a.errorf("%s needs a terminal", verb)
		return 1
	}
	pass, err := a.readSecret("Recovery passphrase for " + path + ": ")
	if err != nil {
		a.errorf("%s: read passphrase: %s", verb, err)
		return 1
	}
	key, err := hdr.UnwrapKey(pass)
	if err != nil {
		a.errorf("%s: %s", verb, err)
		return 1
	}
	if err := a.keys.Put(lockbox.KeyIDString(hdr.KeyID), key); err != nil {
		a.errorf("%s: %s", verb, err)
		return 1
	}
	fmt.Fprintf(a.stdout, "key for %s saved to the login keychain\n", path)
	return 0
}

// newPassphrase asks for a passphrase twice and returns it.
func (a *app) newPassphrase(verb string) ([]byte, int) {
	pass, err := a.readSecret("New recovery passphrase: ")
	if err != nil {
		a.errorf("%s: read passphrase: %s", verb, err)
		return nil, 1
	}
	if len(bytes.TrimSpace(pass)) == 0 {
		a.errorf("%s: passphrase must not be empty", verb)
		return nil, 1
	}
	again, err := a.readSecret("Repeat passphrase: ")
	if err != nil {
		a.errorf("%s: read passphrase: %s", verb, err)
		return nil, 1
	}
	if !bytes.Equal(pass, again) {
		a.errorf("%s: passphrases do not match", verb)
		return nil, 1
	}
	return pass, 0
}

// initNew creates a store, its key, and the passphrase-wrapped recovery copy.
func (a *app) initNew(ref storeRef) int {
	if _, err := os.Stat(ref.path); err == nil {
		a.errorf("init: a store already exists at %s", ref.path)
		return 1
	}
	parent := filepath.Dir(ref.path)
	if ref.source == sourceDefault {
		if err := os.MkdirAll(parent, 0o700); err != nil {
			a.errorf("init: %s", err)
			return 1
		}
	} else if info, err := os.Stat(parent); err != nil || !info.IsDir() {
		a.errorf("init: directory %s does not exist; create the synced folder first, or pass -s PATH to keep the store elsewhere", parent)
		return 1
	}
	if !a.isTerminal() {
		a.errorf("init needs a terminal")
		return 1
	}
	pass, code := a.newPassphrase("init")
	if code != 0 {
		return code
	}
	key, err := lockbox.NewKey()
	if err != nil {
		a.errorf("init: %s", err)
		return 1
	}
	id, err := lockbox.NewKeyID()
	if err != nil {
		a.errorf("init: %s", err)
		return 1
	}
	hdr := lockbox.Header{KeyID: id}
	if err := hdr.WrapKey(key, pass, a.kdf); err != nil {
		a.errorf("init: %s", err)
		return 1
	}
	st, err := lockbox.Create(ref.path, hdr, key)
	if err != nil {
		a.errorf("init: %s", err)
		return 1
	}
	defer st.Close()
	if err := st.Save(); err != nil {
		a.errorf("init: write %s: %s", ref.path, err)
		return 1
	}
	if err := a.keys.Put(lockbox.KeyIDString(id), key); err != nil {
		a.errorf("init: store written but the key was not saved: %s; run `macfit init` and enter the passphrase", err)
		return 1
	}
	fmt.Fprintf(a.stdout, "store created at %s\nkey saved to the login keychain\nkeep the recovery passphrase safe: another Mac needs it once\n", ref.path)
	return a.rememberIfFlagged(ref)
}

func forHost(h string) string {
	if h == "" {
		return ""
	}
	return ", host " + h
}

func (a *app) cmdAdd(ref storeRef, args []string) int {
	flags, pos, err := parseArgs(args, []flagSpec{{"-H", "--host", true}, {"-g", "--global", false}, {"-l", "--literal", false}})
	_, explicitHost := flags["--host"]
	global := flags["--global"] == "true"
	if err != nil || len(pos) == 0 || (explicitHost && global) {
		a.errorf("add: usage: macfit add PATH... [-H HOST | -g] [-l]")
		return 2
	}
	host, err := a.bindingFor(flags["--host"], explicitHost, global)
	if err != nil {
		a.errorf("add: %s", err)
		return 1
	}
	literal := flags["--literal"] == "true"
	st, err := a.openStore(ref.path)
	if err != nil {
		a.errorf("add: %s", err)
		return 1
	}
	defer st.Close()
	rc, added := 0, false
	for _, p := range pos {
		if err := a.addOne(st, p, host, literal); err != nil {
			a.errorf("add: %s", err)
			rc = 1
			continue
		}
		added = true
	}
	if added {
		if err := st.Save(); err != nil {
			a.errorf("add: %s", err)
			return 1
		}
	}
	return rc
}

// addOne registers one live file and captures its content.
func (a *app) addOne(st *lockbox.Store, p, host string, literal bool) error {
	abs, err := filepath.Abs(p)
	if err != nil {
		return err
	}
	info, err := os.Lstat(abs)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", abs)
	}
	content, err := os.ReadFile(abs)
	if err != nil {
		return err
	}
	target := a.env.Template(abs)
	if literal {
		target = a.env.Literal(abs)
	}
	e, err := st.AddEntry(target, info.Mode().Perm(), host, fileOwnership(info))
	if errors.Is(err, lockbox.ErrExists) {
		return fmt.Errorf("%s is already registered%s; use `macfit push` to store its current content", target, forHost(host))
	}
	if err != nil {
		return err
	}
	if _, err := st.AddVersion(e.ID, content, a.host); err != nil {
		return err
	}
	fmt.Fprintln(a.stdout, status("added", fmt.Sprintf("%s (mode %04o, %s)", target, e.Mode, bindingOf(host))))
	return nil
}

// targetKeys lists the store targets an argument may name: a template as
// given, or the templated, literal, and absolute forms of a live path.
func (a *app) targetKeys(arg string) []string {
	if strings.HasPrefix(arg, "~") || strings.HasPrefix(arg, "$") {
		return []string{arg}
	}
	abs, err := filepath.Abs(arg)
	if err != nil {
		return []string{arg}
	}
	return []string{a.env.Template(abs), a.env.Literal(abs), abs}
}

func (a *app) cmdRm(ref storeRef, args []string) int {
	flags, pos, err := parseArgs(args, []flagSpec{{"-H", "--host", true}})
	if err != nil || len(pos) != 1 {
		a.errorf("rm: usage: macfit rm TARGET [-H HOST]")
		return 2
	}
	st, err := a.openStore(ref.path)
	if err != nil {
		a.errorf("rm: %s", err)
		return 1
	}
	defer st.Close()
	all, err := st.Entries()
	if err != nil {
		a.errorf("rm: %s", err)
		return 1
	}
	host, explicit := flags["--host"]
	pick := a.pickEntry(all, pos[0], host, explicit)
	if pick == nil {
		a.errorf("rm: %s is not registered%s", pos[0], forHost(host))
		return 1
	}
	if err := st.RemoveEntry(pick.ID); err != nil {
		a.errorf("rm: %s", err)
		return 1
	}
	if err := st.Save(); err != nil {
		a.errorf("rm: %s", err)
		return 1
	}
	fmt.Fprintf(a.stdout, "removed %s%s\n", pick.Target, forHost(pick.Host))
	return 0
}

// pickEntry selects the entry an argument names: the one bound to host when
// explicit, else this Mac's bound entry, else the unbound entry.
func (a *app) pickEntry(all []lockbox.Entry, arg, host string, explicit bool) *lockbox.Entry {
	keys := a.targetKeys(arg)
	var pick *lockbox.Entry
	for i := range all {
		e := &all[i]
		if !slices.Contains(keys, e.Target) {
			continue
		}
		switch {
		case explicit && e.Host == host:
			pick = e
		case !explicit && e.Host == a.host:
			pick = e
		case !explicit && e.Host == "" && pick == nil:
			pick = e
		}
	}
	return pick
}

func (a *app) cmdLs(ref storeRef, args []string) int {
	if len(args) > 0 {
		a.errorf("ls takes no arguments; run `macfit help`")
		return 2
	}
	st, err := a.openStore(ref.path)
	if err != nil {
		a.errorf("ls: %s", err)
		return 1
	}
	defer st.Close()
	all, err := st.Entries()
	if err != nil {
		a.errorf("ls: %s", err)
		return 1
	}
	rows := [][]string{{"HOST", "OWNER_GROUP", "MODE", "CAPTURED", "TARGET"}}
	for _, e := range all {
		host := e.Host
		if host == "" {
			host = "<global>"
		}
		owner := "?:?"
		if e.Known() {
			owner = e.Owner + ":" + e.Group
		}
		captured := "-"
		if v, ok, err := st.Latest(e.ID); err == nil && ok {
			captured = v.CapturedAt.Local().Format(time.DateTime)
		}
		rows = append(rows, []string{host, owner, fmt.Sprintf("%04o", e.Mode), captured, e.Target})
	}
	widths := make([]int, len(rows[0]))
	for _, r := range rows {
		for i, c := range r {
			widths[i] = max(widths[i], len(c))
		}
	}
	for ri, r := range rows {
		cells := make([]string, len(r))
		for i, c := range r {
			cell := c
			if i < len(r)-1 {
				cell = fmt.Sprintf("%-*s", widths[i], c)
			}
			if ri > 0 {
				cell = color.Gra4(cell)
			}
			cells[i] = cell
		}
		fmt.Fprintln(a.stdout, strings.Join(cells, "  "))
	}
	return 0
}

// selected returns the entries that apply on this Mac, narrowed to args when given.
func (a *app) selected(st *lockbox.Store, args []string) ([]lockbox.Entry, error) {
	all, err := st.Entries()
	if err != nil {
		return nil, err
	}
	sel := lockbox.Select(all, a.host)
	if len(args) == 0 {
		return sel, nil
	}
	var out []lockbox.Entry
	for _, arg := range args {
		keys := a.targetKeys(arg)
		idx := slices.IndexFunc(sel, func(e lockbox.Entry) bool { return slices.Contains(keys, e.Target) })
		if idx < 0 {
			return nil, fmt.Errorf("%s is not registered for this Mac", arg)
		}
		out = append(out, sel[idx])
	}
	return out, nil
}

func (a *app) cmdPush(ref storeRef, args []string) int {
	_, pos, err := parseArgs(args, nil)
	if err != nil {
		a.errorf("push: %s; run `macfit help`", err)
		return 2
	}
	st, err := a.openStore(ref.path)
	if err != nil {
		a.errorf("push: %s", err)
		return 1
	}
	defer st.Close()
	sel, err := a.selected(st, pos)
	if err != nil {
		a.errorf("push: %s", err)
		return 1
	}
	rc, changed := 0, false
	for _, e := range sel {
		live := a.env.Expand(e.Target)
		if li, lerr := os.Lstat(live); lerr == nil && li.Mode()&os.ModeSymlink != 0 {
			fmt.Fprintln(a.stdout, status("symlink", e.Target+" (refusing to read through a link)"))
			rc = 1
			continue
		}
		content, err := os.ReadFile(live)
		if errors.Is(err, fs.ErrNotExist) {
			fmt.Fprintln(a.stdout, status("missing", e.Target))
			continue
		}
		if err != nil {
			a.errorf("push: %s: %s", e.Target, err)
			rc = 1
			continue
		}
		info, err := os.Stat(live)
		if err != nil {
			a.errorf("push: %s: %s", e.Target, err)
			rc = 1
			continue
		}
		mode := info.Mode().Perm()
		latest, ok, err := st.Latest(e.ID)
		if err != nil {
			a.errorf("push: %s: %s", e.Target, err)
			rc = 1
			continue
		}
		own := fileOwnership(info)
		sameOwner := !own.Known() || own == e.Ownership
		sameContent := ok && latest.SHA256 == lockbox.Digest(content)
		if sameContent && mode == e.Mode && sameOwner {
			fmt.Fprintln(a.stdout, status("unchanged", e.Target))
			continue
		}
		if !sameOwner {
			if err := st.SetOwner(e.ID, own); err != nil {
				a.errorf("push: %s: %s", e.Target, err)
				return 1
			}
		}
		if mode != e.Mode {
			if err := st.SetMode(e.ID, mode); err != nil {
				a.errorf("push: %s: %s", e.Target, err)
				return 1
			}
		}
		if !sameContent {
			if _, err := st.AddVersion(e.ID, content, a.host); err != nil {
				a.errorf("push: %s: %s", e.Target, err)
				return 1
			}
		}
		changed = true
		fmt.Fprintln(a.stdout, status("updated", e.Target))
	}
	if changed {
		if err := st.Save(); err != nil {
			a.errorf("push: %s", err)
			return 1
		}
	}
	return rc
}
func (a *app) cmdPull(ref storeRef, args []string) int {
	flags, pos, err := parseArgs(args, []flagSpec{{"-n", "--dry-run", false}, {"-f", "--force", false}})
	if err != nil {
		a.errorf("pull: %s; run `macfit help`", err)
		return 2
	}
	write := flags["--force"] == "true" && flags["--dry-run"] != "true"
	st, err := a.openStore(ref.path)
	if err != nil {
		a.errorf("pull: %s", err)
		return 1
	}
	defer st.Close()
	sel, err := a.selected(st, pos)
	if err != nil {
		a.errorf("pull: %s", err)
		return 1
	}
	rc := 0
	for _, e := range sel {
		latest, ok, err := st.Latest(e.ID)
		if err != nil {
			a.errorf("pull: %s: %s", e.Target, err)
			return 1
		}
		if !ok {
			fmt.Fprintln(a.stdout, status("empty", e.Target+" (nothing stored yet)"))
			continue
		}
		live := a.env.Expand(e.Target)
		info, err := os.Lstat(live)
		if err == nil && info.Mode()&os.ModeSymlink != 0 {
			fmt.Fprintln(a.stdout, status("symlink", e.Target+" (refusing to write through a link)"))
			if write {
				rc = 1
			}
			continue
		}
		exists := err == nil
		if exists {
			cur, rerr := os.ReadFile(live)
			if rerr != nil {
				a.errorf("pull: %s: %s", e.Target, rerr)
				rc = 1
				continue
			}
			if lockbox.Digest(cur) == latest.SHA256 && info.Mode().Perm() == e.Mode {
				fmt.Fprintln(a.stdout, status("unchanged", e.Target))
				continue
			}
		}
		if !write {
			if exists {
				fmt.Fprintln(a.stdout, status("would overwrite", e.Target))
			} else {
				fmt.Fprintln(a.stdout, status("would write", e.Target))
			}
			continue
		}
		dirMode := os.FileMode(0o755)
		if e.Mode == 0o600 {
			dirMode = 0o700
		}
		if err := os.MkdirAll(filepath.Dir(live), dirMode); err != nil {
			a.errorf("pull: %s: %s", e.Target, err)
			rc = 1
			continue
		}
		if err := os.WriteFile(live, latest.Content, e.Mode); err != nil {
			a.errorf("pull: %s: %s", e.Target, err)
			rc = 1
			continue
		}
		if err := os.Chmod(live, e.Mode); err != nil {
			a.errorf("pull: %s: %s", e.Target, err)
			rc = 1
			continue
		}
		fmt.Fprintln(a.stdout, status("restored", e.Target))
	}
	return rc
}

// statusWidth is the column where targets start: the longest status word,
// "would overwrite", plus two spaces.
const statusWidth = len("would overwrite") + 2

// paint colors a whole line by the outcome its first word reports.
func paint(word, line string) string {
	switch word {
	case "=", "unchanged":
		return color.Gra5(line)
	case "M", "differs", "would overwrite", "missing", "empty":
		return color.Yel5(line)
	case "?":
		return color.Org5(line)
	case "would write", "restored", "added", "updated", "set":
		return color.Grn5(line)
	case "symlink":
		return color.Red5(line)
	}
	return line
}

// status renders a padded, colored status line for a target.
func status(word, target string) string {
	return paint(word, fmt.Sprintf("%-*s%s", statusWidth, word, target))
}

// marker renders a one-character diff marker line.
func marker(mark, rest string) string {
	return paint(mark, mark+" "+rest)
}

// diffLine colors one line of a unified diff block.
func diffLine(line string) string {
	switch {
	case strings.HasPrefix(line, "---") || strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "@@"):
		return color.Gra4(line)
	case strings.HasPrefix(line, "-") || strings.HasPrefix(line, "+"):
		return color.Yel8(line)
	}
	return color.Gra5(line)
}

func (a *app) cmdDiff(ref storeRef, args []string) int {
	flags, pos, err := parseArgs(args, []flagSpec{{"-V", "--verbose", false}})
	if err != nil {
		a.errorf("diff: %s; run `macfit help`", err)
		return 2
	}
	verbose := flags["--verbose"] == "true"
	st, err := a.openStore(ref.path)
	if err != nil {
		a.errorf("diff: %s", err)
		return 1
	}
	defer st.Close()
	sel, err := a.selected(st, pos)
	if err != nil {
		a.errorf("diff: %s", err)
		return 1
	}
	drifted := false
	for _, e := range sel {
		latest, ok, err := st.Latest(e.ID)
		if err != nil {
			a.errorf("diff: %s: %s", e.Target, err)
			return 1
		}
		live := a.env.Expand(e.Target)
		if li, lerr := os.Lstat(live); lerr == nil && li.Mode()&os.ModeSymlink != 0 {
			fmt.Fprintln(a.stdout, marker("M", e.Target+" (live is a symlink)"))
			drifted = true
			continue
		}
		cur, rerr := os.ReadFile(live)
		if errors.Is(rerr, fs.ErrNotExist) {
			fmt.Fprintln(a.stdout, marker("?", e.Target))
			drifted = true
			continue
		}
		if rerr != nil {
			a.errorf("diff: %s: %s", e.Target, rerr)
			return 1
		}
		info, _ := os.Stat(live)
		mode := info.Mode().Perm()
		if !ok {
			fmt.Fprintln(a.stdout, marker("M", e.Target+" (nothing stored yet)"))
			drifted = true
			continue
		}
		sameContent := lockbox.Digest(cur) == latest.SHA256
		if sameContent && mode == e.Mode {
			fmt.Fprintln(a.stdout, marker("=", e.Target))
			continue
		}
		drifted = true
		if mode != e.Mode {
			fmt.Fprintln(a.stdout, marker("M", fmt.Sprintf("%s (mode %04o in store, %04o live)", e.Target, e.Mode, mode)))
		} else {
			fmt.Fprintln(a.stdout, marker("M", e.Target))
		}
		if verbose && !sameContent {
			fmt.Fprintln(a.stdout)
			for _, line := range splitLines([]byte(unifiedDiff(e.Target+" (store)", live+" (live)", latest.Content, cur))) {
				fmt.Fprintln(a.stdout, diffLine(line))
			}
			fmt.Fprintln(a.stdout)
		}
	}
	if drifted {
		return 1
	}
	return 0
}

// driftOf classifies one entry against its live path as "=", "M", or "?".
func (a *app) driftOf(st *lockbox.Store, e lockbox.Entry) (string, error) {
	latest, ok, err := st.Latest(e.ID)
	if err != nil {
		return "", err
	}
	live := a.env.Expand(e.Target)
	info, err := os.Lstat(live)
	if errors.Is(err, fs.ErrNotExist) {
		return "?", nil
	}
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeSymlink != 0 || !ok {
		return "M", nil
	}
	cur, err := os.ReadFile(live)
	if err != nil {
		return "", err
	}
	if lockbox.Digest(cur) == latest.SHA256 && info.Mode().Perm() == e.Mode {
		return "=", nil
	}
	return "M", nil
}

// cmdSt prints one status screen for the store, the key, and this Mac's drift.
func (a *app) cmdSt(ref storeRef, args []string) int {
	if len(args) > 0 {
		a.errorf("st takes no arguments; run `macfit help`")
		return 2
	}
	rc := 0
	// kv prints a plain label with a value; grey prints it in dark grey.
	kv := func(label, value string) { fmt.Fprintf(a.stdout, "%s: %s\n", label, value) }
	grey := func(label, value string) { kv(label, color.Gra4(value)) }
	grey("store", fmt.Sprintf("%s (%s)", ref.path, ref.source))
	if b, err := os.ReadFile(a.pointerFile()); err == nil && strings.TrimSpace(string(b)) != "" {
		grey("remembered", a.pointerFile()+" -> "+strings.TrimSpace(string(b)))
	} else {
		grey("remembered", "none")
	}
	var st *lockbox.Store
	opens := "yes"
	file, err := os.ReadFile(ref.path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		grey("store file", "missing")
		grey("key id", "unknown")
		grey("login keychain", "unknown")
		opens = "no (store file missing)"
	case err != nil:
		grey("store file", err.Error())
		grey("key id", "unknown")
		grey("login keychain", "unknown")
		opens = "no (" + err.Error() + ")"
	default:
		hdr, perr := lockbox.ParseHeader(file)
		if perr != nil {
			grey("store file", fmt.Sprintf("present, %d bytes, %s", len(file), perr))
			grey("key id", "unknown")
			grey("login keychain", "unknown")
			opens = "no (" + perr.Error() + ")"
			break
		}
		modified := "unknown"
		if info, serr := os.Stat(ref.path); serr == nil {
			modified = info.ModTime().Local().Format(time.DateTime)
		}
		grey("store file", fmt.Sprintf("present, %d bytes, generation %d, modified %s", len(file), hdr.Generation, modified))
		keyID := lockbox.KeyIDString(hdr.KeyID)
		grey("key id", keyID)
		key, kerr := a.keys.Get(keyID)
		switch {
		case errors.Is(kerr, lockbox.ErrKeyNotFound):
			grey("login keychain", "missing")
			opens = "no (key missing)"
		case kerr != nil:
			grey("login keychain", kerr.Error())
			opens = "no (" + kerr.Error() + ")"
		default:
			grey("login keychain", "present")
			loaded, lerr := lockbox.Load(ref.path, key)
			if lerr != nil {
				opens = "no (" + lerr.Error() + ")"
			} else {
				st = loaded
				defer st.Close()
			}
		}
	}
	if st == nil {
		rc = 1
		kv("store opens", color.Red5(opens))
	} else {
		kv("store opens", color.Grn5(opens))
	}
	grey("host", a.host)
	var sel []lockbox.Entry
	if st == nil {
		grey("entries", "unknown")
	} else {
		all, err := st.Entries()
		if err != nil {
			a.errorf("st: %s", err)
			return 1
		}
		sel = lockbox.Select(all, a.host)
		grey("entries", fmt.Sprintf("%d total, %d for this Mac", len(all), len(sel)))
	}
	if copies := lockbox.ConflictCopies(ref.path); len(copies) > 0 {
		kv("conflict copies", color.Yel5(strings.Join(copies, ", ")))
	} else {
		grey("conflict copies", "none")
	}
	if st == nil {
		grey("drift", "unknown")
		return rc
	}
	counts := map[string]int{}
	for _, e := range sel {
		mark, err := a.driftOf(st, e)
		if err != nil {
			a.errorf("st: %s: %s", e.Target, err)
			return 1
		}
		counts[mark]++
	}
	if counts["M"] == 0 && counts["?"] == 0 {
		kv("drift", color.Grn5("none"))
		return rc
	}
	kv("drift", color.Red5(fmt.Sprintf("M %d, ? %d", counts["M"], counts["?"])))
	return 1
}

func splitLines(b []byte) []string {
	if len(b) == 0 {
		return nil
	}
	return strings.Split(strings.TrimSuffix(string(b), "\n"), "\n")
}

// unifiedDiff renders a single-hunk unified diff of a against b.
func unifiedDiff(aName, bName string, a, b []byte) string {
	al, bl := splitLines(a), splitLines(b)
	n, m := len(al), len(bl)
	var sb strings.Builder
	fmt.Fprintf(&sb, "--- %s\n+++ %s\n", aName, bName)
	if n*m > 4_000_000 {
		sb.WriteString("(files differ; too large to diff line by line)\n")
		return sb.String()
	}
	// lcs[i][j] is the longest common subsequence length of al[i:] and bl[j:].
	lcs := make([][]int, n+1)
	for i := range lcs {
		lcs[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if al[i] == bl[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else {
				lcs[i][j] = max(lcs[i+1][j], lcs[i][j+1])
			}
		}
	}
	fmt.Fprintf(&sb, "@@ -1,%d +1,%d @@\n", n, m)
	i, j := 0, 0
	for i < n || j < m {
		switch {
		case i < n && j < m && al[i] == bl[j]:
			sb.WriteString(" " + al[i] + "\n")
			i++
			j++
		case j < m && (i == n || lcs[i][j+1] > lcs[i+1][j]):
			sb.WriteString("+" + bl[j] + "\n")
			j++
		default:
			sb.WriteString("-" + al[i] + "\n")
			i++
		}
	}
	return sb.String()
}
