#
# Go container based on google/golang
#
FROM anet-base:latest

WORKDIR /gopath/src/github.com/AchievementNetwork/vasco
ADD . /gopath/src/github.com/AchievementNetwork/vasco
# RUN go get go get github.com/kardianos/vendor
# RUN go install vendor
# RUN vendor
RUN go get
RUN go install github.com/AchievementNetwork/vasco

CMD []
ENTRYPOINT ["/gopath/bin/vasco"]
# Document that the service listens on 2 ports: 8080 and 8081.
EXPOSE 8080 8081

