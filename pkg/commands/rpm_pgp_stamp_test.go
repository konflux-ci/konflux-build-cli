package commands

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
)

func TestExtractKeyID(t *testing.T) {
	tests := []struct {
		sig      string
		expected string
	}{
		{"RSA/SHA256, Thu Jun 25 11:38:08 2026, Key ID 199e2f91fd431d51", "199e2f91fd431d51"},
		{"(none)", pgpUnsigned},
		{"RSA/SHA256, no key", pgpUnsigned},
		{"", pgpUnsigned},
		{"Key ID ABC123", "abc123"},
	}
	for _, tt := range tests {
		t.Run(tt.sig, func(t *testing.T) {
			g := NewWithT(t)
			g.Expect(extractKeyID(tt.sig)).To(Equal(tt.expected))
		})
	}
}

func TestFindRpmDB(t *testing.T) {
	g := NewWithT(t)
	tmpDir := t.TempDir()

	g.Expect(findRpmDB(tmpDir)).To(BeEmpty())

	rpmDir := filepath.Join(tmpDir, "usr", "lib", "sysimage", "rpm")
	g.Expect(os.MkdirAll(rpmDir, 0o755)).To(Succeed())
	g.Expect(os.WriteFile(filepath.Join(rpmDir, "rpmdb.sqlite"), []byte("fake"), 0o644)).To(Succeed())
	g.Expect(findRpmDB(tmpDir)).To(Equal(rpmDir))

	ostreeDir := t.TempDir()
	shareRPM := filepath.Join(ostreeDir, "usr", "share", "rpm")
	g.Expect(os.MkdirAll(shareRPM, 0o755)).To(Succeed())
	g.Expect(os.WriteFile(filepath.Join(shareRPM, "Packages"), []byte("fake"), 0o644)).To(Succeed())
	g.Expect(findRpmDB(ostreeDir)).To(Equal(shareRPM))

	varLibDir := t.TempDir()
	varRPM := filepath.Join(varLibDir, "var", "lib", "rpm")
	g.Expect(os.MkdirAll(varRPM, 0o755)).To(Succeed())
	g.Expect(os.WriteFile(filepath.Join(varRPM, "Packages.db"), []byte("fake"), 0o644)).To(Succeed())
	g.Expect(findRpmDB(varLibDir)).To(Equal(varRPM))

	emptyDBDir := t.TempDir()
	emptyRPM := filepath.Join(emptyDBDir, "var", "lib", "rpm")
	g.Expect(os.MkdirAll(emptyRPM, 0o755)).To(Succeed())
	g.Expect(findRpmDB(emptyDBDir)).To(BeEmpty(), "directory without rpmdb files is not a DB")
}

func TestStampSPDX(t *testing.T) {
	g := NewWithT(t)
	doc := map[string]interface{}{
		"spdxVersion": "SPDX-2.3",
		"SPDXID":      "SPDXRef-DOCUMENT",
		"packages": []interface{}{
			map[string]interface{}{"SPDXID": "SPDXRef-Package-rpm-bash", "name": "bash"},
		},
	}
	entries := []rpmSignatureEntry{
		{Name: "bash", Version: "5.2.26-6.el10", Arch: "x86_64", KeyID: "199e2f91fd431d51"},
		{Name: "unsigned-pkg", Version: "1.0-1", Arch: "noarch", KeyID: pgpUnsigned},
	}

	stamped, unsigned := stampSPDX(doc, entries)
	g.Expect(stamped).To(Equal(2))
	g.Expect(unsigned).To(Equal(1))

	anns := asJSONList(g, doc["annotations"])
	g.Expect(anns).To(HaveLen(2))
	comments := annotationCommentSet(g, anns)
	g.Expect(comments).To(HaveKey("bash-5.2.26-6.el10.x86_64 rpm:pgp-key-id:199e2f91fd431d51"))
	g.Expect(comments).To(HaveKey("unsigned-pkg-1.0-1.noarch rpm:pgp-key-id:unsigned"))

	first := asJSONObject(g, anns[0])
	g.Expect(first["annotator"]).To(Equal(pgpAnnotator))
	g.Expect(first["annotationType"]).To(Equal("OTHER"))
	_, hasElementID := first["spdxElementId"]
	g.Expect(hasElementID).To(BeFalse(), "document list stamps must not point at Syft packages")

	pkg := jsonFirstObject(g, doc["packages"])
	_, hasPkgAnns := pkg["annotations"]
	g.Expect(hasPkgAnns).To(BeFalse())
}

