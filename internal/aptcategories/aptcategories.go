// Package aptcategories defines the curated apt package groups offered by
// `cocoon init`'s interactive bootstrap. Categories are presented as a
// multi-select; selected categories are expanded into [apt] packages in the
// generated workspace.toml.
package aptcategories

// AptCategory bundles apt packages into one multi-select checkbox.
type AptCategory struct {
	ID          string
	Label       string
	Description string
	Packages    []string
	// Default pre-checks the box in `cocoon init`'s multi-select.
	Default bool
}

// AptCategories is the curated list `cocoon init` shows; slice order = UI order.
var AptCategories = []AptCategory{
	{
		ID:          "text-editors",
		Label:       "Text editors",
		Description: "vim, nano",
		Packages:    []string{"vim", "nano"},
		Default:     true,
	},
	{
		ID:          "vcs",
		Label:       "Version control & SSH",
		Description: "git, openssh-client, gnupg",
		Packages:    []string{"git", "openssh-client", "gnupg"},
		Default:     true,
	},
	{
		ID:          "utilities",
		Label:       "General utilities",
		Description: "tree, less, rsync, file, bc, wget, gettext-base, uuid-runtime",
		Packages: []string{
			"tree",
			"less",
			"rsync",
			"file",
			"bc",
			"wget",
			"gettext-base",
			"uuid-runtime",
		},
		Default: true,
	},
	{
		ID:          "compression",
		Label:       "Compression",
		Description: "zip, unzip, xz-utils, tar, gzip, bzip2",
		Packages:    []string{"zip", "unzip", "xz-utils", "tar", "gzip", "bzip2"},
		Default:     true,
	},
	{
		ID:          "build",
		Label:       "Build essentials",
		Description: "build-essential, pkg-config, make, patch",
		Packages:    []string{"build-essential", "pkg-config", "make", "patch"},
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
		ID:          "network",
		Label:       "Network tools",
		Description: "netcat-openbsd, dnsutils, traceroute, iproute2, iputils-ping, net-tools",
		Packages: []string{
			"netcat-openbsd",
			"dnsutils",
			"traceroute",
			"iproute2",
			"iputils-ping",
			"net-tools",
		},
		Default: false,
	},
	{
		ID:          "monitoring",
		Label:       "System monitoring",
		Description: "htop, iotop, ncdu, procps",
		Packages:    []string{"htop", "iotop", "ncdu", "procps"},
		Default:     false,
	},
	{
		ID:    "python3",
		Label: "Python 3",
		// system Python from Debian apt — separate from the python image
		// base and the uv plugin. The point is to keep a plain `python3`
		// reachable on $PATH for AI agents and shell one-liners (no
		// `uv run python` wrapping, no specific image required).
		Description: "python3 + pip + venv (system Python for AI / scripts)",
		Packages:    []string{"python3", "python3-pip", "python3-venv"},
		Default:     false,
	},
	{
		ID:          "json-yaml",
		Label:       "JSON/YAML tools",
		Description: "jq, yq",
		Packages:    []string{"jq", "yq"},
		Default:     false,
	},
	{
		ID:    "dev-tools",
		Label: "Developer tools",
		// git-lfs: large-file storage (ML models, media, HF / Civitai assets).
		// strace: trace syscalls of a stuck process inside the container.
		// tmux: terminal multiplexer so long-running builds / training survive
		// a `docker exec` disconnect and pane splits let you watch logs and a
		// server side-by-side.
		Description: "git-lfs, strace, tmux",
		Packages:    []string{"git-lfs", "strace", "tmux"},
		Default:     false,
	},
}

// AptCategoryByID returns nil when id is not in AptCategories.
func AptCategoryByID(id string) *AptCategory {
	for i := range AptCategories {
		if AptCategories[i].ID == id {
			return &AptCategories[i]
		}
	}
	return nil
}

// DefaultAptCategoryIDs preserves catalog order.
func DefaultAptCategoryIDs() []string {
	var ids []string
	for _, c := range AptCategories {
		if c.Default {
			ids = append(ids, c.ID)
		}
	}
	return ids
}

// ExpandAptCategories dedupes packages and skips unknown IDs.
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
