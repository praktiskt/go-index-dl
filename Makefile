.PHONY = build test test-all


build:
	CGO_ENABLED=0 go build -o go-index-dl

test:
	go test -short ./.../


test-all:
	go test ./.../
