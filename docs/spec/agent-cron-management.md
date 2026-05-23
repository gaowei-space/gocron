# Agent Cron 管理方案

## 状态

Final

## 日期

2026-05-22

## 背景

gocron 在组织内承载多个产品的后台 cron 任务。随着任务数量增加，agent 需要能够在排障、发布、运营和日常维护流程中自助管理任务，包括查询、创建、修改、启停、手动运行、停止运行实例和查看日志。删除 cron 任务属于高风险操作，第一版默认不通过 CLI 或 agent API 开放。

本方案不使用 IP 白名单，采用浏览器登录授权和设备授权模型，并定义 agent API middleware 边界、完整 `gocron-cli` 能力、凭据存储、审计表和第一版边界。

## 方案结论

新增 agent-facing REST API 和官方 `gocron-cli`。CLI 不直接调用现有 Vue 后台 API，而是调用独立 agent API；服务端内部复用现有任务校验、保存、调度和权限逻辑。

第一版仅允许超级管理员授权和使用 `gocron-cli`。管理员和普通用户仍通过现有 Vue 后台使用系统，不持有 agent API 访问凭据。

核心原则：

- 完整运维：CLI 覆盖任务查询、详情、创建、修改、启停、手动运行、停止运行实例、日志查询和主机查询；默认不支持删除 cron 任务。
- 超管专用：第一版 agent API 按当前后台超级管理员能力执行，不新增管理员/普通用户的 agent API 读写过滤逻辑。
- 浏览器授权：`gocron login` 使用 device-code 浏览器授权流程，不要求用户复制长期 token。
- 设备可撤销：每次 CLI 授权绑定一个设备授权记录，后台可查看和撤销。
- 独立 API contract：agent API 保持稳定，不被 Vue 页面请求结构牵动。
- 复用业务规则：任务校验、保存和调度状态变更从现有 router 抽到 service，Vue API 和 agent API 共用。

## 第一版不做

- 不支持管理员或普通用户授权使用 `gocron-cli`。
- 不新增 `product_id` 或产品维度隔离字段。
- 不新增任务归属、主机归属或历史任务归属迁移逻辑。
- 不新增审批流；审计日志不阻断任务入库和调度生效。
- 不建设 MCP、工具网关或多 agent 编排层。
- 不改变现有 `gocron` 和 `gocron-node` 的部署形态；新增 `gocron-cli` 作为独立发布物。
- 不让 CLI 直接复用 Vue 后台 API。

## 权限模型

第一版权限模型收敛为超级管理员专用：

- 超级管理员：可以授权 `gocron-cli`，并通过 agent API 管理全部任务、任务日志和主机信息。
- 管理员：不能授权 `gocron-cli`，不能使用 agent API。
- 普通用户：不能授权 `gocron-cli`，不能使用 agent API。

所有 agent API 请求必须满足：

- access token 有效且未过期。
- access token 绑定的设备授权记录存在、未过期、未撤销。
- 设备授权绑定的用户存在、状态正常、仍是超级管理员。
- 请求体通过任务字段校验和调度规则校验。

读操作同样要求有效 access token 和有效设备授权。由于第一版仅超级管理员可用，任务列表、任务详情、任务日志和主机查询均按当前后台超级管理员能力返回。

历史任务 `creater = 0` 按当前后端兼容逻辑处理，不新增历史任务归属修复或迁移流程。

## 路由与 Middleware

建议使用 `/api/agent/v1` 作为 agent API 前缀。该前缀必须使用独立 authentication middleware，不能依赖现有后台 cookie session。

实现时需要处理现有全局登录校验和 URL 权限校验：

- agent API 路由应从现有 cookie auth 和 `urlAuth` 中排除，避免被后台 session 逻辑提前拦截。
- agent API 进入独立 middleware 后，只接受 `Authorization: Bearer <access_token>`。
- middleware 校验 access token、设备授权状态、用户状态、超级管理员身份。
- 校验通过后，将请求上下文中的当前用户设置为设备授权绑定用户，并设置 `source=agent_api`、`device_id`、`client_type`、`client_version`。
- middleware 生成或透传 `request_id`，写入响应 header 和审计日志。

## 浏览器授权协议

第一版采用 device-code 轮询模式，避免依赖本地 callback 端口、浏览器回调防火墙和本机 CSRF 处理。

### 授权流程

