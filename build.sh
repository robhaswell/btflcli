#!/bin/bash
oss=(linux darwin windows)
archs=(amd64 arm64 386)

for os in ${oss[@]}
do
    for arch in ${archs[@]}
    do
        env GOOS=${os} GOARCH=${arch} go build -o bin/btflcli-${os}-${arch} .
    done
done
