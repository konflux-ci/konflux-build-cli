// Package commands - RPM PGP Key ID stamping for SBOMs.
//
// After Syft generates an SBOM from a built container image, this code reads the
// image's RPM database (via the still-mounted buildah filesystem) and writes every
// installed RPM's PGP key ID into the SBOM document (not onto Syft packages).
//
// Why this lives here rather than in the Tekton task shell script:
// In remote builds the task runs inside a nested podman container where `buildah mount`
// fails silently (no CAP_SYS_ADMIN for overlay mounts). konflux-build-cli already has
// a working mount from the Syft scan step, so we piggyback on that.
//
// The stamping is best-effort: any error is logged and the build continues.
package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	l "github.com/konflux-ci/konflux-build-cli/pkg/logger"
)

// createTemp is os.CreateTemp; tests replace it so the failure path works as root.
var createTemp = os.CreateTemp

const (
	// pgpKeyIDName is the CycloneDX metadata property name and the SPDX comment stem.
	pgpKeyIDName = "rpm:pgp-key-id"
	// pgpKeyIDPrefix is prepended to every key ID value in stamp records.
	pgpKeyIDPrefix = pgpKeyIDName + ":"
	// pgpUnsigned is the value used when an RPM has no PGP signature.
	pgpUnsigned = "unsigned"
	// pgpNoRpmDB is written when the image has no RPM database (or an empty one)
	// so the verifier can tell that apart from "stamping never ran".
	pgpNoRpmDB = pgpKeyIDPrefix + "no-rpmdb"
	// pgpStampReasonName is the CycloneDX property name for a human-readable outcome.
	pgpStampReasonName = "rpm:pgp-stamp-reason"
	// pgpStampReasonPrefix is prepended to the reason in SPDX annotation comments.
	pgpStampReasonPrefix = pgpStampReasonName + ":"
	// pgpAnnotator identifies the tool that produced SPDX document annotations.
	pgpAnnotator = "Tool: konflux-build-cli:rpm-signature"

	// reasonRpmNotInPath is stamped when the builder image has no rpm binary.
	reasonRpmNotInPath = "rpm not found in PATH"
	// reasonNoRpmDB is stamped when none of the well-known rpmdb paths exist.
	reasonNoRpmDB = "no RPM database in image"
	// reasonEmptyRpmDB is stamped when rpm -qa succeeds but lists no packages.
	reasonEmptyRpmDB = "RPM database is empty"
)

// rpmSignatureEntry holds one parsed line from `rpm -qa`.
type rpmSignatureEntry struct {
	Name    string
	Version string // VERSION-RELEASE (e.g. "5.2.26-6.el10")
	Arch    string // e.g. "x86_64", "noarch"
	KeyID   string // hex key ID or "unsigned"
}

// record is the document-level stamp payload written into SPDX comments or
// CycloneDX property values, e.g.
// "bash-5.2.26-6.el10.x86_64 rpm:pgp-key-id:199e2f91fd431d51".
func (e rpmSignatureEntry) record() string {
	return e.nevra() + " " + pgpKeyIDPrefix + e.KeyID
}

// nevra returns NAME-VERSION-RELEASE.ARCH for this RPM.
func (e rpmSignatureEntry) nevra() string {
	return e.Name + "-" + e.Version + "." + e.Arch
}

