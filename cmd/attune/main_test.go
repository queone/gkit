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
