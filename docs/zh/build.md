# 构建与测试

仓库有两个 Go 模块：根模块 `playsmith` 与链码模块 `chaincode/playsmith-assets`。
链码模块有独立的 `go.mod`，因为 Fabric 直接把该目录部署为链码。

## Make 目标

| 目标 | 作用 | 依据 |
|---|---|---|
| `make build` | 先跑生成物禁改守卫，再 `go build ./...` | `Makefile:29` |
| `make test` | 先跑守卫，再 `go test ./...` | `Makefile:32` |
| `make vet` | `go vet ./...` | `Makefile:35` |
| `make proto` | 从 `proto/playsmith/v1/gateway.proto` 重新生成网关绑定 | `Makefile:52` |
| `make proto-install` | 安装钉死的插件版本，然后校验工具链 | `Makefile:59` |
| `make proto-check` | 已提交的生成物与一次全新生成不一致即失败 | `Makefile:67` |
| `make proto-doctor` | 检查 `protoc` 存在且版本与钉定值精确匹配 | `Makefile:39` |
| `make clean` | `go clean ./...` | `Makefile:85` |

守卫是关键。`make build` 与 `make test` 都依赖 `proto-check`：它把两个 protobuf
文件重新生成到临时目录，再与 `internal/gatewaypb/gen` 逐字节比较。手工改动生成
文件会在任何代码编译之前就失败，接口变更因此无法藏进生成面里。

工具链要钉住，是因为生成文件在头部嵌入 protoc 版本——换一个 protoc 会盖出不同的
字符串，即使协议根本没变也会报漂移（`Makefile:17-24`）：

| 组件 | 钉定版本 |
|---|---|
| `protoc` | `3.21.12` |
| `protoc-gen-go` | `v1.36.12` |
| `protoc-gen-go-grpc` | `v1.6.2` |

`make proto-check PROTOC_VERSION=9.9.9` 会报版本不匹配，而不是误报
「生成物过期」。

## 本地 Fabric 快速开始

使用 fabric-samples/test-network：

1. 起网，建一个通道并启用 CA：
   `./network.sh up createChannel -c playsmith -ca`
2. 部署链码：
   `./network.sh deployCC -c playsmith -ccn playsmith-assets -ccl go -ccp ../../Playsmith/chaincode/playsmith-assets`
3. 起应用层：`cmd/api`（设 `FABRIC_GATEWAY_ENDPOINT` 等即接真实 Fabric，不设则跑
   `NoopClient`），然后带 `POSTGRES_DSN` 跑 `cmd/indexer`。
4. 导出 `JWT_SECRET`（≥32 字节）开启鉴权；设 `GRPC_PORT` 开启 gRPC 面。

本地变体见仓库内 `deploy/README.md`，全部变量见
[配置页](/zh/configuration)。

## 文档站部署

`docs/` 是 VitePress 站点，发布为 GitHub Pages 项目页，挂在 `/Playsmith/` 下。
向 `docs/**` 推送会触发 `.github/workflows/pages.yml`：用 pnpm 安装、构建
`@playsmith/docs`，再上传 `docs/.vitepress/dist`。
