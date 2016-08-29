FROM scratch

ADD build/vasco

ENTRYPOINT ["/vasco"]

EXPOSE 8080