1. 用户执行 `gocron login --server <base_url>`。
2. CLI 调用 `POST /api/agent/v1/auth/device/start` 创建一次性授权请求。
3. 服务端返回 `device_code`、`user_code`、`verification_uri`、`verification_uri_complete`、`expires_in`、`interval`。
4. CLI 打开 `verification_uri_complete`，并开始按 `interval` 轮询 token 接口。
5. 用户在浏览器中使用现有后台登录态登录。如果当前用户不是超级管理员，确认授权失败。
6. 超级管理员确认授权设备名、客户端类型、客户端版本和授权有效期。
7. CLI 轮询成功后获得 access token 和 refresh token。
8. CLI 将 refresh token 存入本地凭据存储；后续 access token 过期时使用 refresh token 刷新。

### 授权 API

| 能力 | Method | Path | 说明 |
| --- | --- | --- | --- |
| 创建授权请求 | `POST` | `/auth/device/start` | CLI 创建一次性授权请求 |
| 查询授权状态/换 token | `POST` | `/auth/device/token` | CLI 轮询授权状态，成功后返回 token |
| 刷新 token | `POST` | `/auth/token/refresh` | 使用 refresh token 换取新 token，并轮换 refresh token |
| 注销当前设备 | `POST` | `/auth/logout` | 撤销当前设备授权 |
| 查询授权设备 | `GET` | `/auth/devices` | 超级管理员查看设备授权列表 |
| 撤销授权设备 | `DELETE` | `/auth/devices/{device_id}` | 超级管理员撤销指定设备授权 |

### Token 规则

- access token 短有效期，例如 15 到 60 分钟。
- refresh token 以 hash 形式存储，服务端不保存明文。
- refresh token 绑定用户、设备授权记录、客户端类型和客户端版本。
- refresh token 每次使用后轮换；旧 refresh token 被重复使用时，撤销该设备授权并记录审计。
- 设备信息可以包含设备名、系统、架构、客户端版本和创建来源，但设备信息只用于展示和审计，不作为单独强安全边界。
- 撤销设备授权后，agent API 每次请求都检查设备授权状态，因此已撤销设备的 access token 也应立即被拒绝。

## 任务 API 范围

以下路径均位于 `/api/agent/v1` 下。

| 能力 | Method | Path | 说明 |
| --- | --- | --- | --- |
| 任务查询 | `GET` | `/tasks` | 支持分页、关键字、状态、主机等当前后台已有过滤 |
| 任务详情 | `GET` | `/tasks/{id}` | 返回任务配置和调度状态 |
| 创建任务 | `POST` | `/tasks` | 复用后台创建任务的校验和保存逻辑 |
| 修改任务 | `PUT` | `/tasks/{id}` | 复用后台编辑任务的校验和保存逻辑 |
| 启用任务 | `POST` | `/tasks/{id}/enable` | 启用调度 |
| 停用任务 | `POST` | `/tasks/{id}/disable` | 停用调度 |
| 手动运行 | `POST` | `/tasks/{id}/run` | 触发一次运行，记录操作来源 |
| 日志查询 | `GET` | `/tasks/{id}/logs` | 支持分页、状态等当前后台已有过滤 |
| 停止任务 | `POST` | `/tasks/{task_id}/runs/{log_id}/stop` | 停止指定运行实例 |
| 主机查询 | `GET` | `/hosts` | 返回当前系统主机列表，权限与当前后台超级管理员主机查询能力保持一致 |

响应结构建议保持与现有 API 的错误码和分页格式一致，避免 agent 侧出现两套解析逻辑。

### 删除任务语义

`gocron-cli` 第一版默认不支持删除 cron 任务，agent API 也不开放默认删除接口。删除属于高风险操作，仍通过现有 Vue 后台完成。

原因：

- 当前后台删除逻辑会删除任务、清理任务主机关联并从调度器移除，但不保证已经运行中的 RPC/SHELL 实例立即停止。
- CLI 面向 agent 和脚本，误删风险高于人工后台操作。
- 创建、修改、停用和停止运行实例已经能覆盖大多数运维闭环。

未来如确需支持删除，应作为显式高风险能力单独设计，例如要求服务端配置开关、CLI `--force`、二次确认、运行中任务阻断和更严格审计。

### 停止任务语义

第一版仅支持当前已有能力：停止 RPC/SHELL 任务。HTTP 任务不支持停止。

实现要求：

- `log_id` 必须存在。
- `log_id` 必须属于对应 `task_id`。
- 日志状态应为运行中，否则返回明确错误。
- 多主机任务按当前逻辑向任务关联主机发送停止请求。
- 停止操作实时生效，结果以任务日志最终状态为准。

