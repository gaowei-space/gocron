# ⌛️ Gocron

[![Downloads](https://img.shields.io/github/downloads/ouqiang/gocron/total.svg)](https://github.com/gaowei-space/gocron/releases)
[![license](https://img.shields.io/github/license/mashape/apistatus.svg?maxAge=2592000)](https://github.com/ouqiang/gocron/blob/master/LICENSE)
[![Release](https://img.shields.io/github/release/gaowei-space/gocron.svg?label=Release)](https://github.com/gaowei-space/gocron/releases)



## 项目简介

> 该项目fork于[ouqiang/gocron](https://github.com/ouqiang/gocron)，依据自己喜好和实际需求进行了功能迭代，当前发布 **1.6.5** 版本。

**[Gocron-定时任务管理系统](https://github.com/gaowei-space/gocron)**，使用Go语言开发的轻量级定时任务集中调度和管理系统, 用于替代**Linux-crontab**

## 迭代

### v1.6.5

* 优化 `gocron-cli` 本地认证刷新逻辑，记录 access token 过期时间并提前刷新
* CLI 刷新 refresh token 时增加本地锁，避免多个进程并发刷新导致设备授权被撤销
* 普通业务错误不再触发 token 刷新，减少无意义的 refresh token 轮换
* 服务端对刚轮换后的 refresh token 重放增加短时间宽限，降低并发调用时误撤销设备授权的概率

### v1.6.4

* 新增 `gocron-cli`，支持通过浏览器授权后管理 cron 任务
* CLI 支持任务查询、详情、创建、修改、启停、手动运行、停止运行实例、日志查询和主机查询
* CLI 默认不支持删除 cron 任务，删除仍需在 Web 后台操作
* 新增独立 Agent REST API，使用短期 access token 和轮换 refresh token
* 新增 CLI 设备授权管理，超级管理员可在 Web 后台查看和撤销已授权设备
* 新增 agent 操作审计表，记录授权、刷新、写操作、运行和停止等行为

### v1.6

* 优化整体**界面样式与布局**，包括界面色系，列表，详情，按钮组，分页等
* 调整权限等级，增加**超级管理员**，可以管理所有任务；**管理员**调整为管理自己的任务和查看其他任务和日志，普通用户与原有权限一致，仅可查看所有任务和日志
* 任务详情页增加快捷选择crontab按钮组
* 任务详情页支持更改任务状态
* 任务列表支持**标签**,**命令**搜索



## 截图


![列表](https://user-images.githubusercontent.com/10205742/184531121-f5faa1a9-4d13-4132-a96d-848375765cda.jpg)



![日志](https://user-images.githubusercontent.com/10205742/184531126-0f159cda-8774-4185-9132-194e66cd5d3c.jpg)



![节点](https://user-images.githubusercontent.com/10205742/184531128-7a9a07a9-cac2-4dea-a37a-5cb57479a528.jpg)



![webhook](https://user-images.githubusercontent.com/10205742/184531159-582fd407-bed1-4ed4-a469-e8b9d5af67cb.jpg)





## 功能特性

- Web界面管理定时任务

- crontab时间表达式, 精确到秒

- 任务执行失败可重试

- 任务执行超时, 强制结束

- 任务依赖配置, A任务完成后再执行B任务

- 账户权限控制

- 任务类型

  - shell任务

  > 在任务节点上执行shell命令, 支持任务同时在多个节点上运行

  - HTTP任务

  > 访问指定的URL地址, 由调度器直接执行, 不依赖任务节点

- 查看任务执行结果日志

- 任务执行结果通知, 支持邮件、Slack、Webhook

- `gocron-cli` 命令行管理，适合 agent 或自动化脚本使用

## gocron-cli

`gocron-cli` 使用浏览器授权流程，不需要手动复制长期 token。第一版仅支持超级管理员授权使用。

```bash
gocron-cli login --server https://your-gocron.example.com
gocron-cli task list
gocron-cli task get 1
gocron-cli task create --file task.yaml
gocron-cli task update 1 --file task.yaml
gocron-cli task enable 1
gocron-cli task disable 1
gocron-cli task run 1
gocron-cli task logs 1
gocron-cli task stop 1 100
gocron-cli host list
```

需要机器可读输出时添加 `--json`：

```bash
gocron-cli --json task list
```

CLI 凭据保存在本机用户目录下的 `.gocron/config.json`。服务端仅保存 refresh token hash，超级管理员可在 Web 后台的 `Agent授权` 页面撤销设备授权。

#### 了解更多

- 原作 [https://github.com/ouqiang/gocron](https://github.com/ouqiang/gocron)
