# Kratos 项目模板

[English](README.md) | 简体中文

这是一个用于创建 Kratos 服务的项目模板，内置 HTTP 和 gRPC 传输、protobuf
优先的 API、Wire 依赖注入、OpenAPI 生成，以及一个小型 CRUD 示例。

可以将本仓库作为新服务的起点。内置示例资源仅用于展示 API 形态、分层、
代码生成和测试方式。创建真实项目时，请替换为自己的领域模型。

## 相关仓库

- 前端: https://github.com/wuyuetianjian/front_vite_ant_temperate.git

## 创建新项目

1. 从该模板复制或生成一个新仓库。
2. 更新 Go 模块路径：

```bash
go mod edit -module github.com/your-org/your-service
```

3. 替换所有引用模板模块的 import 路径。
4. 重命名命令、服务元数据和示例 API 包，使其匹配你的服务。
5. 将示例 CRUD 资源替换为你的资源。
6. 重新生成代码并验证项目：

```bash
make all
go test ./...
```

## 内置内容

- Kratos HTTP 和 gRPC 服务端配置。
- Protobuf API 定义和生成的 Go 代码。
- OpenAPI 生成。
- 基于 Wire 的依赖注入。
- 分层的 `service`、`biz` 和 `data` 包。
- 示例资源的轻量级内存仓库。
- service 层单元测试。
- 服务端流式和双向流式示例。

## 项目结构

```text
api/                  Protobuf API 和生成绑定
cmd/                  应用入口
configs/              本地配置
internal/server/      HTTP 和 gRPC 服务端构造
internal/service/     面向传输层的服务方法
internal/biz/         Usecase、实体、错误、仓库接口
internal/data/        仓库实现
third_party/          Protobuf 依赖
openapi.yaml          生成的 OpenAPI 文档
```

## API 模板实践

示例 CRUD API 展示了 Kratos 项目的常见约定：

- 面向资源的方法：create、get、list、update、delete。
- 使用 `google.api.http` 标注 HTTP 路由。
- 使用 `google.api.field_behavior` 标注必填字段。
- List 请求包含 `page_size`、`page_token`、`filter` 和 `order_by`。
- 使用 `go.einride.tech/aip/pagination` 处理分页。
- 使用 `google.protobuf.FieldMask` 和 `fieldmask.Update` 处理部分更新。
- 定义单向和双向流式 RPC。

内存数据层刻意保持简单。它展示了数据在各层之间的流转，但没有实现完整的
查询引擎。真实仓库可以在 SQL、Ent 或其他存储层中应用解析后的过滤和排序。

## 开发命令

安装生成器：

```bash
make init
```

重新生成 API 绑定和 OpenAPI：

```bash
make api
```

重新生成配置 protobuf：

```bash
make config
```

运行全部生成步骤、Wire 和模块清理：

```bash
make all
```

构建：

```bash
make build
```

测试：

```bash
go test ./...
```

## 本地运行

```bash
go run ./cmd/temperate -conf ./configs
```

Polaris 配置或注册中心支持通过 `polaris` build tag 编译，因为 Polaris 和
etcd 目前会注册相同 `auth.proto` 文件名的不同 protobuf 描述符。仅在部署
需要 Polaris 时使用该命令：

```bash
go run -tags polaris ./cmd/temperate -conf ./configs
```

默认本地端口在 `configs/config.yaml` 中配置：

- HTTP: `0.0.0.0:8000`
- gRPC: `0.0.0.0:9000`

## 运行时功能

运行时中间件由 `configs/config.yaml` 中的 `data.api` 控制。

应用配置分两阶段加载。通过 `-conf` 传入的本地文件或目录始终会作为引导
配置先加载。随后顶层 `config` 配置决定是否合并环境变量和远程配置源：

```yaml
config:
  env:
    enabled: true
    prefix: KRATOS_
  remote:
    enabled: false
    driver: etcd
    endpoints:
      - 127.0.0.1:2379
    path: /temperate/config.yaml
  watch:
    enabled: false
    keys:
      - server
      - data
      - registry
```

配置组件支持本地文件、环境变量，以及远程驱动 `apollo`、`consul`、`etcd`、
`kubernetes`、`nacos` 和 `polaris`。Kratos v3 没有发布官方
`config/etcd/v3` 包，因此该模板在 `internal/configsource` 中提供了一个
小型 etcd 配置源适配器。

环境变量：

```yaml
config:
  env:
    enabled: true
    prefix: KRATOS_
```

例如，`KRATOS_SERVER_HTTP_ADDR=127.0.0.1:8000` 会在去掉 `KRATOS_` 前缀后读取。
Kratos 配置占位符，例如 `${PORT:8000}`，也会在配置加载时解析。

Apollo：

```yaml
config:
  remote:
    enabled: true
    driver: apollo
    endpoints:
      - http://127.0.0.1:8080
    app_id: temperate
    cluster: default
    namespace: application.yaml
    secret: ""
```

Consul：

```yaml
config:
  remote:
    enabled: true
    driver: consul
    endpoints:
      - 127.0.0.1:8500
    path: app/temperate/configs/
    token: ""
```

Etcd：

