// attune reconciles Azure state from YAML specs kept in an encrypted store
// it manages itself. See README.md in this directory.
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"runtime"
	"slices"
	"strings"

	"github.com/queone/gkit/internal/color"
	"github.com/queone/gkit/internal/lockbox"
	"golang.org/x/term"
)

const programVersion = "1.4.0"

var validKinds = []string{
	"dnsRecordSet",
	"securityGroup",
	"appRegistration",
	"roleDefinition",
	"roleAssignment",
	"resourceGroup",
}

type commandKind int

const (
	cmdPlan commandKind = iota
	cmdApply
	cmdValidate
)

// storeVerbs are the commands that manage the spec store.
var storeVerbs = []string{"init", "st", "add", "edit", "rename", "rm", "ls", "cat", "render", "key"}

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
	editor     func(path string) error
}

func newApp() *app {
	a := &app{
		stdout:     os.Stdout,
		stderr:     os.Stderr,
		stdin:      os.Stdin,
		keys:       lockbox.SecurityKeyStore{Service: "attune"},
		env:        lockbox.EnvFromOS(),
		host:       lockbox.Hostname(lockbox.DefaultExecutor, os.Hostname),
		kdf:        lockbox.DefaultKDF,
		goos:       runtime.GOOS,
		storeEnv:   os.Getenv("ATTUNE_STORE"),
		isTerminal: func() bool { return term.IsTerminal(int(os.Stdin.Fd())) },
		readSecret: terminalSecret,
		editor:     terminalEditor,
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

func (a *app) errorf(format string, args ...any) {
	fmt.Fprintf(a.stderr, "attune: "+format+"\n", args...)
}

// run dispatches attune's commands and returns the process exit code.
func (a *app) run(args []string) int {
	if len(args) == 1 && isVersionArg(args[0]) {
		fmt.Fprintf(a.stdout, "attune v%s\n", programVersion)
		return 0
	}
	if len(args) == 0 || (len(args) == 1 && isHelpArg(args[0])) {
		fmt.Fprint(a.stdout, usage())
		return 0
	}
	if a.goos != "darwin" {
		a.errorf("attune store support is macOS only")
		return 1
	}
	switch args[0] {
	case "p", "plan":
		return a.reconcile(cmdPlan, args[1:])
	case "a", "apply":
		return a.reconcile(cmdApply, args[1:])
	case "c", "validate":
		return a.reconcile(cmdValidate, args[1:])
	}
	if slices.Contains(storeVerbs, args[0]) {
		return a.storeCommand(args[0], args[1:])
	}
	a.errorf("unknown command %q; run `attune help`", args[0])
	return 2
}

// reconcile runs plan, apply, or validate against the store.
func (a *app) reconcile(cmd commandKind, args []string) int {
	overrides, err := parseFlags(args)
	if err != nil {
		a.errorf("%s; run `attune help`", err)
		return 2
	}
	storeFlag := ""
	if overrides.Store != nil {
		storeFlag = *overrides.Store
	}
	ref, err := a.resolveStore(storeFlag)
	if err != nil {
		a.errorf("resolve store path: %s", err)
		return 1
	}
	st, err := a.openStore(ref.path)
	if err != nil {
		a.errorf("%s", err)
		return 1
	}
	defer st.Close()
	config, err := storedConfig(st)
	if err != nil {
		a.errorf("%s", err)
		return 1
	}
	settings := Resolve(config, overrides)
	if settings.Provider != "azure" {
		a.errorf("unsupported provider %q; use `azure`", settings.Provider)
		return 1
	}
	if settings.Kind != "" && !slices.Contains(validKinds, settings.Kind) {
		a.errorf("unknown kind %q", settings.Kind)
		return 1
	}
	bundle, err := loadStoreBundle(st)
	if err != nil {
		a.errorf("validate specs: %s", err)
		return 1
	}
	if bundle.IsEmpty() {
		a.errorf("no specs found")
		return 1
	}
	if cmd == cmdValidate {
		fmt.Fprint(a.stdout, validateLine(bundle.Len(), settings.ContentVersion))
		return 0
	}

	provider := NewAzureProvider(settings.Subscription, settings.ResourceGroup)
	account, err := provider.Ground()
	if err != nil {
		a.errorf("authenticate provider: %s", err)
		return 1
	}
	fmt.Fprint(a.stderr, groundingLine(bundle.Len(), settings.ContentVersion))
	if settings.Diagnostic {
		fmt.Fprintf(a.stderr, "attune: diagnostic tenant=%s subscription=%s identity=%s resource-group=%s %s\n",
			account.Tenant, provider.Subscription, account.Identity, settings.ResourceGroup, "store="+ref.path)
	}

	options := &Options{
		Subscription:        provider.Subscription,
		Kind:                settings.Kind,
		PruneDNS:            settings.PruneDNS,
		PruneIdentities:     settings.PruneIdentities,
		PruneRoles:          settings.PruneRoles,
		PruneResourceGroups: settings.PruneResourceGroups,
	}
	changes, err := Plan(provider, bundle, options)
	if err != nil {
		a.errorf("plan provider changes: %s", err)
		return 1
	}
	fmt.Fprint(a.stdout, renderPlanBlock(changes, settings.Verbose))

	if cmd == cmdApply {
		fmt.Fprintln(a.stdout, applyHeader())
		if err := Apply(provider, changes, provider.Subscription, func(c Change) {
			fmt.Fprintln(a.stdout, renderAppliedLine(c))
		}); err != nil {
			a.errorf("apply provider change: %s", err)
			return 1
		}
		fmt.Fprint(a.stdout, renderApplyTrailer(len(changes)))
	}
	return 0
}

// storedConfig parses the store's attune.yaml entry, or returns nil when
// the store holds none.
func storedConfig(st *lockbox.Store) (*Config, error) {
	entries, err := st.Entries()
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if e.Target != configName {
			continue
		}
		latest, ok, err := st.Latest(e.ID)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, nil
		}
		return ParseConfig(latest.Content)
	}
	return nil, nil
}

