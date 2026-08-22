package main

import "testing"

// fakeProvider is a hand-rolled Provider double for Plan/Apply tests,
// mirroring rkit's AssignmentProvider test fixture.
type fakeProvider struct {
	assignments     []LiveAssignment
	dns             []DnsRecord
	groups          []SecurityGroup
	groupNames      []string
	apps            []AppRegistration
	appNames        []string
	roleDefinitions []RoleDefinition
	resourceGroups  []ResourceGroup
}

func (p *fakeProvider) EnsureZone(string) error             { return nil }
func (p *fakeProvider) ListDNS(string) ([]DnsRecord, error) { return p.dns, nil }
func (p *fakeProvider) PutDNS(DnsRecord) error              { return nil }
func (p *fakeProvider) DeleteDNS(DnsRecord) error           { return nil }
func (p *fakeProvider) ListGroupNames() ([]string, error)   { return p.groupNames, nil }
func (p *fakeProvider) PutGroup(SecurityGroup) error        { return nil }
func (p *fakeProvider) DeleteGroup(string) error            { return nil }
func (p *fakeProvider) ListAppNames() ([]string, error)     { return p.appNames, nil }
func (p *fakeProvider) PutApp(AppRegistration) error        { return nil }
func (p *fakeProvider) DeleteApp(string) error              { return nil }
func (p *fakeProvider) ListRoleDefinitions(string) ([]RoleDefinition, error) {
	return p.roleDefinitions, nil
}
func (p *fakeProvider) PutRoleDefinition(RoleDefinition, string) error  { return nil }
func (p *fakeProvider) DeleteRoleDefinition(string, string) error       { return nil }
func (p *fakeProvider) ResolvePrincipal(name, _ string) (string, error) { return "id-" + name, nil }
func (p *fakeProvider) ResolveRole(name, _ string) (string, error)      { return "role-" + name, nil }
func (p *fakeProvider) ListRoleAssignments(string) ([]LiveAssignment, error) {
	return p.assignments, nil
}
func (p *fakeProvider) PutRoleAssignment(string, string, string) error { return nil }
func (p *fakeProvider) DeleteRoleAssignment(LiveAssignment) error      { return nil }
func (p *fakeProvider) ListResourceGroups() ([]ResourceGroup, error)   { return p.resourceGroups, nil }
func (p *fakeProvider) PutResourceGroup(ResourceGroup) error           { return nil }
func (p *fakeProvider) DeleteResourceGroup(string) error               { return nil }

func (p *fakeProvider) GetGroup(name string) (*SecurityGroup, error) {
	for _, g := range p.groups {
		if g.Name == name {
			found := g
			return &found, nil
		}
	}
	return nil, nil
}

func (p *fakeProvider) GetApp(name string) (*AppRegistration, error) {
	for _, a := range p.apps {
		if a.Name == name {
			found := a
			return &found, nil
		}
	}
	return nil, nil
}

func TestApexProviderRecordsAreProtected(t *testing.T) {
	item := DnsRecord{Zone: "example.com", RecordType: "NS", Name: "@", TTL: 60, Values: []string{"ns.example.com"}}
	if !dnsProtected(item) {
		t.Errorf("expected apex NS record to be protected")
	}
}

// TestDNSPruneNeverDeletesApexSOAOrNS exercises dnsProtected through the
// full Plan path, not just the pure predicate — AT14.
func TestDNSPruneNeverDeletesApexSOAOrNS(t *testing.T) {
	provider := &fakeProvider{
		dns: []DnsRecord{
			{Zone: "example.com", RecordType: "SOA", Name: "@", TTL: 3600, Values: []string{"ns1.example.com admin.example.com 1 3600 600 86400 60"}},
			{Zone: "example.com", RecordType: "NS", Name: "@", TTL: 3600, Values: []string{"ns1.example.com"}},
			{Zone: "example.com", RecordType: "A", Name: "stale", TTL: 300, Values: []string{"192.0.2.5"}},
		},
	}
	bundle := &Bundle{Dns: []DnsRecord{{Zone: "example.com", RecordType: "A", Name: "www", TTL: 300, Values: []string{"192.0.2.10"}}}}
	changes, err := Plan(provider, bundle, &Options{PruneDNS: true})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	var deleted []string
	for _, c := range changes {
		if c.Action == ActionDelete {
			deleted = append(deleted, c.Target.Dns.Name+"|"+c.Target.Dns.RecordType)
		}
	}
	if len(deleted) != 1 || deleted[0] != "stale|A" {
		t.Errorf("deleted = %v, want only [stale|A] (apex SOA/NS must be protected)", deleted)
	}
}

