all:
	CGO_ENABLED=0 go build -v ./...

test:
	go test -mod=readonly ./...

lint:
	@go vet ./...

fmt:
	@go fmt ./...

install:
	CGO_ENABLED=0 go install -v ./cmd/...
