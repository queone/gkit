package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config is attune.yaml's parsed content before override precedence applies.
type Config struct {
	Provider            string
	Specs               string
	ContentVersion      string
	Subscription        string
	ResourceGroup       string
	PruneDNS            *bool
	PruneIdentities     *bool
	PruneRoles          *bool
	PruneResourceGroups *bool
	Directory           string
}

// Overrides holds flag-level values that take precedence over Config.
type Overrides struct {
	Provider            *string
	Specs               *string
	Subscription        *string
	ResourceGroup       *string
	PruneDNS            *bool
	PruneIdentities     *bool
	PruneRoles          *bool
	PruneResourceGroups *bool
	Kind                string
	Diagnostic          bool
	Verbose             bool
}

// Settings is the fully resolved configuration attune runs with.
type Settings struct {
	Provider            string
	Specs               string
	ContentVersion      string
	Subscription        string
	ResourceGroup       string
	PruneDNS            bool
	PruneIdentities     bool
	PruneRoles          bool
	PruneResourceGroups bool
	Kind                string
	Diagnostic          bool
	Verbose             bool
}

// Find walks upward from start looking for the nearest attune.yaml. A
// missing config is not an error — it returns (nil, nil).
func Find(start string) (*Config, error) {
	directory, err := filepath.Abs(start)
	if err != nil {
		return nil, fmt.Errorf("resolve configuration start directory: %w", err)
	}
	directory, err = filepath.EvalSymlinks(directory)
	if err != nil {
		return nil, fmt.Errorf("resolve configuration start directory: %w", err)
	}
	for {
		candidate := filepath.Join(directory, "attune.yaml")
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			cfg, err := LoadConfig(candidate)
			if err != nil {
				return nil, err
			}
			return cfg, nil
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			return nil, nil
		}
		directory = parent
	}
}

// expandEnvironment expands $VAR and ${VAR} references in input using the
// current process environment. An unresolved variable expands to "". A lone
// trailing "$" or "$" followed by a non-identifier character is left as-is.
func expandEnvironment(input string) (string, error) {
	var output strings.Builder
	index := 0
	for index < len(input) {
		offset := strings.IndexByte(input[index:], '$')
		if offset < 0 {
			output.WriteString(input[index:])
			break
		}
		dollar := index + offset
		output.WriteString(input[index:dollar])
		index = dollar

		var name string
		var consumed int
		if index+1 < len(input) && input[index+1] == '{' {
			closeOffset := strings.IndexByte(input[index+2:], '}')
			if closeOffset < 0 {
				return "", fmt.Errorf("unterminated environment reference in attune.yaml")
			}
			end := index + 2 + closeOffset
			name = input[index+2 : end]
			consumed = end - index + 1
		} else {
			end := len(input)
			for i := index + 1; i < len(input); i++ {
				c := input[i]
				if !isASCIIAlnum(c) && c != '_' {
					end = i
					break
				}
			}
			name = input[index+1 : end]
			consumed = end - index
		}
		if name == "" {
			output.WriteByte('$')
			index++
			continue
		}
		output.WriteString(os.Getenv(name))
		index += consumed
	}
	return output.String(), nil
}

