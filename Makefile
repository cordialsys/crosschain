

test:
	go test -mod=readonly ./...

lint:
	@go vet ./... 

fmt:
	@go fmt ./... 
