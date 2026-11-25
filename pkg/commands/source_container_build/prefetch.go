package source_container_build

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/gabriel-vasile/mimetype"
)

// archiveMimeTypes are the types a prefetched dependency is recognised by. A
// dependency is only taken into the source image if it is source in one of
// these forms.
var archiveMimeTypes = []string{
	"application/gzip",
	"application/x-bzip2",
	"application/x-compress",
	"application/x-tar",
	"application/x-xz",
	"application/zip",
}

const srpmMimeType = "application/x-rpm"

// unpackedSourcePackageManagers prefetch their dependencies as source trees
// rather than as archives. Cargo, for example, vendors crates as directories of
// .rs and .toml files, none of which match archiveMimeTypes, so everything
// below such a directory is collected instead.
var unpackedSourcePackageManagers = []string{"cargo"}

// gatherPrefetchedSources collects the prefetched dependencies into directories
// under workDir and adds them to the sources gobsi packs. It reports whether
// anything was found to add.
func (s *SourceContainerBuild) gatherPrefetchedSources(workDir string, sibDirs *sourceImageBuildDirs) (bool, error) {
	prefetchDir := s.Params.PrefetchArtifactsDir
	// Where the prefetch-dependencies command leaves its output.
	outputDir := filepath.Join(prefetchDir, "output")
	depsDir := filepath.Join(outputDir, "deps")

	if !isDir(outputDir) {
		log.Infof("Cannot find prefetch output directory at %s", outputDir)
		return false, nil
	}

	sourcesByPackageManager, err := findPrefetchedSources(depsDir)
	if err != nil {
		return false, err
	}

	preparedSourcesDir, err := createDir(workDir, "prefetched_sources")
	if err != nil {
		return false, err
	}

	// Each package manager gets its own directory, and so its own layer. The
	// names are sorted so that the same prefetch output always numbers them the
	// same way.
	for i, packageManager := range slices.Sorted(maps.Keys(sourcesByPackageManager)) {
		srcDir := filepath.Join(preparedSourcesDir, fmt.Sprintf("src-%d", i))
		destDir := filepath.Join(srcDir, "deps", packageManager)

		for _, source := range sourcesByPackageManager[packageManager] {
			relPath, err := filepath.Rel(filepath.Join(depsDir, packageManager), source)
			if err != nil {
				return false, err
			}
			dest := filepath.Join(destDir, relPath)
			if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
				return false, err
			}
			log.Debugf("Copying prefetched source %s to %s", source, dest)
			if err := copyFile(source, dest); err != nil {
				return false, err
			}
		}

		sibDirs.extraSrcDirs = append(sibDirs.extraSrcDirs, srcDir)
	}

	rpmDir, err := createDir(workDir, "rpm_dir")
	if err != nil {
		return false, err
	}
	sibDirs.rpmDir = rpmDir

	srpmCount, err := gatherPrefetchedSrpms(outputDir, rpmDir)
	if err != nil {
		return false, err
	}

	gathered := len(sourcesByPackageManager)+srpmCount > 0
	if !gathered {
		log.Info("There is no prefetched source archive")
	}

	if err := s.gatherPrefetchEnv(workDir, sibDirs); err != nil {
		return false, err
	}

	return gathered, nil
}

// findPrefetchedSources returns the sources to take from each package manager
// that prefetched anything.
func findPrefetchedSources(depsDir string) (map[string][]string, error) {
	sources := map[string][]string{}

	entries, err := os.ReadDir(depsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return sources, nil
		}
		return nil, fmt.Errorf("failed to read the prefetched dependencies in %s: %w", depsDir, err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		packageManager := entry.Name()
		includeAll := slices.Contains(unpackedSourcePackageManagers, packageManager)

		walkErr := filepath.WalkDir(filepath.Join(depsDir, packageManager), func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			if includeAll || isArchive(path) {
				sources[packageManager] = append(sources[packageManager], path)
			}
			return nil
		})
		if walkErr != nil {
			return nil, fmt.Errorf("failed to collect the sources prefetched by %s: %w", packageManager, walkErr)
		}
	}

	return sources, nil
}

