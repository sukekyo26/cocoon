// Package aptcategories defines the curated apt package groups offered by
// `cocoon init`'s interactive bootstrap. Categories are presented as a
// multi-select; selected categories are expanded into [apt] packages in the
// generated workspace.toml.
package aptcategories

// AptCategory groups commonly-bundled apt packages so `cocoon init` can
// present them as a single checkbox during the interactive bootstrap.
type AptCategory struct {
	ID          string
	Label       string
	Description string
	Packages    []string
	// Default is true when the category should start with the box
	// pre-checked in `cocoon init`'s multi-select.
	Default bool
}

// AptCategories is the curated list `cocoon init` shows. Order is the
// order options appear to the user.
var AptCategories = []AptCategory{
	{
		ID:          "text-editors",
		Label:       "Text editors",
		Description: "vim, nano",
		Packages:    []string{"vim", "nano"},
		Default:     true,
	},
	{
		ID:          "search",
		Label:       "Search & navigation",
		Description: "fzf, ripgrep, bat, fd-find",
		Packages:    []string{"fzf", "ripgrep", "bat", "fd-find"},
		Default:     false,
	},
	{
		ID:          "compression",
		Label:       "Compression",
		Description: "zip, unzip, xz-utils",
		Packages:    []string{"zip", "unzip", "xz-utils"},
		Default:     true,
	},
	{
		ID:          "network",
		Label:       "Network tools",
		Description: "netcat-openbsd, dnsutils, traceroute",
		Packages:    []string{"netcat-openbsd", "dnsutils", "traceroute"},
		Default:     false,
	},
	{
		ID:          "build",
		Label:       "Build essentials",
		Description: "build-essential, pkg-config",
		Packages:    []string{"build-essential", "pkg-config"},
		Default:     true,
	},
	{
		ID:          "python3",
		Label:       "Python 3",
		Description: "python3, python3-pip, python3-venv",
		Packages:    []string{"python3", "python3-pip", "python3-venv"},
		Default:     false,
	},
	{
		ID:          "monitoring",
		Label:       "System monitoring",
		Description: "htop, iotop, ncdu",
		Packages:    []string{"htop", "iotop", "ncdu"},
		Default:     false,
	},
	{
		ID:          "json-yaml",
		Label:       "JSON/YAML tools",
		Description: "jq, yq",
		Packages:    []string{"jq", "yq"},
		Default:     false,
	},
}

// AptCategoryByID returns the category with the given ID, or nil when the
// ID is not in [AptCategories].
func AptCategoryByID(id string) *AptCategory {
	for i := range AptCategories {
		if AptCategories[i].ID == id {
			return &AptCategories[i]
		}
	}
	return nil
}

// DefaultAptCategoryIDs returns the IDs of categories whose Default flag is
// true, preserving the catalog order.
func DefaultAptCategoryIDs() []string {
	var ids []string
	for _, c := range AptCategories {
		if c.Default {
			ids = append(ids, c.ID)
		}
	}
	return ids
}

// ExpandAptCategories returns the deduplicated apt package list for the
// given category IDs. Unknown IDs are skipped. Order matches
// [AptCategories]'s iteration order through the requested IDs.
func ExpandAptCategories(ids []string) []string {
	seen := make(map[string]struct{})
	var pkgs []string
	for _, id := range ids {
		cat := AptCategoryByID(id)
		if cat == nil {
			continue
		}
		for _, pkg := range cat.Packages {
			if _, ok := seen[pkg]; ok {
				continue
			}
			seen[pkg] = struct{}{}
			pkgs = append(pkgs, pkg)
		}
	}
	return pkgs
}