```yaml
config:
  remote:
    enabled: true
    driver: etcd
    endpoints:
      - 127.0.0.1:2379
    path: /temperate/config.yaml
    username: ""
    password: ""
    timeout_seconds: 10
```

Kubernetes：

```yaml
config:
  remote:
    enabled: true
    driver: kubernetes
    namespace: default
    kubeconfig: ""
    master: ""
    label_selector: app=temperate
    field_selector: ""
```

Nacos：

```yaml
config:
  remote:
    enabled: true
    driver: nacos
    endpoints:
      - 127.0.0.1:8848
    namespace: public
    group: DEFAULT_GROUP
    data_id: config.yaml
    username: nacos
    password: nacos
    context_path: /nacos
```

Polaris：

```yaml
config:
  remote:
    enabled: true
    driver: polaris
    endpoints:
      - 127.0.0.1:8091
    namespace: default
    file_group: temperate
    file_name: config.yaml
```

Polaris 配置支持需要使用 `-tags polaris` 构建。

当 `config.watch.enabled` 为 true 时，会监听配置的 key 并记录变更日志。
模板不会在配置变更时重建已创建的 server、registrar 或 exporter；可以将
watch 回调用作运行时可调业务行为的集成点。

Ent 数据库访问从 `data.database` 初始化。Ent schema 和生成代码位于
`internal/data/ent`，长期存活的 Ent client 由 `internal/data.Data` 持有。

```yaml
data:
  database:
    driver: mysql
    source: root:root@tcp(127.0.0.1:3306)/test?parseTime=True&loc=Local
    read_sources:
      - root:root@tcp(127.0.0.1:3306)/test?parseTime=True&loc=Local
    max_idle_conns: 10
    max_open_conns: 100
    conn_max_lifetime: 1h
    debug: false
    auto_migrate: false
```

支持的数据库驱动包括 `mysql`、`postgres`、`postgresql` 和 `pgsql`。写仓库
应使用 `Data.WriteEnt`，读仓库应使用 `Data.ReadEnt`。如果 `read_sources`
为空，读 client 会复用写 client，因此单数据库部署无需额外配置。如果提供
多个读 DSN，默认 client 会使用第一个；如果服务需要按请求进行读负载均衡，
可以添加仓库级或 data 级选择器。

PostgreSQL 示例：

```yaml
data:
  database:
    driver: postgres
    source: postgres://postgres:postgres@127.0.0.1:5432/temperate?sslmode=disable
    read_sources:
      - postgres://postgres:postgres@127.0.0.1:5433/temperate?sslmode=disable
```

Ent 生成：

```bash
go generate ./internal/data/ent
```

模板包含一个最小的 `Template` schema，仅用于在添加真实资源前保持 Ent 生成
可用。请将其替换为服务自己的 schema，放在 `internal/data/ent/schema` 下。
只有在服务允许创建或变更数据库表的环境中才应设置 `auto_migrate: true`。

JWT 认证可同时为 HTTP 和 gRPC 入口启用：

```yaml
data:
  api:
    auth: true
    signing_method: HS512
    jwt_key: testKey
```

支持的签名方法包括 `HS256`、`HS384` 和 `HS512`。空的 `signing_method`
默认使用 `HS512`。请在模板或本地开发以外替换示例 `jwt_key`，不要提交真实
凭据。通过 `GET /health` 暴露的 `Health` RPC 会排除认证，以便无 token 的
readiness 和 liveness 检查能够运行。认证白名单刻意保持为代码级，而不是
配置级；如需暴露更多免认证操作，请更新 `internal/server/auth.go` 中的
`authAllowlist`。

Prometheus 指标可通过现有 HTTP server 启用和暴露：

```yaml
data:
  api:
    metrics: true
    metrics_path: /metrics
```

启用后，HTTP 和 gRPC 的请求数量和耗时指标会通过 Kratos 的 OpenTelemetry
metrics 中间件采集，并暴露在配置的 HTTP 路径上，例如
`http://localhost:8000/metrics`。

HTTP 和 gRPC 入口均可启用限流：

```yaml
data:
  api:
    ratelimit: true
```

启用后，服务使用 Kratos 默认的 BBR limiter。被 limiter 拒绝的请求会返回
`429 RATELIMIT`。

HTTP 和 gRPC 入口均可全局启用链路追踪：

```yaml
data:
  api:
    tracing: true
    tracing_endpoint: localhost:4318
```

启用后，应用会设置全局 OpenTelemetry tracer provider，传播 W3C trace
context 和 baggage，并将 span 导出到配置的 OTLP HTTP collector endpoint。

服务注册由顶层 `registry` 配置控制：

```yaml
registry:
  enabled: false
  driver: etcd
  endpoints:
    - 127.0.0.1:2379
  namespace: /microservices
```

支持的驱动与 Kratos registry 实现一致：`consul`、`discovery`、`etcd`、
`kubernetes`、`nacos`、`polaris` 和 `zookeeper`。启用后，所选 registrar
会传给 `kratos.New`，应用会随生命周期注册和注销其 HTTP 与 gRPC endpoint。
通用字段包括 `endpoints`、`namespace`、`username`、`password` 和 `token`；
驱动特定字段包括 `group`、`cluster`、`service_token`、`protocol`、
`ttl_seconds`、`kubeconfig`、`context_path`、`region`、`zone`、`env` 和
`host`。