// stampRPMPgpKeyIDs is the main entry point. It reads the rpmdb from the mounted
// image filesystem, queries PGP signature headers, and writes that list into the
// SBOM document. All errors are logged as warnings — the build never fails.
//
// Outcomes written into the SBOM (document-level, not on Syft packages):
//   - rpm missing from PATH: reason only (verifier fails with that text)
//   - no rpmdb files: sentinel rpm:pgp-key-id:no-rpmdb + reason (verifier succeeds)
//   - rpm -qa returns no packages: same sentinel + a different reason (verifier succeeds)
//   - rpm -qa fails: reason only, no sentinel (verifier fails with that text)
func stampRPMPgpKeyIDs(mountPoint string, sbomPath string) {
	rpmPath, err := exec.LookPath("rpm")
	if err != nil {
		l.Logger.Warn(reasonRpmNotInPath)
		if err := stampScanOutcome(sbomPath, false, reasonRpmNotInPath); err != nil {
			l.Logger.Warnf("Failed to stamp RPM PGP reason: %v", err)
		}
		return
	}

	dbPath := findRpmDB(mountPoint)
	if dbPath == "" {
		l.Logger.Info(reasonNoRpmDB)
		if err := stampScanOutcome(sbomPath, true, reasonNoRpmDB); err != nil {
			l.Logger.Warnf("Failed to stamp no-rpmdb sentinel: %v", err)
		}
		return
	}

	l.Logger.Infof("Querying RPM signatures from %s", dbPath)

	entries, err := queryRPMSignatures(rpmPath, dbPath)
	if err != nil {
		reason := "rpm -qa failed: " + err.Error()
		l.Logger.Warn(reason)
		if err := stampScanOutcome(sbomPath, false, reason); err != nil {
			l.Logger.Warnf("Failed to stamp RPM PGP reason: %v", err)
		}
		return
	}
	if len(entries) == 0 {
		l.Logger.Info(reasonEmptyRpmDB)
		if err := stampScanOutcome(sbomPath, true, reasonEmptyRpmDB); err != nil {
			l.Logger.Warnf("Failed to stamp no-rpmdb sentinel: %v", err)
		}
		return
	}

	l.Logger.Infof("Found %d RPM signature entries", len(entries))

	if err := stampSBOMFile(sbomPath, entries); err != nil {
		l.Logger.Warnf("Failed to stamp SBOM with RPM PGP Key IDs: %v", err)
	}
}

// findRpmDB looks for an rpm database in well-known locations under mountPoint.
// A directory counts as a DB only if it contains rpmdb.sqlite, Packages, or
// Packages.db. Returns that directory, or "" if none of those files exist.
// An empty result means "do not run rpm -qa"; the caller stamps the no-rpmdb
// sentinel instead.
func findRpmDB(mountPoint string) string {
	// Ordered by likelihood: RHEL 10+ uses sysimage, older uses /var/lib/rpm,
	// OSTree/bootc layouts may keep the DB under /usr/share/rpm.
	dbDirs := []string{
		"usr/lib/sysimage/rpm",
		"var/lib/rpm",
		"usr/share/rpm",
	}
	dbFiles := []string{"rpmdb.sqlite", "Packages", "Packages.db"}

	for _, rel := range dbDirs {
		dir := filepath.Join(mountPoint, rel)
		for _, f := range dbFiles {
			if _, err := os.Stat(filepath.Join(dir, f)); err == nil {
				return dir
			}
		}
	}
	return ""
}

// queryRPMSignatures runs `rpm -qa` against the image's rpmdb and parses each
// line into name, version-release, arch, and PGP key ID. gpg-pubkey rows are
// skipped because they are the signing keys themselves, not installed packages.
func queryRPMSignatures(rpmPath, dbPath string) ([]rpmSignatureEntry, error) {
	// Output example: "bash\t5.2.26-6.el10\tx86_64\tRSA/SHA256, ..., Key ID 199e2f91fd431d51"
	qf := `%{NAME}\t%{VERSION}-%{RELEASE}\t%{ARCH}\t` +
		`%|DSAHEADER?{%{DSAHEADER:pgpsig}}:{%|RSAHEADER?{%{RSAHEADER:pgpsig}}:{(none)}|}|\n`

	cmd := exec.Command(rpmPath, "-qa", "--dbpath", dbPath, "--qf", qf) //nolint:gosec
	out, err := cmd.Output()
	if err != nil {
		return nil, rpmQAError(err)
	}

	var entries []rpmSignatureEntry
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 4)
		if len(parts) != 4 {
			continue
		}
		name, ver, arch, sig := parts[0], parts[1], parts[2], parts[3]

		// gpg-pubkey entries are the signing keys themselves, not real packages.
		if name == "gpg-pubkey" {
			continue
		}

		entries = append(entries, rpmSignatureEntry{
			Name:    name,
			Version: ver,
			Arch:    arch,
			KeyID:   extractKeyID(sig),
		})
	}
	return entries, nil
}

