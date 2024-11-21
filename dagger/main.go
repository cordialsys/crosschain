package main

import (
	"context"

	"dagger/dagger/internal/dagger"
)

type Dagger struct{}

func (m *Dagger) TestChain(
	ctx context.Context,
	chain string,
	image string,
	source *dagger.Directory) (string, error) {
	nodeService := dag.Container().
		From(image).
		WithExposedPort(10000).
		WithExposedPort(10001).
		AsService()

	return dag.Container().
		From("us-docker.pkg.dev/cordialsys/crosschain/rockylinux").
		WithDirectory("/crosschain", source).
		WithServiceBinding("node-service", nodeService).
		WithWorkdir("/crosschain/ci/").
		WithExec([]string{"go", "test", "-tags", "ci", "-run", "TestBalance", "--chain", chain, "--rpc", "http://node-service:10000"}).
		WithExec([]string{"go", "test", "-tags", "ci", "-run", "TestTransfer", "--chain", chain, "--rpc", "http://node-service:10000"}).
		Stdout(ctx)
}
