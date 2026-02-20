module github.com/konflux-ci/konflux-build-cli

go 1.25.0

toolchain go1.26.0

require (
	github.com/containerd/platforms v1.0.0-rc.2
	github.com/containers/image/v5 v5.36.2
	github.com/keilerkonzept/dockerfile-json v1.2.2
	github.com/moby/buildkit v0.27.1
	github.com/onsi/gomega v1.39.1
	github.com/opencontainers/go-digest v1.0.0
	github.com/opencontainers/image-spec v1.1.1
	github.com/sirupsen/logrus v1.9.4
	github.com/spf13/cobra v1.10.2
)

require (
	github.com/agext/levenshtein v1.2.3 // indirect
	github.com/containerd/log v0.1.0 // indirect
	github.com/containerd/typeurl/v2 v2.2.3 // indirect
	github.com/containers/storage v1.59.1 // indirect
	github.com/docker/go-units v0.5.0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/moby/docker-image-spec v1.3.1 // indirect
	github.com/onsi/ginkgo/v2 v2.28.1 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/planetscale/vtprotobuf v0.6.1-0.20240319094008-0393e58bdf10 // indirect
	github.com/spf13/pflag v1.0.9 // indirect
	github.com/tonistiigi/go-csvvalue v0.0.0-20240814133006-030d3b2625d0 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/net v0.50.0 // indirect
	golang.org/x/sys v0.41.0 // indirect
	golang.org/x/text v0.34.0 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
	gopkg.in/check.v1 v1.0.0-20180628173108-788fd7840127 // indirect
)

replace github.com/keilerkonzept/dockerfile-json => github.com/konflux-ci/dockerfile-json v0.0.0-20260211115307-8b6cecfd575e