func TestStampSPDX_SkipAlreadyStamped(t *testing.T) {
	g := NewWithT(t)
	rec := "bash-5.2.26-6.el10.x86_64 rpm:pgp-key-id:199e2f91fd431d51"
	doc := map[string]interface{}{
		"spdxVersion": "SPDX-2.3",
		"annotations": []interface{}{
			map[string]interface{}{
				"annotator": pgpAnnotator,
				"comment":   rec,
			},
		},
	}

	stamped, unsigned := stampSPDX(doc, []rpmSignatureEntry{
		{Name: "bash", Version: "5.2.26-6.el10", Arch: "x86_64", KeyID: "deadbeef"},
	})
	g.Expect(stamped).To(Equal(0))
	g.Expect(unsigned).To(Equal(0))
	g.Expect(asJSONList(g, doc["annotations"])).To(HaveLen(1))
	g.Expect(asJSONObject(g, asJSONList(g, doc["annotations"])[0])["comment"]).To(Equal(rec))
}

func TestStampCycloneDX(t *testing.T) {
	g := NewWithT(t)
	doc := map[string]interface{}{
		"bomFormat": "CycloneDX",
		"components": []interface{}{
			map[string]interface{}{"name": "bash", "purl": "pkg:rpm/redhat/bash@5.2.26-6.el10?arch=x86_64"},
		},
	}
	entries := []rpmSignatureEntry{
		{Name: "bash", Version: "5.2.26-6.el10", Arch: "x86_64", KeyID: "199e2f91fd431d51"},
		{Name: "unsigned-pkg", Version: "1.0-1", Arch: "noarch", KeyID: pgpUnsigned},
	}

	stamped, unsigned := stampCycloneDX(doc, entries)
	g.Expect(stamped).To(Equal(2))
	g.Expect(unsigned).To(Equal(1))

	meta := asJSONObject(g, doc["metadata"])
	props := asJSONList(g, meta["properties"])
	g.Expect(props).To(HaveLen(2))
	g.Expect(asJSONObject(g, props[0])["name"]).To(Equal("rpm:pgp-key-id"))
	g.Expect(asJSONObject(g, props[0])["value"]).To(Equal("bash-5.2.26-6.el10.x86_64 rpm:pgp-key-id:199e2f91fd431d51"))
	g.Expect(asJSONObject(g, props[1])["value"]).To(Equal("unsigned-pkg-1.0-1.noarch rpm:pgp-key-id:unsigned"))

	comp := jsonFirstObject(g, doc["components"])
	_, hasProps := comp["properties"]
	g.Expect(hasProps).To(BeFalse(), "stamps must not be embedded on components")
}

func TestStampCycloneDX_SkipAlreadyStamped(t *testing.T) {
	g := NewWithT(t)
	rec := "bash-5.2.26-6.el10.x86_64 rpm:pgp-key-id:199e2f91fd431d51"
	doc := map[string]interface{}{
		"bomFormat": "CycloneDX",
		"metadata": map[string]interface{}{
			"properties": []interface{}{
				map[string]interface{}{"name": pgpKeyIDName, "value": rec},
			},
		},
	}

	stamped, unsigned := stampCycloneDX(doc, []rpmSignatureEntry{
		{Name: "bash", Version: "5.2.26-6.el10", Arch: "x86_64", KeyID: "deadbeef"},
	})
	g.Expect(stamped).To(Equal(0))
	g.Expect(unsigned).To(Equal(0))
	props := asJSONList(g, asJSONObject(g, doc["metadata"])["properties"])
	g.Expect(props).To(HaveLen(1))
}

func TestStampSBOMFile_SPDX(t *testing.T) {
	g := NewWithT(t)
	sbom := map[string]interface{}{
		"spdxVersion": "SPDX-2.3",
		"packages":    []interface{}{map[string]interface{}{"name": "bash"}},
	}

	tmpDir := t.TempDir()
	sbomPath := filepath.Join(tmpDir, "sbom.json")
	data, err := json.Marshal(sbom)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(os.WriteFile(sbomPath, data, 0o644)).To(Succeed())
	g.Expect(os.Chmod(sbomPath, 0o600)).To(Succeed())

	err = stampSBOMFile(sbomPath, []rpmSignatureEntry{
		{Name: "bash", Version: "5.2.26-6.el10", Arch: "x86_64", KeyID: "199e2f91fd431d51"},
	})
	g.Expect(err).ToNot(HaveOccurred())

	result, err := os.ReadFile(sbomPath)
	g.Expect(err).ToNot(HaveOccurred())
	var resultDoc map[string]interface{}
	g.Expect(json.Unmarshal(result, &resultDoc)).To(Succeed())

	comments := annotationCommentSet(g, asJSONList(g, resultDoc["annotations"]))
	g.Expect(comments).To(HaveKey("bash-5.2.26-6.el10.x86_64 rpm:pgp-key-id:199e2f91fd431d51"))
	g.Expect(string(result)).To(ContainSubstring("\n  "), "stamped SBOM must stay pretty-printed")
	g.Expect(string(result)).To(HaveSuffix("\n"))

	info, err := os.Stat(sbomPath)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(info.Mode().Perm()).To(Equal(os.FileMode(0o600)))

	tmpLeftovers, err := filepath.Glob(filepath.Join(tmpDir, "*.tmp"))
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(tmpLeftovers).To(BeEmpty())
}

