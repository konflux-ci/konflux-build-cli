package common

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	l "github.com/konflux-ci/konflux-build-cli/pkg/logger"
)

func CheckSymlinks(dir string) error {
	l.Logger.Info("Checking for symlinks pointing outside the repository")

	// Resolve the checkout directory to handle symlinks in the path itself (e.g., on macOS /tmp -> /private/tmp)
	absCheckoutDir, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("failed to get absolute path of checkout directory: %w", err)
	}
	absCheckoutDir, err = filepath.EvalSymlinks(absCheckoutDir)
	if err != nil {
		return fmt.Errorf("failed to resolve symlinks in checkout directory path: %w", err)
	}

	var invalidSymlinks []string

	err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.Type()&os.ModeSymlink != 0 {
			// This is a symlink
			target, err := filepath.EvalSymlinks(path)
			if err != nil {
				// Broken symlink - log warning but continue
				l.Logger.Warnf("Broken symlink found: %s", path)
				return nil
			}

			absTarget, err := filepath.Abs(target)
			if err != nil {
				return fmt.Errorf("failed to get absolute path of symlink target: %w", err)
			}

			// Check if target is inside checkout dir
			if !strings.HasPrefix(absTarget, absCheckoutDir+string(os.PathSeparator)) && absTarget != absCheckoutDir {
				l.Logger.Errorf("Symlink points outside repository: %s -> %s", path, target)
				invalidSymlinks = append(invalidSymlinks, path)
			}
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to walk directory: %w", err)
	}

	if len(invalidSymlinks) > 0 {
		return fmt.Errorf("found %d symlink(s) pointing outside the repository", len(invalidSymlinks))
	}

	l.Logger.Info("Symlink check passed")
	return nil
}
