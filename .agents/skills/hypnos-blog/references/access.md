# 公网访问、CPU 源站与临时入口

生产源站通过已有的 SSH 配置访问。维护机沿用别名 `docrag`；其他机器可用 `HYPNOS_BLOG_SSH_HOST` 指定获授权的 SSH 别名。主机密钥由 OpenSSH 校验，凭据和代理配置留在各自机器上。

```sh
python3 "$S/scripts/access.py" public
python3 "$S/scripts/access.py" server
node "$S/scripts/browser.cjs" http://www.hypn0s.cloud/ https://hypn0s.cloud/
```

- `public` 从这台机器直连两个域名，分别检查 HTTP、HTTPS、重定向和博客 HTML 特征。非零退出表示至少有一个组合失败。要复现系统代理路径，先用 `scutil --proxy` 确认当前代理，再向脚本传 `--proxy http://127.0.0.1:7897`（端口以实测为准）。
- `server` 只读检查 `myblogs.service`、正在运行的可执行文件、发布链接、回环就绪接口和监听端口。Nginx 域名配置在 `/etc/nginx/sites-enabled/hypn0s.cloud`。要把 Nginx 与 Go 的响应分开看，在服务器上比较带 `Host: www.hypn0s.cloud` 的 `http://127.0.0.1/readyz` 和 `http://127.0.0.1:8081/readyz`。
- `browser.cjs` 用本机 Chrome 的常规 HTTPS 升级和证书校验，输出实际访问地址。可传首页、归档或文章 URL。它优先使用本项目可解析的 `playwright-core` 或 `playwright`，并可复用 Codex 桌面随附的 Node 依赖。若没有该运行时，直接用普通浏览器按用户原来的方式访问。发布报告分别写明源站、公开 HTTP 和浏览器 HTTPS 的实测结果。

**临时读取线上内容。** 先检查 `lsof -nP -iTCP:18081 -sTCP:LISTEN`。若有 SSH 监听者，用 `ps -p PID -o command=` 核对转发目标和别名，再访问 `/readyz` 和首页。空闲时打开专用前台会话并让它保持运行：

```sh
ssh -nNT -o BatchMode=yes -o ExitOnForwardFailure=yes -o StrictHostKeyChecking=yes \
  -o ConnectTimeout=15 -o ServerAliveInterval=30 -o ServerAliveCountMax=3 \
  -L 127.0.0.1:18081:127.0.0.1:8081 "${HYPNOS_BLOG_SSH_HOST:-docrag}"
```

从另一个终端运行 `node "$S/scripts/browser.cjs" http://127.0.0.1:18081/`。此地址只在本机隧道存活期间有效。需要脱离工具终端时，用独立进程组启动同一条 SSH 命令（`subprocess.Popen(..., start_new_session=True)`，标准输入指向 `DEVNULL`，日志置于本机私有目录），等待回环就绪接口确实返回 `ready`。隧道只能证明生产源站可读。

**2026-11-10 核实的历史基线。** 两个域名的 HTTP 返回博客首页，默认 Chrome 尝试升级 HTTPS，公网 443 超时。Go 源站正常，Nginx 站点只监听 80。临时监听 443 时本机能连，公网直连和经代理仍超时，未能区分是云安全组还是主机防火墙。下次重新实测，尤其不要用关闭升级、忽略证书错误或 SSH 转发后的成功代替公网验收。改 Nginx、证书和安全组涉及不同权限，本次任务要求处理哪一层才操作哪一层。