func TestReplaceFileAtomically(t *testing.T) {
	g := NewWithT(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "sbom.json")
	g.Expect(os.WriteFile(path, []byte("original\n"), 0o644)).To(Succeed())

	g.Expect(replaceFileAtomically(path, []byte("stamped\n"), 0o600)).To(Succeed())

	got, err := os.ReadFile(path)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(string(got)).To(Equal("stamped\n"))
	info, err := os.Stat(path)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(info.Mode().Perm()).To(Equal(os.FileMode(0o600)))
	g.Expect(info.Name()).To(Equal("sbom.json"))

	leftovers, err := filepath.Glob(filepath.Join(dir, "*.tmp"))
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(leftovers).To(BeEmpty(), "successful rename must remove the temp file")
}

func TestReplaceFileAtomically_CreateTempFails(t *testing.T) {
	g := NewWithT(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "sbom.json")
	original := []byte("original\n")
	g.Expect(os.WriteFile(path, original, 0o644)).To(Succeed())
	stubCreateTemp(t, errors.New("no space left on device"))

	err := replaceFileAtomically(path, []byte("stamped\n"), 0o644)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("creating temporary SBOM"))
	g.Expect(err.Error()).To(ContainSubstring("no space left on device"))

	got, readErr := os.ReadFile(path)
	g.Expect(readErr).ToNot(HaveOccurred())
	g.Expect(got).To(Equal(original), "failed temp create must not truncate the original")
}

func TestStampSBOMFile_Idempotent(t *testing.T) {
	g := NewWithT(t)
	dir := t.TempDir()
	sbomPath := filepath.Join(dir, "sbom.json")
	g.Expect(os.WriteFile(sbomPath, []byte(`{"spdxVersion":"SPDX-2.3","packages":[]}`), 0o644)).To(Succeed())
	entries := []rpmSignatureEntry{
		{Name: "bash", Version: "5.2.26-6.el10", Arch: "x86_64", KeyID: "199e2f91fd431d51"},
	}

	g.Expect(stampSBOMFile(sbomPath, entries)).To(Succeed())
	g.Expect(stampSBOMFile(sbomPath, entries)).To(Succeed())

	result, err := os.ReadFile(sbomPath)
	g.Expect(err).ToNot(HaveOccurred())
	var resultDoc map[string]interface{}
	g.Expect(json.Unmarshal(result, &resultDoc)).To(Succeed())
	g.Expect(asJSONList(g, resultDoc["annotations"])).To(HaveLen(1))

	leftovers, err := filepath.Glob(filepath.Join(dir, "*.tmp"))
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(leftovers).To(BeEmpty())
}

func TestStampSBOMFile_EmptyEntriesLeavesFileUnchanged(t *testing.T) {
	g := NewWithT(t)
	sbomPath := filepath.Join(t.TempDir(), "sbom.json")
	original := []byte(`{"spdxVersion":"SPDX-2.3","packages":[{"name":"bash"}]}`)
	g.Expect(os.WriteFile(sbomPath, original, 0o644)).To(Succeed())

	g.Expect(stampSBOMFile(sbomPath, nil)).To(Succeed())

	got, err := os.ReadFile(sbomPath)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(got).To(Equal(original))
}

