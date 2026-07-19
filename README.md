# Go My-Blog


项目保留了原博客的前台与后台功能、路由和数据表命名，默认使用零配置
SQLite，也可直接连接 MySQL `tale` 数据库。

## 功能

- 前台：首页分页、文章/页面、分类、标签、搜索、归档、友链、评论。
- 后台：登录、文章、页面、评论、分类/标签、附件、友链、系统设置、个人资料。
- 运维：数据库/附件备份、健康检查、优雅退出、Docker 部署。
- 安全：bcrypt 密码、旧 Java MD5 自动升级、签名 Cookie、无状态 CSRF、登录防爆破、按 IP 限流、上传白名单。
- 并发：连接池、SQLite WAL、分片 TTL 缓存、singleflight 防击穿、读缓存、异步点击落库。

### 访问权限

- 首页、文章正文、普通页面、搜索、分类、标签、归档、友链和评论均可公开访问，无需登录。
- `/admin/**` 后台管理需要登录。
- 草稿预览 `/article/:id/preview` 需要登录。

公开前端参考并适配了仓库内的
[`hexo-theme-fluid`](./hexo-theme-fluid)（GPL-3.0），保留了其许可证和主题来源说明。

## 快速启动

要求 Go 1.26+。

```bash
make run
```

启动后访问：

- 博客首页：`http://127.0.0.1:8081`
- 管理后台：`http://127.0.0.1:8081/admin/login`
- 存活检查：`http://127.0.0.1:8081/healthz`
- 就绪检查：`http://127.0.0.1:8081/readyz`

首次启动会自动创建 `data/blog.db`、管理员、站点设置、欢迎文章和关于页面。
管理员初始化信息只从环境变量读取，不会写入代码仓库。

生产部署前请复制 `.env.example` 为部署主机上的 `.env`，填入真实值；`.env` 已被
Git 忽略，不要把它提交到公开仓库：

```bash
cp .env.example .env
openssl rand -hex 32
```

将生成的随机值填入 `SESSION_SECRET`，并填写 `ADMIN_USERNAME`、`ADMIN_EMAIL` 和
`ADMIN_INITIAL_PASSWORD`。已有数据库不会重复创建管理员；这三个管理员初始化变量只在
数据库没有用户时生效。

## 日常使用

### 1. 登录后台

启动服务后打开：

```text
http://127.0.0.1:8081/admin/login
```

首次启动使用你在 `.env` 中设置的管理员账号和密码。登录后会进入管理首页，建议马上
打开顶部的 `个人设置` 检查显示名称、邮箱和密码。

登录功能主要用于管理，不是给普通读者使用的：

- `/admin/**`：文章、页面、评论、分类、标签、附件、友链和站点设置。
- `/article/:id/preview`：查看草稿预览。
- 首页、已发布文章、页面、搜索、分类、标签、归档、友链和公开评论：不需要登录。
- 草稿不会出现在公开首页；未登录访问草稿预览会跳转到后台登录页。

### 2. 发布一篇文章

登录后台后，按以下步骤操作：

1. 点击顶部 `文章`。
2. 点击右上角 `写文章`。
3. 填写标题。
4. 在 `内容 (Markdown)` 中写正文。
5. 填写标签和分类，多个值用英文逗号分隔，例如 `Go,Blog`。
6. `自定义路径 slug` 可留空；如果填写，只能使用英文字母、数字、下划线和连字符，
   长度为 5-100 个字符，例如 `go-blog-guide`。发布后访问
   `/article/go-blog-guide`。
7. 状态选择：
   - `发布`：保存后立即出现在公开首页，可以被所有人阅读。
   - `草稿`：只保存到后台，不出现在公开首页；登录后可以通过
     `/article/<文章ID>/preview` 预览。
8. 点击 `保存`。

文章正文使用 Markdown，例如：

````markdown
# 我的第一篇文章

这是一段正文，**这里是加粗文字**。

## 代码

```go
fmt.Println("hello blog")
```
````

发布后可以在 `文章`列表中点击标题打开公开页面，点击 `编辑`修改内容。

### 3. 上传并插入图片

图片不要直接塞进项目源码，推荐使用后台附件功能：

