# Security Policy · 安全策略

**English** · [简体中文](#安全策略-简体中文)

## Supported Versions

| Version | Supported |
| --- | --- |
| `main` (latest commit) | ✅ Yes |
| Tagged releases | Not published yet — the first planned release is `v1.0.0`; security fixes land on `main` only for now |
| Anything older than `main` | ❌ No |

## Security Boundary

SubConv Next Custom runs as a single Linux binary (AMD64 / ARM64) and supports three deployment modes: local single-user, trusted-LAN, and workspace-isolated public converter.

**Authentication and sessions**

- The web UI exchanges the access token on `/login` for a signed, HttpOnly, SameSite session cookie; unsafe browser requests additionally require a session-bound CSRF token.
- API clients may use `Authorization: Bearer <token>` or HTTP Basic credentials.
- `/healthz` and bearer-style `/s/{token}/...` subscription links sit outside the management login boundary; tokens must never be passed in API query parameters.
- Every non-loopback listener requires an access token of at least **24 characters**; loopback and private-network source addresses do not bypass this. `SUBCONV_ALLOW_INSECURE_PUBLIC=true` is an explicit local-preview escape hatch only and is not supported for Internet-facing production use.

**Public converter mode (`SUBCONV_PUBLIC_CONVERTER=true`)**

- Passwordless use of the converter allowlist; anonymous state is isolated by cryptographically random workspace capabilities, and publication management requires the owning workspace capability.
- Enforced server-side fetch limits, configuration complexity quotas, final-node and rendered-output limits, a bounded publication store, common Web-port restrictions, DNS-rebinding-resistant dialing, cross-site API rejection, and automatic cleanup.
- Public workspaces cannot disable upstream TLS certificate verification.
- Caps default to 256 workspaces / 256 publications / 4 concurrent refreshes (`SUBCONV_MAX_WORKSPACES`, `SUBCONV_MAX_PUBLICATIONS`, `SUBCONV_MAX_CONCURRENT_REFRESHES`); the browser login endpoint is disabled, while protected API routes keep accepting the management Bearer token.

**Published subscription links**

- A published `/s/{token}/...` link only authorizes downloading the rendered YAML. In public mode it cannot restore the editable workspace or reveal upstream URLs, credentials, or manual source content; editable recovery is limited to browser-local drafts and the separate high-entropy `publish_id` capability they store.

## Public Deployment Checklist

Public deployments must:

- Keep the backend port private — terminate HTTPS at a reverse proxy and never expose the management port to untrusted networks.
- Add rate limiting at the edge.
- Run as a dedicated unprivileged user with an owner-only data directory (`--data-dir`).
- Pass absolute paths for `--config` and `--data-dir` when running as a system service (the default config path is relative to the working directory).

## Sensitive Data

- SubConv Next Custom attempts to redact sensitive values in APIs and logs, including upstream subscription URL tokens, published subscription tokens, passwords, UUIDs, WireGuard private keys, pre-shared keys, `Authorization`, and `Cookie` values.
- New runtime configuration, node state, cache, logs, generated YAML, and publication metadata are written with owner-only permissions; restrict access to the data directory as well.
- Never publish real subscription URLs, tokens, node secrets, or unredacted logs in public issues.

## Private Subscription Links

Published subscriptions use bearer-style private URLs:

```text
/s/{token}/mihomo.yaml
```

Anyone holding the full URL can fetch the generated YAML. If a link leaks, use **Regenerate private link**（界面上的「重新生成私密链接」按钮）in the web UI — the old token is invalidated immediately. Deleting a published subscription also invalidates its URL.

## Reporting a Vulnerability

Please report vulnerabilities **privately** via GitHub Security Advisory: open the [Security tab → "Report a vulnerability"](https://github.com/bettermgit/subconv-next-custom/security/advisories/new) form. Avoid public issues for security problems.

Include the affected commit, deployment mode (local / LAN / public converter), a minimal reproduction, and redacted logs or configuration. Do not include real tokens, upstream subscription links, node passwords, UUIDs, private keys, cookies, authorization headers, or full published subscription URLs.

---

# 安全策略 · 简体中文

## 支持版本

| 版本 | 是否支持 |
| --- | --- |
| `main`（最新提交） | ✅ 支持 |
| 正式版本（tag） | 尚未发布——首个版本计划为 `v1.0.0`；当前安全修复仅随 `main` 分支提供 |
| 早于 `main` 的旧快照 | ❌ 不支持 |

## 安全边界

SubConv Next Custom 以单个 Linux 二进制（AMD64 / ARM64）运行，支持三种部署形态：本机单人使用、受信局域网、工作区隔离的公共转换器。

**认证与会话**

- Web 控制台在 `/login` 用访问令牌换取签名 HttpOnly、SameSite 会话 Cookie；非安全方法请求额外校验会话绑定的 CSRF 令牌。
- API 客户端可使用 `Authorization: Bearer <token>` 或 HTTP Basic 凭据。
- `/healthz` 与 `/s/{token}/...` 形式的订阅链接位于管理登录边界之外；令牌一律不得通过 API 查询参数传递。
- 任何非回环监听强制要求**至少 24 位**访问令牌；回环与私网来源地址同样不豁免。`SUBCONV_ALLOW_INSECURE_PUBLIC=true` 仅限本机预览，不支持面向互联网的生产使用。

**公共转换器模式（`SUBCONV_PUBLIC_CONVERTER=true`）**

- 转换器白名单可免密使用；匿名状态由密码学随机的能力令牌隔离，发布物管理必须持有所属工作区能力令牌。
- 强制服务端抓取上限、配置复杂度配额、最终节点数与渲染输出上限、有限发布物存储、常用 Web 端口限制、防 DNS 重绑定拨接、跨站 API 拒绝与自动清理。
- 公共工作区不可关闭上游 TLS 证书校验。
- 上限默认 256 工作区 / 256 发布物 / 4 并发刷新（`SUBCONV_MAX_WORKSPACES`、`SUBCONV_MAX_PUBLICATIONS`、`SUBCONV_MAX_CONCURRENT_REFRESHES`）；浏览器登录入口关闭，受保护 API 仍接受管理 Bearer 令牌。

**已发布订阅链接**

- `/s/{token}/...` 链接仅授权下载已渲染 YAML；公共模式下不能借它恢复可编辑工作区，也不会暴露上游 URL、凭据或手动源内容；可编辑恢复仅限浏览器本地草稿及其保存的高熵 `publish_id` 能力令牌。

## 公网部署清单

公网部署必须：

- 后端端口保持私有——由反向代理终止 HTTPS，管理端口绝不暴露到不可信网络。
- 在边缘增加访问频率限制。
- 使用专用低权限用户运行，数据目录（`--data-dir`）仅属主可访问。
- 以系统服务方式运行时，`--config` 与 `--data-dir` 必须传绝对路径（默认配置路径相对工作目录）。

## 敏感数据

- 程序尽力脱敏 API 与日志中的敏感值：上游订阅 URL token、发布订阅 token、密码、UUID、WireGuard 私钥、预共享密钥、`Authorization`、`Cookie`。
- 新建的运行时配置、节点状态、缓存、日志、生成的 YAML 与发布物元数据均以仅属主权限写入；同时应对数据目录做访问限制。
- 切勿在公开 issue 中贴出真实订阅 URL、令牌、节点密码或未脱敏日志。

## 私有订阅链接

发布的订阅使用 bearer 式私有 URL：

```text
/s/{token}/mihomo.yaml
```

持完整 URL 的任何人都能取回生成的 YAML。链接泄露时在 Web 界面点击「**重新生成私密链接**」，旧 token 立即失效；删除发布物同样使其 URL 失效。

## 漏洞报告

请通过 GitHub 安全通告**私下**报告漏洞：打开 [安全（Security）页 → 报告漏洞（Report a vulnerability）](https://github.com/bettermgit/subconv-next-custom/security/advisories/new)表单，不要公开提 issue。

报告请包含：受影响的提交号、部署形态（本机 / 局域网 / 公共转换器）、最小复现步骤、脱敏后的日志或配置。请勿包含真实令牌、上游订阅链接、节点密码、UUID、私钥、Cookie、Authorization 头或完整的私有订阅 URL。
