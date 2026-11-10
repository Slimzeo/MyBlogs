---
name: hypnos-blog
description: 在 Slimzeo/MyBlogs 仓库中访问和维护 Hypnos 的个人博客 HYPNOS Cloud（hypn0s.cloud），用于本地预览、日记与读书笔记导入、GitHub 推送、CPU 服务器部署及公网访问排障。
---

# HYPNOS 个人博客

从当前 `Slimzeo/MyBlogs` 工作树操作，默认发布分支是 `master`。域名 `hypn0s.cloud` 中间是数字 **0**，站名文字是 **HYPNOS Cloud**。

在仓库的任意子目录执行下面的命令，再按本次请求选择操作。`S` 是当前分支中的 skill 目录。

```sh
REPO=$(git rev-parse --show-toplevel)
cd "$REPO"
S="$REPO/.agents/skills/hypnos-blog"
git status --short --branch
```

| 用户要做什么 | 从哪里开始 |
| --- | --- |
| 启动或微调博客 | 复用已有本地数据用 `python3 "$S/scripts/preview.py"`（8098）。全新克隆用 `python3 "$S/scripts/preview.py" --demo`（8099）。页面、源码和动效生成入口见[开发与预览](references/development.md)。 |
| 写日记、导入读书笔记或管理文章 | 用 `blogctl` 本地校验，按要求导入草稿或进入后台管理。见[文章操作](references/content.md)。 |
| 看正式站、登录后台、排查打不开 | `python3 "$S/scripts/access.py" public` 分别检查两个域名的 HTTP 和 HTTPS。`server` 检查生产源站。默认浏览器及临时 SSH 入口见[访问与排障](references/access.md)。 |
| 推 GitHub、上线 CPU、回滚 | 根据本次请求核对本地提交、远端分支或线上运行版本。见[发布与恢复](references/release.md)。 |

本机的 8098、8099 都是开发预览。已核实指向 CPU 服务器的 18081 隧道访问的是**生产数据**。使用该隧道导入文章或操作后台会更改正式站。
