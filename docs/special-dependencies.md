
## Updating the dockerfile-json dependency

The `github.com/konflux-ci/dockerfile-json` dependency uses a replace directive in `go.mod` that points to a specific commit from the dev branch.
Update to the latest version with:
```sh
go mod edit -replace github.com/keilerkonzept/dockerfile-json=github.com/konflux-ci/dockerfile-json@dev
go mod tidy
```