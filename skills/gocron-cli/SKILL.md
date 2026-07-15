---
name: gocron-cli
description: "Use when Codex needs to operate a gocron scheduler through the gocron-cli command line client: browser-login authorization, task listing/detail/create/update/enable/disable/run/stop/logs, host listing, JSON output for automation, troubleshooting auth/profile issues, or fixing task create/update failures caused by write-form parameters. Use for gocron cron operations instead of calling private REST APIs directly."
---

# gocron-cli

## Overview

Use `gocron-cli` as the supported automation surface for gocron task management. It talks to `/api/agent/v1`, uses browser device authorization, and stores credentials locally in `~/.gocron/config.json`.

## Safety Rules

- Do not print, paste, commit, or log `~/.gocron/config.json`, access tokens, refresh tokens, or authorization URLs after login succeeds.
- Do not invent a delete workflow. The CLI and agent API intentionally do not support deleting cron tasks by default.
- Prefer `--json` for agent-readable output; include request IDs from errors when reporting failures.
- Use named profiles with `--profile` when working with more than one gocron server.
- For production cron changes, inspect the existing task first, make the smallest update, then verify with `task get` or `task list`.

## Setup

Check the CLI is installed:

```bash
command -v gocron-cli
gocron-cli --version
```

Log in with browser authorization:

```bash
gocron-cli login --server https://your-gocron.example.com
```

The CLI prints a browser URL. Open it while logged in as a gocron super administrator, approve the device, then let the CLI finish polling. Use a profile if needed:

```bash
gocron-cli --profile prod login --server https://your-gocron.example.com --device-name codex-prod
gocron-cli --profile prod --json task list
```

Log out and revoke the current local profile:

```bash
gocron-cli --profile prod logout
```

If requests fail after a device was revoked, log in again.

## Read Operations

Use JSON output by default when another tool or script will parse the result:

```bash
gocron-cli --json task list
gocron-cli --json task list --page 2 --pagesize 50
gocron-cli --json task list --name sync --tag ops --status 1
gocron-cli --json task get 123
gocron-cli --json task logs 123
gocron-cli --json host list
```

Without `--json`, the CLI prints the API message and raw data for quick human inspection.

`task list` supports these query flags:

- Pagination: `--page`, `--page-size`, `--pagesize`
- Filters: `--id`, `--host-id`, `--name`, `--protocol`, `--tag`, `--command`, `--status`

Use `--pagesize` as a convenient alias for the API's `page_size`. If both `--page-size` and `--pagesize` are present, `--page-size` wins.

## Task Changes

Create and update tasks from a flat JSON or simple `key: value` YAML file. Prefer JSON for automation and for values that contain shell quoting, `:`, `#`, or long commands. The YAML parser is intentionally simple: one `key: value` per line, scalar values only, no nested structures.

Before writing a task, gather the concrete IDs and a reference shape:

```bash
gocron-cli --json host list
gocron-cli --json task list --name similar-name
gocron-cli --json task get 123
```

Do not guess `host_id`, notification receiver IDs, or whether a task should be HTTP vs shell. For shell/RPC tasks, `host_id` is a comma-separated list of host IDs from `host list`; host names are not accepted in write files. For HTTP tasks, omit `host_id` or set it to an empty string.

Omitted optional fields use the Web create-page defaults:

- `level: 1` 主任务
- `dependency_status: 1` 强依赖
- `protocol: 2` shell
- `http_method: 1` GET
- `timeout: 0`
- `multi: 2` 单实例运行=是
- `retry_times: 0`
- `retry_interval: 0`
- `notify_status: 1` 不通知
- `notify_type: 2` 邮件
- `status: 1` 启用
- `tag`、`dependency_task_id`、`host_id`、`notify_receiver_id`、`notify_keyword`、`remark` default to empty strings.

Use full write files for create and update unless the task is trivial. The server validates and rewrites several fields, so sparse files are the main source of create/update failures.

Required fields for a default shell parent task:

```yaml
name: nightly-sync
spec: "0 0 2 * * *"
command: "php /data/app/index.php sync/run"
host_id: "1"
```

Required-field rules:

