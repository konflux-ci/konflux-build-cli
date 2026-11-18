package source_container_build

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	. "github.com/onsi/gomega"

	"github.com/konflux-ci/konflux-build-cli/testutil"
)

const (
	testCommitSha  = "4f2a1c9e8b7d6a5f4e3c2b1a0998877665544332"
	testCommitTime = "2026-10-27T09:15:00+01:00"
)

// archiveEntry is what a test asserts about one member of the source archive.
type archiveEntry struct {
	name     string
	content  string
	linkName string
	mode     int64
	modTime  time.Time
	uid      int
	gid      int
}

// readArchive reads back every entry of a gzipped tar, keyed by entry name.
func readArchive(t *testing.T, path string) map[string]archiveEntry {
	t.Helper()
	g := NewWithT(t)

	file, err := os.Open(path)
	g.Expect(err).ToNot(HaveOccurred())
	defer file.Close() //nolint:errcheck // read only

	gzipReader, err := gzip.NewReader(file)
	g.Expect(err).ToNot(HaveOccurred())

	entries := map[string]archiveEntry{}
	tarReader := tar.NewReader(gzipReader)
	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		g.Expect(err).ToNot(HaveOccurred())

		content, err := io.ReadAll(tarReader)
		g.Expect(err).ToNot(HaveOccurred())

		entries[header.Name] = archiveEntry{
			name:     header.Name,
			content:  string(content),
			linkName: header.Linkname,
			mode:     header.Mode,
			modTime:  header.ModTime,
			uid:      header.Uid,
			gid:      header.Gid,
		}
	}

	return entries
}

// setupSourceRepo creates a source directory holding the given files, and a
// command wired to a git mock that reports a repository at a fixed commit.
func setupSourceRepo(t *testing.T, files map[string]string) (*SourceContainerBuild, *mockGitCli, string) {
	t.Helper()

	sourceDir := t.TempDir()
	testutil.WriteFileTree(t, sourceDir, files)

	// git ls-files reports paths in a stable, sorted order. Sort here too, so
	// that the map iteration order does not leak into the test.
	tracked := slices.Sorted(maps.Keys(files))

	gitCli := &mockGitCli{
		RevParseFunc:  func(ref string, short bool, length int) (string, error) { return testCommitSha, nil },
		ConfigGetFunc: func(key string) (string, error) { return "https://github.com/namespace/app.git", nil },
		ShowFunc:      func(format string) (string, error) { return testCommitTime, nil },
		LsFilesFunc:   func(recurseSubmodules bool) ([]string, error) { return tracked, nil },
	}

	scb := &SourceContainerBuild{
		Params: &Params{SourceDir: sourceDir},
		GitCli: gitCli,
	}

	return scb, gitCli, t.TempDir()
}

func Test_repoNameFromUrl(t *testing.T) {
	testCases := []struct {
		repoUrl string
		want    string
	}{
		{"https://github.com/namespace/app.git", "app"},
		{"https://github.com/namespace/app", "app"},
		{"https://github.com/namespace/app.git/", "app"},
		{"git@github.com:namespace/app.git", "app"},
		{"https://github.com/namespace/my.app.git", "my.app"},
		{"https://github.com/namespace/kubernetes.io", "kubernetes.io"},
		{"https://github.com/kubernetes/kubernetes.github.io", "kubernetes.github.io"},
		{"app.git", "app"},
		{"", ""},
	}

	for _, tc := range testCases {
		t.Run(tc.repoUrl, func(t *testing.T) {
			g := NewWithT(t)
			g.Expect(repoNameFromUrl(tc.repoUrl)).To(Equal(tc.want))
		})
	}
}

func Test_commitTime(t *testing.T) {
	g := NewWithT(t)

	t.Run("should parse the commit time", func(t *testing.T) {
		scb := &SourceContainerBuild{GitCli: &mockGitCli{
			ShowFunc: func(format string) (string, error) {
				g.Expect(format).To(Equal("%cI"))
				return testCommitTime, nil
			},
		}}

		commitTime, err := scb.commitTime()

		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(commitTime).To(BeTemporally("==", time.Date(2026, 10, 27, 9, 15, 0, 0, time.FixedZone("", 3600))))
	})

	t.Run("should report an unparsable commit time", func(t *testing.T) {
		scb := &SourceContainerBuild{GitCli: &mockGitCli{
			ShowFunc: func(format string) (string, error) { return "not a timestamp", nil },
		}}

		_, err := scb.commitTime()

		g.Expect(err).To(MatchError(ContainSubstring("not a timestamp")))
	})

	t.Run("should report a git failure", func(t *testing.T) {
		scb := &SourceContainerBuild{GitCli: &mockGitCli{
			ShowFunc: func(format string) (string, error) { return "", errors.New("exit status 128") },
		}}

		_, err := scb.commitTime()

		g.Expect(err).To(MatchError(ContainSubstring("exit status 128")))
	})
}

