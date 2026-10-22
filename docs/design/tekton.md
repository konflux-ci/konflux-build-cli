## Tekton integration

The CLI does not depend on Tekton, but made with it in mind, so the CLI can be easily used in Tekton tasks.
A simplified example:
```yaml
apiVersion: tekton.dev/v1beta1
kind: Task
metadata:
  name: apply-tags
spec:
  description: Applies additional tags to the built image.
  params:
  - name: IMAGE_URL
    description: Image repository and tag reference of the the built image.
    type: string
  - name: IMAGE_DIGEST
    description: Image digest of the built image.
    type: string
  - name: ADDITIONAL_TAGS
    description: Additional tags that will be applied to the image in the registry.
    type: array
    default: []
  steps:
    - name: apply-additional-tags
      image: quai.io/org/tekton-catalog/konflux-build-cli:latest
      command: ["konflux-build-cli", "image", "apply-tags"]
      args:
        - --image-url
        - $(params.IMAGE_URL)
        - --digest
        - $(params.IMAGE_DIGEST)
        - --tags
        - $(params.ADDITIONAL_TAGS[*])
```

## Base image outputs

`image build --resolved-base-images-json-output /shared/base_image_artifacts.json`
writes one file in the [Tekton artifacts format](https://tekton.dev/docs/pipelines/artifacts/):

```json
{"inputs":[{"name":"base-images","values":[{"uri":"oci://registry.example.com/base:latest","digest":{"sha256":"<64 hex characters>"}}]}]}
```

The file contains only `uri` and digest-map values. An empty input list produces
`{"inputs":[{"name":"base-images","values":[]}]}`. Repeated references are
preserved, and the CLI imposes no artifact count limit. URIs use `oci://` with a
lowercase registry authority. Output directories must exist, output files must
be regular files, and output paths must not alias one another.

The existing `--resolved-base-images-output` flag remains unchanged. Its
independent text output contains the original reference followed by its
digest-qualified reference on each line, for existing SBOM consumers.

Tasks can adapt the JSON with jq to existing Chains object results, converting
`{"sha256":"<hex>"}` to `"sha256:<hex>"`. The task owns deduplication and any
fixed result count limit. Each result must declare string properties `uri` and
`digest` and a name ending in `ARTIFACT_INPUTS`.

Once the deployment supports the alpha artifacts API and Chains consumption,
the same JSON can be written to `/tekton/artifacts/provenance.json` without the
result adapter. This requires the `enable-artifacts` feature flag. Deploy a CLI
release providing the JSON flag before updating task definitions.

Both formats share one resolution of the CLI's collected image list. The list
uses Containerfile parsing, build argument expansion and stage dependency
selection. It excludes unsupported transports. Digests are resolved from the
local image inventory after the build; this is not an execution report from
Buildah and does not eliminate index-versus-platform digest ambiguity.
