# Kratos Project Template

English | [简体中文](README.zh-CN.md)

A project template for creating new Kratos services with HTTP and gRPC
transports, protobuf-first APIs, Wire dependency injection, OpenAPI generation,
and a small CRUD example.

Use this repository as a starting point for a new service. The included sample
resource is only reference code for API shape, layering, code generation, and
testing. Replace it with your own domain model when creating a real project.

## Related Repositories

- Frontend: https://github.com/wuyuetianjian/front_vite_ant_temperate.git

## Create a New Project

1. Copy or generate a repository from this template.
2. Update the Go module path:

```bash
go mod edit -module github.com/your-org/your-service
```

3. Replace existing import paths that reference this template module.
4. Rename the command, service metadata, and sample API package to match your
   service.
5. Replace the sample CRUD resource with your own resource.
6. Regenerate code and verify the project:

```bash
make all
go test ./...
```

## What Is Included

- Kratos HTTP and gRPC server setup.
- Protobuf API definitions and generated Go code.
- OpenAPI generation.
- Wire-based dependency injection.
- Layered `service`, `biz`, and `data` packages.
- A lightweight in-memory repository for the sample resource.
- Unit tests for the service layer.
- Server-streaming and bidirectional-streaming examples.

## Project Layout

```text
api/                  Protobuf APIs and generated bindings
cmd/                  Application entrypoints
configs/              Local configuration
internal/server/      HTTP and gRPC server construction
internal/service/     Transport-facing service methods
internal/biz/         Usecases, entities, errors, repository interfaces
internal/data/        Repository implementations
third_party/          Protobuf dependencies
openapi.yaml          Generated OpenAPI document
```

## API Template Practices

The sample CRUD API demonstrates common conventions for Kratos projects:

- Resource-oriented methods: create, get, list, update, delete.
- HTTP annotations with `google.api.http`.
- Required fields with `google.api.field_behavior`.
- List requests with `page_size`, `page_token`, `filter`, and `order_by`.
- Pagination with `go.einride.tech/aip/pagination`.
- Partial updates with `google.protobuf.FieldMask` and `fieldmask.Update`.
- Streaming RPC definitions for one-way and bidirectional streams.

The in-memory data layer intentionally stays simple. It demonstrates flow across
layers, but does not implement a full query engine. Real repositories can apply
parsed filters and ordering in SQL, Ent, or another storage layer.

## Development Commands

Install generators:

```bash
make init
```

Regenerate API bindings and OpenAPI:

```bash
make api
```

Regenerate config protobufs:

```bash
make config
```

Run all generation steps, Wire, and module cleanup:

```bash
make all
```

Build:

```bash
make build
```

Test:

```bash
go test ./...
```

## Run Locally

```bash
go run ./cmd/temperate -conf ./configs
```

Polaris config or registry support is compiled behind the `polaris` build tag
because Polaris and etcd currently register different protobuf descriptors with
the same `auth.proto` filename. Use this command only when the deployment needs
Polaris:

```bash
go run -tags polaris ./cmd/temperate -conf ./configs
```

Default local ports are configured in `configs/config.yaml`:

- HTTP: `0.0.0.0:8000`
- gRPC: `0.0.0.0:9000`

## Runtime Features

Runtime middleware is controlled from `configs/config.yaml` under `data.api`.

Application configuration is loaded in two phases. The local file or directory
passed by `-conf` is always loaded first as bootstrap configuration. The
top-level `config` block then decides whether environment variables and a remote
configuration source are merged in:

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

The config component supports local files, environment variables, and remote
drivers `apollo`, `consul`, `etcd`, `kubernetes`, `nacos`, and `polaris`.
Kratos v3 does not publish an official `config/etcd/v3` package, so this
template includes a small etcd config source adapter in
`internal/configsource`.

Environment variables:

```yaml
config:
  env:
    enabled: true
    prefix: KRATOS_
```

For example, `KRATOS_SERVER_HTTP_ADDR=127.0.0.1:8000` is read without the
`KRATOS_` prefix. Kratos config placeholders such as `${PORT:8000}` are also
resolved during config loading.

Apollo:

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

Consul:

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

Etcd:

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

Kubernetes:

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

Nacos:

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

Polaris:

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

Polaris config support requires building with `-tags polaris`.

When `config.watch.enabled` is true, configured keys are watched and changes
are logged. The template does not rebuild already-created servers, registrars,
or exporters on config changes; use the watch callback as the integration point
for runtime-adjustable business behavior.

Ent database access is initialized from `data.database`. Ent schema and
generated code live under `internal/data/ent`, and long-lived Ent clients are
owned by `internal/data.Data`.

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

Supported database drivers are `mysql`, `postgres`, `postgresql`, and `pgsql`.
Write repositories should use `Data.WriteEnt`; read repositories should use
`Data.ReadEnt`. If `read_sources` is empty, the read client reuses the write
client, so a single database deployment works without additional configuration.
When multiple read DSNs are provided, the first one is used by the default
client; add a repository-level or data-level selector if the service needs
per-request read load balancing.

PostgreSQL example:

