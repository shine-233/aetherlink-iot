# syntax=docker/dockerfile:1
FROM golang:1.26.6-alpine AS builder
WORKDIR $GOPATH/src/app
COPY . ./
ENV GO111MODULE=on
ENV GOPROXY="https://goproxy.cn,direct"
WORKDIR $GOPATH/src/app/cmd/modbus-plugin
RUN go build

FROM alpine:3.24
WORKDIR /modbus-plugin
RUN apk add --no-cache tzdata ca-certificates \
    && addgroup -S -g 10001 aetherlink \
    && adduser -S -D -H -u 10001 -G aetherlink aetherlink \
    && install -d -o aetherlink -g aetherlink config
# 运行镜像只包含插件二进制；点表配置由部署侧以只读卷挂载（含设备凭证，禁止打进镜像）。
COPY --from=builder --chown=aetherlink:aetherlink /go/src/app/cmd/modbus-plugin/modbus-plugin ./modbus-plugin
COPY --from=builder --chown=aetherlink:aetherlink /go/src/app/config.example.json ./config/config.example.json
RUN chmod 0555 modbus-plugin
USER aetherlink:aetherlink
EXPOSE 8090
ENV MODBUS_PLUGIN_CONFIG=/modbus-plugin/config/config.json
ENTRYPOINT ["./modbus-plugin"]