func TestStampSBOMFile_CycloneDX(t *testing.T) {
	g := NewWithT(t)
	sbom := map[string]interface{}{
		"bomFormat":   "CycloneDX",
		"specVersion": "1.6",
		"components":  []interface{}{map[string]interface{}{"name": "bash"}},
	}
	sbomPath := filepath.Join(t.TempDir(), "sbom.json")
	data, err := json.Marshal(sbom)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(os.WriteFile(sbomPath, data, 0o644)).To(Succeed())

	err = stampSBOMFile(sbomPath, []rpmSignatureEntry{
		{Name: "bash", Version: "5.2.26-6.el10", Arch: "x86_64", KeyID: "199e2f91fd431d51"},
	})
	g.Expect(err).ToNot(HaveOccurred())

	result, err := os.ReadFile(sbomPath)
	g.Expect(err).ToNot(HaveOccurred())
	var resultDoc map[string]interface{}
	g.Expect(json.Unmarshal(result, &resultDoc)).To(Succeed())
	prop := jsonFirstObject(g, asJSONObject(g, resultDoc["metadata"])["properties"])
	g.Expect(prop["name"]).To(Equal("rpm:pgp-key-id"))
	g.Expect(prop["value"]).To(Equal("bash-5.2.26-6.el10.x86_64 rpm:pgp-key-id:199e2f91fd431d51"))
	leftovers, err := filepath.Glob(filepath.Join(filepath.Dir(sbomPath), "*.tmp"))
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(leftovers).To(BeEmpty())
}

func TestStampRPMPgpKeyIDs_StampFileMissing(t *testing.T) {
	g := NewWithT(t)
	dir := t.TempDir()
	writeFakeRPMTo(t, dir, `#!/bin/sh
printf 'bash\t5.2.26-6.el10\tx86_64\tKey ID 199e2f91fd431d51\n'
`)
	t.Setenv("PATH", dir)

	mount := t.TempDir()
	db := filepath.Join(mount, "usr", "lib", "sysimage", "rpm")
	g.Expect(os.MkdirAll(db, 0o755)).To(Succeed())
	g.Expect(os.WriteFile(filepath.Join(db, "rpmdb.sqlite"), []byte("fake"), 0o644)).To(Succeed())

	stampRPMPgpKeyIDs(mount, filepath.Join(t.TempDir(), "missing.json"))
}

func TestStampSBOMFile_MissingAndBadJSON(t *testing.T) {
	g := NewWithT(t)
	err := stampSBOMFile(filepath.Join(t.TempDir(), "missing.json"), nil)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("reading SBOM"))

	badPath := filepath.Join(t.TempDir(), "bad.json")
	g.Expect(os.WriteFile(badPath, []byte("{not-json"), 0o644)).To(Succeed())
	err = stampSBOMFile(badPath, nil)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("parsing SBOM JSON"))
}

func TestStampSBOMFile_PreservesOriginalOnWriteFailure(t *testing.T) {
	g := NewWithT(t)
	dir := t.TempDir()
	sbomPath := filepath.Join(dir, "sbom.json")
	original := []byte(`{"spdxVersion":"SPDX-2.3","packages":[{"name":"bash"}]}`)
	g.Expect(os.WriteFile(sbomPath, original, 0o644)).To(Succeed())
	stubCreateTemp(t, errors.New("no space left on device"))

	err := stampSBOMFile(sbomPath, []rpmSignatureEntry{
		{Name: "bash", Version: "5.2.26-6.el10", Arch: "x86_64", KeyID: "199e2f91fd431d51"},
	})
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("creating temporary SBOM"))

	got, readErr := os.ReadFile(sbomPath)
	g.Expect(readErr).ToNot(HaveOccurred())
	g.Expect(got).To(Equal(original))
}

func TestQueryRPMSignatures(t *testing.T) {
	g := NewWithT(t)
	rpmPath := writeFakeRPM(t, `#!/bin/sh
cat <<'EOF'
bash	5.2.26-6.el10	x86_64	RSA/SHA256, Thu Jun 25 11:38:08 2026, Key ID 199e2f91fd431d51
unsigned-pkg	1.0-1	noarch	(none)
gpg-pubkey	fd431d51-4ae0493b	x86_64	(none)
not-a-valid-line
EOF
`)

	entries, err := queryRPMSignatures(rpmPath, "/unused")
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(entries).To(HaveLen(2))
	g.Expect(entries[0]).To(Equal(rpmSignatureEntry{Name: "bash", Version: "5.2.26-6.el10", Arch: "x86_64", KeyID: "199e2f91fd431d51"}))
	g.Expect(entries[1]).To(Equal(rpmSignatureEntry{Name: "unsigned-pkg", Version: "1.0-1", Arch: "noarch", KeyID: pgpUnsigned}))
}

func TestQueryRPMSignatures_EmptyAndError(t *testing.T) {
	g := NewWithT(t)
	emptyRPM := writeFakeRPM(t, "#!/bin/sh\nexit 0\n")
	entries, err := queryRPMSignatures(emptyRPM, "/unused")
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(entries).To(BeEmpty())

	failRPM := writeFakeRPM(t, `#!/bin/sh
echo 'error: cannot open Packages database in /broken' >&2
exit 1
`)
	_, err = queryRPMSignatures(failRPM, "/broken")
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("rpm -qa"))
	g.Expect(err.Error()).To(ContainSubstring("cannot open Packages database"))
}

