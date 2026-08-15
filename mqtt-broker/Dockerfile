# syntax=docker/dockerfile:1
FROM golang:1.26.4-alpine AS builder
WORKDIR $GOPATH/src/app
COPY . ./
ENV GO111MODULE on
ENV GOPROXY="https://goproxy.cn,direct"
WORKDIR $GOPATH/src/app/cmd/gmqttd
RUN go build

FROM alpine:3.20
WORKDIR /gmqttd
RUN apk add --no-cache tzdata ca-certificates \
    && addgroup -S -g 10001 aetherlink \
    && adduser -S -D -H -u 10001 -G aetherlink aetherlink \
    && install -d -o aetherlink -g aetherlink data
# Keep the runtime image limited to the broker binary and its default config.
# Copying the whole source directory would also ship Go sources and test files.
COPY --from=builder --chown=aetherlink:aetherlink /go/src/app/cmd/gmqttd/gmqttd ./gmqttd
COPY --from=builder --chown=aetherlink:aetherlink /go/src/app/cmd/gmqttd/default_config.yml ./default_config.yml
RUN chmod 0555 gmqttd
USER 10001:10001
EXPOSE 1883 8883 8082 8083 8084
ENTRYPOINT ["./gmqttd", "start", "-c", "/gmqttd/default_config.yml"]
