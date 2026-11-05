.PHONY: build test check race install clean

build:
	go build -trimpath -o bin/wraith ./cmd/wraith

test:
	go test ./...

race:
	go test -race -count=1 ./...

check:
	test -z "$$(gofmt -l cmd internal)"
	go vet ./...
	go test -race -count=1 ./...

install: build
	install -Dm755 bin/wraith "$(HOME)/.local/bin/wraith"

clean:
	rm -rf bin dist coverage.out
