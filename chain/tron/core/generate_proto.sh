#!/bin/bash
set -euo pipefail

parent_path=$(
	cd "$(dirname "${BASH_SOURCE[0]}")"
	pwd -P
)
repodir="protofiles"
repopath="$parent_path/$repodir"

if [ ! -d "$repopath" ]; then
	git clone --depth=1 https://github.com/tronprotocol/protocol "$repopath"
fi

cd "$repopath"
# repository contains some empty proto files, which can cause generation to fail
# make sure to filter out empty files with find
find core -name "*.proto" -size +0 -print0 |
	xargs -0 protoc \
		--go_out="$parent_path" \
		--go_opt=module=github.com/tronprotocol/grpc-gateway/core

if [ -d "$repopath" ]; then
	rm -rf "$repopath"
fi
