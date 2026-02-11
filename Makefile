.PHONY: build test coverage run clean

build:
	go build -o bin/worktree-ui ./cmd/worktree-ui

test:
	go test ./... -v

coverage:
	go test ./... -coverprofile=coverage.out
	go tool cover -func=coverage.out

run: build
	./bin/worktree-ui --config config.yaml

clean:
	rm -rf bin coverage.out
