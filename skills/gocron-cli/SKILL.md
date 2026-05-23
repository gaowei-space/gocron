---
name: gocron-cli
description: "Use when Codex needs to operate a gocron scheduler through the gocron-cli command line client: browser-login authorization, task listing/detail/create/update/enable/disable/run/stop/logs, host listing, JSON output for automation, or troubleshooting gocron-cli auth/profile issues. Use for gocron cron operations instead of calling private REST APIs directly."
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
gocron-cli --json task get 123
gocron-cli --json task logs 123
gocron-cli --json host list
```

Without `--json`, the CLI prints the API message and raw data for quick human inspection.

## Task Changes

Create and update tasks from a flat JSON or simple `key: value` YAML file. The parser supports scalar strings, numbers, and booleans only; nested YAML is not supported.

Common fields mirror the existing Web form names:

```yaml
name: nightly-sync
level: 1
dependency_status: 1
dependency_task_id: ""
spec: "0 0 2 * * *"
protocol: 2
command: "php /data/app/index.php sync/run"
host_id: "1"
timeout: 3600
multi: 2
retry_times: 0
retry_interval: 0
tag: ops
remark: "nightly sync"
notify_status: 1
notify_type: 1
notify_receiver_id: ""
notify_keyword: ""
status: 1
```

Use `protocol: 1` for HTTP tasks and `protocol: 2` for shell/RPC tasks. HTTP tasks use `command` as the URL and may set `http_method` to `1` or `2`; shell tasks require `host_id`.

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