// rpmQAError wraps a failed rpm -qa invocation. Output() stores stderr on
// *exec.ExitError; Error() is only "exit status N", so include stderr here.
func rpmQAError(err error) error {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		if stderr := strings.TrimSpace(string(exitErr.Stderr)); stderr != "" {
			return fmt.Errorf("rpm -qa: %w: %s", err, stderr)
		}
	}
	return fmt.Errorf("rpm -qa: %w", err)
}

// extractKeyID parses the PGP signature string from rpm to extract the hex key ID.
// Example input: "RSA/SHA256, Thu Jun 25 11:38:08 2026, Key ID 199e2f91fd431d51"
// Returns the lowercased key ID, or "unsigned" if no key is found.
func extractKeyID(sig string) string {
	const marker = "Key ID "
	idx := strings.Index(sig, marker)
	if idx < 0 {
		return pgpUnsigned
	}
	kid := strings.TrimSpace(sig[idx+len(marker):])
	if kid == "" {
		return pgpUnsigned
	}
	return strings.ToLower(kid)
}

// stampSBOMFile writes one NEVRA stamp record per installed RPM onto the SBOM
// (SPDX document annotations or CycloneDX metadata.properties) and replaces the
// file atomically. A no-op if every NEVRA is already present.
func stampSBOMFile(sbomPath string, entries []rpmSignatureEntry) error {
	data, err := os.ReadFile(sbomPath) //nolint:gosec
	if err != nil {
		return fmt.Errorf("reading SBOM: %w", err)
	}

	var doc map[string]interface{}
	if err := json.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("parsing SBOM JSON: %w", err)
	}

	var stamped, unsigned int
	if isSPDX(doc) {
		stamped, unsigned = stampSPDX(doc, entries)
	} else {
		stamped, unsigned = stampCycloneDX(doc, entries)
	}

	if stamped == 0 {
		l.Logger.Info("No new RPM PGP Key ID records to stamp")
		return nil
	}
	if err := writeSBOMDoc(sbomPath, doc); err != nil {
		return err
	}
	l.Logger.Infof("Stamped %s on %d RPMs (%d unsigned)", pgpKeyIDName, stamped, unsigned)
	return nil
}

// stampNoRpmDB writes the no-rpmdb sentinel and the "no RPM database in image"
// reason. Tests call this helper directly; production uses stampScanOutcome
// with a case-specific reason.
func stampNoRpmDB(sbomPath string) error {
	return stampScanOutcome(sbomPath, true, reasonNoRpmDB)
}

// stampScanOutcome records how RPM PGP stamping finished so the verifier can
// distinguish "no rpmdb" (pass) from builder errors (fail) from "stamping never
// ran" (fail, generic missing-stamps message).
//
// If sentinel is true, also writes rpm:pgp-key-id:no-rpmdb (verifier success).
// reason is always written as rpm:pgp-stamp-reason (SPDX comment prefix, or a
// CycloneDX property of that name). Idempotent: existing identical comments or
// properties are not duplicated.
func stampScanOutcome(sbomPath string, sentinel bool, reason string) error {
	data, err := os.ReadFile(sbomPath) //nolint:gosec
	if err != nil {
		return fmt.Errorf("reading SBOM: %w", err)
	}

	var doc map[string]interface{}
	if err := json.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("parsing SBOM JSON: %w", err)
	}

	var added bool
	if isSPDX(doc) {
		if sentinel {
			added = appendSPDXComment(doc, pgpNoRpmDB) || added
		}
		if reason != "" {
			added = appendSPDXComment(doc, pgpStampReasonPrefix+reason) || added
		}
	} else {
		if sentinel {
			added = appendCycloneDXProperty(doc, pgpKeyIDName, pgpNoRpmDB) || added
		}
		if reason != "" {
			added = appendCycloneDXProperty(doc, pgpStampReasonName, reason) || added
		}
	}
	if !added {
		l.Logger.Info("RPM PGP stamp outcome already present")
		return nil
	}

	if err := writeSBOMDoc(sbomPath, doc); err != nil {
		return err
	}
	if sentinel {
		l.Logger.Infof("Stamped %s (%s)", pgpNoRpmDB, reason)
	} else {
		l.Logger.Infof("Stamped %s%s", pgpStampReasonPrefix, reason)
	}
	return nil
}