func TestPruneRetainsAllDesiredAssignmentsAtSharedScope(t *testing.T) {
	scope := Scope{Subscription: "00000000-0000-0000-0000-000000000001"}
	scopeID, err := scope.ArmID("")
	if err != nil {
		t.Fatal(err)
	}
	bundle := &Bundle{
		RoleAssignments: []RoleAssignment{
			{Principal: "one", PrincipalType: "group", Role: "reader", Scope: scope},
			{Principal: "two", PrincipalType: "group", Role: "reader", Scope: scope},
		},
	}
	provider := &fakeProvider{
		assignments: []LiveAssignment{
			{ID: "assignment-one", PrincipalID: "id-one", RoleID: "role-reader", Scope: scopeID},
			{ID: "assignment-two", PrincipalID: "id-two", RoleID: "role-reader", Scope: scopeID},
		},
	}
	changes, err := Plan(provider, bundle, &Options{Subscription: scope.Subscription, PruneRoles: true})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if len(changes) != 0 {
		t.Errorf("changes = %+v, want none", changes)
	}
}

// TestPlansEveryKindInLegacyCompatibleOrder pins the fixed per-kind
// processing order — AT7.
func TestPlansEveryKindInLegacyCompatibleOrder(t *testing.T) {
	subscription := "00000000-0000-0000-0000-000000000001"
	bundle := &Bundle{
		ResourceGroups: []ResourceGroup{{Name: "resources", Location: "eastus"}},
		Groups:         []SecurityGroup{{Name: "readers"}},
		Apps:           []AppRegistration{{Name: "application", ServicePrincipal: true}},
		RoleDefinitions: []RoleDefinition{{
			Name:             "reader",
			Description:      "synthetic role",
			AssignableScopes: []Scope{{Subscription: subscription}},
			Actions:          []string{"Microsoft.Network/dnsZones/read"},
		}},
		RoleAssignments: []RoleAssignment{{Principal: "readers", PrincipalType: "group", Role: "reader", Scope: Scope{Subscription: subscription}}},
		Dns:             []DnsRecord{{Zone: "example.com", RecordType: "A", Name: "docs", TTL: 300, Values: []string{"192.0.2.10"}}},
	}
	changes, err := Plan(&fakeProvider{}, bundle, &Options{Subscription: subscription})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	want := []string{"resourceGroup", "securityGroup", "appRegistration", "roleDefinition", "roleAssignment", "dnsRecordSet"}
	if len(changes) != len(want) {
		t.Fatalf("got %d changes, want %d: %+v", len(changes), len(want), changes)
	}
	for i, w := range want {
		if changes[i].Kind != w {
			t.Errorf("changes[%d].Kind = %q, want %q", i, changes[i].Kind, w)
		}
	}
}

