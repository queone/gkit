package main

import (
	"fmt"
	"os"
	"slices"
	"sort"
	"strings"
)

const programVersion = "1.0.0"

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

func main() {
	os.Exit(run(os.Args[1:]))
}

// run dispatches attune's subcommands and returns the process exit code.
func run(args []string) int {
	if len(args) == 1 && isVersionArg(args[0]) {
		fmt.Printf("attune %s\n", programVersion)
		return 0
	}
	if len(args) == 0 || (len(args) == 1 && isHelpArg(args[0])) {
		fmt.Print(usage())
		return 0
	}

	var cmd commandKind
	switch args[0] {
	case "p", "plan":
		cmd = cmdPlan
	case "a", "apply":
		cmd = cmdApply
	case "c", "validate":
		cmd = cmdValidate
	default:
		fmt.Fprintf(os.Stderr, "attune: unknown command %q; run `attune help`\n", args[0])
		return 2
	}

	overrides, err := parseFlags(args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "attune: %s; run `attune help`\n", err)
		return 2
	}

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "attune: resolve current directory: %s\n", err)
		return 1
	}
	found, err := Find(cwd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "attune: %s\n", err)
		return 1
	}
	settings := Resolve(found, overrides)
	if settings.Provider != "azure" {
		fmt.Fprintf(os.Stderr, "attune: unsupported provider %q; use `azure`\n", settings.Provider)
		return 1
	}
	if settings.Kind != "" && !slices.Contains(validKinds, settings.Kind) {
		fmt.Fprintf(os.Stderr, "attune: unknown kind %q\n", settings.Kind)
		return 1
	}
	bundle, err := Load(settings.Specs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "attune: validate specs: %s\n", err)
		return 1
	}
	if bundle.IsEmpty() {
		fmt.Fprintln(os.Stderr, "attune: no specs found")
		return 1
	}
	if cmd == cmdValidate {
		fmt.Printf("attune validate: OK (%d specs)\n", bundle.Len())
		return 0
	}

	provider := NewAzureProvider(settings.Subscription, settings.ResourceGroup)
	account, err := provider.Ground()
	if err != nil {
		fmt.Fprintf(os.Stderr, "attune: authenticate provider: %s\n", err)
		return 1
	}
	fmt.Fprintf(os.Stderr, "attune: provider=azure specs=%d authenticated=yes\n", bundle.Len())
	if settings.Diagnostic {
		fmt.Fprintf(os.Stderr, "attune: diagnostic tenant=%s subscription=%s identity=%s resource-group=%s specs=%s\n",
			account.Tenant, provider.Subscription, account.Identity, settings.ResourceGroup, settings.Specs)
	}

	if cmd == cmdApply {
		zoneSet := map[string]bool{}
		var zones []string
		for _, item := range bundle.Dns {
			if settings.Kind == "" || settings.Kind == "dnsRecordSet" {
				if !zoneSet[item.Zone] {
					zoneSet[item.Zone] = true
					zones = append(zones, item.Zone)
				}
			}
		}
		sort.Strings(zones)
		for _, zone := range zones {
			if err := provider.EnsureZone(zone); err != nil {
				fmt.Fprintf(os.Stderr, "attune: ensure DNS zone: %s\n", err)
				return 1
			}
		}
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
		fmt.Fprintf(os.Stderr, "attune: plan provider changes: %s\n", err)
		return 1
	}
	fmt.Println("attune plan: provider=azure")
	for _, c := range changes {
		symbol := "~"
		switch c.Action {
		case ActionCreate:
			symbol = "+"
		case ActionDelete:
			symbol = "-"
		}
		fmt.Printf("  %s %-6s %-15s %s  %s\n", symbol, c.Action.String(), c.Kind, c.Key, c.Summary)
	}
	fmt.Printf("\n%d change(s).\n", len(changes))

	if cmd == cmdApply {
		if err := Apply(provider, changes, provider.Subscription); err != nil {
			fmt.Fprintf(os.Stderr, "attune: apply provider change: %s\n", err)
			return 1
		}
	}
	return 0
}

// parseFlags parses attune's flag surface. String flags (-s/-P/-g/-S/-k)
// consume the next argument unless given inline as flag=value. Boolean
// flags (-r/-I/-R/-G) accept bare form (true) or flag=true/false.
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
		case "-s", "--specs":
			v, err := value()
			if err != nil {
				return overrides, err
			}
			overrides.Specs = &v
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

func usage() string {
	return "attune — reconcile provider state with neutral YAML specs.\n\n" +
		"Usage:\n" +
		"  attune (p|plan) [flags]\n" +
		"  attune (a|apply) [flags]\n" +
		"  attune (c|validate) [flags]\n" +
		"  attune (v|version)\n" +
		"  attune (h|help)\n\n" +
		"Flags:\n" +
		"  -s, --specs PATH\n" +
		"  -P, --provider NAME\n" +
		"  -g, --resource-group NAME\n" +
		"  -S, --subscription ID\n" +
		"  -k, --kind KIND\n" +
		"  -r, --prune[=BOOL]\n" +
		"  -I, --prune-identities[=BOOL]\n" +
		"  -R, --prune-roles[=BOOL]\n" +
		"  -G, --prune-resource-groups[=BOOL]\n" +
		"  -d, --diagnostic\n\n" +
		"Live commands require an authenticated Azure CLI (`az login`).\n"
}