## gocron-cli 范围

`gocron-cli` 是官方 agent/CLI 客户端。CLI 调用 agent-facing REST API，不调用 Vue 后台 API。

第一版 CLI 命令：

| 命令 | 说明 |
| --- | --- |
| `gocron login` | 打开浏览器完成登录授权 |
| `gocron logout` | 撤销当前设备授权并清理本地凭据 |
| `gocron task list` | 查询任务列表 |
| `gocron task get <id>` | 查看任务详情 |
| `gocron task create --file task.yaml` | 创建任务 |
| `gocron task update <id> --file task.yaml` | 修改任务 |
| `gocron task enable <id>` | 启用任务 |
| `gocron task disable <id>` | 停用任务 |
| `gocron task run <id>` | 手动运行任务 |
| `gocron task stop <task_id> <log_id>` | 停止运行中的任务实例 |
| `gocron task logs <id>` | 查询任务日志 |
| `gocron host list` | 查询可用主机 |

CLI 输出要求：

- 默认输出人类可读表格或文本。
- 支持 `--json` 输出稳定 JSON，便于 agent 和脚本解析。
- 创建和修改任务优先支持 `--file` 传入 YAML 或 JSON，避免复杂任务配置全部堆在命令行参数中。
- 错误输出应包含可读 message、机器可读 code 和 request id。

### 本地凭据存储

CLI 必须避免将 refresh token 明文暴露在 shell history、日志或命令输出中。

存储策略：

- 优先使用系统 keychain/keyring。
- keychain 不可用时，使用本地配置文件，文件权限必须为 `0600`。
- 支持多 server/profile，例如 `default`、`prod`、`staging`。
- 本地只保存 server URL、device_id、refresh token 和必要展示信息。
- `gocron logout` 必须同时清理本地凭据并调用服务端撤销设备授权。

## 数据模型

### CLI 设备授权表

建议新增独立表，例如 `agent_device_authorization`。

| 字段 | 说明 |
| --- | --- |
| `id` | 主键 |
| `user_id` | 授权用户 |
| `device_id` | 服务端生成的设备标识 |
| `device_name` | 用户或 CLI 上报的设备名 |
| `client_type` | 客户端类型，例如 `gocron-cli`、`codex-cli` |
| `client_version` | 客户端版本 |
| `refresh_token_hash` | 当前有效 refresh token hash |
| `expires_at` | 授权过期时间 |
| `revoked_at` | 撤销时间 |
| `last_used_at` | 最近使用时间 |
| `last_used_ip` | 最近使用 IP |
| `created_at` | 创建时间 |
| `updated_at` | 更新时间 |

### 授权请求表

建议新增短生命周期表，例如 `agent_device_authorization_request`。

| 字段 | 说明 |
| --- | --- |
| `id` | 主键 |
| `device_code_hash` | CLI 轮询使用的 device code hash |
| `user_code_hash` | 浏览器确认使用的 user code hash |
| `device_name` | CLI 上报设备名 |
| `client_type` | 客户端类型 |
| `client_version` | 客户端版本 |
| `status` | pending、approved、denied、expired |
| `approved_by` | 确认授权的超级管理员 |
| `expires_at` | 授权请求过期时间 |
| `created_at` | 创建时间 |
| `updated_at` | 更新时间 |

### 审计表

建议新增独立审计表，例如 `agent_audit_log`，不改变现有 `task_log` 的任务执行日志语义。

| 字段 | 说明 |
| --- | --- |
| `id` | 主键 |
| `request_id` | 请求 ID 或 trace ID |
| `user_id` | 操作用户 |
| `device_id` | CLI 设备授权标识 |
| `client_type` | 客户端类型 |
| `client_version` | 客户端版本 |
| `source_ip` | 来源 IP |
| `source` | 来源，例如 `agent_api` |
| `action` | 操作类型 |
| `target_type` | 目标类型，例如 `task`、`task_run`、`device_authorization` |
| `target_id` | 目标 ID |
| `request_summary` | 请求摘要，需脱敏和截断 |
| `success` | 是否成功 |
| `error_message` | 失败原因 |
| `created_at` | 创建时间 |

审计不是审批。agent API 操作实时生效，审计日志用于事后追踪和排查。

## 实现建议

当前任务 router 中包含较多保存、校验和调度状态变更逻辑。实现 agent API 前，建议先抽出 service 层，避免 Vue API 和 agent API 出现两套规则。

