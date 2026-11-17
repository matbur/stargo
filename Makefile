.PHONY: test vet race build run

test:
	go test ./...

vet:
	go vet ./...

race:
	go test -race ./...

build:
	go build ./...

run:
	go run ./examples/hello