// loadStoreBundle parses every stored spec, attune.yaml excepted, exactly
// as directory mode parses files.
func loadStoreBundle(st *lockbox.Store) (*Bundle, error) {
	entries, err := st.Entries()
	if err != nil {
		return nil, err
	}
	var sources []specSource
	for _, e := range entries {
		if e.Target == configName || !isSpecKey(e.Target) {
			continue
		}
		latest, ok, err := st.Latest(e.ID)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		sources = append(sources, specSource{name: e.Target, content: latest.Content})
	}
	return loadSources(sources)
}

// changeSymbol maps an Action to its one-character change marker.
func changeSymbol(a Action) string {
	switch a {
	case ActionCreate:
		return "+"
	case ActionDelete:
		return "-"
	default:
		return "~"
	}
}

// pastTense maps an Action to the past-tense verb used on apply
// confirmation lines.
func pastTense(a Action) string {
	switch a {
	case ActionCreate:
		return "created"
	case ActionUpdate:
		return "updated"
	case ActionDelete:
		return "deleted"
	default:
		return "unknown"
	}
}

// renderPlanBlock renders the prospective change block: an uncolored
// header, one yellow line per change, and the yellow "would be made"
// trailer. With verbose set, each change's field-level diffs follow its
// line as indented yellow lines.
func renderPlanBlock(changes []Change, verbose bool) string {
	var b strings.Builder
	b.WriteString("attune plan: provider=azure\n")
	for _, c := range changes {
		line := fmt.Sprintf("  %s %-6s %-15s %s  %s", changeSymbol(c.Action), c.Action.String(), c.Kind, c.Key, c.Summary)
		b.WriteString(color.Yel5(line) + "\n")
		if verbose {
			for _, d := range c.Diffs {
				b.WriteString(color.Yel5("      "+renderFieldDiff(d)) + "\n")
			}
		}
	}
	b.WriteString("\n" + color.Yel5(fmt.Sprintf("%d change(s) would be made.", len(changes))) + "\n")
	return b.String()
}

// renderFieldDiff renders one field-level difference for verbose output:
// scalar changes as `field: old -> new`, set-valued changes as the added
// or removed entries alone.
func renderFieldDiff(d FieldDiff) string {
	switch {
	case d.Old == "":
		return fmt.Sprintf("%s: %s", d.Field, d.New)
	case d.New == "":
		return fmt.Sprintf("%s: %s", d.Field, d.Old)
	default:
		return fmt.Sprintf("%s: %s -> %s", d.Field, d.Old, d.New)
	}
}

// applyHeader renders the green header that opens the apply confirmation
// block.
func applyHeader() string {
	return color.Grn5("attune apply: provider=azure")
}

// renderAppliedLine renders one green confirmation line, mirroring the
// plan-line columns with the past-tense verb in place of the action.
func renderAppliedLine(c Change) string {
	return color.Grn5(fmt.Sprintf("  %s %-7s %-15s %s  %s", changeSymbol(c.Action), pastTense(c.Action), c.Kind, c.Key, c.Summary))
}

