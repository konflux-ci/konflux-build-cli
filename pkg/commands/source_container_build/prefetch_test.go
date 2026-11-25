package source_container_build

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/gomega"
)

// writeFile writes content at path, creating the directories leading to it.
func writeFile(t *testing.T, path string, content []byte) {
	t.Helper()
	g := NewWithT(t)

	g.Expect(os.MkdirAll(filepath.Dir(path), 0755)).To(Succeed())
	g.Expect(os.WriteFile(path, content, 0644)).To(Succeed())
}

// writeGzip writes a real gzip file, so that it is detected by its content
// rather than by its name.
func writeGzip(t *testing.T, path string) {
	t.Helper()
	g := NewWithT(t)

	var buf bytes.Buffer
	gzipWriter := gzip.NewWriter(&buf)
	_, err := gzipWriter.Write([]byte("source"))
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(gzipWriter.Close()).To(Succeed())

	writeFile(t, path, buf.Bytes())
}

// writeZip writes a real zip file whose first entry is firstEntry.
func writeZip(t *testing.T, path, firstEntry string) {
	t.Helper()
	g := NewWithT(t)

	var buf bytes.Buffer
	zipWriter := zip.NewWriter(&buf)
	entry, err := zipWriter.Create(firstEntry)
	g.Expect(err).ToNot(HaveOccurred())
	_, err = entry.Write([]byte("Manifest-Version: 1.0\n"))
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(zipWriter.Close()).To(Succeed())

	writeFile(t, path, buf.Bytes())
}

// writeSrpm writes a file carrying the RPM magic number.
func writeSrpm(t *testing.T, path, content string) {
	t.Helper()
	writeFile(t, path, append([]byte{0xed, 0xab, 0xee, 0xdb}, []byte(content)...))
}

// setupPrefetch returns a command whose prefetch directory holds the given
// files, plus the work directory to gather them into.
func setupPrefetch(t *testing.T) (*SourceContainerBuild, string, string) {
	t.Helper()

	prefetchDir := t.TempDir()
	scb := &SourceContainerBuild{Params: &Params{PrefetchArtifactsDir: prefetchDir}}

	return scb, prefetchDir, t.TempDir()
}