// appendSPDXComment adds one document-level SPDX annotation with the given
// comment, annotator pgpAnnotator, and no spdxElementId (so Mobster keeps it).
// Returns false if that comment is already present.
func appendSPDXComment(doc map[string]interface{}, comment string) bool {
	docAnns, _ := doc["annotations"].([]interface{})
	if commentsHave(docAnns, comment) {
		return false
	}
	now := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	doc["annotations"] = append(docAnns, map[string]interface{}{
		"annotationDate": now,
		"annotationType": "OTHER",
		"annotator":      pgpAnnotator,
		"comment":        comment,
	})
	return true
}

// appendCycloneDXProperty adds one metadata.property. Creates metadata if it
// is missing. Returns false if name+value is already present.
func appendCycloneDXProperty(doc map[string]interface{}, name, value string) bool {
	meta, _ := doc["metadata"].(map[string]interface{})
	if meta == nil {
		meta = map[string]interface{}{}
		doc["metadata"] = meta
	}
	props, _ := meta["properties"].([]interface{})
	if propertiesHave(props, name, value) {
		return false
	}
	meta["properties"] = append(props, map[string]interface{}{
		"name":  name,
		"value": value,
	})
	return true
}

// commentsHave reports whether any SPDX annotation already has this comment.
func commentsHave(docAnns []interface{}, comment string) bool {
	for _, aRaw := range docAnns {
		a, ok := aRaw.(map[string]interface{})
		if !ok {
			continue
		}
		c, _ := a["comment"].(string)
		if c == comment {
			return true
		}
	}
	return false
}

// propertiesHave reports whether any CycloneDX property already has this name
// and value.
func propertiesHave(props []interface{}, name, value string) bool {
	for _, pRaw := range props {
		p, ok := pRaw.(map[string]interface{})
		if !ok {
			continue
		}
		if p["name"] == name {
			val, _ := p["value"].(string)
			if val == value {
				return true
			}
		}
	}
	return false
}

// writeSBOMDoc pretty-prints doc and atomically replaces sbomPath, keeping the
// original file mode.
func writeSBOMDoc(sbomPath string, doc map[string]interface{}) error {
	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling stamped SBOM: %w", err)
	}
	if len(out) == 0 || out[len(out)-1] != '\n' {
		out = append(out, '\n')
	}
	mode := os.FileMode(0o644)
	if info, statErr := os.Stat(sbomPath); statErr == nil {
		mode = info.Mode().Perm()
	}
	return replaceFileAtomically(sbomPath, out, mode)
}

// isSPDX reports whether doc is an SPDX SBOM (spdxVersion or a packages array).
// Anything else is treated as CycloneDX.
func isSPDX(doc map[string]interface{}) bool {
	if doc["spdxVersion"] != nil {
		return true
	}
	if _, ok := doc["packages"]; ok {
		return true
	}
	return false
}

// replaceFileAtomically writes data to a temp file in the same directory as path,
// then renames it over path so a failed write cannot truncate the original.
// Rename failures are logged; the original file is left in place.
func replaceFileAtomically(path string, data []byte, mode os.FileMode) error {
	tmp, err := createTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return fmt.Errorf("creating temporary SBOM: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpName)
		}
	}()

	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("setting temporary SBOM permissions: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("writing temporary SBOM: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("syncing temporary SBOM: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("closing temporary SBOM: %w", err)
	}

	if err := os.Rename(tmpName, path); err != nil {
		l.Logger.Warnf("Failed to replace SBOM with stamped copy at %s: %v; leaving original SBOM in place", path, err)
		return fmt.Errorf("replacing SBOM with stamped copy: %w", err)
	}
	cleanup = false
	return nil
}

