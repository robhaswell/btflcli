#!/bin/bash
oss=(linux darwin)
archs=(amd64 arm64 386)

for os in ${oss[@]}
do
    for arch in ${archs[@]}
    do
        env GOOS=${os} GOARCH=${arch} go build -o bin/btfl-${os}-${arch} .
    done
done

env GOOS=windows GOARCH=amd64 go build -o bin/btfl.exe