- `name` is always required, max 32 characters, and must be unique among enabled tasks.
- `command` is always required, max 256 characters. For HTTP tasks it must start with `http://` or `https://`.
- `spec` is required for parent tasks (`level: 1`) and should use the 6-field gocron format with seconds, for example `0 0 2 * * *`. Child tasks (`level: 2`) do not use `spec`.
- `host_id` is required for shell/RPC tasks (`protocol: 2`); use comma-separated numeric host IDs. HTTP tasks (`protocol: 1`) do not use `host_id`.
- `protocol` must be `1` for HTTP or `2` for shell/RPC.
- `http_method` must be `1` for GET or `2` for POST. For HTTP tasks, `timeout` must be `0..300`; for shell/RPC tasks, `timeout` can be `0..86400`.
- `dependency_status` must be `1` strong or `2` weak, even when there are no dependencies.
- `multi` is write-form encoded: `1` allows concurrent runs; `2` means single-instance behavior.
- `retry_times` must be `0..10`; `retry_interval` must be `0..3600`.
- `notify_status` is write-form encoded: `1` no notification, `2` failure notification, `3` completion notification, `4` keyword notification. When notification is enabled (`2..4`) and `notify_type` is not webhook (`4` in write form), provide `notify_receiver_id`.

For `task update`, omitted optional fields are still filled with Web defaults. To preserve existing optional values, inspect the task first and include every writable field in the update file. Do not paste `task get` JSON directly into an update file: `task get` returns storage-format values, while `task create/update` expects write-form values.

When converting `task get` output into a write file:

- Keep: `name`, `level`, `dependency_task_id`, `dependency_status`, `spec`, `protocol`, `command`, `http_method`, `timeout`, `retry_times`, `retry_interval`, `tag`, `remark`, `notify_receiver_id`, `notify_keyword`, `status`.
- Convert `multi`: if detail `multi` is `1`, write `multi: 1`; otherwise write `multi: 2`.
- Convert `notify_status`: write `detail.notify_status + 1`.
- Convert `notify_type`: write `detail.notify_type + 1`.
- Convert hosts: write `host_id` as comma-separated IDs from `detail.hosts[].id`; do not include the nested `hosts` array.
- Do not include read-only fields such as `id`, `created`, `updated`, `deleted`, `creater`, `updater`, `next_run_time`, or nested `hosts`.

Full shell/RPC parent-task example with explicit Web defaults:

```yaml
name: nightly-sync
level: 1
dependency_status: 1
dependency_task_id: ""
spec: "0 0 2 * * *"
protocol: 2
command: "php /data/app/index.php sync/run"
host_id: "1"
timeout: 0
multi: 2
retry_times: 0
retry_interval: 0
tag: ""
remark: ""
notify_status: 1
notify_type: 2
notify_receiver_id: ""
notify_keyword: ""
status: 1
```

Full HTTP parent-task example:

```yaml
name: healthcheck
level: 1
dependency_status: 1
dependency_task_id: ""
spec: "0 */5 * * * *"
protocol: 1
http_method: 1
command: "https://example.com/health"
host_id: ""
timeout: 30
multi: 2
retry_times: 0
retry_interval: 0
tag: "ops"
remark: ""
notify_status: 1
notify_type: 2
notify_receiver_id: ""
notify_keyword: ""
status: 1
```

Use `protocol: 1` for HTTP tasks and `protocol: 2` for shell/RPC tasks. HTTP tasks use `command` as the URL and may set `http_method` to `1` or `2`; shell tasks require `host_id`.

Recommended write workflow:

1. Read hosts and a similar task when available.
2. Create a complete `task.json` or `task.yaml` with only writable scalar fields.
3. Run `gocron-cli task create --file task.json` or `gocron-cli task update <id> --file task.json`.
4. Immediately verify with `gocron-cli --json task list --name <name>` and then `gocron-cli --json task get <id>`.
5. If the server returns `表单验证失败`, `请选择主机名`, `请输入正确的URL地址`, cron parse errors, retry range errors, or notification receiver errors, fix the file fields rather than retrying unchanged.

Run changes:

```bash
gocron-cli task create --file task.yaml
gocron-cli task update 123 --file task.yaml
gocron-cli task enable 123
gocron-cli task disable 123
gocron-cli task run 123
gocron-cli task stop 123 456
```

`task stop <task_id> <log_id>` only applies to running shell/RPC task logs. Get `log_id` from `task logs`.

## Troubleshooting

- `not logged in`: run `gocron-cli login` for the target profile.
- Authorization pending or timeout: open the printed URL, approve as a super administrator, and retry before the device code expires.
- `refresh token无效` or revoked-device errors: run `logout` if possible, then `login` again.
- Permission errors: confirm the approving gocron user is still enabled and is a super administrator.
- Form validation errors: compare the file fields with the Web task form; check `protocol`, `host_id`, `spec`, notification fields, retry bounds, and HTTP timeout limits.
