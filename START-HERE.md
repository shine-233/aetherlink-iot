# AetherLink IoT 傻瓜式部署入口

如果你要验证开源源码在自己的机器上是否真的可部署，请在启动后继续执行 [TESTING-AND-DEPLOYMENT.md](TESTING-AND-DEPLOYMENT.md)；那里包含完整的 dev 健康检查、源码测试、API/E2E 和失败分类。

如果你只是想把系统先跑起来，不想研究前端、后端、Broker、数据库这些模块，从这里开始。

> 部署包边界：当前包是“源码 + Docker Compose + 本机镜像构建”的私有化部署包，不是完全离线镜像包。目标机器需要能使用 Docker 拉取基础镜像并构建后端、前端和 broker 镜像；完全离线环境还需要额外准备镜像 tar 包或私有镜像仓库。

## 你需要先准备

- 一台 Windows、Linux 或 macOS 机器。
- 已安装并启动 Docker Desktop / Docker Engine。
- 服务器磁盘建议至少 8GB 可用空间，内存建议至少 2GB。
- 如果设备不在服务器本机，准备一个浏览器和设备都能访问的服务器 IP 或域名。

## 机器配置怎么选

- 配置低、只是先接第一台设备：选 `light`，这是默认档位，目标是让 1C/2GB 起步机器负担更轻。
- 普通小型私有部署：选 `standard`，建议从 2C/4GB 起步。
- 更大的现场部署：选 `production`，建议从 4C/8GB 起步。

这些档位只是 Docker Compose 的 CPU/内存资源预设，不是“能撑多少设备/多少消息”的实测承诺。真正容量要以后看 `verification/performance-*` 里的基准报告。

## Windows 最短流程

最简单：双击根目录的 `start-aetherlink.cmd`。第一次没有 `.env` 时，它会先问这是本机体验还是服务器/私有化部署；如果设备或浏览器在另一台机器上，请选择服务器模式并填写公开访问地址。启动成功后它会显示浏览器地址和设备 MQTT 地址，并自动打开浏览器。

在项目根目录打开 PowerShell：

```powershell
.\start-aetherlink.ps1 -Doctor
.\start-aetherlink.ps1
```

低配置机器可以明确选轻量档：

```powershell
.\start-aetherlink.ps1 -Doctor -PerformanceTier light
.\start-aetherlink.ps1 -PerformanceTier light
```

想启动成功后自动打开浏览器，可以运行：

```powershell
.\start-aetherlink.ps1 -Open
```

如果部署在服务器上，把 `1.2.3.4` 换成你的服务器公网 IP、局域网 IP 或域名：

```powershell
.\start-aetherlink.ps1 -Doctor -Server -PublicUrl "http://1.2.3.4:8080" -MqttAddress "1.2.3.4:1883"
.\start-aetherlink.ps1 -Server -PublicUrl "http://1.2.3.4:8080" -MqttAddress "1.2.3.4:1883"

.\deploy\start-windows.ps1 -Doctor -Server -PublicUrl "http://1.2.3.4:8080" -MqttAddress "1.2.3.4:1883"
.\deploy\start-windows.ps1 -Server -PublicUrl "http://1.2.3.4:8080" -MqttAddress "1.2.3.4:1883"
```

## Linux / macOS 最短流程

在项目根目录打开终端：

```sh
sh ./start-aetherlink.sh --doctor
sh ./start-aetherlink.sh
```

低配置机器可以明确选轻量档：

```sh
sh ./start-aetherlink.sh --doctor --performance-tier light
sh ./start-aetherlink.sh --performance-tier light
```

如果部署在服务器上，把 `1.2.3.4` 换成你的服务器公网 IP、局域网 IP 或域名：

```sh
sh ./start-aetherlink.sh --doctor --server --public-url http://1.2.3.4:8080 --mqtt-address 1.2.3.4:1883
sh ./start-aetherlink.sh --server --public-url http://1.2.3.4:8080 --mqtt-address 1.2.3.4:1883
```

## 跑起来以后

启动后先只打开一个入口：`AETHERLINK_PUBLIC_URL/first-device`。这就是“接入第一台设备”页面；第一台设备没准备好之前，不需要先去设备管理、命令中心、自动化、OTA 或可视化里找功能。

1. 浏览器打开 `.env` 里的 `AETHERLINK_PUBLIC_URL/first-device`，本机默认是 `http://localhost:8080/first-device`。
2. 第一次进入系统后，按页面提示完成超级管理员 / 租户管理员初始化。
3. 回到“接入第一台设备”工作区。
4. 先检查部署状态，再创建设备。
5. 复制页面给出的 MQTT / HTTP 测试命令，让设备或浏览器测试发送一条遥测。
6. 看到在线状态、最新遥测和第一张图表后，下载成功证明；交给维护人员配合 `verification/startup-.../manifest.json` 运行 `deploy/first-device-closeout.*` 生成最终 closeout manifest，才算首台设备闭环完成。

## 第一次建管理员页面打不开时

如果浏览器第一次初始化页面打不开、白屏、或者你只想用命令行先把第一个超级管理员建出来，先确认后端已经启动，再运行：

```powershell
.\deploy\first-admin.ps1
```

```sh
sh ./deploy/first-admin.sh
```

脚本会先检查系统里是否已经有超级管理员；只有确实还没有管理员、下一步也是创建超级管理员时，才会让你输入邮箱和密码并调用初始化接口。创建成功后，重新打开 `.env` 里的 `AETHERLINK_PUBLIC_URL`，先用超级管理员登录并创建租户管理员，再用租户管理员进入首页的“接入第一台设备”工作区。

## 地址怎么填

- `AETHERLINK_PUBLIC_URL`：人用浏览器打开的平台地址，例如 `http://1.2.3.4:8080`。
- `AETHERLINK_MQTT_ACCESS_ADDRESS`：设备连接 MQTT 时填写的地址，例如 `1.2.3.4:1883`。
- `AETHERLINK_PERFORMANCE_TIER`：部署资源档位，支持 `light`、`standard`、`production`。
- 如果手动改 `.env`，让 `GOTP_OTA_DOWNLOAD_ADDRESS` 跟 `AETHERLINK_PUBLIC_URL` 一致，让 `GOTP_MQTT_ACCESS_ADDRESS` 跟 `AETHERLINK_MQTT_ACCESS_ADDRESS` 一致。
- 只有浏览器和设备都在服务器本机时才用 `localhost`。
- 真实服务器部署时，通常不要给设备填 `localhost:1883`，因为设备会把它理解成“设备自己”。
- 使用 `-Server` / `--server` 时，`localhost` 浏览器地址或 MQTT 地址会被预检直接拦住，避免服务器装好了但设备连不上。

## 卡住时先看

- 预检：`.\start-aetherlink.ps1 -Doctor` 或 `sh ./start-aetherlink.sh --doctor`
- 底层部署脚本仍在：`.\deploy\init.ps1` 或 `sh ./deploy/init.sh`
- 第一次管理员命令行兜底：`.\deploy\first-admin.ps1` 或 `sh ./deploy/first-admin.sh`
- 容器状态：`docker compose ps`
- 启动证据：`verification/startup-<时间>/manifest.json`
- 详细部署说明：[deploy/README.md](deploy/README.md)
