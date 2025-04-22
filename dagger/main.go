package main

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"time"

	"dagger/dagger/internal/dagger"
)

type Dagger struct{}

func isTrue(s string) bool {
	s = strings.ToLower(s)
	s = strings.TrimSpace(s)
	return s == "true" || s == "1" || s == "yes"
}

var defaultTests = []string{
	"TestBalance",
	"TestTransfer",
	"TestFetchBlock",
}

func (m *Dagger) TestChain(
	ctx context.Context,
	chain string,
	image string,
	source *dagger.Directory,
	// +optional
	contract string,
	// +optional
	decimals string,
	// +optional
	network string,
	// +optional
	algorithm string,
	// +optional
	feePayer string,
	// +optional
	tests []string,
) (string, error) {
	nodeService := dag.Container().
		From(image).
		WithExposedPort(10000).
		WithExposedPort(10001).
		AsService()

	goPaths := "/root/go/bin:/usr/local/go/bin"
	goVersion := "1.24.0"
	arch := runtime.GOARCH
	arch = strings.Replace(arch, "x86_64", "amd64", 1)
	arch = strings.Replace(arch, "aarch64", "arm64", 1)

	cache := dag.CacheVolume("cache")
	modules := dag.CacheVolume("modules")
	if contract != "" && decimals == "" {
		panic("decimals is required when contract is provided")
	}
	if len(tests) == 0 {
		tests = defaultTests
	}

	testCommands := [][]string{}
	for _, test := range tests {
		switch test {
		case "TestBalance", "TestTransfer", "TestFetchBlock", "TestMultiTransfer":
			// ok
		default:
			panic("unknown test: " + test)
		}
		command := []string{
			"go", "test", "-v", "-tags", "ci", "./ci/...", "-run", test,
			"--chain", chain, "--contract", contract, "--rpc", "http://node-service:10000", "--network", network, "--algorithm", algorithm,
			"--decimals", decimals,
		}

		if isTrue(feePayer) {
			fmt.Println("using feePayer")
			command = append(command, "--fee-payer")
		}
		testCommands = append(testCommands, command)
	}

	container := dag.Container().
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
		WithEnvVariable("cache-bust", time.Now().String())

	for _, command := range testCommands {
		container = container.WithExec(command)
	}

	return container.Stdout(ctx)
}
