# 推送、部署与恢复

正式站由 `myblogs.service` 管理，GitHub 仓库是 `Slimzeo/MyBlogs`。远端命令使用 `HYPNOS_BLOG_SSH_HOST` 指定的 SSH 别名，维护机默认 `docrag`。一次 Git 推送不会改变线上进程。用户只要求 push 时做到远端分支核验；请求上线时才切换正式服务。

## 看清当前版本

在当前仓库根目录执行：

```sh
git status --short --branch
git remote -v
git ls-remote --heads origin master
python3 "$S/scripts/access.py" server
```

服务器运行目录以 `systemctl show myblogs.service -p WorkingDirectory --value` 的实测值为准。其中 `blog`、`templates`、`static`、`refs_blog` 是链接，分别指向 `MyBlogs-releases/<12位提交号>/blog` 和同一发布的 `source/` 下的三个目录。`.env`、`data/blog.db`、`data/upload/` 留在运行目录。运行目录旁的 `MyBlogs` 是曾有本地改动和分叉历史的旧 checkout。使用独立发布目录，保留旧 checkout 的工作树。实时版本认 `systemctl show myblogs.service -p MainPID` 对应的 `/proc/PID/exe`。

## 从已审查的提交准备发布包

检查改动、运行相关测试。涉及公共路由或模板时可运行 `go test ./...`。按用户指定的目标分支提交和推送，只纳入这次源码改动。核对 `origin/master` 已指向目标提交。原始页面镜像、生成资源、数据库、上传文件和凭据不进入公开 Git。

```sh
WORK=$(mktemp -d)
BUNDLE="$WORK/release"
python3 "$S/scripts/prepare_release.py" --revision HEAD --out "$BUNDLE"
REV=$(cat "$BUNDLE/revision")
SHORT=$(cut -c1-12 "$BUNDLE/revision")
BLOG_SSH="${HYPNOS_BLOG_SSH_HOST:-docrag}"
RUNTIME=$(ssh -nT -o BatchMode=yes -o StrictHostKeyChecking=yes "$BLOG_SSH" systemctl show myblogs.service -p WorkingDirectory --value)
REMOTE="$(dirname "$RUNTIME")/MyBlogs-releases/$SHORT"
git ls-remote --heads origin master
```

准备脚本只在本机写包。它从当前运行版本只读取 `release-host.py`，保留已经在线用过的 staging、备份和回滚逻辑，替换目标提交、前一版和包摘要。它在独立 checkout 交叉编译 Linux amd64，核对二进制内嵌的 Git 提交，并用该提交钉住的参考 ZIP 重新生成和校验动效。产物为 `release.tar`、`release-host.py`、`revision`。`--baseline FILE` 可使用已核实的旧 helper 离线打包。参考包默认取仓库根目录下 Git 忽略的 `payathon-2026.zip`。在线打包若缺少该文件，会通过同一条已授权 SSH 连接读取刚核实的运行版本的 `scene.zip`，按目标提交里的 SHA-256 校验。离线打包可传 `--scene-zip /path/to/payathon-2026.zip` 或设置 `HYPNOS_BLOG_SCENE_ZIP`。若主机结构或 helper 格式变化，先读新结构再适配。

在 `"$BUNDLE/release-host.py"` 中审阅 `smoke()` 与本次功能是否一致。旧断言可能包含已改的 CSS 版本或界面名称，按本次需求更新对应验收项。核对其 `PREVIOUS` 等于线上当前版本，`REV` 与 GitHub 目标相符。若目标已经在运行，直接核验线上行为即可。

## 在副本上试跑，再切换

下面在同一个 shell 中使用上面取得的 `REMOTE` 和 `BUNDLE`。新目录名取自完整提交号。先确保目标还未存在，再上传：

```sh
(
  set -eu
  test "$(git ls-remote --exit-code --heads https://github.com/Slimzeo/MyBlogs.git master | cut -f1)" = "$REV"
  ssh -nT -o BatchMode=yes -o StrictHostKeyChecking=yes "$BLOG_SSH" mkdir -m 700 -- "$REMOTE"
  scp -q -o BatchMode=yes -o StrictHostKeyChecking=yes "$BUNDLE/release.tar" "$BUNDLE/release-host.py" "$BLOG_SSH:$REMOTE/"
  ssh -nT -o BatchMode=yes -o StrictHostKeyChecking=yes "$BLOG_SSH" python3 -I "$REMOTE/release-host.py" stage
)
```

逐条检查退出状态，尤其是 `mkdir` 成功后才运行 `scp`。若同名目录已存在，先读其 `prepared.json`、`stage-ok.json`、`active.json` 及实际进程，确认是相同发布再决定是否恢复中断步骤。不要覆盖正在运行或来源不明的目录。`stage` 会核对包摘要、复用上一版的同一份已校验 `scene.zip`，用 SQLite 的备份接口在服务器本机创建副本，独立启动并检查路由、文章可见性和资源，试跑完清理副本。`stage` 失败先解决具体原因，不执行激活。

```sh
ssh -nT -o BatchMode=yes -o StrictHostKeyChecking=yes "$BLOG_SSH" python3 -I "$REMOTE/release-host.py" activate
python3 "$S/scripts/access.py" server
python3 "$S/scripts/access.py" public
node "$S/scripts/browser.cjs" http://www.hypn0s.cloud/ https://hypn0s.cloud/
```

`activate` 使用发布锁，核验四个活动链接和服务 PID，备份 SQLite 并 quick_check，再切换程序与资源。在已核实的 `Restart=always` 配置下，向本服务的 MainPID 发送 SIGTERM，由 systemd 拉起新版本。完成后核验进程所指提交和域名入口，再走一次本次修改的用户路径（例如首页 → 归档 → 文章）。如果 HTTPS 在发布前已故障，报告“新版本已在源站运行，公网 HTTPS 仍故障”，不要把这两件事混成“上线正常”。

## 故障恢复

激活过程中健康检查失败，现有 helper 会把已交换的应用链接按逆序换回，并确认旧服务就绪。先读实际 PID 和运行链接，确认是否已经回滚，再决定下一步。正常激活后的备份位置见那一版 `active.json` 的 `backup` 字段，目录形如 `deploy-backups/before-<sha>-<UTC>/`；`previous/` 保存上一版的四个链接，`blog.db` 是切换前的一致性快照。若需人工回退程序，使用同一发布锁，验证当前和备选路径后恢复链接并重启本服务。恢复数据库是单独的数据恢复操作，不能用旧库覆盖发布后新写的日记。遇到公网问题先分清公网、Nginx 和 Go 源站，见[访问与排障](access.md)。
