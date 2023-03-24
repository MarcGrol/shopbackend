

all: gen fmt test install

deps:
	go get -u golang.org/x/tools/cmd/goimports
	go get -u github.com/MarcGrol/yakshop

clean:
	go clean ./...

install:
	go install ./...

test:
	go test ./...

gen:
	go generate ./...

fmt:
	find . -name '*.go' -exec goimports -l -w {} \;

.PHONY: all deps clean install test gen fmt
