FROM golang:1.9
ADD . /go/src/github.com/Colstuwjx/kstore
ENV CGO_ENABLED=0
RUN go install github.com/Colstuwjx/kstore

FROM alpine:latest
COPY --from=0 /go/bin/kstore /usr/local/bin/kstore
RUN apk --no-cache add ca-certificates && \
  chmod +x /usr/local/bin/kstore
ENTRYPOINT ["/usr/local/bin/kstore"]