1. 点击顶部 `附件`。
2. 选择图片并点击 `上传`。
3. 上传成功后，在附件列表中复制图片地址，例如：
   `/upload/2026/07/xxxxxxxx.jpg`。
4. 回到文章编辑器，在 Markdown 中写：

```markdown
![图片说明](/upload/2026/07/xxxxxxxx.jpg)
```

目前单个附件最大 `1 MB`。图片支持 `jpg`、`jpeg`、`png`、`gif`、`webp`、`bmp`；
普通附件支持 `txt`、`md`、`pdf`、Office 文档和压缩包。

### 4. 创建普通页面

`页面`和`文章`的写法相同，也支持 Markdown。适合放关于我、项目说明等长期内容：

1. 点击顶部 `页面`。
2. 点击 `新页面`。
3. 填写标题、正文和 slug。
4. 选择 `发布` 或 `草稿`。
5. 保存后通过 `/<slug>` 访问。

当前导航栏中的 `关于` 默认指向 `/about`，因此可以创建或编辑 slug 为 `about` 的页面。

### 5. 管理评论、分类和友链

- `评论`：查看访客评论、回复或删除评论。
- `分类`：新增、重命名和删除分类。
- `标签`：查看文章使用的标签，标签通常在文章编辑页直接用逗号填写。
- `友链`：新增友链名称、URL、Logo/描述和排序。

访客阅读文章不需要登录，文章允许评论时可以直接在文章底部发表评论。评论接口有
CSRF 校验和频率限制，防止重复提交。

## 修改网站文字和主题

### 后台可以直接修改的内容

打开 `后台 -> 设置`，可以修改：

- `网站标题`：导航栏站名、浏览器标题和后台标题。
- `网站关键词`：页面 SEO 的 keywords。
- `网站描述`：页面 SEO 描述和页脚描述。
- `首页标语`：首页头图中央的文字。
- `首页头图`、`文章头图`、`其他页面头图`：Fluid 风格 Banner。
- `正文字体`：霞鹜文楷、系统无衬线或系统宋体。
- 微博、知乎、Github、Twitter 地址。

头图支持 `/user/img/...`、`/upload/...` 和完整的 `https://...` 地址。推荐使用至少
`1920×1080` 的横图，并给标题留出中央空白。

也可以直接替换：

```text
static/user/img/blog-banner.jpg
```

保持文件名不变即可继续使用默认配置。

### 需要改代码的固定文案

导航栏文字、页脚中的 `Powered by Go`、评论区提示、搜索弹窗标题等固定文案目前写在
Go 模板中。对应位置是：

- 公共导航和 Banner：`templates/theme/header.html`
- 首页文章列表：`templates/theme/index.html`
- 文章正文和目录：`templates/theme/post.html`
- 评论区：`templates/theme/comments.html`
- 页脚：`templates/theme/footer.html`
- 后台导航：`templates/admin/header.html`

修改模板后重启 `make run` 即可生效。文章正文里的文字则不需要改模板，直接在后台文章
编辑器中修改。

## 配置

配置全部通过环境变量注入：

| 环境变量 | 默认值 | 说明 |
| --- | --- | --- |
| `PORT` | `8081` | HTTP 端口 |
| `DB_DRIVER` | `sqlite` | `sqlite` 或 `mysql` |
| `DB_DSN` | `data/blog.db?...` | 数据库 DSN |
| `DB_MAX_OPEN_CONNS` | `100` | 最大连接数；SQLite 内部上限为 20 |
| `DB_MAX_IDLE_CONNS` | `20` | 最大空闲连接数；SQLite 内部上限为 10 |
| `DB_CONN_MAX_LIFETIME_MIN` | `30` | 连接最大生命周期（分钟） |
| `SESSION_SECRET` | 无默认值 | 至少 32 字节；启动前必须设置 |
| `COOKIE_SECURE` | 无默认值 | HTTPS 部署设为 `true`；本地 HTTP 开发设为 `false` |
| `ADMIN_USERNAME` | 无默认值 | 首次初始化管理员用户名 |
| `ADMIN_EMAIL` | 无默认值 | 首次初始化管理员邮箱 |
| `ADMIN_INITIAL_PASSWORD` | 无默认值 | 首次初始化管理员密码，不写入仓库 |
| `UPLOAD_DIR` | `data/upload` | 上传文件目录 |
| `HIT_FLUSH_EVERY` | `100` | 单文章点击异步落库阈值 |
| `RATE_LIMIT_RPS` | `200` | 单 IP 每秒请求数；`0` 表示关闭 |
| `RATE_LIMIT_BURST` | `400` | 单 IP 令牌桶突发容量 |
| `READ_TIMEOUT_SEC` | `15` | 请求读取超时 |
| `WRITE_TIMEOUT_SEC` | `30` | 响应写入超时 |
| `SHUTDOWN_TIMEOUT_SEC` | `10` | 优雅退出超时 |

