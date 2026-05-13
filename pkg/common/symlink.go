package common

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	l "github.com/konflux-ci/konflux-build-cli/pkg/logger"
)

// CheckSymlinks walks the given directory tree and returns an error if any
// symlink points to a target outside the directory.
func CheckSymlinks(dir string) error {
	l.Logger.Debugf("Checking for symlinks pointing outside the directory %s", dir)

	baseDir, err := ResolvePath(dir)
	if err != nil {
		return fmt.Errorf("failed to resolve directory path: %w", err)
	}

	var invalidSymlinks []string

	err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.Type()&os.ModeSymlink != 0 {
			resolvedTarget, err := ResolvePath(path)
			if err != nil {
				l.Logger.Errorf("Broken symlink found: %s", path)
				invalidSymlinks = append(invalidSymlinks, path)
				return nil //nolint:nilerr
			}

			if !resolvedTarget.IsRelativeTo(baseDir) {
				l.Logger.Errorf("Symlink points outside directory: %s -> %s", path, resolvedTarget)
				invalidSymlinks = append(invalidSymlinks, path)
			}
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to walk directory: %w", err)
	}

	if len(invalidSymlinks) > 0 {
		return fmt.Errorf("found %d symlink(s) pointing outside the directory", len(invalidSymlinks))
	}

	l.Logger.Debug("Symlink check passed")
	return nil
}
