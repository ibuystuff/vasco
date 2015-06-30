#! /bin/bash

OUTPUTFILE=build/vasco

mkdir -p build
if go get -d; then
    if go build -o $OUTPUTFILE; then
        cp start.sh build
    else
        echo "build failed"
        exit 1
    fi
else
    echo "get failed"
    exit 1
fi