func TestStampRPMPgpKeyIDs_NoRPM(t *testing.T) {
	g := NewWithT(t)
	t.Setenv("PATH", t.TempDir())
	sbomPath := writeSPDXSBOM(t)

	stampRPMPgpKeyIDs(t.TempDir(), sbomPath)

	comments := spdxComments(t, sbomPath)
	g.Expect(comments).NotTo(HaveKey(pgpNoRpmDB), "missing rpm binary must not write a no-rpmdb sentinel")
	g.Expect(comments).To(HaveKey(pgpStampReasonPrefix + reasonRpmNotInPath))
}

func TestStampRPMPgpKeyIDs_NoRpmDB(t *testing.T) {
	g := NewWithT(t)
	called := filepath.Join(t.TempDir(), "rpm-called")
	dir := t.TempDir()
	writeFakeRPMTo(t, dir, "#!/bin/sh\ntouch '"+called+"'\nexit 0\n")
	t.Setenv("PATH", dir)
	sbomPath := writeSPDXSBOM(t)

	stampRPMPgpKeyIDs(t.TempDir(), sbomPath)
	_, err := os.Stat(called)
	g.Expect(os.IsNotExist(err)).To(BeTrue(), "rpm must not be invoked when no rpmdb is present")
	comments := spdxComments(t, sbomPath)
	g.Expect(comments).To(HaveKey(pgpNoRpmDB))
	g.Expect(comments).To(HaveKey(pgpStampReasonPrefix + reasonNoRpmDB))
}

func TestStampRPMPgpKeyIDs_EmptyRpmQA(t *testing.T) {
	g := NewWithT(t)
	dir := t.TempDir()
	writeFakeRPMTo(t, dir, "#!/bin/sh\nexit 0\n")
	t.Setenv("PATH", dir)

	mount := t.TempDir()
	db := filepath.Join(mount, "usr", "lib", "sysimage", "rpm")
	g.Expect(os.MkdirAll(db, 0o755)).To(Succeed())
	g.Expect(os.WriteFile(filepath.Join(db, "rpmdb.sqlite"), []byte("fake"), 0o644)).To(Succeed())
	sbomPath := writeSPDXSBOM(t)

	stampRPMPgpKeyIDs(mount, sbomPath)

	comments := spdxComments(t, sbomPath)
	g.Expect(comments).To(HaveKey(pgpNoRpmDB))
	g.Expect(comments).To(HaveKey(pgpStampReasonPrefix + reasonEmptyRpmDB))
}

func TestStampRPMPgpKeyIDs_QueryErrorWritesReason(t *testing.T) {
	g := NewWithT(t)
	dir := t.TempDir()
	writeFakeRPMTo(t, dir, `#!/bin/sh
echo 'error: cannot open Packages database' >&2
exit 1
`)
	t.Setenv("PATH", dir)

	mount := t.TempDir()
	db := filepath.Join(mount, "usr", "lib", "sysimage", "rpm")
	g.Expect(os.MkdirAll(db, 0o755)).To(Succeed())
	g.Expect(os.WriteFile(filepath.Join(db, "rpmdb.sqlite"), []byte("fake"), 0o644)).To(Succeed())
	sbomPath := writeSPDXSBOM(t)

	stampRPMPgpKeyIDs(mount, sbomPath)

	comments := spdxComments(t, sbomPath)
	g.Expect(comments).NotTo(HaveKey(pgpNoRpmDB), "rpm -qa failure must not write a no-rpmdb sentinel")
	var reason string
	for c := range comments {
		if strings.HasPrefix(c, pgpStampReasonPrefix) {
			reason = c
		}
	}
	g.Expect(reason).To(ContainSubstring(pgpStampReasonPrefix + "rpm -qa failed:"))
	g.Expect(reason).To(ContainSubstring("cannot open Packages database"))
}

func TestStampRPMPgpKeyIDs_NoRPM_CycloneDX(t *testing.T) {
	g := NewWithT(t)
	t.Setenv("PATH", t.TempDir())
	sbomPath := writeCycloneDXSBOM(t)

	stampRPMPgpKeyIDs(t.TempDir(), sbomPath)

	props := cycloneDXPropertySet(t, sbomPath)
	g.Expect(props).NotTo(HaveKey(pgpKeyIDName + "=" + pgpNoRpmDB))
	g.Expect(props).To(HaveKey(pgpStampReasonName + "=" + reasonRpmNotInPath))
}

