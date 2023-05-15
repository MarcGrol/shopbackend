

all: gen fmt test install run

clean:
	go clean ./...

install:
	go install ./...

test:
	go test ./...

cover:
	go test ./... -coverprofile=/tmp/go-cover.$$.tmp && go tool cover -html=/tmp/go-cover.$$.tmp

gen:
	go generate ./...

fmt:
	find . -name '*.go' -exec goimports -l -w {} \;

run:
	go install && shopbackend && open http://localhost:8082/

.PHONY: all clean install test gen fmt run