示例：

```bash
SESSION_SECRET='replace-with-a-random-value-at-least-32-bytes' \
ADMIN_USERNAME='your-admin-name' \
ADMIN_EMAIL='you@example.com' \
ADMIN_INITIAL_PASSWORD='your-strong-password' \
PORT=8081 \
make run
```

### 更换前台头图和字体

登录后台后打开 `系统设置`，可以直接修改：

- `首页头图`、`文章头图`、`其他页面头图`
- `首页标语`
- `正文字体`

头图支持 `/user/img/...`、`/upload/...` 和完整的 `https://...` 地址。建议使用
至少 `1920×1080` 的横图，并选择主体靠两侧或下方、中央留白足够的图片，避免标题
盖住主体。

也可以直接替换本地文件 `static/user/img/blog-banner.jpg`；保持文件名不变时无需修改
任何配置。

## 使用 MySQL

数据表名、字段名与原 Java 版本的 `tale` 数据库保持兼容。已有 MySQL 数据库可以直接
通过 `DB_DRIVER=mysql` 和 `DB_DSN` 接入；如果需要全新导入，请使用你自己的数据库
备份或 SQL 导出文件，不要把数据库密码写进仓库。

再启动：

```bash
DB_DRIVER=mysql \
DB_DSN='your-db-user:your-db-password@tcp(host:3306)/tale?charset=utf8mb4&parseTime=true&loc=Local' \
SESSION_SECRET='replace-with-a-random-value-at-least-32-bytes' \
ADMIN_USERNAME='your-admin-name' \
ADMIN_EMAIL='you@example.com' \
ADMIN_INITIAL_PASSWORD='your-strong-password' \
make run
```

应用启动时会执行 GORM AutoMigrate，只补充必要索引，不改变原表名。

## Docker

```bash
docker compose up --build
```

默认持久化到 Docker volume `blog_data`。生产部署时请在部署主机的 `.env` 中设置
`SESSION_SECRET`、`COOKIE_SECURE`、`ADMIN_USERNAME`、`ADMIN_EMAIL` 和
`ADMIN_INITIAL_PASSWORD`，不要把这些值写进 `docker-compose.yml` 或 Git。

## 验证

```bash
make fmt
make test
go vet ./...
go test -race ./internal/cache ./internal/handler ./internal/middleware
```

本机压测环境（Apple Silicon，SQLite WAL，100 并发，10,000 请求）：

| 页面 | 错误 | 吞吐 | P95 | P99 |
| --- | ---: | ---: | ---: | ---: |
| 首页 `/` | 0 | 22,433 RPS | 8.8 ms | 14.1 ms |
| 文章 `/article/welcome` | 0 | 14,029 RPS | 18.3 ms | 33.6 ms |

文章页压测后优雅退出，数据库点击数核对为准确的 `10,000`，无丢计数。
该结果是本机单进程参考值，线上容量仍应按机器、MySQL/SQLite 和内容规模重新压测。

## 目录

```text
cmd/blog/          启动入口
config/            环境配置
internal/cache/    分片 TTL 缓存
internal/db/       SQLite/MySQL、迁移与种子数据
internal/model/    与 Java Vo/Bo 对应的数据模型
internal/service/  业务逻辑
internal/handler/  HTTP Handler、路由、页面渲染和响应
internal/middleware/ HTTP 中间件、Session、CSRF 和限流
templates/         Go html/template 页面
static/            前后台静态资源
```

