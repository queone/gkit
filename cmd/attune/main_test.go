package main

import (
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/queone/gkit/internal/color"
)

// captureRun invokes run(args) in-process, capturing stdout/stderr by
// swapping the package-level os.Stdout/os.Stderr vars around the call.
func captureRun(t *testing.T, args []string) (code int, stdout, stderr string) {
	t.Helper()
	origOut, origErr := os.Stdout, os.Stderr
	ro, wo, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	re, we, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout, os.Stderr = wo, we
	defer func() { os.Stdout, os.Stderr = origOut, origErr }()

	code = run(args)

	wo.Close()
	we.Close()
	outBytes, _ := io.ReadAll(ro)
	errBytes, _ := io.ReadAll(re)
	return code, string(outBytes), string(errBytes)
}

func TestVersionOutput(t *testing.T) {
	for _, args := range [][]string{{"-v"}, {"--version"}, {"v"}, {"version"}} {
		code, out, errOut := captureRun(t, args)
		if code != 0 {
			t.Errorf("%v: code = %d, want 0", args, code)
		}
		want := "attune " + programVersion + "\n"
		if out != want {
			t.Errorf("%v: stdout = %q, want %q", args, out, want)
		}
		if errOut != "" {
			t.Errorf("%v: stderr not empty: %q", args, errOut)
		}
	}
}

func TestHelpOutput(t *testing.T) {
	for _, args := range [][]string{{}, {"-h"}, {"--help"}, {"h"}, {"help"}} {
		code, out, _ := captureRun(t, args)
		if code != 0 {
			t.Errorf("%v: code = %d, want 0", args, code)
		}
		for _, want := range []string{"(p|plan)", "-d, --diagnostic"} {
			if !strings.Contains(out, want) {
				t.Errorf("%v: help output missing %q", args, want)
			}
		}
	}
}

func TestValidateOffline(t *testing.T) {
	before, err := os.ReadDir("testdata/specs")
	if err != nil {
		t.Fatal(err)
	}
	code, out, errOut := captureRun(t, []string{"validate", "--specs", "testdata/specs"})
	if code != 0 {
		t.Fatalf("code = %d, stderr = %q", code, errOut)
	}
	want := "attune validate: OK (6 specs)\n"
	if out != want {
		t.Errorf("stdout = %q, want %q", out, want)
	}
	after, err := os.ReadDir("testdata/specs")
	if err != nil {
		t.Fatal(err)
	}
	if len(before) != len(after) {
		t.Errorf("validate changed the spec directory: before=%d after=%d", len(before), len(after))
	}
}

func TestInvalidInputsIncludeRecoveryGuidance(t *testing.T) {
	code, _, errOut := captureRun(t, []string{"unknown"})
	if code != 2 {
		t.Errorf("code = %d, want 2", code)
	}
	if !strings.Contains(errOut, "attune help") {
		t.Errorf("stderr = %q, missing recovery guidance", errOut)
	}

	code, _, errOut = captureRun(t, []string{"validate", "--unknown"})
	if code != 2 {
		t.Errorf("code = %d, want 2", code)
	}
	if !strings.Contains(errOut, "attune help") {
		t.Errorf("stderr = %q, missing recovery guidance", errOut)
	}
}

// validateSyntheticSpec writes yaml as the only spec file in a fresh temp
// directory and runs `validate` against it, asserting no files were
// created or removed as a side effect.
func validateSyntheticSpec(t *testing.T, yaml string) (code int, stdout, stderr string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "role-assignment.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr = captureRun(t, []string{"validate", "--specs", dir})
	after, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(before) != len(after) {
		t.Errorf("validate changed the spec directory")
	}
	return code, stdout, stderr
}

