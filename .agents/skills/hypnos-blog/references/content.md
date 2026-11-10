# 日记、读书笔记与文章

文章存在博客数据库。源码的 Git push 不会写入文章。后台在目标站的 `/admin/login`，Agent 导入用仓库自己的 `cmd/blogctl`。本机如果没有独立的 `blogctl`，可直接在当前仓库根目录运行下面的命令。

```sh
go run ./cmd/blogctl article validate --json /absolute/path/entry.md
```

`validate` 只检查本地文件和引用的素材，不需要账号。Markdown front matter 的 `categories` 填一个已有专栏，`tags` 可以有多个，`display_time` 决定文章在时间线的位置。收录日记精华时，只取用户指定的内容。具体大小限制、可打包的相对图片和附件、HTML/ZIP 输入见仓库 [docs/blogctl.md](../../../../docs/blogctl.md)。

**导入到正式站。** 先确认具有 `article:import` 权限的目标地址和身份。公网 HTTPS 正常时用 `https://www.hypn0s.cloud`，HTTPS 故障时可使用已核实归属本博客的 SSH 回环隧道。CLI 仅允许对回环地址使用 HTTP。命令示例中的 `TARGET` 指已经核实的地址。

```sh
BLOGCTL_SERVER="$TARGET" go run ./cmd/blogctl auth status --json
BLOGCTL_SERVER="$TARGET" go run ./cmd/blogctl article import --json /absolute/path/entry.md
```

首次使用由管理员在后台「Agent 密钥」创建限权 Token，在交互终端运行 `go run ./cmd/blogctl auth login --server "$TARGET"`，通过隐藏输入录入。CLI 自行读取 macOS 用户配置 `~/Library/Application Support/myblogs/credentials.json`。使用 `auth status` 核对身份即可，不读取或打印凭据文件、环境变量值或 Cookie。先确认 18081 的 SSH 转发指向已授权的博客 SSH 主机，再将生产 Token 发到这个回环端口。

导入成功返回草稿 ID、编辑或预览路径。`article:import` 只建草稿。明确要求发表、改已有文章或删除时，使用获授权的后台会话完成相应动作，再核对可见性。草稿预览需要管理员登录，私密文章匿名访问返回 404。重试超时的导入前先查原草稿，尤其是在服务重启后，服务进程的幂等缓存不会跨重启保留。
