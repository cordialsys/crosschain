package main

import (
	"context"
	"runtime"
	"strings"
	"time"

	"dagger/dagger/internal/dagger"
)

type Dagger struct{}

func (m *Dagger) TestChain(
	ctx context.Context,
	chain string,
	image string,
	source *dagger.Directory,
	// +optional
	network string,
	signatureAlgorithm string,
) (string, error) {
	nodeService := dag.Container().
		From(image).
		WithExposedPort(10000).
		WithExposedPort(10001).
		AsService()

	goPaths := "/root/go/bin:/usr/local/go/bin"
	goVersion := "1.23.3"
	arch := runtime.GOARCH
	arch = strings.Replace(arch, "x86_64", "amd64", 1)
	arch = strings.Replace(arch, "aarch64", "arm64", 1)

	cache := dag.CacheVolume("cache")
	modules := dag.CacheVolume("modules")

	return dag.Container().
		From("alpine:latest").
		WithExec([]string{"apk", "update"}).
		WithExec([]string{"apk", "add", "curl"}).
		// Install go
		WithExec([]string{"sh", "-c", "curl -fsSLO https://go.dev/dl/go" + goVersion + ".linux-" + arch + ".tar.gz"}).
		WithExec([]string{"sh", "-c", "cat go*.linux-*.tar.gz  | tar -C /usr/local -xz"}).
		WithEnvVariable("PATH", "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"+":"+goPaths).

		// Set cache dirs
		WithMountedCache("/root/go/pkg", modules).
		WithMountedCache("/root/.cache", cache).

		// Mount and build crosschain
		WithDirectory("/crosschain", source).
		WithWorkdir("/crosschain/").
		WithExec([]string{"go", "build", "-v", "./..."}).

		// Add node service
		WithServiceBinding("node-service", nodeService).
		WithEnvVariable("cache-bust", time.Now().String()).
		// Run tests
		WithExec([]string{"go", "test", "-v", "-tags", "ci", "./ci/...", "-run", "TestBalance", "--chain", chain, "--rpc", "http://node-service:10000", "--network", network, "--signature-algorithm", signatureAlgorithm}).
		WithExec([]string{"go", "test", "-v", "-tags", "ci", "./ci/...", "-run", "TestTransfer", "--chain", chain, "--rpc", "http://node-service:10000", "--network", network, "--signature-algorithm", signatureAlgorithm}).
		Stdout(ctx)
}