建议拆分：

- 任务校验 service：cron 表达式、命令、HTTP URL、超时、重试、主机、依赖字段、通知字段。
- 任务保存 service：创建和更新任务配置，维护 task host 关系。
- 调度控制 service：启用、停用、手动运行、停止运行。
- 权限 service：第一版校验是否为有效超级管理员设备授权。
- CLI 授权 service：device-code 请求、确认授权、token 签发、refresh token 轮换、撤销设备。
- 审计 service：统一写入 agent 审计日志。

实现顺序建议：

1. 新增数据表和 migration。
2. 抽出任务 service，并保证现有 Vue API 行为不变。
3. 新增 agent API middleware。
4. 实现 device-code 授权 API。
5. 实现任务/日志/主机 agent API。
6. 实现 `gocron-cli` 登录、凭据存储和任务命令。
7. 补齐后台设备授权管理入口。

## 开发完成与部署生效流程

gocron 的调度器和后台 UI 由 `gocron web` 进程提供，默认监听 `5920`。RPC/SHELL 任务由各业务机器上的 `gocron-node` 执行，默认监听 `5921`。本方案主要变更调度器 Web 进程和新增 `gocron-cli`；除非修改 RPC 协议、节点认证或任务执行协议，否则不需要升级 `gocron-node`。

### 本地验证

开发完成后先在本地完成以下验证：

1. 运行 Go 测试：`make test`。
2. 如果改动 Vue 后台设备授权管理入口，运行前端 lint：`cd web/vue && yarn run lint`。
3. 如果改动 Vue 页面，构建并重新嵌入静态资源：`make build-vue && make statik`。
4. 构建二进制：`make build`。
5. 本地启动调度器和节点：`make run`，或分别执行 `./bin/gocron web -e dev`、`./bin/gocron-node`。
6. 验证 `gocron-cli login`、token 刷新、任务创建/修改/启停/运行/停止/日志查询和 `--json` 输出。

`cmd/gocron/gocron.go` 通过 `go:generate statik -src=../../web/public -dest=../../internal -f` 嵌入 `web/public`。因此只改 Go 后端接口时不需要重新构建前端静态资源；只要改动 Vue 或 `web/public`，就必须先执行 `make build-vue`，再执行 `make statik`，最后重新构建 `gocron`。

### 数据库变更

新增表和字段必须同时覆盖首次安装和存量升级：

- 首次安装：把新模型加入 `internal/models/migration.go` 的 `Install` 表列表。
- 存量升级：新增对应 upgrade 函数，把版本号加入 `versionIds` 和 `upgradeFuncs`。
- 发布包含数据库变更的版本时，需要提升 `cmd/gocron/gocron.go` 中的 `AppVersion`。

调度器启动时会执行 `app.InitEnv`，读取工作目录下的 `conf/install.lock` 判断是否已安装。已安装环境会读取 `conf/app.ini`、连接数据库、执行 `upgradeIfNeed`，然后初始化调度器。`upgradeIfNeed` 只在 `conf/.version` 存在且小于当前 `AppVersion` 时执行 migration；migration 成功后会更新 `conf/.version`。

部署前必须备份数据库和生产环境的 `conf/` 目录，尤其是 `conf/app.ini`、`conf/install.lock`、`conf/.version`。不要用构建产物覆盖生产配置目录。

### 打包

常规打包命令：

```bash
make package
```

跨平台打包命令：

```bash
make package-all
```

`make package` 会执行 `make build-vue`、`make statik`，再运行 `package.sh` 编译 `gocron` 和 `gocron-node`。当前 `package.sh` 只打包二进制，前端资源已经通过 statik 嵌入 `gocron` 二进制；生产配置和日志目录由运行环境保留。

新增 `gocron-cli` 后，需要同步扩展 Makefile 和 `package.sh`：

- `make build` 应同时构建 `bin/gocron-cli`。
- `make package` 应生成 `gocron-cli` 对应压缩包或把 CLI 放入独立发布产物。
- `gocron-cli -v` 应输出版本、构建时间和 git commit，便于排查客户端问题。

### 二进制部署

二进制部署推荐流程：

