package ir

import (
	"embed"

	"gopkg.in/yaml.v3"
)

//go:embed seed_catalog.yaml
var seedCatalogFS embed.FS

// DefaultCatalog returns the built-in seed IR catalog.
func DefaultCatalog() (*Catalog, error) {
	data, err := seedCatalogFS.ReadFile("seed_catalog.yaml")
	if err != nil {
		return nil, err
	}
	var cat Catalog
	if err := yaml.Unmarshal(data, &cat); err != nil {
		return nil, err
	}
	return &cat, nil
}
