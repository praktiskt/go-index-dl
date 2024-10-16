.PHONY = build test test-all


build:
	bash hack/build.sh

test:
	go test -short ./.../


test-all:
	go test ./.../