func isASCIIAlnum(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

// LoadConfig reads and validates attune.yaml at path.
func LoadConfig(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read attune.yaml: %w", err)
	}
	expanded, err := expandEnvironment(string(raw))
	if err != nil {
		return nil, err
	}
	var root any
	if err := yaml.Unmarshal([]byte(expanded), &root); err != nil {
		return nil, fmt.Errorf("parse attune.yaml: %w", err)
	}
	if root == nil {
		return nil, fmt.Errorf("attune.yaml is empty")
	}
	m, err := mapping(root, "attune.yaml")
	if err != nil {
		return nil, err
	}
	if err := rejectUnknown(m, []string{"provider", "specs", "content_version", "azure", "prune"}, "attune.yaml"); err != nil {
		return nil, err
	}
	azure, err := cfgOptionalMapping(m, "azure")
	if err != nil {
		return nil, err
	}
	if err := rejectUnknown(azure, []string{"subscription", "resource_group"}, "attune.yaml azure"); err != nil {
		return nil, err
	}
	prune, err := cfgOptionalMapping(m, "prune")
	if err != nil {
		return nil, err
	}
	if err := rejectUnknown(prune, []string{"dns", "identities", "roles", "resource_groups"}, "attune.yaml prune"); err != nil {
		return nil, err
	}

	provider, err := optionalString(m, "provider")
	if err != nil {
		return nil, err
	}
	specs, err := optionalString(m, "specs")
	if err != nil {
		return nil, err
	}
	contentVersion, err := optionalString(m, "content_version")
	if err != nil {
		return nil, err
	}
	subscription, err := optionalString(azure, "subscription")
	if err != nil {
		return nil, err
	}
	resourceGroup, err := optionalString(azure, "resource_group")
	if err != nil {
		return nil, err
	}
	pruneDNS, err := optionalBool(prune, "dns")
	if err != nil {
		return nil, err
	}
	pruneIdentities, err := optionalBool(prune, "identities")
	if err != nil {
		return nil, err
	}
	pruneRoles, err := optionalBool(prune, "roles")
	if err != nil {
		return nil, err
	}
	pruneResourceGroups, err := optionalBool(prune, "resource_groups")
	if err != nil {
		return nil, err
	}

	directory := filepath.Dir(path)
	if directory == "" {
		directory = "."
	}
	return &Config{
		Provider:            provider,
		Specs:               specs,
		ContentVersion:      contentVersion,
		Subscription:        subscription,
		ResourceGroup:       resourceGroup,
		PruneDNS:            pruneDNS,
		PruneIdentities:     pruneIdentities,
		PruneRoles:          pruneRoles,
		PruneResourceGroups: pruneResourceGroups,
		Directory:           directory,
	}, nil
}

// Resolve merges built-in defaults, an optional Config, environment
// variables (ARM_SUBSCRIPTION_ID, ARM_RESOURCE_GROUP), and Overrides, in
// that increasing order of precedence.
func Resolve(config *Config, overrides Overrides) Settings {
	settings := Settings{
		Provider:            "azure",
		Specs:               "dns",
		PruneDNS:            true,
		PruneIdentities:     false,
		PruneRoles:          false,
		PruneResourceGroups: false,
		Kind:                overrides.Kind,
		Diagnostic:          overrides.Diagnostic,
		Verbose:             overrides.Verbose,
	}
	if config != nil {
		if config.Provider != "" {
			settings.Provider = config.Provider
		}
		if config.Specs != "" {
			specs := config.Specs
			if !filepath.IsAbs(specs) {
				specs = filepath.Join(config.Directory, specs)
			}
			settings.Specs = specs
		}
		if config.ContentVersion != "" {
			settings.ContentVersion = config.ContentVersion
		}
		if config.Subscription != "" {
			settings.Subscription = config.Subscription
		}
		if config.ResourceGroup != "" {
			settings.ResourceGroup = config.ResourceGroup
		}
		if config.PruneDNS != nil {
			settings.PruneDNS = *config.PruneDNS
		}
		if config.PruneIdentities != nil {
			settings.PruneIdentities = *config.PruneIdentities
		}
		if config.PruneRoles != nil {
			settings.PruneRoles = *config.PruneRoles
		}
		if config.PruneResourceGroups != nil {
			settings.PruneResourceGroups = *config.PruneResourceGroups
		}
	}
	if v := os.Getenv("ARM_SUBSCRIPTION_ID"); v != "" {
		settings.Subscription = v
	}
	if v := os.Getenv("ARM_RESOURCE_GROUP"); v != "" {
		settings.ResourceGroup = v
	}
	if overrides.Provider != nil {
		settings.Provider = *overrides.Provider
	}
	if overrides.Specs != nil {
		settings.Specs = *overrides.Specs
	}
	if overrides.Subscription != nil {
		settings.Subscription = *overrides.Subscription
	}
	if overrides.ResourceGroup != nil {
		settings.ResourceGroup = *overrides.ResourceGroup
	}
	if overrides.PruneDNS != nil {
		settings.PruneDNS = *overrides.PruneDNS
	}
	if overrides.PruneIdentities != nil {
		settings.PruneIdentities = *overrides.PruneIdentities
	}
	if overrides.PruneRoles != nil {
		settings.PruneRoles = *overrides.PruneRoles
	}
	if overrides.PruneResourceGroups != nil {
		settings.PruneResourceGroups = *overrides.PruneResourceGroups
	}
	return settings
}

func cfgOptionalMapping(m map[string]any, name string) (map[string]any, error) {
	v, ok := m[name]
	if !ok || v == nil {
		return map[string]any{}, nil
	}
	return mapping(v, name)
}
