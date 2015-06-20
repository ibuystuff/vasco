# # Start from a Debian image with the latest version of Go installed
# # and a workspace (GOPATH) configured at /go.
# FROM golang

# # Copy the local package files to the container's workspace.
# ADD . /go/src/github.com/golang/example/outyet

# # Build the outyet command inside the container.
# # (You may fetch or manage dependencies here,
# # either manually or with a tool like "godep".)
# RUN go install github.com/golang/example/outyet

# # Run the outyet command by default when the container starts.
# ENTRYPOINT /go/bin/outyet

# # Document that the service listens on port 8080.
# EXPOSE 8080


# note that it does no good to enable swagger on the docker container -- it
# doesn't work because swagger needs to know where you come from, and
# so the .dockerignore file excludes the swagger directory

FROM aegypius/golang

WORKDIR /gopath/src/app
ADD . /gopath/src/app/
RUN go get

CMD []
ENTRYPOINT ["/gopath/bin/app"]
# Document that the service listens on port 8080 and 8081.
EXPOSE 8080 8081

