all:
	CGO_ENABLED=0 go build -v ./...

test:
	go test -mod=readonly -tags not_ci ./...

lint:
	@go vet ./...

fmt:
	@go fmt ./...

install:
	CGO_ENABLED=0 go install -v ./cmd/...

# you may need to create a builder in docker to enable multi-platform builds
setup-builder:
	docker buildx create --name multi-arch --platform linux/amd64,linux/arm64 --use

build-push-images:
	docker buildx bake --push
build-images-arm64:
	docker buildx bake --set '*.platform=linux/arm64'
build-images-amd:
	docker buildx bake --set '*.platform=linux/amd64'

build-evm-arm64:
	docker buildx bake --set '*.platform=linux/arm64' evm

build-sol-amd:
	docker buildx bake --set '*.platform=linux/amd64' solana
build-sol-arm:
	docker buildx bake --set '*.platform=linux/arm64' solana

build-bitcoin-amd:
	docker buildx bake --set '*.platform=linux/amd64' bitcoin
build-bitcoin-arm:
	docker buildx bake --set '*.platform=linux/arm64' bitcoin

build-push-bitcoin:
	docker buildx bake bitcoin --push

build-proto:
	./chain/tron/core/generate_proto.sh # generate tron protobufs