func TestLegacyPrincipalFormsValidateOfflineWithoutArtifacts(t *testing.T) {
	code, _, errOut := validateSyntheticSpec(t,
		"kind: roleAssignment\nprincipal: synthetic-group\nprincipalType: SeCuRiTyGrOuP\nrole: synthetic-role\nscope:\n  resourceGroup: synthetic-resources\n")
	if code != 0 {
		t.Errorf("securityGroup alias: code = %d, stderr = %q", code, errOut)
	}

	code, _, errOut = validateSyntheticSpec(t,
		"kind: roleAssignment\nprincipal: 00000000-0000-0000-0000-000000000004\nrole: synthetic-role\nscope:\n  resourceGroup: synthetic-resources\n")
	if code != 0 {
		t.Errorf("literal id: code = %d, stderr = %q", code, errOut)
	}
}

func TestNamedPrincipalWithoutTypeHasRecoveryGuidance(t *testing.T) {
	code, _, errOut := validateSyntheticSpec(t,
		"kind: roleAssignment\nprincipal: synthetic-group\nrole: synthetic-role\nscope:\n  resourceGroup: synthetic-resources\n")
	if code != 1 {
		t.Fatalf("code = %d, want 1 (stderr=%q)", code, errOut)
	}
	for _, principalType := range []string{"group", "securityGroup", "servicePrincipal", "user"} {
		if !strings.Contains(errOut, principalType) {
			t.Errorf("stderr = %q, missing mention of %q", errOut, principalType)
		}
	}
}

// TestInvalidSpecInputExitsOneWithRecoveryGuidance covers AT4's remaining
// named categories (named-principal-without-type is covered separately
// above): unknown kind, malformed YAML, and a missing required field.
// Confirms exit 1 (not 2 — spec-content errors, not CLI-argument errors)
// with the "attune: validate specs:" contextual prefix.
func TestInvalidSpecInputExitsOneWithRecoveryGuidance(t *testing.T) {
	cases := []struct {
		name string
		yaml string
	}{
		{"unknown_kind", "kind: bogusKind\nname: x\n"},
		{"malformed_yaml", "kind: [unterminated\n"},
		{"missing_required_field", "kind: securityGroup\nowners: []\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, _, errOut := validateSyntheticSpec(t, tc.yaml)
			if code != 1 {
				t.Errorf("code = %d, want 1 (stderr=%q)", code, errOut)
			}
			if !strings.Contains(errOut, "attune: validate specs:") {
				t.Errorf("stderr = %q, missing contextual recovery guidance prefix", errOut)
			}
		})
	}
}

const publicExampleRoot = "../../examples/attune"

func TestPublicExampleValidatesOfflineWithoutWrites(t *testing.T) {
	before := collectFileNames(t, publicExampleRoot)
	code, out, errOut := captureRun(t, []string{"validate", "-s", filepath.Join(publicExampleRoot, "specs")})
	if code != 0 {
		t.Fatalf("code = %d, stderr = %q", code, errOut)
	}
	want := "attune validate: OK (6 specs)\n"
	if out != want {
		t.Errorf("stdout = %q, want %q", out, want)
	}
	after := collectFileNames(t, publicExampleRoot)
	if len(before) != len(after) {
		t.Errorf("validate modified the example bundle")
	}
}