// renderApplyTrailer renders the green trailer printed after every change
// applied successfully.
func renderApplyTrailer(applied int) string {
	return "\n" + color.Grn5(fmt.Sprintf("%d change(s) made.", applied)) + "\n"
}

// contentSuffix renders the optional ` content=<value>` output suffix for a
// declared spec content version.
func contentSuffix(contentVersion string) string {
	if contentVersion == "" {
		return ""
	}
	return " content=" + contentVersion
}

// validateLine renders the `attune validate` success line, appending the
// declared content version when one is set.
func validateLine(specs int, contentVersion string) string {
	return fmt.Sprintf("attune validate: OK (%d specs)%s\n", specs, contentSuffix(contentVersion))
}

// groundingLine renders the non-secret grounding line every live run
// prints, appending the declared content version when one is set.
func groundingLine(specs int, contentVersion string) string {
	return fmt.Sprintf("attune: provider=azure specs=%d authenticated=yes%s\n", specs, contentSuffix(contentVersion))
}

// parseFlags parses the reconciler flag surface. String flags (-t/-P/-g/-S/
// -k) consume the next argument unless given inline as flag=value.
// Boolean flags (-r/-I/-R/-G) accept bare form (true) or flag=true/false.
func parseFlags(args []string) (Overrides, error) {
	var overrides Overrides
	for i := 0; i < len(args); i++ {
		arg := args[i]
		name := arg
		var inline *string
		if before, after, ok := strings.Cut(arg, "="); ok {
			name = before
			v := after
			inline = &v
		}
		value := func() (string, error) {
			if inline != nil {
				return *inline, nil
			}
			i++
			if i >= len(args) {
				return "", fmt.Errorf("%s requires a value", name)
			}
			return args[i], nil
		}
		switch name {
		case "-t", "--store":
			v, err := value()
			if err != nil {
				return overrides, err
			}
			overrides.Store = &v
		case "-P", "--provider":
			v, err := value()
			if err != nil {
				return overrides, err
			}
			overrides.Provider = &v
		case "-g", "--resource-group":
			v, err := value()
			if err != nil {
				return overrides, err
			}
			overrides.ResourceGroup = &v
		case "-S", "--subscription":
			v, err := value()
			if err != nil {
				return overrides, err
			}
			overrides.Subscription = &v
		case "-k", "--kind":
			v, err := value()
			if err != nil {
				return overrides, err
			}
			overrides.Kind = v
		case "-r", "--prune":
			b, err := boolValue(inline)
			if err != nil {
				return overrides, err
			}
			overrides.PruneDNS = &b
		case "-I", "--prune-identities":
			b, err := boolValue(inline)
			if err != nil {
				return overrides, err
			}
			overrides.PruneIdentities = &b
		case "-R", "--prune-roles":
			b, err := boolValue(inline)
			if err != nil {
				return overrides, err
			}
			overrides.PruneRoles = &b
		case "-G", "--prune-resource-groups":
			b, err := boolValue(inline)
			if err != nil {
				return overrides, err
			}
			overrides.PruneResourceGroups = &b
		case "-d", "--diagnostic":
			if inline != nil {
				return overrides, fmt.Errorf("%s does not take a value", name)
			}
			overrides.Diagnostic = true
		case "-V", "--verbose":
			if inline != nil {
				return overrides, fmt.Errorf("%s does not take a value", name)
			}
			overrides.Verbose = true
		default:
			return overrides, fmt.Errorf("unknown flag %q", name)
		}
	}
	return overrides, nil
}

func boolValue(inline *string) (bool, error) {
	if inline == nil {
		return true, nil
	}
	switch *inline {
	case "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return false, fmt.Errorf("boolean flag value must be true or false")
	}
}

func isVersionArg(a string) bool { return a == "-v" || a == "--version" || a == "v" || a == "version" }
func isHelpArg(a string) bool    { return a == "-h" || a == "--help" || a == "h" || a == "help" }

// heading renders a help heading in bold white, like the name on line one.
func heading(s string) string { return color.Bold(color.Gra10(s)) }

// helpLine renders one aligned help line: a command or flag form, then its meaning.
func helpLine(form, meaning string) string { return fmt.Sprintf("  %-36s%s\n", form, meaning) }

