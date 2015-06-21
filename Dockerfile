#
# Go container based on google/golang
#
FROM ubuntu:latest

ENV DEBIAN_FRONTEND noninteractive

RUN apt-get update -y && \
    apt-get install --no-install-recommends -qy curl build-essential ca-certificates git mercurial bzr

RUN mkdir /goroot /gopath
RUN curl -Ls https://storage.googleapis.com/golang/go1.4.2.linux-amd64.tar.gz | tar xvzf - -C /goroot --strip-components=1

ENV GOROOT /goroot
ENV GOPATH /gopath
ENV PATH $PATH:$GOROOT/bin:$GOPATH/bin

WORKDIR /gopath/src/github.com/AchievementNetwork/vasco
ADD . /gopath/src/github.com/AchievementNetwork/vasco
# RUN go get go get github.com/kardianos/vendor
# RUN go install vendor
# RUN vendor
RUN go get
RUN go install github.com/AchievementNetwork/vasco

CMD []
ENTRYPOINT ["/gopath/bin/vasco"]
# Document that the service listens on port 8080 and 8081.
EXPOSE 8080 8081

