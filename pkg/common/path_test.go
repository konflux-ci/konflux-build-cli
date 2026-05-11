package common

import (
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/gomega"
)

func TestResolvePath(t *testing.T) {
	g := NewWithT(t)

	// .
	// ├── target
	// ├── rel-link -> target
	// └── abs-link -> {t.TempDir()}/target
	dir := t.TempDir()
	absTarget := filepath.Join(dir, "target")
	g.Expect(os.Mkdir(absTarget, 0755)).To(Succeed())
	g.Expect(os.Symlink("target", filepath.Join(dir, "rel-link"))).To(Succeed())
	g.Expect(os.Symlink(absTarget, filepath.Join(dir, "abs-link"))).To(Succeed())

	origDir, err := os.Getwd()
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(os.Chdir(dir)).To(Succeed())
	t.Cleanup(func() { os.Chdir(origDir) })

	tests := []struct {
		name  string
		input string
	}{
		{name: "relative input, not symlink", input: "target"},
		{name: "absolute input, not symlink", input: absTarget},
		{name: "relative input, relative symlink", input: "rel-link"},
		{name: "absolute input, relative symlink", input: filepath.Join(dir, "rel-link")},
		{name: "relative input, absolute symlink", input: "abs-link"},
		{name: "absolute input, absolute symlink", input: filepath.Join(dir, "abs-link")},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			rp, err := ResolvePath(tc.input)
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(rp.String()).To(Equal(absTarget))
		})
	}

	t.Run("returns error for non-existent path", func(t *testing.T) {
		g := NewWithT(t)

		_, err := ResolvePath("/no/such/path")
		g.Expect(err).To(HaveOccurred())
	})
}

func TestIsRelativeTo(t *testing.T) {
	tests := []struct {
		name     string
		base     string
		path     string
		expected bool
	}{
		{
			name:     "path equals base",
			base:     "/a/b",
			path:     "/a/b",
			expected: true,
		},
		{
			name:     "path is child of base",
			base:     "/a/b",
			path:     "/a/b/c",
			expected: true,
		},
		{
			name:     "path is parent of base",
			base:     "/a/b",
			path:     "/a",
			expected: false,
		},
		{
			name:     "path is outside base",
			base:     "/a/b",
			path:     "/a/c",
			expected: false,
		},
		{
			name:     "path shares prefix but diverges",
			base:     "/foo/bar",
			path:     "/foo/barbar",
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			p := ResolvedPath(tc.path)
			base := ResolvedPath(tc.base)
			g.Expect(p.IsRelativeTo(base)).To(Equal(tc.expected))
		})
	}
}
