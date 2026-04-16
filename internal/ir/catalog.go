package ir

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Capability represents a single atomic capability in the IR taxonomy.
type Capability struct {
	Verbs  []string `yaml:"verbs"`
	Params []string `yaml:"params"`
	Source string   `yaml:"source,omitempty"` // "seed" or "llm"
	Added  string   `yaml:"added,omitempty"`  // timestamp for LLM-generated entries
}

// Catalog is the top-level IR catalog structure.
type Catalog struct {
	Version int                              `yaml:"version"`
	Domains map[string]map[string]Capability `yaml:"domains"`
}

// CatalogEntry is a flattened view of a single capability with its domain and ID.
type CatalogEntry struct {
	Domain       string
	CapabilityID string
	Capability
}

// LoadCatalog reads an IR catalog from a YAML file.
func LoadCatalog(path string) (*Catalog, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading catalog: %w", err)
	}
	var cat Catalog
	if err := yaml.Unmarshal(data, &cat); err != nil {
		return nil, fmt.Errorf("parsing catalog: %w", err)
	}
	return &cat, nil
}

// SaveCatalog writes the catalog to a YAML file.
func SaveCatalog(path string, cat *Catalog) error {
	data, err := yaml.Marshal(cat)
	if err != nil {
		return fmt.Errorf("marshaling catalog: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

// AllEntries returns a flat list of all capabilities across all domains.
func (c *Catalog) AllEntries() []CatalogEntry {
	var entries []CatalogEntry
	for domain, caps := range c.Domains {
		for id, cap := range caps {
			entries = append(entries, CatalogEntry{
				Domain:       domain,
				CapabilityID: id,
				Capability:   cap,
			})
		}
	}
	return entries
}
