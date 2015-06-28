#! /bin/bash

mkdir -p build
go get -d
go build -o build/vasco
cp start.sh build