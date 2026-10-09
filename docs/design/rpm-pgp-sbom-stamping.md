# Recording RPM PGP key IDs in the image SBOM

TLDR: after Syft writes the image SBOM, while the buildah mount is still live,
`image build` queries the image rpmdb with `rpm -qa` and writes one document-level
record per installed RPM (PGP key ID, or `unsigned`). The image build does not
fail if this step fails. A later `rpms-signature-scan` will read these records
from the attached SBOM instead of extracting the image rpmdb, using the same
signed/unsigned logic it uses today.

This is an internal step of `image build`, not a new CLI command.

## Why this lives in konflux-build-cli

The scan today extracts the **final image** rpmdb and runs `rpm -qa`. That query
is the source of truth we want to keep.

Doing it in the Tekton task shell after the image is pushed does not work for
remote builds: the task runs in a nested podman container where `buildah mount`
fails silently (no `CAP_SYS_ADMIN` for overlay mounts). `konflux-build-cli`
already has a working mount from the Syft scan, so we piggyback on that.

We do not join Syft packages. Syft can omit RPMs that `rpm -qa` reports. The
records are an inventory of what is installed in the shipped image, not
package-level metadata on Syft’s list.

## What we write

One record per installed RPM except `gpg-pubkey` (those are the signing keys
themselves):

```text
bash-5.2.26-6.el10.x86_64 rpm:pgp-key-id:199e2f91fd431d51
some-pkg-1.0-1.noarch rpm:pgp-key-id:unsigned
```

| Format | Where |
|---|---|
| SPDX 2.3 | document `annotations` (no `spdxElementId`) |
| CycloneDX | `metadata.properties` named `rpm:pgp-key-id` |

Outcomes other than a NEVRA list:

| Situation | Written | Intended scan result |
|---|---|---|
| RPMs found | NEVRA records as above | same as today’s `rpm -qa` (unsigned counted as unsigned) |
| No rpmdb, or `rpm -qa` lists nothing | sentinel `rpm:pgp-key-id:no-rpmdb` plus a reason | success, unsigned 0 |
| `rpm` missing from PATH, or `rpm -qa` fails | reason only (`rpm:pgp-stamp-reason:…`) | fail with that reason |
| Stamping never ran (old builder) | nothing | fail: stamps missing |

The SBOM file is replaced with temp-file + rename so a failed write cannot
truncate Syft’s output.

## Why a custom document record

SPDX 2.3 and CycloneDX have no field that means “this installed RPM was signed
by PGP key X”. Checksums/hashes, CycloneDX `signature` (signs the BOM), and
Syft RPM metadata are all something else.

Package-level SPDX annotations would be the obvious place for a signature, but
Mobster drops them on parse/export. Document-level annotations and CycloneDX
`metadata.properties` survive that round-trip. The attached SBOM is what the
scan will read, so the records have to still be there after Mobster.

Generic SBOM tools will ignore these records. That is acceptable: the consumer
is our scan, matching today’s `rpm -qa` set. Making this first-class belongs in
Syft/Mobster/the spec later, not in a field we invent here.

We are not attaching a separate signature artifact in this work.

## What “missing” means for consumers

Do not treat “this Syft package has no signature field” as one bucket. The
scan should not look at Syft packages for this. It should only read the
document records.

- **Unsigned RPM:** an explicit `… rpm:pgp-key-id:unsigned` record. That is a
  real installed unsigned package, same as today.
- **Stamp could not run:** a reason, no list, no sentinel. Fail the scan. Do
  not treat this as “all signed” or “all unsigned”.
- **No stamps at all:** fail. The builder did not stamp.
- **Hermeto / builder-stage RPMs that are not in the final image:** not in
  `rpm -qa`, not in these records, not in today’s scan either. This inventory
  is the shipped image, not every RPM that ever appeared in a prefetch SBOM.

The **build** is best-effort (stamping must not fail `image build`). The
**scan** is not: missing or failed stamps are a fail.

## Can the builder’s `rpm` always read the image db?

No. We run whatever `rpm` is on `PATH` in the build image (`konflux-build-cli`,
based on `task-runner`) against the mounted filesystem. We look for
`rpmdb.sqlite`, `Packages`, or `Packages.db` under `usr/lib/sysimage/rpm`,
`var/lib/rpm`, and `usr/share/rpm`.

That is the same class of tool the current scan uses (image db + `rpm` from
the tools image). We move the query to build time; we do not add a new parser.

A newer `rpm` reading an older db is the usual case. If the image db is newer
than the builder’s `rpm` (or a layout we do not look for), `rpm -qa` fails, we
write the reason, and the scan fails. Fix is to ship a builder whose `rpm` can
read that db. No change to the record format.

## Namespace

Records use `rpm:pgp-key-id` / `rpm:pgp-stamp-reason`. That names the RPM
header field, not the producing tool. Hermeto’s `hermeto:*` properties are a
different producer; we are not putting this data in Hermeto’s namespace.

## Out of scope for this change

- Changing `rpms-signature-scan` or the tools image (follow-up: read these
  records from the attached SBOM instead of extracting the rpmdb).
- Changing Mobster.
- Failing the image build on stamp errors.
- Signing the SBOM, or recording checksums as a substitute for PGP key IDs.
