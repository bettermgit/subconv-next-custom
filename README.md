# SubConv Next Custom

**English** · [简体中文](#中文文档)

> A self-hosted subscription converter and rule manager for **Mihomo / Clash Meta**, shipped as a single Go binary with a built-in web console. Target platform: **Linux (AMD64 / ARM64)**.

SubConv Next Custom fetches airport subscriptions (Clash/Mihomo YAML, Base64 bundles, per-line URIs), normalizes them into a unified node model, applies dedup / filtering / renaming / flag-emoji, renders a **validated** Mihomo YAML, and publishes it as a private subscription link like `/s/<token>/mihomo.yaml`. One process serves both the API and the web console (vanilla JS embedded via `//go:embed`) — no databases, no external services.

This project is a fork of [Earl9/subconv-next](https://github.com/Earl9/subconv-next). See [Acknowledgments](#acknowledgments).

## Features

- Aggregate multiple subscription sources; parse Clash/Mihomo YAML, Base64 bundles, per-line node URIs and WireGuard `.conf` files.
- Node editing, disable / delete / restore, batch rename, manual add.
- Built-in routing rule presets, remote rule providers (mrs / text), custom policy groups.
- Country groups and auto-test groups; automatic flag emoji for node names.
- Output is validated before publishing: proxy references, rule providers, policy groups and `MATCH` termination must all pass, or nothing is written.
- Un-guessable private publish links `/s/<token>/mihomo.yaml`, with a publication manager (list / refresh / note / delete).
- Subscription usage bar: upstream `Subscription-Userinfo` (traffic & expiry) passed through to the UI.
- Scheduled refresh plus refresh-on-request with stale-if-error; three-layer configuration separation (base / workspace / publication).
- Backup and restore from CLI: `subconv-next backup export | inspect | restore`.

Supported node protocols: `ss`, `ssr`, `vmess`, `vless`, `trojan`, `hysteria`, `hysteria2`, `tuic`, `anytls`, `wireguard`, `mieru`, plus `http` / `socks5` relay nodes.

## What this fork adds

- **Server-wide global custom rule library** — custom rules live on the server (`data/global_custom_rules.json`) instead of a single browser workspace, and are shared automatically by every workspace when generating subscriptions. Add / delete / enable / disable from the UI, each change persisted immediately; survives page refresh, browser switch and service restart. API: `GET/POST /api/global-rules`, `PUT/DELETE /api/global-rules/<key>`.
- **Workspace persistence across refreshes** — the browser remembers its server-side workspace, so configuration and rules are restored after a refresh. Default workspace retention: 30 days (`workspace_ttl_seconds`).
- **Quick-add remote rule providers** — paste a name plus multiple URLs (one per line, `name URL` supported); entries are deduplicated and mrs / text format is detected automatically.
- **Rule target governance** — routing rules always resolve to a selectable proxy group and the output ends with `MATCH`, keeping generated configs loadable by Mihomo.
- A slimmer single-binary self-hosting experience: Docker / OpenWrt packaging removed, CI focused on Linux AMD64/ARM64.

## Quick start (build from source)

Prerequisites: Linux on AMD64 or ARM64, Go 1.22+ and git. Prebuilt binaries are not published yet; building from source takes about a minute.

```bash
git clone https://github.com/bettermgit/subconv-next-custom.git
cd subconv-next-custom
make serve        # builds, then listens on http://127.0.0.1:9876
```

Open <http://127.0.0.1:9876/> in a browser.

To reach the console from other machines on your LAN, an access token of at least 24 characters is **required**:

```bash
export SUBCONV_ACCESS_TOKEN="$(openssl rand -hex 32)"
./subconv-next serve --host 0.0.0.0 --port 9876 --data-dir ./data
```

## Running as a service

Run the binary under a dedicated unprivileged user with absolute paths (the default `--config` value `config/config.json` is relative to the working directory):

```bash
subconv-next serve \
  --config /etc/subconv-next/config.json \
  --data-dir /var/lib/subconv-next
```

Useful flags: `--host`, `--port`, `--public-base-url`, `--log-level`. Upgrades replace the binary only; everything under `--data-dir` (workspaces, publications, global rule library, state) is untouched. Back up or restore the whole installation with `subconv-next backup export` / `restore`.

## Development

```bash
make test        # go test ./...
make test-race   # go test -race ./...
make vet
make build       # produces ./subconv-next
```

The web console lives in `internal/api/static/` and is embedded into the binary via `//go:embed`; rebuild after changing it.

## Security

Binding to a non-loopback address without a strong access token is refused by design; do not expose the management port to untrusted networks, and terminate HTTPS at a reverse proxy for public deployments. See [SECURITY.md](SECURITY.md).

## Acknowledgments

This project is a derivative work based on [Earl9/subconv-next](https://github.com/Earl9/subconv-next). Many thanks to [Earl9](https://github.com/Earl9) for the generous contribution to the community.

Released under the MIT License, preserving the upstream copyright notice and the full [LICENSE](LICENSE).

## License

[MIT](LICENSE) — upstream copyright (SubConv Next contributors) retained; modifications are (c) 2026 bettermgit.

---

# 中文文档

> 面向 **Mihomo / Clash Meta** 的自托管订阅转换与规则管理工具，单个 Go 二进制内置 Web 控制台。目标平台：**Linux（AMD64 / ARM64）**。

SubConv Next Custom 抓取机场订阅（Clash/Mihomo YAML、Base64、逐行 URI）→ 统一节点模型 → 去重 / 过滤 / 改名 / 加国旗 → 渲染**经过校验的** Mihomo YAML → 以 `/s/<token>/mihomo.yaml` 形式的私有链接发布。单个进程同时提供 API 与 Web 控制台（原生 JS，经 `//go:embed` 内嵌），无数据库、无外部服务依赖。

本项目基于 [Earl9/subconv-next](https://github.com/Earl9/subconv-next) 二次开发，见[致谢与开源声明](#致谢与开源声明)。

## 功能特性

- 聚合多个订阅源；解析 Clash/Mihomo YAML、Base64 合集、逐行节点 URI 与 WireGuard `.conf` 文件。
- 节点编辑、停用 / 删除 / 恢复、批量改名、手动添加。
- 内置路由规则预设、远程规则集（mrs / text）、自定义策略组。
- 国家分组与自动测试分组；节点名自动前置国旗 emoji。
- 发布前强制校验：代理引用、规则提供方、策略组与 `MATCH` 收尾任一不通过即拒绝落盘。
- 不可猜测的私有订阅链接 `/s/<token>/mihomo.yaml`，附发布物管理器（列表 / 刷新 / 备注 / 删除）。
- 订阅用量条：透传上游 `Subscription-Userinfo`（流量与到期时间）。
- 定时刷新 + 按请求刷新（stale-if-error）；配置三层分离（基础 / 工作区 / 发布物）。
- 命令行备份恢复：`subconv-next backup export | inspect | restore`。

支持节点协议：`ss`、`ssr`、`vmess`、`vless`、`trojan`、`hysteria`、`hysteria2`、`tuic`、`anytls`、`wireguard`、`mieru`，以及 `http` / `socks5` 中转节点。

## 本分支增强

- **服务器全局自定义规则库**（核心改动）：自定义规则集中存储于服务器（`data/global_custom_rules.json`），不再局限于单个工作区，**所有工作区生成订阅时自动共用**；界面上手动**添加 / 删除 / 启用 / 停用**，任何操作即时持久化，刷新、换浏览器、重启服务均不丢失。API：`GET/POST /api/global-rules`、`PUT/DELETE /api/global-rules/<key>`。
- **工作区跨刷新持久化**：浏览器记住服务器工作区，刷新后配置与规则自动恢复；工作区默认保留 30 天（`workspace_ttl_seconds` 可调）。
- **远程规则集快速添加**：一次输入名称与多条 URL（每行一条，支持「名称 URL」格式），自动去重并识别 mrs / text 格式。
- **规则目标治理**：分流规则必须落到可选节点的代理组、输出以 `MATCH` 收尾，保证生成的配置可被 Mihomo 正常加载。
- 更轻量的单机自托管体验：移除 Docker / OpenWrt 打包链路，CI 聚焦 Linux AMD64/ARM64。

## 快速开始（源码构建）

前置条件：Linux AMD64 或 ARM64、Go 1.22+、git。暂未发布预编译二进制，源码构建约一分钟。

```bash
git clone https://github.com/bettermgit/subconv-next-custom.git
cd subconv-next-custom
make serve        # 构建并监听 http://127.0.0.1:9876
```

浏览器打开 <http://127.0.0.1:9876/> 即可使用。

如需局域网内其他设备访问，必须设置至少 24 位的访问令牌：

```bash
export SUBCONV_ACCESS_TOKEN="$(openssl rand -hex 32)"
./subconv-next serve --host 0.0.0.0 --port 9876 --data-dir ./data
```

## 注册为系统服务

建议使用专用低权限用户运行，并使用绝对路径（`--config` 默认值 `config/config.json` 是相对工作目录的相对路径）：

```bash
subconv-next serve \
  --config /etc/subconv-next/config.json \
  --data-dir /var/lib/subconv-next
```

常用参数：`--host`、`--port`、`--public-base-url`、`--log-level`。升级只需替换二进制，`--data-dir` 下的一切（工作区、发布物、全局规则库、状态）不受影响；整站备份恢复用 `subconv-next backup export` / `restore`。

## 开发

```bash
make test        # go test ./...
make test-race   # go test -race ./...
make vet
make build       # 产出 ./subconv-next
```

前端位于 `internal/api/static/`（经 `//go:embed` 内嵌），修改后需重新构建。

## 安全

设计上拒绝在无强访问令牌时绑定非回环地址；请勿将管理端口暴露于不可信网络，公网部署请在反向代理终止 HTTPS。详见 [SECURITY.md](SECURITY.md)。

## 致谢与开源声明

本项目基于 [Earl9/subconv-next](https://github.com/Earl9/subconv-next) 进行二次开发。
感谢原作者 [Earl9](https://github.com/Earl9) 的无私贡献。

本项目在 MIT License 下发布，完整保留上游版权声明与 [LICENSE](LICENSE) 全文。

## 许可证

[MIT](LICENSE)——上游版权（SubConv Next contributors）保留；本分支修改部分 © 2026 bettermgit。