1. 在构建机执行测试和打包。
2. 上传新的 `gocron` 二进制到调度器服务器。
3. 如果本次改动需要发布 CLI，上传对应平台的 `gocron-cli` 给使用方或发布到内部下载地址。
4. 保留生产服务器现有 `conf/` 和 `log/` 目录。
5. 在维护窗口内停止旧 `gocron web` 进程。
6. 替换 `gocron` 二进制。
7. 启动新进程，例如 `./gocron web --host 0.0.0.0 -p 5920 -e prod`。
8. 查看 `log/cron.log`，确认配置读取、migration、调度器初始化和 Web 监听正常。
9. 使用后台 UI 和 `gocron-cli` 分别做 smoke test。

重启 `gocron web` 会停止本进程内的调度器并重新加载启用状态的父任务。维护窗口内可能影响调度触发；已经下发到 `gocron-node` 的 RPC/SHELL 任务不一定会随 Web 进程重启而停止，HTTP 任务则由 Web 进程执行，重启会中断正在执行的 HTTP 任务。发布前应避开关键任务运行窗口。

### Docker 部署

上游 Dockerfile 使用多阶段构建：先构建 Vue 静态资源、执行 statik，再编译 `gocron`，最终镜像只包含 `gocron` Web 进程。README 也说明 Docker 镜像不包含 `gocron-node`，节点需要和具体业务一起构建或单独部署。

Docker 部署推荐流程：

1. 构建新镜像，确保镜像中包含最新 statik 资源和 `gocron` 二进制。
2. 保持 `/app/conf/app.ini`、`/app/conf/install.lock`、`/app/conf/.version` 和 `/app/log` 使用持久化卷或外部配置。
3. 使用新镜像滚动或重启调度器容器。
4. 查看容器日志和 `/app/log/cron.log`，确认 migration 和调度器初始化成功。
5. 如果有 `gocron-cli` 发布物，单独发布 CLI；不要假设 Web 镜像内包含 CLI。

### 部署后生效检查

发布完成后至少检查：

- 浏览器后台能正常登录，原有任务列表、任务详情、启停、手动运行和日志查询正常。
- `gocron-cli login` 能完成 device-code 授权。
- 非超级管理员无法授权 CLI。
- `gocron-cli task list --json` 能返回任务列表。
- 创建一条测试任务后能启用、手动运行、查看日志、停用。
- RPC/SHELL 停止运行接口能正确校验 `task_id` 和 `log_id`。
- 后台设备授权列表能看到新授权设备，并能撤销；撤销后 CLI 请求失败并提示重新登录。
- `agent_audit_log` 记录登录授权、token 刷新、任务写操作、运行操作、停止操作和撤销设备。

## 测试计划

服务端测试：

- 鉴权：缺失 token、无效 token、过期 token、设备撤销、用户禁用、用户不再是超级管理员。
- 浏览器授权：一次性授权请求创建、超级管理员确认授权、非超级管理员确认授权失败、授权过期、重复确认失败。
- 设备授权：有效设备可刷新 token，被撤销设备不能刷新 token，refresh token 轮换后旧 token 不能复用。
- Middleware：`/api/agent/v1` 不依赖后台 cookie session，非 agent 路由现有登录校验不变。
- 权限：超级管理员可以授权并管理全部任务，管理员和普通用户不能授权 CLI。
- 任务校验：创建和修改时复用现有字段校验，非法 cron、非法主机和缺失必填字段失败。
- 调度器状态：启用、停用、手动运行、停止任务后状态与调度器一致。
- 停止任务：`log_id` 不存在、`log_id` 不属于 `task_id`、非运行中日志、HTTP 任务停止均返回明确错误。
- 删除任务：第一版 agent API 和 CLI 不提供默认删除能力。
- 审计记录：写操作、运行操作、登录授权、token 刷新和设备撤销成功或失败时都能落审计。
- 现有接口回归：后台 UI 原有任务创建、修改、启停、运行和停止行为不变。

CLI 测试：

- `gocron login` 能完成 device-code 授权并保存凭据。
- `gocron logout` 能撤销服务端设备并删除本地凭据。
- access token 过期后自动 refresh。
- refresh token 轮换失败时提示重新登录。
- `--json` 输出稳定结构。
- `task create/update --file` 能读取 YAML 和 JSON。
- 多 profile 不互相覆盖凭据。

## 后续演进

- 支持管理员按任务归属或产品维度使用 CLI。
- 支持更细粒度 scope，例如只读、运行、写入、主机读取。
- 支持任务模板、审批流和变更 diff。
- 支持显式高风险删除能力，例如服务端开关、CLI `--force`、二次确认和删除前阻断策略。
- 将 agent API 包装成 MCP 工具，但 MCP 只作为协议适配层，仍复用同一套服务和权限逻辑。
