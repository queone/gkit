// modules.go

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/go-tfe"
	"github.com/queone/gkit/color"
)

// unknownDate is printed when a module version cannot be read.
const unknownDate = "<updated_at?>"

// fmtDate renders an RFC 3339 timestamp as 2006-Jan-02 15:04, or returns it unchanged.
func fmtDate(s string) string {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return s
	}
	return t.Format("2006-Jan-02 15:04")
}

// moduleAddress renders the registry address of a module under host.
func moduleAddress(host string, m *tfe.RegistryModule) string {
	return host + "/" + m.Namespace + "/" + m.Name + "/" + m.Provider
}

// moduleID builds the identifier go-tfe needs to read one module's versions.
func moduleID(org string, m *tfe.RegistryModule) tfe.RegistryModuleID {
	if m.RegistryName == tfe.PrivateRegistry {
		return tfe.NewPrivateRegistryModuleID(org, m.Name, m.Provider)
	}
	return tfe.NewPublicRegistryModuleID(org, m.Namespace, m.Name, m.Provider)
}

// latestIndex returns the index of the highest version in m, or -1 when it has none.
func latestIndex(m *tfe.RegistryModule) int {
	best := -1
	for i, v := range m.VersionStatuses {
		if best < 0 || compareVersions(v.Version, m.VersionStatuses[best].Version) > 0 {
			best = i
		}
	}
	return best
}

// listAllModules walks every page of the organization's registry modules.
func listAllModules(ctx context.Context, a api, org string) ([]*tfe.RegistryModule, error) {
	var all []*tfe.RegistryModule
	for page := 1; page != 0; {
		items, nextPage, err := a.ListModules(ctx, org, page)
		if err != nil {
			return nil, fmt.Errorf("listing modules for organization %s: %w", org, err)
		}
		all = append(all, items...)
		page = nextPage
	}
	return all, nil
}

// readVersion fetches one module version, or nil when it cannot be read.
func readVersion(ctx context.Context, a api, org string, m *tfe.RegistryModule, version string) *tfe.RegistryModuleVersion {
	v, err := a.ReadModuleVersion(ctx, moduleID(org, m), version)
	if err != nil {
		return nil
	}
	return v
}

// versionLine prints one module version row: address, version, updated date.
func versionLine(ctx context.Context, a api, org, host string, m *tfe.RegistryModule, version string, w io.Writer) {
	updated := unknownDate
	if v := readVersion(ctx, a, org, m, version); v != nil {
		updated = fmtDate(v.UpdatedAt)
	}
	fmt.Fprintf(w, "%-80s %-10s %s\n", moduleAddress(host, m), version, updated)
}

// listModules prints the modules matching filter: every version with all, the
// latest version otherwise, and the detail block or JSON when exactly one matches.
func listModules(ctx context.Context, a api, org, host, filter string, all, asJSON bool, w io.Writer) error {
	mods, err := listAllModules(ctx, a, org)
	if err != nil {
		return err
	}
	var matched []*tfe.RegistryModule
	for _, m := range mods {
		if matches(m.Name, filter) {
			matched = append(matched, m)
		}
	}
	switch {
	case len(matched) == 0:
		return nil
	case len(matched) == 1 && asJSON:
		b, err := json.MarshalIndent(matched[0], "", "  ")
		if err != nil {
			return fmt.Errorf("rendering module %s as JSON: %w", matched[0].Name, err)
		}
		fmt.Fprintln(w, string(b))
		return nil
	case len(matched) == 1:
		printModuleDetail(ctx, a, org, matched[0], w)
		return nil
	}
	for _, m := range matched {
		if all {
			for _, v := range m.VersionStatuses {
				versionLine(ctx, a, org, host, m, v.Version, w)
			}
			continue
		}
		if i := latestIndex(m); i >= 0 {
			versionLine(ctx, a, org, host, m, m.VersionStatuses[i].Version, w)
		}
	}
	return nil
}

// printModuleDetail prints the main attributes of one module in a YAML-like layout.
func printModuleDetail(ctx context.Context, a api, org string, m *tfe.RegistryModule, w io.Writer) {
	k, v := color.Blu8, color.Grn5
	fmt.Fprintln(w, color.Gra5("# Terraform Cloud Registry Module"))
	fmt.Fprintf(w, "%s: %s\n", k("id"), v(m.ID))
	fmt.Fprintf(w, "%s: %s\n", k("name"), v(m.Name))
	fmt.Fprintf(w, "%s: %s\n", k("provider"), v(m.Provider))
	fmt.Fprintf(w, "%s: %s\n", k("namespace"), v(m.Namespace))
	fmt.Fprintf(w, "%s: %s\n", k("created_at"), v(fmtDate(m.CreatedAt)))
	fmt.Fprintf(w, "%s: %s\n", k("updated_at"), v(fmtDate(m.UpdatedAt)))
	if m.VCSRepo != nil {
		fmt.Fprintf(w, "%s: %s\n", k("repo_url"), v(m.VCSRepo.RepositoryHTTPURL))
	}
	if len(m.VersionStatuses) == 0 {
		return
	}
	fmt.Fprintf(w, "%s:\n", k("versions"))
	for _, vs := range m.VersionStatuses {
		id, created, updated := "<id?>", "<created_at?>", unknownDate
		if ver := readVersion(ctx, a, org, m, vs.Version); ver != nil {
			id, created, updated = ver.ID, fmtDate(ver.CreatedAt), fmtDate(ver.UpdatedAt)
		}
		fmt.Fprintf(w, "  %-26s %-8s %-20s %s\n", id, vs.Version, created, updated)
	}
}

// compareVersions orders dotted version strings numerically part by part, so
// 1.10.0 sorts after 1.9.0. A missing part counts as 0, and a pre-release tag
// such as 1.0.0-rc1 sorts before the release it precedes.
func compareVersions(a, b string) int {
	pa := strings.Split(strings.TrimPrefix(a, "v"), ".")
	pb := strings.Split(strings.TrimPrefix(b, "v"), ".")
	for i := 0; i < len(pa) || i < len(pb); i++ {
		x, y := "0", "0"
		if i < len(pa) {
			x = pa[i]
		}
		if i < len(pb) {
			y = pb[i]
		}
		nx, px := splitPre(x)
		ny, py := splitPre(y)
		if nx != ny {
			if nx < ny {
				return -1
			}
			return 1
		}
		if px != py {
			switch {
			case px == "":
				return 1
			case py == "":
				return -1
			default:
				return strings.Compare(px, py)
			}
		}
	}
	return 0
}

// splitPre splits a version part such as "0-rc1" into its number and tag; a
// part with no leading number counts as 0 with the whole part as its tag.
func splitPre(part string) (int, string) {
	num, pre, _ := strings.Cut(part, "-")
	n, err := strconv.Atoi(num)
	if err != nil {
		return 0, part
	}
	return n, pre
}