func TestStampRPMPgpKeyIDs_NoRpmDB_CycloneDX(t *testing.T) {
	g := NewWithT(t)
	dir := t.TempDir()
	writeFakeRPMTo(t, dir, "#!/bin/sh\nexit 0\n")
	t.Setenv("PATH", dir)
	sbomPath := writeCycloneDXSBOM(t)

	stampRPMPgpKeyIDs(t.TempDir(), sbomPath)

	props := cycloneDXPropertySet(t, sbomPath)
	g.Expect(props).To(HaveKey(pgpKeyIDName + "=" + pgpNoRpmDB))
	g.Expect(props).To(HaveKey(pgpStampReasonName + "=" + reasonNoRpmDB))
}

func TestStampRPMPgpKeyIDs_EmptyRpmQA_CycloneDX(t *testing.T) {
	g := NewWithT(t)
	dir := t.TempDir()
	writeFakeRPMTo(t, dir, "#!/bin/sh\nexit 0\n")
	t.Setenv("PATH", dir)

	mount := t.TempDir()
	db := filepath.Join(mount, "usr", "lib", "sysimage", "rpm")
	g.Expect(os.MkdirAll(db, 0o755)).To(Succeed())
	g.Expect(os.WriteFile(filepath.Join(db, "rpmdb.sqlite"), []byte("fake"), 0o644)).To(Succeed())
	sbomPath := writeCycloneDXSBOM(t)

	stampRPMPgpKeyIDs(mount, sbomPath)

	props := cycloneDXPropertySet(t, sbomPath)
	g.Expect(props).To(HaveKey(pgpKeyIDName + "=" + pgpNoRpmDB))
	g.Expect(props).To(HaveKey(pgpStampReasonName + "=" + reasonEmptyRpmDB))
}

func TestStampRPMPgpKeyIDs_QueryErrorWritesReason_CycloneDX(t *testing.T) {
	g := NewWithT(t)
	dir := t.TempDir()
	writeFakeRPMTo(t, dir, `#!/bin/sh
echo 'error: cannot open Packages database' >&2
exit 1
`)
	t.Setenv("PATH", dir)

	mount := t.TempDir()
	db := filepath.Join(mount, "usr", "lib", "sysimage", "rpm")
	g.Expect(os.MkdirAll(db, 0o755)).To(Succeed())
	g.Expect(os.WriteFile(filepath.Join(db, "rpmdb.sqlite"), []byte("fake"), 0o644)).To(Succeed())
	sbomPath := writeCycloneDXSBOM(t)

	stampRPMPgpKeyIDs(mount, sbomPath)

	props := cycloneDXPropertySet(t, sbomPath)
	g.Expect(props).NotTo(HaveKey(pgpKeyIDName + "=" + pgpNoRpmDB))
	var reason string
	for p := range props {
		if strings.HasPrefix(p, pgpStampReasonName+"=") {
			reason = strings.TrimPrefix(p, pgpStampReasonName+"=")
		}
	}
	g.Expect(reason).To(HavePrefix("rpm -qa failed:"))
	g.Expect(reason).To(ContainSubstring("cannot open Packages database"))
}

func TestStampRPMPgpKeyIDs_WritesPackageRecordsWithoutReason(t *testing.T) {
	g := NewWithT(t)
	dir := t.TempDir()
	writeFakeRPMTo(t, dir, `#!/bin/sh
printf 'bash\t5.2.26-6.el10\tx86_64\tKey ID 199e2f91fd431d51\n'
`)
	t.Setenv("PATH", dir)

	mount := t.TempDir()
	db := filepath.Join(mount, "usr", "lib", "sysimage", "rpm")
	g.Expect(os.MkdirAll(db, 0o755)).To(Succeed())
	g.Expect(os.WriteFile(filepath.Join(db, "rpmdb.sqlite"), []byte("fake"), 0o644)).To(Succeed())
	sbomPath := writeSPDXSBOM(t)

	stampRPMPgpKeyIDs(mount, sbomPath)

	comments := spdxComments(t, sbomPath)
	g.Expect(comments).To(HaveKey("bash-5.2.26-6.el10.x86_64 rpm:pgp-key-id:199e2f91fd431d51"))
	g.Expect(comments).NotTo(HaveKey(pgpNoRpmDB))
	for c := range comments {
		g.Expect(c).NotTo(HavePrefix(pgpStampReasonPrefix))
	}
}

