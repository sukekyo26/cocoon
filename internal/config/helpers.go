package config

import "strings"

// BasenameFromGitURL derives the default clone target directory name from a
// git URL. It mirrors src/wsd/cli/config._basename_from_git_url so output is
// byte-identical to the Python implementation.
func BasenameFromGitURL(url string) string {
	s := strings.TrimRight(strings.TrimSpace(url), "/")
	if s == "" {
		return ""
	}
	for _, sep := range []string{"?", "#"} {
		if i := strings.Index(s, sep); i >= 0 {
			s = s[:i]
		}
	}
	if strings.Contains(s, ":") &&
		!strings.HasPrefix(s, "http://") &&
		!strings.HasPrefix(s, "https://") &&
		!strings.HasPrefix(s, "ssh://") &&
		!strings.HasPrefix(s, "git://") &&
		!strings.HasPrefix(s, "file://") {
		if i := strings.Index(s, ":"); i >= 0 {
			s = s[i+1:]
		}
	}
	last := s
	if i := strings.LastIndex(s, "/"); i >= 0 {
		last = s[i+1:]
	}
	return strings.TrimSuffix(last, ".git")
}

// ResolveRepoPath resolves the final clone target path for a [repositories]
// .clone entry. It mirrors src/wsd/cli/config._resolve_repo_path so output is
// byte-identical to the Python implementation.
func ResolveRepoPath(pathField, url string) string {
	if pathField == "" {
		return BasenameFromGitURL(url)
	}
	if strings.Trim(pathField, "/") == "" {
		return ""
	}
	if strings.HasSuffix(pathField, "/") {
		base := BasenameFromGitURL(url)
		if base == "" {
			return ""
		}
		return strings.TrimRight(pathField, "/") + "/" + base
	}
	return pathField
}