func TestPublicExampleHasOneOfEverySupportedKind(t *testing.T) {
	bundle, err := Load(filepath.Join(publicExampleRoot, "specs"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if bundle.Len() != 6 {
		t.Errorf("bundle.Len() = %d, want 6", bundle.Len())
	}
	counts := map[string]int{
		"dns": len(bundle.Dns), "groups": len(bundle.Groups), "apps": len(bundle.Apps),
		"roleDefinitions": len(bundle.RoleDefinitions), "roleAssignments": len(bundle.RoleAssignments),
		"resourceGroups": len(bundle.ResourceGroups),
	}
	for name, count := range counts {
		if count != 1 {
			t.Errorf("%s count = %d, want 1", name, count)
		}
	}
}

// TestPublicExampleContainsOnlySafeDocumentationValues — AT15.
func TestPublicExampleContainsOnlySafeDocumentationValues(t *testing.T) {
	forbidden := []string{
		"/users/", "/home/", "c:\\",
		"-----begin certificate-----", "-----begin private key-----", "-----begin rsa private key-----",
	}
	err := filepath.WalkDir(publicExampleRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		lower := strings.ToLower(string(content))
		for _, f := range forbidden {
			if strings.Contains(lower, f) {
				t.Errorf("%s: unsafe content %q found", path, f)
			}
		}
		for line := range strings.SplitSeq(lower, "\n") {
			name, value, ok := strings.Cut(line, ":")
			if !ok {
				continue
			}
			for _, word := range []string{"password", "secret", "token", "credential"} {
				if strings.Contains(name, word) && strings.TrimSpace(value) != "" {
					t.Errorf("%s: secret-shaped assignment %q", path, line)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	bundle, err := Load(filepath.Join(publicExampleRoot, "specs"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	for _, record := range bundle.Dns {
		if !isReservedName(record.Zone) {
			t.Errorf("DNS zone %q is not an RFC 2606 reserved name", record.Zone)
		}
		switch strings.ToUpper(record.RecordType) {
		case "CNAME", "MX", "NS":
			for _, value := range record.Values {
				fields := strings.Fields(value)
				host := ""
				if len(fields) > 0 {
					host = fields[len(fields)-1]
				}
				host = strings.TrimSuffix(host, ".")
				if !isReservedName(host) {
					t.Errorf("DNS value %q does not target an RFC 2606 reserved name", value)
				}
			}
		}
	}
}

func isReservedName(name string) bool {
	name = strings.ToLower(strings.TrimSuffix(name, "."))
	if slices.Contains([]string{"example", "example.com", "example.net", "example.org"}, name) {
		return true
	}
	for _, suffix := range []string{".example", ".example.com", ".example.net", ".example.org"} {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	return false
}

func collectFileNames(t *testing.T, root string) []string {
	t.Helper()
	var names []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			names = append(names, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(names)
	return names
}

// yellowCode/greenCode are the raw SGR prefixes color.Yel5/color.Grn5 emit,
// asserted directly so a palette change fails loudly here.
const (
	yellowCode = "\x1b[38;5;220m"
	greenCode  = "\x1b[38;5;46m"
)

func TestPlanBlockRendersYellowProspectiveWording(t *testing.T) {
	restore := color.SetEnabled(true)
	defer restore()
	changes := []Change{
		{Kind: "dnsRecordSet", Action: ActionCreate, Key: "dnsRecordSet|www", Summary: "missing"},
		{Kind: "securityGroup", Action: ActionUpdate, Key: "securityGroup|eng", Summary: "owners or members differ"},
		{Kind: "appRegistration", Action: ActionDelete, Key: "appRegistration|old", Summary: "absent from specs"},
	}
	out := renderPlanBlock(changes, false)
	lines := strings.Split(out, "\n")
	if lines[0] != "attune plan: provider=azure" {
		t.Errorf("header = %q, want uncolored %q", lines[0], "attune plan: provider=azure")
	}
	for i := 1; i <= len(changes); i++ {
		if !strings.HasPrefix(lines[i], yellowCode) {
			t.Errorf("change line %d = %q, want yellow prefix", i, lines[i])
		}
	}
	if !strings.Contains(out, color.Yel5("3 change(s) would be made.")) {
		t.Errorf("output %q missing yellow prospective trailer", out)
	}
	plain := color.ClearCode(out)
	for _, want := range []string{"+ create", "~ update", "- delete", "dnsRecordSet|www", "missing"} {
		if !strings.Contains(plain, want) {
			t.Errorf("plain output %q missing %q", plain, want)
		}
	}
	if zero := color.ClearCode(renderPlanBlock(nil, false)); !strings.Contains(zero, "0 change(s) would be made.") {
		t.Errorf("zero-change plan = %q, want prospective zero trailer", zero)
	}
}

func TestApplyConfirmationRendersGreenPastTense(t *testing.T) {
	restore := color.SetEnabled(true)
	defer restore()
	if got := color.ClearCode(applyHeader()); got != "attune apply: provider=azure" {
		t.Errorf("applyHeader = %q, want %q", got, "attune apply: provider=azure")
	}
	if !strings.HasPrefix(applyHeader(), greenCode) {
		t.Errorf("applyHeader = %q, want green prefix", applyHeader())
	}
	verbs := map[Action]string{ActionCreate: "created", ActionUpdate: "updated", ActionDelete: "deleted"}
	for action, verb := range verbs {
		c := Change{Kind: "dnsRecordSet", Action: action, Key: "dnsRecordSet|www", Summary: "missing"}
		line := renderAppliedLine(c)
		if !strings.HasPrefix(line, greenCode) {
			t.Errorf("%s line = %q, want green prefix", verb, line)
		}
		fields := strings.Fields(color.ClearCode(line))
		wantFields := []string{changeSymbol(action), verb, c.Kind, c.Key, c.Summary}
		if !slices.Equal(fields, wantFields) {
			t.Errorf("%s line fields = %v, want %v", verb, fields, wantFields)
		}
	}
	trailer := renderApplyTrailer(3)
	if color.ClearCode(trailer) != "\n3 change(s) made.\n" {
		t.Errorf("trailer = %q, want %q", color.ClearCode(trailer), "\n3 change(s) made.\n")
	}
	if !strings.Contains(trailer, greenCode) {
		t.Errorf("trailer = %q, want green wrapping", trailer)
	}
}

func TestRenderingUncoloredWhenColorDisabled(t *testing.T) {
	restore := color.SetEnabled(false)
	defer restore()
	change := Change{Kind: "dnsRecordSet", Action: ActionCreate, Key: "dnsRecordSet|www", Summary: "missing",
		Diffs: []FieldDiff{{Field: "ttl", Old: "300", New: "600"}}}
	for name, out := range map[string]string{
		"plan block":    renderPlanBlock([]Change{change}, true),
		"apply header":  applyHeader(),
		"applied line":  renderAppliedLine(change),
		"apply trailer": renderApplyTrailer(1),
	} {
		if color.ClearCode(out) != out {
			t.Errorf("%s = %q, want no ANSI escapes when color disabled", name, out)
		}
	}
}

func TestZeroChangeApplyTrailerHasNoChangeLines(t *testing.T) {
	restore := color.SetEnabled(true)
	defer restore()
	got := color.ClearCode(applyHeader() + "\n" + renderApplyTrailer(0))
	want := "attune apply: provider=azure\n\n0 change(s) made.\n"
	if got != want {
		t.Errorf("zero-change apply block = %q, want %q", got, want)
	}
}

func TestGroundingAndValidateLinesAppendContentVersion(t *testing.T) {
	if got, want := groundingLine(6, "v1.2.3"), "attune: provider=azure specs=6 authenticated=yes content=v1.2.3\n"; got != want {
		t.Errorf("groundingLine with version = %q, want %q", got, want)
	}
	if got, want := groundingLine(6, ""), "attune: provider=azure specs=6 authenticated=yes\n"; got != want {
		t.Errorf("groundingLine without version = %q, want %q", got, want)
	}
	if got, want := validateLine(6, "v1.2.3"), "attune validate: OK (6 specs) content=v1.2.3\n"; got != want {
		t.Errorf("validateLine with version = %q, want %q", got, want)
	}
	if got, want := validateLine(6, ""), "attune validate: OK (6 specs)\n"; got != want {
		t.Errorf("validateLine without version = %q, want %q", got, want)
	}
}

// validateWithConfig runs `attune validate` from a temp directory holding
// the given attune.yaml content and one valid DNS spec.
func validateWithConfig(t *testing.T, config string) (code int, stdout, stderr string) {
	t.Helper()
	dir := t.TempDir()
	specs := filepath.Join(dir, "specs")
	if err := os.MkdirAll(specs, 0o755); err != nil {
		t.Fatal(err)
	}
	spec := "kind: dnsRecordSet\nzone: example.com\ntype: A\nname: www\nttl: 300\nvalues:\n  - 192.0.2.10\n"
	if err := os.WriteFile(filepath.Join(specs, "dns.yaml"), []byte(spec), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "attune.yaml"), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	return captureRun(t, []string{"validate"})
}

func TestValidateOutputIncludesDeclaredContentVersion(t *testing.T) {
	code, out, errOut := validateWithConfig(t, "provider: azure\nspecs: specs\ncontent_version: v1.2.3\n")
	if code != 0 {
		t.Fatalf("code = %d, stderr = %q", code, errOut)
	}
	if want := "attune validate: OK (1 specs) content=v1.2.3\n"; out != want {
		t.Errorf("stdout = %q, want %q", out, want)
	}
}

func TestValidateOutputUnchangedWithoutContentVersion(t *testing.T) {
	code, out, errOut := validateWithConfig(t, "provider: azure\nspecs: specs\n")
	if code != 0 {
		t.Fatalf("code = %d, stderr = %q", code, errOut)
	}
	if want := "attune validate: OK (1 specs)\n"; out != want {
		t.Errorf("stdout = %q, want %q", out, want)
	}
}

func TestDnsZoneCreateRendersStandardCreatedLine(t *testing.T) {
	restore := color.SetEnabled(true)
	defer restore()
	line := renderAppliedLine(Change{Kind: "dnsZone", Action: ActionCreate, Key: "dnsZone|example.com", Summary: "missing"})
	if !strings.HasPrefix(line, greenCode) {
		t.Errorf("line = %q, want green prefix", line)
	}
	fields := strings.Fields(color.ClearCode(line))
	want := []string{"+", "created", "dnsZone", "dnsZone|example.com", "missing"}
	if !slices.Equal(fields, want) {
		t.Errorf("fields = %v, want %v", fields, want)
	}
}

func TestVerboseFlagParses(t *testing.T) {
	for _, args := range [][]string{{"-V"}, {"--verbose"}} {
		overrides, err := parseFlags(args)
		if err != nil {
			t.Fatalf("%v: parseFlags: %v", args, err)
		}
		if !overrides.Verbose {
			t.Errorf("%v: Verbose = false, want true", args)
		}
		if !Resolve(nil, overrides).Verbose {
			t.Errorf("%v: resolved Verbose = false, want true", args)
		}
	}
	if _, err := parseFlags([]string{"-V=true"}); err == nil {
		t.Error("parseFlags(-V=true): expected does-not-take-a-value error")
	}
}

func TestVerbosePlanBlockShowsFieldDiffs(t *testing.T) {
	restore := color.SetEnabled(true)
	defer restore()
	changes := []Change{{
		Kind: "resourceGroup", Action: ActionUpdate, Key: "resourceGroup|rg", Summary: "location or declared tags differ",
		Diffs: []FieldDiff{
			{Field: "location", Old: "eastus", New: "westus2"},
			{Field: "tag config_version", Old: "0.13.0", New: "0.14.0"},
		},
	}}
	verbose := renderPlanBlock(changes, true)
	plain := color.ClearCode(verbose)
	for _, want := range []string{"      location: eastus -> westus2", "      tag config_version: 0.13.0 -> 0.14.0"} {
		if !strings.Contains(plain, want) {
			t.Errorf("verbose output %q missing %q", plain, want)
		}
	}
	for line := range strings.SplitSeq(verbose, "\n") {
		if strings.Contains(line, "->") && !strings.HasPrefix(line, yellowCode) {
			t.Errorf("diff line %q not yellow", line)
		}
	}
	terse := renderPlanBlock(changes, false)
	if strings.Contains(color.ClearCode(terse), "location: eastus") {
		t.Errorf("default output leaked diff detail: %q", terse)
	}
	if got, want := len(strings.Split(terse, "\n")), len(strings.Split(verbose, "\n"))-len(changes[0].Diffs); got != want {
		t.Errorf("terse line count = %d, want %d", got, want)
	}
}

func TestRenderFieldDiffForms(t *testing.T) {
	cases := []struct {
		diff FieldDiff
		want string
	}{
		{FieldDiff{Field: "ttl", Old: "300", New: "600"}, "ttl: 300 -> 600"},
		{FieldDiff{Field: "members added", New: "alice, carol"}, "members added: alice, carol"},
		{FieldDiff{Field: "members removed", Old: "bob"}, "members removed: bob"},
	}
	for _, c := range cases {
		if got := renderFieldDiff(c.diff); got != c.want {
			t.Errorf("renderFieldDiff(%+v) = %q, want %q", c.diff, got, c.want)
		}
	}
}
