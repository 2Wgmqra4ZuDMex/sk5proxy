FROM golang:1.23.6-alpine3.21 AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/sk5proxy ./cmd/sk5proxy

FROM alpine:3.21.3

RUN addgroup -S -g 10001 sk5proxy \
    && adduser -S -D -H -u 10001 -G sk5proxy sk5proxy \
    && mkdir /data \
    && chown sk5proxy:sk5proxy /data
COPY --from=build --chown=sk5proxy:sk5proxy /out/sk5proxy /usr/local/bin/sk5proxy

USER 10001:10001
WORKDIR /data
VOLUME ["/data"]
EXPOSE 1080 8080 8081 10080-10180/tcp
ENTRYPOINT ["/usr/local/bin/sk5proxy"]