// stampSPDX appends one document annotation per installed RPM. Annotations live
// on the document so Mobster's SPDX round-trip keeps them. They are not bound
// to Syft packages (no spdxElementId).
func stampSPDX(doc map[string]interface{}, entries []rpmSignatureEntry) (stamped, unsigned int) {
	now := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	docAnns, _ := doc["annotations"].([]interface{})
	existing := stampedNEVRAsFromComments(docAnns)

	for _, e := range entries {
		if existing[e.nevra()] {
			continue
		}
		rec := e.record()
		docAnns = append(docAnns, map[string]interface{}{
			"annotationDate": now,
			"annotationType": "OTHER",
			"annotator":      pgpAnnotator,
			"comment":        rec,
		})
		existing[e.nevra()] = true
		stamped++
		if e.KeyID == pgpUnsigned {
			unsigned++
		}
	}

	if stamped > 0 {
		doc["annotations"] = docAnns
	}
	return stamped, unsigned
}

// stampedNEVRAsFromComments returns NEVRAs already present in SPDX document
// annotation comments, used to keep re-stamping idempotent.
func stampedNEVRAsFromComments(docAnns []interface{}) map[string]bool {
	out := make(map[string]bool, len(docAnns))
	for _, aRaw := range docAnns {
		a, ok := aRaw.(map[string]interface{})
		if !ok {
			continue
		}
		comment, _ := a["comment"].(string)
		if nevra, ok := nevraFromRecord(comment); ok {
			out[nevra] = true
		}
	}
	return out
}

// nevraFromRecord extracts the NEVRA prefix from a stamp record
// ("<nevra> rpm:pgp-key-id:<key>"). Returns false for the no-rpmdb sentinel
// and other non-record comments.
func nevraFromRecord(rec string) (string, bool) {
	nevra, _, ok := strings.Cut(rec, " "+pgpKeyIDPrefix)
	if !ok || nevra == "" {
		return "", false
	}
	return nevra, true
}

// stampCycloneDX appends one metadata.property per installed RPM that is not
// already recorded. Property name is rpm:pgp-key-id; value is the NEVRA record.
func stampCycloneDX(doc map[string]interface{}, entries []rpmSignatureEntry) (stamped, unsigned int) {
	meta, _ := doc["metadata"].(map[string]interface{})
	if meta == nil {
		meta = map[string]interface{}{}
		doc["metadata"] = meta
	}
	props, _ := meta["properties"].([]interface{})
	existing := stampedNEVRAsFromProperties(props)

	for _, e := range entries {
		if existing[e.nevra()] {
			continue
		}
		rec := e.record()
		props = append(props, map[string]interface{}{
			"name":  pgpKeyIDName,
			"value": rec,
		})
		existing[e.nevra()] = true
		stamped++
		if e.KeyID == pgpUnsigned {
			unsigned++
		}
	}

	if stamped > 0 {
		meta["properties"] = props
	}
	return stamped, unsigned
}

// stampedNEVRAsFromProperties returns NEVRAs already present in CycloneDX
// rpm:pgp-key-id properties, used to keep re-stamping idempotent.
func stampedNEVRAsFromProperties(props []interface{}) map[string]bool {
	out := make(map[string]bool, len(props))
	for _, pRaw := range props {
		p, ok := pRaw.(map[string]interface{})
		if !ok {
			continue
		}
		if p["name"] != pgpKeyIDName {
			continue
		}
		val, _ := p["value"].(string)
		if nevra, ok := nevraFromRecord(val); ok {
			out[nevra] = true
		}
	}
	return out
}
