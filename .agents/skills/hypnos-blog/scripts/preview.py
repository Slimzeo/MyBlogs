#!/usr/bin/env python3
"""Run the personal blog in the foreground on loopback, using only local preview data."""
import argparse
import os
from pathlib import Path
import secrets
import shutil
import socket
import sys

def local_secret(directory, filename):
    path = directory / filename
    try:
        with path.open("x") as file:
            file.write(secrets.token_hex(32) + "\n")
    except FileExistsError:
        pass
    value = path.read_text().strip()
    if len(value) < 32:
        raise SystemExit(f"Invalid local preview secret file: {path}")
    return value

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--repo", type=Path, default=Path(__file__).resolve().parents[4],
                        help="MyBlogs checkout (defaults to the one containing this skill)")
    parser.add_argument("--demo", action="store_true", help="Use a separate local demo database")
    parser.add_argument("--fallback", action="store_true", help="Use the Go homepage without the native scene")
    parser.add_argument("--port", type=int)
    args = parser.parse_args()
    repo = args.repo.resolve()
    if not (repo / "cmd/blog/main.go").is_file():
        parser.error(f"Not the MyBlogs source checkout: {repo}")
    port = args.port if args.port is not None else (8099 if args.demo else 8098)
    if not 1 <= port <= 65535:
        parser.error("Invalid port")
    folder = repo / "data" / ("preview-demo" if args.demo else "preview")
    db = folder / "blog.db"
    if not args.demo and not db.is_file():
        parser.error(f"Local preview DB missing: {db}. Use --demo for an independent seeded preview.")
    go = shutil.which("go")
    if not go:
        parser.error("Go is not installed or missing from PATH")
    with socket.socket() as sock:
        try:
            sock.bind(("127.0.0.1", port))
        except OSError as error:
            parser.exit(1, f"Cannot bind 127.0.0.1:{port}: {error}. Inspect the listener before restarting.\n")
    os.umask(0o077)
    folder.mkdir(mode=0o700, parents=True, exist_ok=True)
    env = {key: value for key, value in os.environ.items() if not key.startswith("BLOG_")}
    env.update(BLOG_PORT=str(port), BLOG_BIND_ADDRESS="127.0.0.1", BLOG_DB_DRIVER="sqlite",
               BLOG_DB_DSN=str(db) + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)",
               BLOG_UPLOAD_DIR=str(folder / "upload"), BLOG_SESSION_SECRET=local_secret(folder, ".session-key"),
               BLOG_COOKIE_SECURE="false", BLOG_GIN_MODE="release")
    if args.demo:
        env.update(BLOG_ADMIN_USERNAME="preview", BLOG_ADMIN_EMAIL="preview@example.invalid",
                   BLOG_ADMIN_INITIAL_PASSWORD=local_secret(folder, ".demo-admin-password"))
    if args.fallback:
        env["BLOG_REFERENCE_DIR"] = str(folder / "without-reference")
    os.chdir(repo)
    print(f"{'Demo' if args.demo else 'Local'} preview: http://127.0.0.1:{port}/  database: {db}", flush=True)
    os.execve(go, [go, "run", "./cmd/blog"], env)

if __name__ == "__main__":
    main()