```yaml
data:
  database:
    driver: postgres
    source: postgres://postgres:postgres@127.0.0.1:5432/temperate?sslmode=disable
    read_sources:
      - postgres://postgres:postgres@127.0.0.1:5433/temperate?sslmode=disable
```

Ent generation:

```bash
go generate ./internal/data/ent
```

The template includes a minimal `Template` schema only to keep Ent generation
active before a real resource is added. Replace it with service-owned schemas
under `internal/data/ent/schema`. Set `auto_migrate: true` only in environments
where the service is allowed to create or change database tables.

JWT authentication can be enabled for both HTTP and gRPC entrypoints:

```yaml
data:
  api:
    auth: true
    signing_method: HS512
    jwt_key: testKey
```

Supported signing methods are `HS256`, `HS384`, and `HS512`. An empty
`signing_method` defaults to `HS512`. Replace the sample `jwt_key` outside of
template/local development and do not commit real credentials.
The `Health` RPC exposed as `GET /health` is excluded from authentication so
readiness and liveness checks can run without a token. Authentication
allowlisting is intentionally code-level, not configuration-level; update
`authAllowlist` in `internal/server/auth.go` to expose more unauthenticated
operations.

Prometheus metrics can be enabled and exposed through the existing HTTP server:

```yaml
data:
  api:
    metrics: true
    metrics_path: /metrics
```

When enabled, HTTP and gRPC request count and duration metrics are collected
with Kratos' OpenTelemetry metrics middleware and exposed at the configured
HTTP path, for example `http://localhost:8000/metrics`.

Rate limiting can be enabled for both HTTP and gRPC entrypoints:

```yaml
data:
  api:
    ratelimit: true
```

When enabled, the server uses Kratos' default BBR limiter. Requests rejected by
the limiter return `429 RATELIMIT`.

Tracing can be enabled globally for both HTTP and gRPC entrypoints:

```yaml
data:
  api:
    tracing: true
    tracing_endpoint: localhost:4318
```

When enabled, the application sets a global OpenTelemetry tracer provider,
propagates W3C trace context and baggage, and exports spans to the configured
OTLP HTTP collector endpoint.

Service registration is controlled by the top-level `registry` block:

```yaml
registry:
  enabled: false
  driver: etcd
  endpoints:
    - 127.0.0.1:2379
  namespace: /microservices
```

Supported drivers match the Kratos registry implementations: `consul`,
`discovery`, `etcd`, `kubernetes`, `nacos`, `polaris`, and `zookeeper`.
When enabled, the selected registrar is passed to `kratos.New`, so the app
registers and deregisters its HTTP and gRPC endpoints with the application
lifecycle. Common fields are `endpoints`, `namespace`, `username`, `password`,
and `token`; driver-specific fields include `group`, `cluster`,
`service_token`, `protocol`, `ttl_seconds`, `kubeconfig`, `context_path`,
`region`, `zone`, `env`, and `host`.

Etcd:

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

Consul:

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

Nacos:

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

Zookeeper:

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

Kubernetes:

```yaml
registry:
  enabled: true
  driver: kubernetes
  namespace: default
  kubeconfig: ""
```

For in-cluster use, leave `kubeconfig` empty. The Kubernetes registrar patches
the current Pod labels and annotations, so the service account needs permission
to patch pods in the configured namespace.

Polaris:

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

Polaris registry support requires building with `-tags polaris`.

Discovery:

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

If `host` is empty, the discovery driver falls back to the local hostname.

## Session Management

When JWT authentication is enabled, every successful login creates a
`UserSession` record in the database. Each session stores:

- `token_hash` — SHA-256 of the raw JWT string (unique identifier).
- `ip`, `browser`, `os` — parsed from request headers at login time.
- `status` — `active`, `kicked`, or `expired`.
- `login_at`, `last_access_at` — updated on each authenticated request.

The auth middleware verifies the session status on every authenticated request.
A kicked session immediately returns `401 UNAUTHORIZED`. Administrators can
view all sessions at `GET /v1/sessions` and force-logout a session with
`POST /v1/sessions/{id}/kick`.

## Audit Logs

Every mutating operation (create, update, delete, login, kick) is recorded as
an `AuditLog` entry with the acting user's identity, IP address, resource type,
and resource name. Audit logs are append-only; only cleanup jobs remove them
based on the configured retention period.

Query audit logs at `GET /v1/audit-logs`. An optional `action` query parameter
filters by operation type (`login`, `create`, `update`, `delete`, `kick`).

## System Settings

Runtime-adjustable settings are stored as key-value rows in the
`system_settings` table and managed through the API:

```
GET   /v1/settings          # read current settings
PATCH /v1/settings          # update settings
```

| Key                          | Default | Description                              |
|------------------------------|---------|------------------------------------------|
| `audit_log_retention_days`   | 90      | Days to retain audit log entries         |
| `session_log_retention_days` | 30      | Days to retain session records           |

A cron job runs at midnight each day and deletes records older than the
configured retention periods. Settings take effect at the next midnight run
without a restart.

## Docker

```bash
docker build -t <your-image-name> .
docker run --rm -p 8000:8000 -p 9000:9000 \
  -v </path/to/your/configs>:/data/conf \
  <your-image-name>
```