func TestStampNoRpmDB_SPDX(t *testing.T) {
	g := NewWithT(t)
	sbomPath := writeSPDXSBOM(t)
	g.Expect(os.Chmod(sbomPath, 0o600)).To(Succeed())

	g.Expect(stampNoRpmDB(sbomPath)).To(Succeed())

	result, err := os.ReadFile(sbomPath)
	g.Expect(err).ToNot(HaveOccurred())
	var doc map[string]interface{}
	g.Expect(json.Unmarshal(result, &doc)).To(Succeed())
	anns := asJSONList(g, doc["annotations"])
	g.Expect(anns).To(HaveLen(2))
	comments := annotationCommentSet(g, anns)
	g.Expect(comments).To(HaveKey(pgpNoRpmDB))
	g.Expect(comments).To(HaveKey(pgpStampReasonPrefix + reasonNoRpmDB))
	ann := asJSONObject(g, anns[0])
	g.Expect(ann["annotator"]).To(Equal(pgpAnnotator))
	g.Expect(ann["annotationType"]).To(Equal("OTHER"))
	_, hasElementID := ann["spdxElementId"]
	g.Expect(hasElementID).To(BeFalse())
	g.Expect(string(result)).To(HaveSuffix("\n"))
	info, err := os.Stat(sbomPath)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(info.Mode().Perm()).To(Equal(os.FileMode(0o600)))
}

func TestStampNoRpmDB_CycloneDX(t *testing.T) {
	g := NewWithT(t)
	sbomPath := filepath.Join(t.TempDir(), "sbom.json")
	data, err := json.Marshal(map[string]interface{}{
		"bomFormat":   "CycloneDX",
		"specVersion": "1.6",
		"components":  []interface{}{},
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(os.WriteFile(sbomPath, data, 0o644)).To(Succeed())

	g.Expect(stampNoRpmDB(sbomPath)).To(Succeed())

	result, err := os.ReadFile(sbomPath)
	g.Expect(err).ToNot(HaveOccurred())
	var doc map[string]interface{}
	g.Expect(json.Unmarshal(result, &doc)).To(Succeed())
	props := asJSONList(g, asJSONObject(g, doc["metadata"])["properties"])
	g.Expect(props).To(HaveLen(2))
	g.Expect(asJSONObject(g, props[0])).To(Equal(map[string]interface{}{
		"name":  pgpKeyIDName,
		"value": pgpNoRpmDB,
	}))
	g.Expect(asJSONObject(g, props[1])).To(Equal(map[string]interface{}{
		"name":  pgpStampReasonName,
		"value": reasonNoRpmDB,
	}))
}

func TestStampNoRpmDB_Idempotent(t *testing.T) {
	g := NewWithT(t)
	sbomPath := writeSPDXSBOM(t)
	g.Expect(stampNoRpmDB(sbomPath)).To(Succeed())
	first, err := os.ReadFile(sbomPath)
	g.Expect(err).ToNot(HaveOccurred())

	g.Expect(stampNoRpmDB(sbomPath)).To(Succeed())
	second, err := os.ReadFile(sbomPath)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(second).To(Equal(first))
	g.Expect(asJSONList(g, mustParseSBOM(t, sbomPath)["annotations"])).To(HaveLen(2))
}

func TestStampNoRpmDB_MissingFile(t *testing.T) {
	g := NewWithT(t)
	err := stampNoRpmDB(filepath.Join(t.TempDir(), "missing.json"))
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("reading SBOM"))
}

func TestStampScanOutcome_ReasonOnlyNoSentinel_SPDX(t *testing.T) {
	g := NewWithT(t)
	sbomPath := writeSPDXSBOM(t)

	g.Expect(stampScanOutcome(sbomPath, false, reasonRpmNotInPath)).To(Succeed())

	comments := spdxComments(t, sbomPath)
	g.Expect(comments).To(Equal(map[string]struct{}{
		pgpStampReasonPrefix + reasonRpmNotInPath: {},
	}))
}

func TestStampScanOutcome_ReasonOnlyNoSentinel_CycloneDX(t *testing.T) {
	g := NewWithT(t)
	sbomPath := writeCycloneDXSBOM(t)

	g.Expect(stampScanOutcome(sbomPath, false, "rpm -qa failed: boom")).To(Succeed())

	props := cycloneDXPropertySet(t, sbomPath)
	g.Expect(props).To(Equal(map[string]struct{}{
		pgpStampReasonName + "=rpm -qa failed: boom": {},
	}))
}

func TestStampScanOutcome_ReasonOnlyIdempotent(t *testing.T) {
	g := NewWithT(t)
	sbomPath := writeSPDXSBOM(t)
	g.Expect(stampScanOutcome(sbomPath, false, reasonRpmNotInPath)).To(Succeed())
	first, err := os.ReadFile(sbomPath)
	g.Expect(err).ToNot(HaveOccurred())

	g.Expect(stampScanOutcome(sbomPath, false, reasonRpmNotInPath)).To(Succeed())
	second, err := os.ReadFile(sbomPath)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(second).To(Equal(first))
	g.Expect(asJSONList(g, mustParseSBOM(t, sbomPath)["annotations"])).To(HaveLen(1))
}

func TestStampScanOutcome_BadJSON(t *testing.T) {
	g := NewWithT(t)
	sbomPath := filepath.Join(t.TempDir(), "bad.json")
	g.Expect(os.WriteFile(sbomPath, []byte("{not-json"), 0o644)).To(Succeed())

	err := stampScanOutcome(sbomPath, true, reasonNoRpmDB)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("parsing SBOM JSON"))
}

