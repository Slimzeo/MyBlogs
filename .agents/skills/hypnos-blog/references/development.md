# 本地开发与预览

在当前仓库根目录运行。`scripts/preview.py` 使用现有 `data/preview/blog.db` 和 `data/preview/upload`，只监听回环地址。`--demo` 用单独的 `data/preview-demo/` 和 8099，首次由 Go 初始化示例数据。预览凭据留在相应的本地数据目录，不复制到 skill 或 Git。脚本以前台方式运行，保留工具进程或终端，然后另开命令确认结果。

```sh
python3 "$S/scripts/preview.py"
# 在另一终端
curl -fsS http://127.0.0.1:8098/readyz
node "$S/scripts/browser.cjs" http://127.0.0.1:8098/
```

端口已占用时先用 `lsof -nP -iTCP:8098 -sTCP:LISTEN` 确认监听者，再验证首页。只重启自己确认的预览进程。Go 模板在启动时解析，改模板或 Go 处理器后要重启。生成的首页 HTML 和静态资源从磁盘读取，改资源时核对实际 HTML 引用的版本或内容哈希。

| 要改的部位 | 源码入口（相对仓库根） |
| --- | --- |
| 森林开屏、≈ 波纹、数字生物和开场字标 | `refs_blog/payathon-2026/blog/transform.py` 与同目录 `assets/`。完整动效首页由 Go 读取生成的 `index.html`。 |
| 左下角代码 | 自我介绍的文字顺序在 `static/user/js/home.js` 的 `ABOUT`。两个首页共用 `static/user/js/home-code.js` 和 `static/user/css/journal.css` 进行语法高亮。 |
| 顶部导航与 chat bot | `templates/theme/topbar.html`、`static/user/css/topbar.css`、`static/user/js/topbar.js`。站名链接和 chat bot 是两个控件。改导航时只调整点名的元素。 |
| 首屏后的文章卡片、文章正文 | 卡片用 `templates/theme/journal.html` 和 `journal.css`；正文用 `templates/theme/post.html`、`static/user/css/reader.css` 和 `markdown.css`。Go 模板是本项目移植的 Fluid 前台。 |
| 归档、专栏、标签及搜索 | `internal/handler/blog_library.go`、`internal/handler/public.go`、`internal/service/`、`templates/theme/{blogs,archives,page-category}.html`、`static/user/css/library.css`。目录头图在 `templates/theme/header.html`。 |

首页开屏后直接按时间展示文章。`/blogs` 跳过开屏，`/archives` 按年列日期和标题，`/categories` 是单归属的文章专栏，`/tags` 跨专栏筛选。`/notes`、`/topics` 和旧的分类、标签书签有兼容处理。按已发表的真实内容展示专栏和数量。

**重新生成动效。** `refs_blog/payathon-2026/dist` 已存在时运行下面一行。新克隆默认可预览 Go 备用首页。要复现完整开屏，把你已有的私有参考包放到 Git 已忽略的 `$REPO/payathon-2026.zip`，按[仓库中的校验与解压说明](../../../../refs_blog/payathon-2026/blog/NOTES.md)只解压 `payathon-2026/dist/*` 到 `refs_blog/`。不要把 ZIP、原始镜像、生成页面或数据库提交到 Git，也不用从支付宝 CDN 热链。单独验证 Go 备用首页可加 `--fallback`。

```sh
python3 -I refs_blog/payathon-2026/blog/transform.py
```

按改动选择验收项。导航应在首页和内页一致。动效改动看正常动画和减少动态效果模式，配色改动看昼夜切换，小屏排版在手机宽度实看。Go 的现有集成测试需要本机回环端口。
