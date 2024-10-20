#!/usr/bin/env bash

set -e

commit_hash() {
    git rev-parse HEAD
}

now() {
    date +%Y-%m-%dT%H:%M:%S%z
}

main() {
    LDFLAGS="-X praktiskt/go-index-dl/cmd.GIT_COMMIT_SHA=$(commit_hash)"
    LDFLAGS="$LDFLAGS -X praktiskt/go-index-dl/cmd.BUILD_TIME=$(now)"
    set -x
    CGO_ENABLED=0 go build -ldflags="$LDFLAGS" -o go-index-dl
}

main $@