func Test_gatherPrefetchedSources(t *testing.T) {
	g := NewWithT(t)

	t.Run("should do nothing without a prefetch output directory", func(t *testing.T) {
		scb, _, workDir := setupPrefetch(t)
		sibDirs := &sourceImageBuildDirs{}

		gathered, err := scb.gatherPrefetchedSources(workDir, sibDirs)

		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(gathered).To(BeFalse())
		g.Expect(sibDirs.extraSrcDirs).To(BeEmpty())
	})

	t.Run("should take archives and leave everything else", func(t *testing.T) {
		scb, prefetchDir, workDir := setupPrefetch(t)
		deps := filepath.Join(prefetchDir, "output", "deps", "pip")
		writeGzip(t, filepath.Join(deps, "requests-2.0.tar.gz"))
		writeFile(t, filepath.Join(deps, "notes.txt"), []byte("not an archive"))
		sibDirs := &sourceImageBuildDirs{}

		gathered, err := scb.gatherPrefetchedSources(workDir, sibDirs)

		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(gathered).To(BeTrue())
		g.Expect(sibDirs.extraSrcDirs).To(ConsistOf(filepath.Join(workDir, "prefetched_sources", "src-0")))

		depsDest := filepath.Join(workDir, "prefetched_sources", "src-0", "deps", "pip")
		g.Expect(filepath.Join(depsDest, "requests-2.0.tar.gz")).To(BeAnExistingFile())
		g.Expect(filepath.Join(depsDest, "notes.txt")).ToNot(BeAnExistingFile())
	})

	t.Run("should take a jar, which is a zip under a more specific type", func(t *testing.T) {
		scb, prefetchDir, workDir := setupPrefetch(t)
		deps := filepath.Join(prefetchDir, "output", "deps", "maven")
		writeZip(t, filepath.Join(deps, "lib-1.0-sources.jar"), "META-INF/MANIFEST.MF")
		sibDirs := &sourceImageBuildDirs{}

		gathered, err := scb.gatherPrefetchedSources(workDir, sibDirs)

		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(gathered).To(BeTrue())
		g.Expect(filepath.Join(workDir, "prefetched_sources", "src-0", "deps", "maven", "lib-1.0-sources.jar")).
			To(BeAnExistingFile())
	})

	t.Run("should take a compress archive", func(t *testing.T) {
		scb, prefetchDir, workDir := setupPrefetch(t)
		deps := filepath.Join(prefetchDir, "output", "deps", "generic")
		writeFile(t, filepath.Join(deps, "old.tar.Z"), append([]byte{0x1f, 0x9d}, []byte("compressed")...))

		gathered, err := scb.gatherPrefetchedSources(workDir, &sourceImageBuildDirs{})

		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(gathered).To(BeTrue())
		g.Expect(filepath.Join(workDir, "prefetched_sources", "src-0", "deps", "generic", "old.tar.Z")).
			To(BeAnExistingFile())
	})

	t.Run("should take every file of a package manager that vendors source trees", func(t *testing.T) {
		scb, prefetchDir, workDir := setupPrefetch(t)
		deps := filepath.Join(prefetchDir, "output", "deps", "cargo")
		writeFile(t, filepath.Join(deps, "serde-1.0", "src", "lib.rs"), []byte("pub fn f() {}"))
		writeFile(t, filepath.Join(deps, "serde-1.0", "Cargo.toml"), []byte("[package]"))

		gathered, err := scb.gatherPrefetchedSources(workDir, &sourceImageBuildDirs{})

		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(gathered).To(BeTrue())

		// The relative path below the package manager directory is preserved.
		depsDest := filepath.Join(workDir, "prefetched_sources", "src-0", "deps", "cargo")
		g.Expect(filepath.Join(depsDest, "serde-1.0", "src", "lib.rs")).To(BeAnExistingFile())
		g.Expect(filepath.Join(depsDest, "serde-1.0", "Cargo.toml")).To(BeAnExistingFile())
	})

	t.Run("should give each package manager its own directory, named in a stable order", func(t *testing.T) {
		scb, prefetchDir, workDir := setupPrefetch(t)
		depsDir := filepath.Join(prefetchDir, "output", "deps")
		writeGzip(t, filepath.Join(depsDir, "pip", "a.tar.gz"))
		writeGzip(t, filepath.Join(depsDir, "npm", "b.tar.gz"))
		sibDirs := &sourceImageBuildDirs{}

		_, err := scb.gatherPrefetchedSources(workDir, sibDirs)

		g.Expect(err).ToNot(HaveOccurred())
		prepared := filepath.Join(workDir, "prefetched_sources")
		g.Expect(sibDirs.extraSrcDirs).To(ConsistOf(
			filepath.Join(prepared, "src-0"),
			filepath.Join(prepared, "src-1"),
		))
		// Sorted by package manager name, so npm comes first.
		g.Expect(filepath.Join(prepared, "src-0", "deps", "npm", "b.tar.gz")).To(BeAnExistingFile())
		g.Expect(filepath.Join(prepared, "src-1", "deps", "pip", "a.tar.gz")).To(BeAnExistingFile())
	})

	t.Run("should keep a symlinked dependency a symlink", func(t *testing.T) {
		scb, prefetchDir, workDir := setupPrefetch(t)
		deps := filepath.Join(prefetchDir, "output", "deps", "cargo")
		writeFile(t, filepath.Join(deps, "real.rs"), []byte("real"))

		// A link leaving the prefetch tree: following it would copy the content
		// of its target into the source image.
		secret := filepath.Join(t.TempDir(), "secret")
		g.Expect(os.WriteFile(secret, []byte("do not publish"), 0600)).To(Succeed())
		g.Expect(os.Symlink(secret, filepath.Join(deps, "link.rs"))).To(Succeed())

		_, err := scb.gatherPrefetchedSources(workDir, &sourceImageBuildDirs{})
		g.Expect(err).ToNot(HaveOccurred())

		copied := filepath.Join(workDir, "prefetched_sources", "src-0", "deps", "cargo", "link.rs")
		info, err := os.Lstat(copied)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(info.Mode() & os.ModeSymlink).ToNot(BeZero())
	})

	t.Run("should collect the prefetched SRPMs", func(t *testing.T) {
		scb, prefetchDir, workDir := setupPrefetch(t)
		output := filepath.Join(prefetchDir, "output")
		writeSrpm(t, filepath.Join(output, "deps", "rpm", "foo-1.0.src.rpm"), "foo")
		writeSrpm(t, filepath.Join(output, "bar-2.0.src.rpm"), "bar")
		// Named like an SRPM but not one, so not a source to take.
		writeFile(t, filepath.Join(output, "fake-3.0.src.rpm"), []byte("not an rpm"))
		sibDirs := &sourceImageBuildDirs{}

		gathered, err := scb.gatherPrefetchedSources(workDir, sibDirs)

		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(gathered).To(BeTrue())
		g.Expect(sibDirs.rpmDir).To(Equal(filepath.Join(workDir, "rpm_dir")))
		g.Expect(filepath.Join(sibDirs.rpmDir, "foo-1.0.src.rpm")).To(BeAnExistingFile())
		g.Expect(filepath.Join(sibDirs.rpmDir, "bar-2.0.src.rpm")).To(BeAnExistingFile())
		g.Expect(filepath.Join(sibDirs.rpmDir, "fake-3.0.src.rpm")).ToNot(BeAnExistingFile())
	})

	t.Run("should keep one copy of the same SRPM found twice", func(t *testing.T) {
		scb, prefetchDir, workDir := setupPrefetch(t)
		output := filepath.Join(prefetchDir, "output")
		writeSrpm(t, filepath.Join(output, "a", "foo-1.0.src.rpm"), "identical")
		writeSrpm(t, filepath.Join(output, "b", "foo-1.0.src.rpm"), "identical")
		sibDirs := &sourceImageBuildDirs{}

		_, err := scb.gatherPrefetchedSources(workDir, sibDirs)

		g.Expect(err).ToNot(HaveOccurred())
		entries, err := os.ReadDir(sibDirs.rpmDir)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(entries).To(HaveLen(1))
		g.Expect(entries[0].Name()).To(Equal("foo-1.0.src.rpm"))
	})

	t.Run("should keep both SRPMs that share a name but differ", func(t *testing.T) {
		scb, prefetchDir, workDir := setupPrefetch(t)
		output := filepath.Join(prefetchDir, "output")
		writeSrpm(t, filepath.Join(output, "a", "foo-1.0.src.rpm"), "built for x86_64")
		writeSrpm(t, filepath.Join(output, "b", "foo-1.0.src.rpm"), "built for aarch64")
		sibDirs := &sourceImageBuildDirs{}

		_, err := scb.gatherPrefetchedSources(workDir, sibDirs)

		g.Expect(err).ToNot(HaveOccurred())
		entries, err := os.ReadDir(sibDirs.rpmDir)
		g.Expect(err).ToNot(HaveOccurred())

		names := make([]string, 0, len(entries))
		for _, entry := range entries {
			names = append(names, entry.Name())
		}
		// One keeps the original name, the other is stored under a name that
		// includes its checksum, so neither is lost.
		g.Expect(names).To(HaveLen(2))
		g.Expect(names).To(ContainElement("foo-1.0.src.rpm"))
		g.Expect(names).To(ContainElement(And(
			HaveSuffix("-foo-1.0.src.rpm"),
			Not(Equal("foo-1.0.src.rpm")),
		)))
	})

	t.Run("should take the prefetch env file", func(t *testing.T) {
		scb, prefetchDir, workDir := setupPrefetch(t)
		writeFile(t, filepath.Join(prefetchDir, "output", "deps", ".keep"), nil)
		writeFile(t, filepath.Join(prefetchDir, "prefetch.env"), []byte("export GOPROXY=off\n"))
		sibDirs := &sourceImageBuildDirs{}

		_, err := scb.gatherPrefetchedSources(workDir, sibDirs)

		g.Expect(err).ToNot(HaveOccurred())
		envDir := filepath.Join(workDir, "prefetch_env")
		g.Expect(sibDirs.extraSrcDirs).To(ContainElement(envDir))

		content, err := os.ReadFile(filepath.Join(envDir, "prefetch.env"))
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(string(content)).To(Equal("export GOPROXY=off\n"))
	})

	t.Run("should report nothing gathered from an empty prefetch output", func(t *testing.T) {
		scb, prefetchDir, workDir := setupPrefetch(t)
		g.Expect(os.MkdirAll(filepath.Join(prefetchDir, "output"), 0755)).To(Succeed())
		sibDirs := &sourceImageBuildDirs{}

		gathered, err := scb.gatherPrefetchedSources(workDir, sibDirs)

		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(gathered).To(BeFalse())
		g.Expect(sibDirs.extraSrcDirs).To(BeEmpty())
		// The directory is still handed to gobsi, which ignores it when empty.
		g.Expect(sibDirs.rpmDir).To(BeADirectory())
	})
}