func usage() string {
	return heading("attune") + " v" + programVersion + "\n" +
		color.Gra5("Reconcile Azure state from YAML specs kept in an encrypted store.") + "\n\n" +
		heading("Overview") + "\n" +
		"  The specs live in one sealed store file that attune manages itself: init\n" +
		"  creates or unlocks it, add captures spec files, edit changes a stored spec\n" +
		"  in your editor, and render writes a browsable copy. validate checks the\n" +
		"  stored specs offline, plan reads live Azure state and lists the changes,\n" +
		"  and apply makes them. The key lives in the login keychain, with a\n" +
		"  passphrase-wrapped copy in the store for other Macs.\n\n" +
		heading("Usage") + "\n" +
		helpLine("attune (c|validate) [flags]", "check the stored specs offline") +
		helpLine("attune (p|plan) [flags]", "read live state and show the changes") +
		helpLine("attune (a|apply) [flags]", "create, update, and permitted prune operations") +
		helpLine("attune init [-N] [-t PATH]", "unlock an existing store, or create one with -N") +
		helpLine("attune st", "status of the store, key, and entries") +
		helpLine("attune add DIR | NAME [FILE]", "import a directory of specs, or store one from a file or stdin") +
		helpLine("attune edit NAME", "change a stored spec in $EDITOR and save a validated version") +
		helpLine("attune rename OLD NEW", "change an entry's name, keeping its versions") +
		helpLine("attune rm NAME", "forget an entry and its stored versions") +
		helpLine("attune ls [-b FIELD]", "list entries by name, or by captured") +
		helpLine("attune cat NAME", "print one stored spec") +
		helpLine("attune render [-o DIR] [-a] [-f]", "write the stored specs into a browsable directory") +
		helpLine("attune key show", "store path, key id, keychain and store state") +
		helpLine("attune key restore", "put the key back in the keychain with the passphrase") +
		helpLine("attune key rm [-f]", "delete the keychain item after a prompt") +
		helpLine("attune key passphrase", "change the recovery passphrase") +
		helpLine("attune (v|version)", "print attune v"+programVersion) +
		helpLine("attune (h|help)", "show this help") + "\n" +
		heading("Options") + "\n" +
		helpLine("-t, --store PATH", "Store file for this command; init -t also remembers it") +
		helpLine("-P, --provider NAME", "Provider name; azure is the only one") +
		helpLine("-S, --subscription ID", "Azure subscription for live commands") +
		helpLine("-g, --resource-group NAME", "Azure resource group for live commands") +
		helpLine("-k, --kind KIND", "Limit plan and apply to one spec kind") +
		helpLine("-r, --prune[=BOOL]", "Delete unmanaged DNS records; on by default") +
		helpLine("-I, --prune-identities[=BOOL]", "Delete unmanaged groups and app registrations; off by default") +
		helpLine("-R, --prune-roles[=BOOL]", "Delete unmanaged role definitions and assignments; off by default") +
		helpLine("-G, --prune-resource-groups[=BOOL]", "Delete unmanaged resource groups; off by default") +
		helpLine("-d, --diagnostic", "Print non-secret account and target grounding on live commands") +
		helpLine("-V, --verbose", "Field-level detail on plan and apply") +
		helpLine("-N, --new", "Create a new store (init)") +
		helpLine("-f, --force", "Render into a non-empty directory; skip the key rm prompt") +
		helpLine("-o, --output DIR", "Render into DIR instead of a fresh private temp directory (render)") +
		helpLine("-a, --all", "Render every stored version too, under versions/ (render)") +
		helpLine("-b, --by FIELD", "Sort ls by name (the default) or captured, newest first") +
		helpLine("-v, --version", "Print attune v"+programVersion+" and exit") +
		helpLine("-h, --help", "Show this help message and exit") + "\n" +
		heading("Notes") + "\n" +
		"  Store path order: -t, then ATTUNE_STORE, then the path init -t remembered in\n" +
		"  $XDG_CONFIG_HOME/attune/store, then $XDG_DATA_HOME/attune/attune.store.\n" +
		"  A NAME is a relative path ending in .yaml or .yml (res/dns/zone.yaml), or\n" +
		"  attune.yaml for the configuration. add and edit validate before saving.\n" +
		"  Drift against Azure is what plan reports; st never contacts Azure.\n" +
		"  Live commands need an authenticated Azure CLI (az login). init and edit\n" +
		"  need a terminal. The store is macOS only.\n" +
		"  render writes plaintext copies of the store; delete the directory when done.\n"
}
