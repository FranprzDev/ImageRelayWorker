build:
	go build -o bin/image-relay-worker ./cmd/worker
test:
	go test ./...
test-race:
	go test -race ./...
fmt:
	gofmt -w $$(find . -name '*.go' -not -path './.git/*')
vet:
	go vet ./...
run:
	go run ./cmd/worker
