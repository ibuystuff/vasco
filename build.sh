#! /bin/bash

OUTPUTFILE=build/vasco

if [ "$REVISION" == "" ]; then
    export REVISION=$(git rev-parse HEAD)
fi

if [ "$VERSION" == "" ]; then
    BRANCH=$(git branch |sort |tail -1 |cut -c 3-)
    export VERSION="Branch:$BRANCH"
fi

mkdir -p build
if go get -d; then
    # stamp the revision and version into the built executable
    if go build -o $OUTPUTFILE -ldflags "-X main.SourceRevision $REVISION -X main.SourceDeployTag $VERSION"; then
        cp start.sh build
    else
        echo "build failed"
        exit 1
    fi
else
    echo "get failed"
    exit 1
fi

