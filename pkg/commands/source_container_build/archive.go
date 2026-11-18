package source_container_build

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// makeSourceArchive packs the source repository at HEAD into an archive in a
// new directory under workDir, and adds that directory to the sources gobsi
// packs into the source image.
func (s *SourceContainerBuild) makeSourceArchive(workDir string, sibDirs *sourceImageBuildDirs) error {
	archiveDir, err := createDir(workDir, "source_archive")
	if err != nil {
		return err
	}

	name, commitSha, err := s.repoInfo()
	if err != nil {
		return err
	}
	prefix := name + "-" + commitSha
	archivePath := filepath.Join(archiveDir, prefix+".tar.gz")

	// git ls-files lists what is tracked, but the files are read from the
	// working tree, so stash first to keep uncommitted changes out of an archive
	// named after the commit at HEAD.
	if err := s.GitCli.Stash(); err != nil {
		return fmt.Errorf("failed to stash the working tree: %w", err)
	}
	defer func() {
		// Stashing a clean working tree stores nothing, which makes the pop
		// fail. That is the ordinary case here, so it is not an error.
		if err := s.GitCli.StashPop(); err != nil {
			log.Debugf("No stashed changes to restore: %s", err)
		}
	}()

	commitTime, err := s.commitTime()
	if err != nil {
		return err
	}

	files, err := s.GitCli.LsFiles(true)
	if err != nil {
		return fmt.Errorf("failed to list the tracked files: %w", err)
	}
	log.Debugf("Packing %d tracked files into %s", len(files), archivePath)

	if err := writeSourceArchive(archivePath, s.Params.SourceDir, prefix, files, commitTime); err != nil {
		return fmt.Errorf("failed to write the source archive %s: %w", archivePath, err)
	}

	log.Infof("Added the source archive directory to the sources: %s", archiveDir)
	sibDirs.extraSrcDirs = append(sibDirs.extraSrcDirs, archiveDir)

	return nil
}

// repoInfo returns the name of the repository and the SHA of the commit at
// HEAD, which together name the source archive.
func (s *SourceContainerBuild) repoInfo() (string, string, error) {
	commitSha, err := s.GitCli.RevParse("HEAD", false, 0)
	if err != nil {
		return "", "", fmt.Errorf("failed to resolve HEAD: %w", err)
	}

	repoUrl, err := s.GitCli.ConfigGet("remote.origin.url")
	if err != nil {
		return "", "", fmt.Errorf("failed to read the url of the origin remote: %w", err)
	}

	return repoNameFromUrl(repoUrl), commitSha, nil
}

// repoNameFromUrl takes the repository name out of its clone url, so that
// https://github.com/namespace/app.git gives app. Only the .git suffix is
// dropped, because a dot is a legitimate part of a name like kubernetes.io.
func repoNameFromUrl(repoUrl string) string {
	name := strings.TrimRight(repoUrl, "/")
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}

	return strings.TrimSuffix(name, ".git")
}

// commitTime returns the time of the commit at HEAD.
func (s *SourceContainerBuild) commitTime() (time.Time, error) {
	// %cI is the committer date in strict ISO 8601.
	out, err := s.GitCli.Show("%cI")
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to read the time of the commit at HEAD: %w", err)
	}

	commitTime, err := time.Parse(time.RFC3339, out)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse the commit time '%s': %w", out, err)
	}

	return commitTime, nil
}

// writeSourceArchive packs files, each resolved relative to sourceDir, into a
// gzipped tar at archivePath. Every entry is stored under prefix and stamped
// with commitTime, and ownership is dropped, which keeps most of the machine
// that happened to run the build out of the archive. Entries keep the order of
// files, which git ls-files reports sorted.
func writeSourceArchive(archivePath, sourceDir, prefix string, files []string, commitTime time.Time) error {
	archive, err := os.Create(archivePath) //nolint:gosec // path is built from the work directory
	if err != nil {
		return err
	}
	defer archive.Close() //nolint:errcheck // the writers below are closed explicitly

	gzipWriter := gzip.NewWriter(archive)
	tarWriter := tar.NewWriter(gzipWriter)

	for _, file := range files {
		if err := addFileToArchive(tarWriter, sourceDir, prefix, file, commitTime); err != nil {
			return fmt.Errorf("failed to add '%s': %w", file, err)
		}
	}

	// Both writers buffer, so a failure to write the archive out surfaces here
	// rather than on any of the calls above.
	if err := tarWriter.Close(); err != nil {
		return err
	}

	return gzipWriter.Close()
}

// addFileToArchive adds a single tracked file to the archive. Symlinks are
// stored as symlinks rather than followed, because a link pointing outside the
// repository would otherwise copy the content of its target into an archive
// that gets published.
func addFileToArchive(tarWriter *tar.Writer, sourceDir, prefix, file string, commitTime time.Time) error {
	path := filepath.Join(sourceDir, file)

	info, err := os.Lstat(path)
	if err != nil {
		return err
	}

	linkTarget := ""
	if info.Mode()&os.ModeSymlink != 0 {
		if linkTarget, err = os.Readlink(path); err != nil {
			return err
		}
	}

	header, err := tar.FileInfoHeader(info, linkTarget)
	if err != nil {
		return err
	}
	header.Name = prefix + "/" + file
	if info.IsDir() {
		header.Name += "/"
	}
	header.ModTime = commitTime
	// Ownership and the remaining timestamps describe the machine that happened
	// to run the build, not the source, and would make the archive differ
	// between builds of the same commit.
	header.Uid, header.Gid = 0, 0
	header.Uname, header.Gname = "", ""
	header.AccessTime, header.ChangeTime = time.Time{}, time.Time{}

	if err := tarWriter.WriteHeader(header); err != nil {
		return err
	}

	if !info.Mode().IsRegular() {
		return nil
	}

	source, err := os.Open(path) //nolint:gosec // path is a file tracked in the source repository
	if err != nil {
		return err
	}
	defer source.Close() //nolint:errcheck // read only

	_, err = io.Copy(tarWriter, source)

	return err
}