func TestIdentityAndResourceGroupPruningRespectsPolicy(t *testing.T) {
	bundle := &Bundle{
		Groups:         []SecurityGroup{{Name: "wanted-group"}},
		Apps:           []AppRegistration{{Name: "wanted-app"}},
		ResourceGroups: []ResourceGroup{{Name: "wanted-resources"}},
	}
	newProvider := func() *fakeProvider {
		return &fakeProvider{
			groupNames: []string{"wanted-group", "stale-group"},
			appNames:   []string{"wanted-app", "stale-app"},
			resourceGroups: []ResourceGroup{
				{Name: "wanted-resources"},
				{Name: "stale-resources"},
			},
			groups: []SecurityGroup{{Name: "wanted-group"}},
			apps:   []AppRegistration{{Name: "wanted-app"}},
		}
	}

	changes, err := Plan(newProvider(), bundle, &Options{})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	for _, c := range changes {
		if c.Action == ActionDelete {
			t.Errorf("unexpected delete without prune enabled: %+v", c)
		}
	}

	changes, err = Plan(newProvider(), bundle, &Options{PruneIdentities: true, PruneResourceGroups: true})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	var deleted []string
	for _, c := range changes {
		if c.Action == ActionDelete {
			deleted = append(deleted, c.Key)
		}
	}
	want := []string{"resourceGroup|stale-resources", "securityGroup|stale-group", "appRegistration|stale-app"}
	if len(deleted) != len(want) {
		t.Fatalf("deleted = %v, want %v", deleted, want)
	}
	for i, w := range want {
		if deleted[i] != w {
			t.Errorf("deleted[%d] = %q, want %q", i, deleted[i], w)
		}
	}
}

func TestRoleDefinitionScopeComparisonDetectsOnlyRealDrift(t *testing.T) {
	subscription := "00000000-0000-0000-0000-000000000001"
	resourceScope := Scope{Subscription: subscription, ResourceGroup: "resources"}
	desired := RoleDefinition{
		Name:             "reader",
		Description:      "synthetic role",
		AssignableScopes: []Scope{{Subscription: subscription}, resourceScope},
		Actions:          []string{"Microsoft.Network/dnsZones/read"},
	}
	bundle := &Bundle{RoleDefinitions: []RoleDefinition{desired}}

	drifted := &fakeProvider{roleDefinitions: []RoleDefinition{{
		Name:             "reader",
		Description:      "synthetic role",
		AssignableScopes: []Scope{{Subscription: "00000000-0000-0000-0000-000000000002"}},
		Actions:          []string{"Microsoft.Network/dnsZones/read"},
	}}}
	changes, err := Plan(drifted, bundle, &Options{Subscription: subscription})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if len(changes) != 1 || changes[0].Action != ActionUpdate {
		t.Fatalf("changes = %+v, want exactly one Update", changes)
	}

	equivalent := &fakeProvider{roleDefinitions: []RoleDefinition{{
		Name:             "reader",
		Description:      "synthetic role",
		AssignableScopes: []Scope{resourceScope, {Subscription: subscription}, resourceScope},
		Actions:          []string{"Microsoft.Network/dnsZones/read"},
	}}}
	changes, err = Plan(equivalent, bundle, &Options{Subscription: subscription})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if len(changes) != 0 {
		t.Errorf("changes = %+v, want none (scope sets are equivalent)", changes)
	}
}

func TestResourceGroupUpdatesMergeDeclaredTagsOnlyWhenNeeded(t *testing.T) {
	bundle := &Bundle{ResourceGroups: []ResourceGroup{{Name: "resources", Location: "eastus", Tags: map[string]string{"managed": "new"}}}}
	provider := &fakeProvider{resourceGroups: []ResourceGroup{{Name: "resources", Location: "eastus", Tags: map[string]string{"managed": "old", "preserved": "live"}}}}
	changes, err := Plan(provider, bundle, &Options{})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if len(changes) != 1 {
		t.Fatalf("changes = %+v, want exactly one", changes)
	}
	updated := changes[0].Target.ResourceGroup
	if updated.Tags["managed"] != "new" {
		t.Errorf("managed tag = %q, want %q", updated.Tags["managed"], "new")
	}
	if updated.Tags["preserved"] != "live" {
		t.Errorf("preserved tag = %q, want %q", updated.Tags["preserved"], "live")
	}

	satisfied := &fakeProvider{resourceGroups: []ResourceGroup{{Name: "resources", Location: "eastus", Tags: map[string]string{"managed": "new", "preserved": "live"}}}}
	changes, err = Plan(satisfied, bundle, &Options{})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if len(changes) != 0 {
		t.Errorf("changes = %+v, want none", changes)
	}
}
