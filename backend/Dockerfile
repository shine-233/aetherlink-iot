# syntax=docker/dockerfile:1
FROM golang:1.26.6-alpine AS builder
WORKDIR $GOPATH/src/app
COPY . ./
ENV GO111MODULE=on
ENV GOPROXY="https://goproxy.cn,direct"
RUN go build -o AetherLink-Go .

FROM alpine:3.24
LABEL description="AetherLink IoT Go Backend"
WORKDIR /go/src/app
RUN apk add --no-cache tzdata ca-certificates \
    && addgroup -S -g 10001 aetherlink \
    && adduser -S -D -H -u 10001 -G aetherlink aetherlink \
    && install -d -o aetherlink -g aetherlink files data/telemetry-spool data/uplink-spool
# Keep the runtime image limited to the executable and resources read after startup.
# Frontend assets are embedded in the executable; source, tests, and build tools stay in the builder.
COPY --from=builder --chown=aetherlink:aetherlink /go/src/app/AetherLink-Go ./AetherLink-Go
COPY --from=builder --chown=aetherlink:aetherlink /go/src/app/configs ./configs
COPY --from=builder --chown=aetherlink:aetherlink /go/src/app/sql ./sql
RUN chmod 0555 AetherLink-Go
USER 10001:10001
EXPOSE 9999
ENTRYPOINT [ "./AetherLink-Go" ]
