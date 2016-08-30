REVISION=$(git rev-parse HEAD)
BRANCH=$(git branch |sort |tail -1 |cut -c 3-)
VERSION="Branch:$(BRANCH)"

.PHONY: default test update build image

default: test

test:
	go test -v -race $(shell glide novendor)

update:
	glide update

build:
	go generate $(shell glide novendor)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o build/vasco -ldflags "-X github.com/AchievementNetwork/go-util/vascoClient.SourceRevision=$(REVISION) -X github.com/AchievementNetwork/go-util/vascoClient.SourceDeployTag=$(VERSION)" .;

image: build
	docker build -t achievementnetwork/vasco .
