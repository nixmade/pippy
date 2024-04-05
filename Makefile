all: lint test app

app: pippy

# Run tests
test:
	go test -timeout 30s ./... -coverprofile cover.out
	go tool cover -func=cover.out
# Run go fmt against code
fmt:
	go fmt ./...

# Run go vet against code
vet:
	go vet ./...

lint: fmt vet
	golangci-lint run --enable=testifylint

pippy:
	go build -race -ldflags "-extldflags '-static'" -o bin/pippy cmd/cli/main.go

