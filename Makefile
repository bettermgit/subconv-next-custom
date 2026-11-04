BINARY := subconv-next

.PHONY: build test test-race vet fmt-check serve

build:
	go build -o $(BINARY) ./cmd/subconv-next

serve: build
	./$(BINARY) serve

test:
	go test ./...

test-race:
	go test -race ./...

vet:
	go vet ./...

fmt-check:
	@test -z "$$(gofmt -l $$(find . -path './.git' -prune -o -path './vendor' -prune -o -name '*.go' -print))"