func Test_makeSourceArchive(t *testing.T) {
	g := NewWithT(t)
	wantPrefix := "app-" + testCommitSha
	wantModTime := time.Date(2026, 10, 27, 9, 15, 0, 0, time.FixedZone("", 3600))

	t.Run("should archive the tracked files under a name and commit prefix", func(t *testing.T) {
		scb, _, workDir := setupSourceRepo(t, map[string]string{
			"README.md":   "readme\n",
			"cmd/main.go": "package main\n",
		})
		sibDirs := &sourceImageBuildDirs{}

		err := scb.makeSourceArchive(workDir, sibDirs)
		g.Expect(err).ToNot(HaveOccurred())

		archiveDir := filepath.Join(workDir, "source_archive")
		g.Expect(sibDirs.extraSrcDirs).To(Equal([]string{archiveDir}))

		entries := readArchive(t, filepath.Join(archiveDir, wantPrefix+".tar.gz"))
		g.Expect(entries).To(HaveLen(2))
		g.Expect(entries).To(HaveKey(wantPrefix + "/README.md"))
		g.Expect(entries).To(HaveKey(wantPrefix + "/cmd/main.go"))
		g.Expect(entries[wantPrefix+"/README.md"].content).To(Equal("readme\n"))
		g.Expect(entries[wantPrefix+"/cmd/main.go"].content).To(Equal("package main\n"))
	})

	t.Run("should stamp every entry with the commit time and drop ownership", func(t *testing.T) {
		scb, _, workDir := setupSourceRepo(t, map[string]string{"a.txt": "a", "b.txt": "b"})

		err := scb.makeSourceArchive(workDir, &sourceImageBuildDirs{})
		g.Expect(err).ToNot(HaveOccurred())

		entries := readArchive(t, filepath.Join(workDir, "source_archive", wantPrefix+".tar.gz"))
		g.Expect(entries).ToNot(BeEmpty())
		for _, entry := range entries {
			g.Expect(entry.modTime).To(BeTemporally("==", wantModTime))
			g.Expect(entry.uid).To(Equal(0))
			g.Expect(entry.gid).To(Equal(0))
		}
	})

	t.Run("should store a symlink without following it", func(t *testing.T) {
		scb, gitCli, workDir := setupSourceRepo(t, map[string]string{"real.txt": "real content\n"})

		// A link escaping the repository: following it would copy the content of
		// its target into an archive that gets published.
		secret := filepath.Join(t.TempDir(), "secret")
		g.Expect(os.WriteFile(secret, []byte("do not publish"), 0600)).To(Succeed())
		link := filepath.Join(scb.Params.SourceDir, "link.txt")
		g.Expect(os.Symlink(secret, link)).To(Succeed())

		gitCli.LsFilesFunc = func(recurseSubmodules bool) ([]string, error) {
			return []string{"link.txt", "real.txt"}, nil
		}

		err := scb.makeSourceArchive(workDir, &sourceImageBuildDirs{})
		g.Expect(err).ToNot(HaveOccurred())

		entries := readArchive(t, filepath.Join(workDir, "source_archive", wantPrefix+".tar.gz"))
		g.Expect(entries[wantPrefix+"/link.txt"].linkName).To(Equal(secret))
		g.Expect(entries[wantPrefix+"/link.txt"].content).To(BeEmpty())
		g.Expect(entries[wantPrefix+"/real.txt"].content).To(Equal("real content\n"))
	})

	t.Run("should name a directory entry with a trailing separator", func(t *testing.T) {
		scb, gitCli, workDir := setupSourceRepo(t, map[string]string{"a.txt": "a"})

		// A submodule that was never initialized: git ls-files reports the
		// gitlink itself, which is an empty directory in the working tree.
		g.Expect(os.Mkdir(filepath.Join(scb.Params.SourceDir, "sub"), 0755)).To(Succeed())
		gitCli.LsFilesFunc = func(recurseSubmodules bool) ([]string, error) {
			return []string{"a.txt", "sub"}, nil
		}

		err := scb.makeSourceArchive(workDir, &sourceImageBuildDirs{})
		g.Expect(err).ToNot(HaveOccurred())

		entries := readArchive(t, filepath.Join(workDir, "source_archive", wantPrefix+".tar.gz"))
		g.Expect(entries).To(HaveKey(wantPrefix + "/sub/"))
		g.Expect(entries).To(HaveKey(wantPrefix + "/a.txt"))
	})

	t.Run("should produce an identical archive for the same commit", func(t *testing.T) {
		files := map[string]string{"a.txt": "a\n", "dir/b.txt": "b\n"}

		scbOne, _, workDirOne := setupSourceRepo(t, files)
		g.Expect(scbOne.makeSourceArchive(workDirOne, &sourceImageBuildDirs{})).To(Succeed())

		scbTwo, _, workDirTwo := setupSourceRepo(t, files)
		g.Expect(scbTwo.makeSourceArchive(workDirTwo, &sourceImageBuildDirs{})).To(Succeed())

		name := wantPrefix + ".tar.gz"
		first, err := os.ReadFile(filepath.Join(workDirOne, "source_archive", name))
		g.Expect(err).ToNot(HaveOccurred())
		second, err := os.ReadFile(filepath.Join(workDirTwo, "source_archive", name))
		g.Expect(err).ToNot(HaveOccurred())

		g.Expect(first).To(Equal(second))
	})

	t.Run("should stash the working tree and restore it afterwards", func(t *testing.T) {
		scb, gitCli, workDir := setupSourceRepo(t, map[string]string{"a.txt": "a"})

		var calls []string
		gitCli.StashFunc = func() error { calls = append(calls, "stash"); return nil }
		gitCli.StashPopFunc = func() error { calls = append(calls, "pop"); return nil }
		gitCli.LsFilesFunc = func(recurseSubmodules bool) ([]string, error) {
			calls = append(calls, "ls-files")
			return []string{"a.txt"}, nil
		}

		err := scb.makeSourceArchive(workDir, &sourceImageBuildDirs{})

		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(calls).To(Equal([]string{"stash", "ls-files", "pop"}))
	})

	t.Run("should restore the working tree when archiving fails", func(t *testing.T) {
		scb, gitCli, workDir := setupSourceRepo(t, map[string]string{"a.txt": "a"})

		popped := false
		gitCli.StashPopFunc = func() error { popped = true; return nil }
		gitCli.LsFilesFunc = func(recurseSubmodules bool) ([]string, error) {
			return nil, errors.New("exit status 128")
		}

		err := scb.makeSourceArchive(workDir, &sourceImageBuildDirs{})

		g.Expect(err).To(MatchError(ContainSubstring("exit status 128")))
		g.Expect(popped).To(BeTrue())
	})

	t.Run("should tolerate a stash pop that finds nothing to restore", func(t *testing.T) {
		scb, gitCli, workDir := setupSourceRepo(t, map[string]string{"a.txt": "a"})
		gitCli.StashPopFunc = func() error { return errors.New("No stash entries found.") }

		err := scb.makeSourceArchive(workDir, &sourceImageBuildDirs{})

		g.Expect(err).ToNot(HaveOccurred())
	})

	t.Run("should report a failure to stash", func(t *testing.T) {
		scb, gitCli, workDir := setupSourceRepo(t, map[string]string{"a.txt": "a"})
		gitCli.StashFunc = func() error { return errors.New("not a git repository") }

		err := scb.makeSourceArchive(workDir, &sourceImageBuildDirs{})

		g.Expect(err).To(MatchError(ContainSubstring("not a git repository")))
	})

	t.Run("should report a tracked file that is missing from the working tree", func(t *testing.T) {
		scb, gitCli, workDir := setupSourceRepo(t, map[string]string{"a.txt": "a"})
		gitCli.LsFilesFunc = func(recurseSubmodules bool) ([]string, error) {
			return []string{"a.txt", "gone.txt"}, nil
		}

		err := scb.makeSourceArchive(workDir, &sourceImageBuildDirs{})

		g.Expect(err).To(MatchError(ContainSubstring("gone.txt")))
	})
}