Etcd：

```yaml
registry:
  enabled: true
  driver: etcd
  endpoints:
    - 127.0.0.1:2379
  namespace: /microservices
  username: ""
  password: ""
  ttl_seconds: 15
  timeout_seconds: 10
```

Consul：

```yaml
registry:
  enabled: true
  driver: consul
  endpoints:
    - 127.0.0.1:8500
  token: ""
  datacenter: dc1
  health_check: true
  heartbeat: true
```

Nacos：

```yaml
registry:
  enabled: true
  driver: nacos
  endpoints:
    - 127.0.0.1:8848
  namespace: public
  username: nacos
  password: nacos
  group: DEFAULT_GROUP
  cluster: DEFAULT
  context_path: /nacos
  timeout_seconds: 10
```

Zookeeper：

```yaml
registry:
  enabled: true
  driver: zookeeper
  endpoints:
    - 127.0.0.1:2181
  namespace: /microservices
  username: ""
  password: ""
  timeout_seconds: 10
```

Kubernetes：

```yaml
registry:
  enabled: true
  driver: kubernetes
  namespace: default
  kubeconfig: ""
```

集群内使用时，请保持 `kubeconfig` 为空。Kubernetes registrar 会 patch 当前
Pod 的 labels 和 annotations，因此 service account 需要拥有 patch 配置
命名空间内 pods 的权限。

Polaris：

```yaml
registry:
  enabled: true
  driver: polaris
  endpoints:
    - 127.0.0.1:8091
  namespace: default
  service_token: ""
  protocol: grpc
  weight: 100
  ttl_seconds: 5
  timeout_seconds: 10
  retry_count: 1
  heartbeat: true
```

Polaris registry 支持需要使用 `-tags polaris` 构建。

Discovery：

```yaml
registry:
  enabled: true
  driver: discovery
  endpoints:
    - 127.0.0.1:7171
  region: sh
  zone: sh001
  env: prod
  host: ""
```

如果 `host` 为空，discovery 驱动会回退使用本机 hostname。

## 服务账号

服务账号允许机器、CI 流水线和其他后端服务使用长期有效的 Token 直接调用
API，无需经过交互式登录流程。

### Token 格式

Token 格式为 `svc_<base64url(expiry_8bytes + random_24bytes)>`。解码后的
前 8 字节以大端 int64 格式保存过期时间（Unix 时间戳），0 表示永不过期。
这样服务端可以在不查询数据库的情况下快速拒绝已过期的 Token。

Token 明文不会写入数据库，仅保存其 SHA-256 哈希值（存储在
`auth_service_accounts` 表中）。

### 认证方式

服务账号 Token 通过与用户 JWT 相同的 `Authorization: Bearer <token>` 请求头
发送。Auth 中间件检测到 `svc_` 前缀后，会将请求路由到服务账号校验逻辑，
而不是 JWT 校验逻辑。

服务账号的有效权限由其绑定角色推导得出，使用与用户账号相同的角色继承逻辑。

### 管理 API

| 方法 | 路径 | 说明 |
|------|------|------|
| `POST` | `/v1/service-accounts` | 创建服务账号并返回 Token（仅显示一次） |
| `GET` | `/v1/service-accounts` | 列出所有服务账号 |
| `GET` | `/v1/service-accounts/{id}` | 查询单个服务账号 |
| `PATCH` | `/v1/service-accounts/{id}` | 更新描述、禁用状态或角色 |
| `DELETE` | `/v1/service-accounts/{id}` | 删除服务账号 |
| `POST` | `/v1/service-accounts/{id}/regenerate-token` | 轮换 Token |

完整 Token 值**仅在** `CreateServiceAccount` 和
`RegenerateServiceAccountToken` 的响应中返回。后续查询仅返回 Token 前缀
用于展示。

所有创建、删除和重新生成操作均会以资源类型 `service_account` 记录到审计日志中。

## 审计日志

所有变更类操作（创建、更新、删除、登录、退出、踢出、2FA
绑定/启用/关闭/管理员重置、令牌重置）都会记录为 `AuditLog`，包含操作用户、IP
地址、资源类型和资源名称。审计日志为追加写入；仅清理任务会按配置的保留周期删除旧记录。

新写入的审计详情使用结构化 JSON 载荷。前端审计日志页面会在详情悬浮框中展示字段详情和
before/after 对比。TOTP 密钥、二维码地址和一次性验证码不会写入审计日志。

可通过 `GET /v1/audit-logs` 查询审计日志。可选的 `action` 查询参数支持按操作类型过滤，
例如 `login`、`create`、`update`、`delete`、`kick`、`setup_2fa`、
`enable_2fa`、`disable_2fa`、`admin_reset_2fa`。

## Docker

```bash
docker build -t <your-image-name> .
docker run --rm -p 8000:8000 -p 9000:9000 \
  -v </path/to/your/configs>:/data/conf \
  <your-image-name>
```
