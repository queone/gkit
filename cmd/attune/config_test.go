package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultsAreSafe(t *testing.T) {
	settings := Resolve(nil, Overrides{})
	if !settings.PruneDNS {
		t.Errorf("PruneDNS = false, want true")
	}
	if settings.PruneIdentities {
		t.Errorf("PruneIdentities = true, want false")
	}
	if settings.Provider != "azure" {
		t.Errorf("Provider = %q, want %q", settings.Provider, "azure")
	}
}

func TestDefaultSpecsIsDns(t *testing.T) {
	settings := Resolve(nil, Overrides{})
	if settings.Specs != "dns" {
		t.Errorf("Specs = %q, want %q", settings.Specs, "dns")
	}
}

func TestExpansionPreservesUnicodeText(t *testing.T) {
	got, err := expandEnvironment("specs: café\n")
	if err != nil {
		t.Fatalf("expandEnvironment: %v", err)
	}
	if got != "specs: café\n" {
		t.Errorf("expandEnvironment(unicode) = %q, want unchanged", got)
	}
}

func TestConfigFileEnvironmentExpansion(t *testing.T) {
	t.Setenv("ATTUNE_TEST_SUBSCRIPTION", "expanded-subscription-id")
	dir := t.TempDir()
	content := "provider: azure\nazure:\n  subscription: $ATTUNE_TEST_SUBSCRIPTION\n  resource_group: ${ATTUNE_TEST_SUBSCRIPTION}-rg\nspecs: specs\n"
	path := filepath.Join(dir, "attune.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Subscription != "expanded-subscription-id" {
		t.Errorf("Subscription = %q, want %q", cfg.Subscription, "expanded-subscription-id")
	}
	if cfg.ResourceGroup != "expanded-subscription-id-rg" {
		t.Errorf("ResourceGroup = %q, want %q", cfg.ResourceGroup, "expanded-subscription-id-rg")
	}
}

func TestResolvePrecedenceFlagOverEnvOverFileOverDefault(t *testing.T) {
	t.Setenv("ARM_SUBSCRIPTION_ID", "")
	t.Setenv("ARM_RESOURCE_GROUP", "")

	// No config, no env, no override: built-in default (empty subscription).
	settings := Resolve(nil, Overrides{})
	if settings.Subscription != "" {
		t.Fatalf("baseline Subscription = %q, want empty", settings.Subscription)
	}

	// Config sets it.
	cfg := &Config{Subscription: "from-config"}
	settings = Resolve(cfg, Overrides{})
	if settings.Subscription != "from-config" {
		t.Errorf("config-level Subscription = %q, want %q", settings.Subscription, "from-config")
	}

	// Environment overrides config.
	t.Setenv("ARM_SUBSCRIPTION_ID", "from-env")
	settings = Resolve(cfg, Overrides{})
	if settings.Subscription != "from-env" {
		t.Errorf("env-level Subscription = %q, want %q", settings.Subscription, "from-env")
	}

	// Flag override wins over both.
	flagValue := "from-flag"
	settings = Resolve(cfg, Overrides{Subscription: &flagValue})
	if settings.Subscription != "from-flag" {
		t.Errorf("flag-level Subscription = %q, want %q", settings.Subscription, "from-flag")
	}
}