// gatherPrefetchedSrpms copies every prefetched SRPM into rpmDir and reports how
// many were found.
func gatherPrefetchedSrpms(outputDir, rpmDir string) (int, error) {
	count := 0

	// WalkDir walks in lexical order, so the SRPMs are found in a stable order
	// and a name clash is always resolved the same way.
	err := filepath.WalkDir(outputDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".src.rpm") {
			return nil
		}
		if detected, derr := mimetype.DetectFile(path); derr != nil || detected.String() != srpmMimeType {
			return nil //nolint:nilerr // a file that is not an RPM is simply not a source to take
		}
		count++

		dest := filepath.Join(rpmDir, d.Name())
		if _, serr := os.Stat(dest); serr == nil {
			// Two SRPMs of the same name: keep both unless they are the same
			// file, in which case there is nothing to add.
			srcName, nerr := uniqueSrpmArtifactName(path)
			if nerr != nil {
				return nerr
			}
			destName, nerr := uniqueSrpmArtifactName(dest)
			if nerr != nil {
				return nerr
			}
			if srcName == destName {
				log.Debugf("Identical artifacts found for %s, skipping %s", dest, path)
				return nil
			}
			dest = filepath.Join(rpmDir, srcName)
		}

		log.Debugf("Copying prefetched rpm source %s to %s", path, dest)

		return copyFile(path, dest)
	})
	if err != nil {
		return 0, fmt.Errorf("failed to collect the prefetched SRPMs: %w", err)
	}

	return count, nil
}

// gatherPrefetchEnv takes the environment file the prefetch wrote, when there is
// one, so that the source image records how the dependencies were fetched.
func (s *SourceContainerBuild) gatherPrefetchEnv(workDir string, sibDirs *sourceImageBuildDirs) error {
	prefetchEnv := filepath.Join(s.Params.PrefetchArtifactsDir, "prefetch.env")
	if !fileExists(prefetchEnv) {
		return nil
	}

	envDir, err := createDir(workDir, "prefetch_env")
	if err != nil {
		return err
	}

	dest := filepath.Join(envDir, "prefetch.env")
	log.Debugf("Copying prefetch env file %s to %s", prefetchEnv, dest)
	if err := copyFile(prefetchEnv, dest); err != nil {
		return err
	}

	sibDirs.extraSrcDirs = append(sibDirs.extraSrcDirs, envDir)

	return nil
}

// uniqueSrpmArtifactName names an SRPM by its content as well as its file name,
// so that two different SRPMs sharing a name can be told apart.
func uniqueSrpmArtifactName(path string) (string, error) {
	file, err := os.Open(path) //nolint:gosec // path comes from walking the prefetch output
	if err != nil {
		return "", err
	}
	defer file.Close() //nolint:errcheck // read only

	digest := sha256.New()
	if _, err := io.Copy(digest, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(digest.Sum(nil)) + "-" + filepath.Base(path), nil
}

// isDir reports whether path is an existing directory.
func isDir(path string) bool {
	info, err := os.Stat(path)

	return err == nil && info.IsDir()
}

// fileExists reports whether path exists.
func fileExists(path string) bool {
	_, err := os.Stat(path)

	return err == nil
}

// isArchive reports whether a file is one of the archive forms a prefetched
// dependency arrives in.
func isArchive(path string) bool {
	// The compress format is not among the types mimetype knows, so look for
	// its magic number directly.
	if isCompressed(path) {
		return true
	}

	detected, err := mimetype.DetectFile(path)
	if err != nil {
		return false
	}
	// A more specific type, such as a jar, is still the archive it is built on,
	// so walk up to the types it was detected from.
	for ; detected != nil; detected = detected.Parent() {
		if slices.Contains(archiveMimeTypes, detected.String()) {
			return true
		}
	}

	return false
}

// isCompressed reports whether a file is in the compress format, the one
// archive form mimetype does not recognise.
func isCompressed(path string) bool {
	file, err := os.Open(path) //nolint:gosec // path comes from walking the prefetch output
	if err != nil {
		return false
	}
	defer file.Close() //nolint:errcheck // read only

	magic := make([]byte, 2)
	if _, err := io.ReadFull(file, magic); err != nil {
		return false
	}

	return magic[0] == 0x1f && magic[1] == 0x9d
}

// copyFile copies a file, keeping a symlink a symlink. Following a link whose
// target is outside the tree would copy the content of that target into an
// image that gets published.
func copyFile(src, dest string) error {
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}

	if info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(src)
		if err != nil {
			return err
		}
		return os.Symlink(target, dest)
	}

	source, err := os.Open(src) //nolint:gosec // path comes from walking the prefetch output
	if err != nil {
		return err
	}
	defer source.Close() //nolint:errcheck // read only

	destination, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode().Perm()) //nolint:gosec // path is built from the work directory
	if err != nil {
		return err
	}

	if _, err := io.Copy(destination, source); err != nil {
		destination.Close() //nolint:errcheck,gosec // the copy error is the one that matters
		return err
	}

	return destination.Close()
}
