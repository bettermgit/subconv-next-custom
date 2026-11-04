# SubConv Next（个人定制分支）

面向 Mihomo / Clash Meta 的自托管订阅转换服务：抓取机场订阅（Clash/Mihomo YAML、Base64、逐行 URI）→ 统一节点模型 → 去重 / 过滤 / 改名 / 加国旗 → 渲染校验后的 Mihomo YAML → 以静态文件发布为 `/s/<token>/mihomo.yaml` 私有订阅链接。单个 Go 二进制同时提供 API 与 Web 控制台（原生 JS，`//go:embed` 内嵌），适合局域网 / NAS / 单机部署。

A self-hosted subscription converter for Mihomo / Clash Meta, packaged as a single Go binary with a built-in web console. This fork focuses on a server-wide custom rule library and durable per-browser workspaces.

## 致谢与开源声明

本项目基于 [Earl9/subconv-next](https://github.com/Earl9/subconv-next) 进行二次开发。
感谢原作者 [Earl9](https://github.com/Earl9) 的无私贡献。

本分支在 MIT License 下发布，完整保留上游版权声明与 [LICENSE](LICENSE) 全文。

## 本分支修改记录

### 2026-11-04 · 全局规则库与持久化重构

- **全局自定义规则库**（本分支核心改动）：自定义规则不再存储在工作区配置中，而是集中存储于服务器（`data/global_custom_rules.json`），**所有工作区生成订阅时自动共用**；全局规则支持手动**添加 / 删除 / 启用 / 停用**，任何操作即时持久化，刷新页面、更换浏览器、重启服务均不丢失。新增 API：`GET/POST /api/global-rules`、`PUT/DELETE /api/global-rules/<key>`。
- **自定义规则即时落盘**：修复旧版规则修改仅存在页面内存、刷新即丢失的问题；保存、删除、启停、预设切换、直写规则编辑等所有规则操作均即时写入服务端。
- **工作区跨刷新持久化**：浏览器记住服务器工作区并跨刷新复用，配置与规则在页面刷新后自动恢复；工作区保留期延长为 30 天。
- **远程规则集（mrs）快速添加**：一次输入名称与多条外链（每行一条，支持「名称 URL」格式），自动去重、自动识别 mrs / text 格式并入库。
- **规则预设行新增「＋ 手动添加」按钮**：一键直达自定义规则编辑器（内联 / 远程规则集 / 本地文件 / 仅代理组四种来源）。
- **移除部分内置规则预设分组**及相关前后端代码与文档。
- **个人自托管精简**：移除 Docker、OpenWrt / LuCI 打包链路及内部开发文档，保留 CI；仓库聚焦单机部署场景。

### 2026-10 · 规则体系与订阅管理迭代

- **规则引擎增强**：规则支持静态域名（`DOMAIN-SUFFIX`）、`Priority` 置顶、静态 IP 段（`IP-CIDR` / `IP-CIDR6`，支持 `no-resolve`）；局域网直连规则固定最前。
- **规则目标治理**：所有分流规则必须落到可选节点的代理组，禁止将 `DIRECT` / `REJECT` 直接写入规则；直连类分组在组内置顶 `DIRECT` 成员，节点仍可手动切换。
- **国家 / 地区代理组**默认开启。
- **节点旗帜 emoji**：按节点名 / 地区标识自动为节点名前置国家地区旗帜 emoji，支持开关、幂等处理；输出校验层统一 `RenderedNodeName` 命名口径并识别 emoji 变体，杜绝校验误报。
- **已生成订阅管理器**：`/s/<token>/mihomo.yaml` 发布物的列表、打开、刷新、删除与备注管理。
- **订阅用量条**：透传并展示上游 `Subscription-Userinfo`（流量与到期时间）。
- 输出选项白名单分组与表单回填修复，默认值调整。

## 功能总览

- 聚合多个订阅源，解析 Base64、Clash/Mihomo YAML 与节点 URI。
- 节点编辑、停用、删除、恢复、批量改名、手动添加。
- 内置路由规则预设 + **全局自定义规则库** + 远程规则集（mrs / text）+ 自定义策略组。
- 可选国家与自动测试国家分组；节点旗帜 emoji。
- 渲染前校验代理引用、规则提供方、策略组与 `MATCH` 收尾顺序，校验通过才落盘。
- 不可猜的 `/s/<token>/mihomo.yaml` 订阅发布链接；发布物管理。
- 定时刷新 + 按请求刷新（stale-if-error），配置三层分离（基础 / 工作区 / 发布物）。

支持协议：`ss`、`ssr`、`vmess`、`vless`、`trojan`、`hysteria2`、`tuic`、`anytls`、`wireguard`、`mieru`。

## 快速开始

```bash
# 构建并本机运行（默认 127.0.0.1:9876）
make serve

# 局域网访问：必须设置至少 24 位访问令牌
export SUBCONV_ACCESS_TOKEN="$(openssl rand -hex 32)"
go run ./cmd/subconv-next serve --host 0.0.0.0 --port 9876 --data-dir ./data
```

浏览器打开 `http://127.0.0.1:9876/`（局域网则用主机 IP），使用访问令牌登录。后台常驻可注册为 systemd 服务（`subconv-next serve --config /etc/subconv/config.json --data-dir /var/lib/subconv`）。

> ⚠️ 绑定 `0.0.0.0` 时服务强制要求强访问令牌；请勿在未防护的网络上暴露管理端口。详见 [SECURITY.md](SECURITY.md)。

## 开发

```bash
make test   # go test ./...
make vet
make build  # 产出 ./subconv-next
```

前端位于 `internal/api/static/`（`//go:embed` 内嵌），修改后需重新 `go build`。

## License

[MIT](LICENSE) · 版权归上游 SubConv Next contributors 与本分支修改者共同所有，遵循上游 MIT 条款。
