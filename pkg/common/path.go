package common

import (
	"path/filepath"
	"strings"
)

// ResolvedPath represents an absolute, symlink-resolved filesystem path.
type ResolvedPath string

// ResolvePath resolves the given path to an absolute path with all symlinks evaluated.
// If EvalSymlinks fails (e.g. because the path doesn't exist), returns ("", err).
func ResolvePath(path string) (ResolvedPath, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", err
	}
	return ResolvedPath(resolved), nil
}

func (p ResolvedPath) String() string {
	return string(p)
}

// IsRelativeTo reports whether p is equal to or contained within base.
func (p ResolvedPath) IsRelativeTo(base ResolvedPath) bool {
	rel, err := filepath.Rel(base.String(), p.String())
	if err != nil {
		return false
	}
	return !strings.HasPrefix(rel, "..")
}