func stubCreateTemp(t *testing.T, err error) {
	t.Helper()
	orig := createTemp
	createTemp = func(string, string) (*os.File, error) {
		return nil, err
	}
	t.Cleanup(func() { createTemp = orig })
}

func writeFakeRPM(t *testing.T, body string) string {
	t.Helper()
	return writeFakeRPMTo(t, t.TempDir(), body)
}

func writeFakeRPMTo(t *testing.T, dir, body string) string {
	t.Helper()
	g := NewWithT(t)
	path := filepath.Join(dir, "rpm")
	g.Expect(os.WriteFile(path, []byte(body), 0o755)).To(Succeed())
	g.Expect(os.Chmod(path, 0o755)).To(Succeed())
	return path
}

func writeSPDXSBOM(t *testing.T) string {
	t.Helper()
	g := NewWithT(t)
	sbomPath := filepath.Join(t.TempDir(), "sbom.json")
	data, err := json.Marshal(map[string]interface{}{
		"spdxVersion": "SPDX-2.3",
		"packages":    []interface{}{map[string]interface{}{"name": "bash"}},
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(os.WriteFile(sbomPath, data, 0o644)).To(Succeed())
	return sbomPath
}

func writeCycloneDXSBOM(t *testing.T) string {
	t.Helper()
	g := NewWithT(t)
	sbomPath := filepath.Join(t.TempDir(), "sbom.json")
	data, err := json.Marshal(map[string]interface{}{
		"bomFormat":   "CycloneDX",
		"specVersion": "1.6",
		"components":  []interface{}{},
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(os.WriteFile(sbomPath, data, 0o644)).To(Succeed())
	return sbomPath
}

func mustParseSBOM(t *testing.T, sbomPath string) map[string]interface{} {
	t.Helper()
	g := NewWithT(t)
	data, err := os.ReadFile(sbomPath)
	g.Expect(err).ToNot(HaveOccurred())
	var doc map[string]interface{}
	g.Expect(json.Unmarshal(data, &doc)).To(Succeed())
	return doc
}

func spdxComments(t *testing.T, sbomPath string) map[string]struct{} {
	t.Helper()
	g := NewWithT(t)
	return annotationCommentSet(g, asJSONList(g, mustParseSBOM(t, sbomPath)["annotations"]))
}

func asJSONList(g *WithT, v interface{}) []interface{} {
	list, ok := v.([]interface{})
	g.Expect(ok).To(BeTrue(), "expected a JSON array")
	return list
}

func asJSONObject(g *WithT, v interface{}) map[string]interface{} {
	obj, ok := v.(map[string]interface{})
	g.Expect(ok).To(BeTrue(), "expected a JSON object")
	return obj
}

func jsonFirstObject(g *WithT, v interface{}) map[string]interface{} {
	items := asJSONList(g, v)
	g.Expect(items).ToNot(BeEmpty())
	return asJSONObject(g, items[0])
}

func annotationCommentSet(g *WithT, anns []interface{}) map[string]struct{} {
	out := map[string]struct{}{}
	for _, aRaw := range anns {
		comment, _ := asJSONObject(g, aRaw)["comment"].(string)
		out[comment] = struct{}{}
	}
	return out
}

func cycloneDXPropertySet(t *testing.T, sbomPath string) map[string]struct{} {
	t.Helper()
	g := NewWithT(t)
	props := asJSONList(g, asJSONObject(g, mustParseSBOM(t, sbomPath)["metadata"])["properties"])
	out := map[string]struct{}{}
	for _, raw := range props {
		p := asJSONObject(g, raw)
		name, _ := p["name"].(string)
		val, _ := p["value"].(string)
		out[name+"="+val] = struct{}{}
	}
	return out
}
