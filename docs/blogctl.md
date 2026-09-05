# blogctl：本地 Agent 文章导入

`blogctl` 与 MyBlogs 服务端放在同一个仓库，但运行位置不同：服务端部署在博客主机，
CLI 编译后运行在写作电脑或本地 Agent 环境。文章、图片、Token 和本机凭据都不需要放进
项目仓库。

## 安装

在克隆后的 MyBlogs 仓库中执行：

```bash
make install-cli
blogctl version
```

`make install-cli` 等同于 `go install ./cmd/blogctl`，二进制会进入 Go 的 `GOBIN`。
也可以用 `make build-cli` 在 `bin/blogctl` 生成当前平台的二进制；`bin/` 已被 Git 忽略。

仓库推送 `blogctl-v*` 标签时，GitHub Actions 会为 macOS、Linux、Windows 的 amd64/arm64
生成带 SHA-256 校验文件的 Release 压缩包。

## 一次性认证

1. 登录 MyBlogs 后台。
2. 打开 `Agent 密钥`。
3. 填写设备/用途名称和有效期，创建权限为 `article:import` 的密钥。
4. 立即在本机执行：

```bash
blogctl auth login --server https://www.hypn0s.cloud
```

Token 输入不会回显。CLI 会先访问 `/api/v1/auth` 确认密钥有效，再保存到操作系统的用户
配置目录，文件权限为 `0600`。macOS 默认位置是
`~/Library/Application Support/myblogs/credentials.json`，Linux 通常是
`~/.config/myblogs/credentials.json`。

```bash
blogctl auth status
blogctl auth status --json
blogctl auth logout
```

不要把 Token 放在命令参数中，以免进入 shell history。更不要把它写进 Markdown、代码、
Git、Agent 提示词或日志。密钥遗失时直接在后台撤销。

## Markdown 格式

Front matter 可省略，也可以使用下面这些字段：

```markdown
---
title: Agent 系统阅读笔记
slug: agent-system-notes
tags: [Agent, Paper]
categories: 研究, 学习
display_time: "2026-09-05T21:30"
status: draft
---

# 正文

![架构图](images/architecture.png)
[论文 PDF](attachments/paper.pdf)
```

- `title`：可省略；省略时服务端使用文件名。
- `slug`：可省略；填写时只允许字母、数字、`_`、`-`，长度 5–100。
- `tags` / `categories`：可写 YAML 字符串列表或英文逗号分隔字符串。
- `display_time`：支持 `YYYY-MM-DDTHH:MM` 或 RFC3339。
- `status`：只能省略或写 `draft`。CLI 和服务端都会拒绝通过 Agent Token 直接发布。

命令行 flags 会覆盖同名 front matter：

```bash
blogctl article import \
  --title "覆盖后的标题" \
  --tags "Agent,源码" \
  --categories "学习" \
  --display-time "2026-09-05T21:30" \
  --json \
  ./article.md
```

建议 Agent 始终先校验再导入：

```bash
blogctl article validate --json ./article.md
blogctl article import --json ./article.md
```

`--json` 的输出不会包含 Token，适合 Agent 稳定解析。导入成功会返回文章 ID、后台编辑路径、
草稿预览路径，以及本次结果是否来自幂等重放。

## 本地资源

输入 `.md` 或 `.markdown` 时，CLI 会扫描标准 Markdown 链接以及内联 HTML 的
`src`、`href`、`poster`，只打包正文实际引用的相对资源。外部 URL、站内绝对 URL、锚点和
其他 Markdown 文件不会打包。

安全与容量限制和服务端一致：

- 主文件与资源合计最多 100 个条目，解压后最多 32 MB；
- 打包结果最多 16 MB；Markdown 最多 200 KB，HTML 最多 8 MB；
- 单个资源最多 4 MB；
- 资源必须位于文章所在目录内，不能使用隐藏路径或越界符号链接；
- 图片支持 jpg/jpeg/png/gif/webp/bmp；附件支持 txt/pdf/zip 和常见 Office 格式。

输入单个 `.html` / `.htm` 时会直接上传。若 HTML 引用了相对图片，请按原目录结构先打成
ZIP；HTML ZIP 只允许图片资源，CSS 和 JavaScript 需要内联。已有 `.zip` 也可直接交给
CLI，客户端和服务端都会再做结构及容量检查。

## 无交互 Agent / CI

无交互环境可以通过环境变量提供配置：

```bash
export BLOGCTL_SERVER='https://www.hypn0s.cloud'
export BLOGCTL_TOKEN='从安全凭据存储注入，不要写进仓库'
blogctl article import --json ./article.md
```

可用变量：

| 变量 | 作用 |
| --- | --- |
| `BLOGCTL_SERVER` | 覆盖保存的博客地址 |
| `BLOGCTL_TOKEN` | 覆盖保存的 Token |
| `BLOGCTL_CONFIG` | 覆盖凭据文件路径，便于隔离不同博客 |

同一篇内容与元数据会产生稳定的 `Idempotency-Key`。网络重试命中同一服务进程的 24 小时
结果缓存时会返回原文章，不会再次创建；服务重启后仍应先在后台检查结果，再人工重试。

## 权限边界

Agent Token 与管理员登录 Cookie、访客文章访问密钥是三套独立凭据。v1 只有
`article:import`：

- 可以：认证自身、导入 Markdown/HTML/ZIP 为草稿；
- 不可以：发布文章、修改或删除既有文章、上传任意独立附件、管理评论、页面或站点设置；
- Token 明文不入库，后台只能查看名称、Token ID、权限、有效期和最近使用时间；
- 撤销或到期后，下一个 API 请求立即返回 `401`。

服务端接口为：

```text
GET  /api/v1/auth
POST /api/v1/articles/import
```

它们使用 `Authorization: Bearer ...`，不复用后台 Session 或 CSRF。文章导入请求必须包含
`Idempotency-Key`；普通使用者应通过 `blogctl` 调用，不需要手写 HTTP 请求。
