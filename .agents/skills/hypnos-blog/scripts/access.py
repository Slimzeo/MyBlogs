#!/usr/bin/env python3
"""Read-only checks for the public site and its production origin."""
import argparse
from concurrent.futures import ThreadPoolExecutor
import os
import shutil
import subprocess
from urllib.parse import urlsplit

HOSTS = ("hypn0s.cloud", "www.hypn0s.cloud")
SSH = ["ssh", "-nT", "-o", "BatchMode=yes", "-o", "StrictHostKeyChecking=yes", "-o", "ConnectTimeout=12",
       "--", os.environ.get("HYPNOS_BLOG_SSH_HOST") or "docrag"]

def public_check(url, proxy):
    route = ["--noproxy", "", "--proxy", proxy] if proxy else ["--noproxy", "*"]
    res = subprocess.run(["curl", "-sSL", "--proto", "=http,https", "--proto-redir",
                          "=https" if url.startswith("https:") else "=http,https",
                          "--max-redirs", "4", "--connect-timeout", "5", "--max-time", "15",
                          *route, "-w", "\n%{http_code} %{url_effective}", url], capture_output=True)
    body, _, metadata = res.stdout.rpartition(b"\n")
    status, _, final = metadata.decode(errors="replace").partition(" ")
    target = urlsplit(final)
    blog = b'id="blog-topbar"' in body and (b'id="blogs"' in body or b'HYPNOS' in body)
    ok = not res.returncode and status == "200" and target.hostname in HOSTS and blog
    detail = res.stderr.decode(errors="replace").strip() if res.returncode else f"{status} {final} blog={blog}"
    return ok, f"{'OK' if ok else 'FAIL'} {url}  {detail}"

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest="mode", required=True)
    public = commands.add_parser("public", help="Probe HTTP and HTTPS for the apex and www")
    public.add_argument("--proxy", help="Use this local HTTP proxy explicitly, e.g. http://127.0.0.1:7897")
    commands.add_parser("server", help="Read service state, active release, origin and listeners over SSH")
    args = parser.parse_args()
    if args.mode == "server":
        remote = """
set -eu
runtime=$(systemctl show myblogs.service -p WorkingDirectory --value)
test -n "$runtime"
cd -- "$runtime"
systemctl show myblogs.service -p ActiveState -p MainPID -p Restart -p WorkingDirectory
pid=$(systemctl show myblogs.service -p MainPID --value)
if [ "$pid" -gt 1 ]; then readlink -f "/proc/$pid/exe"; fi
for item in blog templates static refs_blog; do readlink -f "$item"; done
ready=0
curl -fsS --max-time 5 http://127.0.0.1:8081/readyz || ready=$?
printf '\\n'
ss -ltn '( sport = :80 or sport = :443 or sport = :8081 )'
exit "$ready"
"""
        return subprocess.run([*SSH, remote]).returncode
    if not shutil.which("curl"):
        parser.error("curl is required")
    if args.proxy and (urlsplit(args.proxy).hostname not in ("127.0.0.1", "localhost", "::1") or urlsplit(args.proxy).username):
        parser.error("--proxy expects a local proxy address without credentials")
    urls = [f"{scheme}://{host}/" for scheme in ("http", "https") for host in HOSTS]
    with ThreadPoolExecutor(max_workers=len(urls)) as pool:
        results = list(pool.map(lambda url: public_check(url, args.proxy), urls))
    for _, message in results:
        print(message)
    return int(not all(ok for ok, _ in results))

if __name__ == "__main__":
    raise SystemExit(main())
