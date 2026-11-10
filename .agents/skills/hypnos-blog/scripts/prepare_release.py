#!/usr/bin/env python3
"""Build the three-file CPU release kit locally. This command never uploads or activates it."""
import argparse
import ast
import gzip
import hashlib
import io
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import tarfile
import tempfile
import zipfile

REF = Path("refs_blog/payathon-2026/blog")
SSH = ["ssh", "-nT", "-o", "BatchMode=yes", "-o", "StrictHostKeyChecking=yes", "-o", "ConnectTimeout=12",
       "--", os.environ.get("HYPNOS_BLOG_SSH_HOST") or "docrag"]
LIVE_RELEASE = r"""
set -eu
runtime=$(systemctl show myblogs.service -p WorkingDirectory --value)
test -n "$runtime"
cd -- "$runtime"
pid=$(systemctl show myblogs.service -p MainPID --value)
live=$(readlink -f "/proc/$pid/exe")
test "$live" = "$(readlink -f blog)"
dir=$(dirname "$live")
test "$(dirname "$dir")" = "$(dirname "$runtime")/MyBlogs-releases"
"""

def digest(path):
    h = hashlib.sha256()
    with path.open("rb") as file:
        for block in iter(lambda: file.read(1024 * 1024), b""):
            h.update(block)
    return h.hexdigest()

def baseline(path):
    if path:
        return None, path.read_text()
    read_helper = r'''printf '%s\n' "$(basename "$dir")"; cat "$dir/release-host.py"'''
    result = subprocess.check_output([*SSH, LIVE_RELEASE + read_helper], text=True)
    folder, code = result.split("\n", 1)
    if not re.fullmatch(r"[0-9a-f]{12}", folder):
        raise ValueError("Unexpected production release layout")
    return folder, code

def reference_zip(explicit, repo, tmp, live):
    local = explicit.expanduser() if explicit else repo / "payathon-2026.zip"
    if local.is_file():
        return local
    if explicit or not live:
        raise ValueError(f"Missing reference ZIP at {local}. Supply --scene-zip or HYPNOS_BLOG_SCENE_ZIP.")
    # An authorized online build can reuse the pinned mirror from the same live release.
    archive = tmp / "scene.zip"
    read_zip = f'test "$(basename "$dir")" = "{live}"\ncat "$dir/scene.zip"'
    with archive.open("xb") as out:
        subprocess.run([*SSH, LIVE_RELEASE + read_zip], stdout=out, check=True)
    return archive

def bind_host_helper(code, live, rev, archive_hash):
    # Preserve the host's tested stage/activation/rollback protocol. Change only the three pins.
    previous = re.findall(r"(?m)^REV = '([0-9a-f]{40})'$", code)
    if len(previous) != 1 or (live and previous[0][:12] != live):
        raise ValueError("The live helper revision differs from its executable; inspect the release.")
    for name, length, value in [("REV", 40, rev), ("PREVIOUS", 12, previous[0][:12]), ("PACKAGE_HASH", 64, archive_hash)]:
        expression = rf"(?m)^{name} = '[0-9a-f]{{{length}}}'$"
        code, count = re.subn(expression, f"{name} = '{value}'", code)
        if count != 1:
            raise ValueError(f"The host helper format has changed at {name}; review it before packaging.")
    ast.parse(code)
    return code

def build(args):
    repo, out = args.repo.expanduser().resolve(), args.out.expanduser().resolve()
    if out.exists() or not out.parent.is_dir():
        raise ValueError("--out must name a new directory under an existing parent")
    rev = subprocess.check_output(["git", "-C", str(repo), "rev-parse", "--verify", "--end-of-options",
                                   args.revision + "^{commit}"], text=True).strip()
    if not re.fullmatch(r"[0-9a-f]{40}", rev):
        raise ValueError("Cannot resolve a full Git commit")
    live, helper = baseline(args.baseline)
    with tempfile.TemporaryDirectory(prefix=".blog-build-", dir=out.parent) as name:
        tmp = Path(name)
        source, bundle = tmp / "source", tmp / "bundle"
        bundle.mkdir(mode=0o700)
        subprocess.run(["git", "clone", "--quiet", "--no-checkout", "--local", str(repo), str(source)], check=True)
        subprocess.run(["git", "-C", str(source), "checkout", "--quiet", "--detach", rev], check=True)
        binary = tmp / "blog"
        subprocess.run(["go", "build", "-mod=readonly", "-trimpath", "-buildvcs=true", "-ldflags=-s -w",
                        "-o", str(binary), "./cmd/blog"], cwd=source, check=True,
                       env=dict(os.environ, GOOS="linux", GOARCH="amd64", CGO_ENABLED="0", GOWORK="off"))
        info = subprocess.check_output(["go", "version", "-m", str(binary)], text=True)
        if not all(value in info for value in (f"vcs.revision={rev}", "vcs.modified=false", "GOOS=linux", "GOARCH=amd64")):
            raise ValueError("Binary build metadata does not match the requested clean commit")
        archive = tmp / "source.tar.gz"
        subprocess.run(["git", "archive", "--format=tar.gz", "-o", str(archive), rev], cwd=source, check=True)
        pin, = re.findall(r"\b([0-9a-f]{64})\s+payathon-2026\.zip", (source / REF / "NOTES.md").read_text())
        scene = reference_zip(args.scene_zip, repo, tmp, live)
        if digest(scene) != pin:
            raise ValueError("Reference ZIP differs from the pin in this commit's NOTES.md")
        with zipfile.ZipFile(scene) as z:
            for entry in z.infolist():
                if not entry.filename.startswith("payathon-2026/dist/") or entry.is_dir():
                    continue
                folder = source / "refs_blog"
                dest = (folder / entry.filename).resolve()
                if not dest.is_relative_to(folder / "payathon-2026/dist"):
                    raise ValueError("Escaping reference ZIP path")
                dest.parent.mkdir(parents=True, exist_ok=True)
                with z.open(entry) as src, dest.open("xb") as dst:
                    shutil.copyfileobj(src, dst)
        subprocess.run(["python3", "-I", str(REF / "transform.py")], cwd=source, check=True)
        files = [REF / n for n in ("index.html", "journal.html", "assets/brand.svg", "assets/agent-topbar.js")]
        files += [p.relative_to(source) for p in sorted((source / REF / "assets/generated").rglob("*")) if p.is_file()]
        manifest = {"revision": rev, "source": digest(archive), "scene_zip": pin, "binary": digest(binary),
                    "css": digest(source / "static/user/css/topbar.css"), "generated": {str(p): digest(source / p) for p in files}}
        with binary.open("rb") as src, gzip.GzipFile(str(tmp / "blog.gz"), "wb", compresslevel=6, mtime=0) as dst:
            shutil.copyfileobj(src, dst)
        manifest_bytes = (json.dumps(manifest, indent=2) + "\n").encode()
        with tarfile.open(bundle / "release.tar", "w") as pkg:
            entry = tarfile.TarInfo("release.json")
            entry.size = len(manifest_bytes)
            pkg.addfile(entry, io.BytesIO(manifest_bytes))
            for filename in ("source.tar.gz", "blog.gz"):
                pkg.add(tmp / filename, arcname=filename)
        bound = bind_host_helper(helper, live, rev, digest(bundle / "release.tar"))
        (bundle / "release-host.py").write_text(bound)
        (bundle / "revision").write_text(rev + "\n")
        if out.exists():
            raise ValueError(f"Output directory appeared during build: {out}")
        bundle.rename(out)
    print(f"Prepared {rev[:12]} in {out}. Review smoke() against the requested change before staging.")

if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--out", required=True, type=Path, help="New directory for the local release kit")
    parser.add_argument("--repo", type=Path, default=Path(__file__).resolve().parents[4],
                        help="MyBlogs checkout (defaults to the one containing this skill)")
    parser.add_argument("--revision", default="HEAD")
    parser.add_argument("--scene-zip", type=Path, default=os.getenv("HYPNOS_BLOG_SCENE_ZIP") or None,
                        help="Private ZIP (default: repository root, then active SSH release for online builds)")
    parser.add_argument("--baseline", type=Path, help="Previously retrieved, reviewed host helper (offline packaging)")
    try:
        build(parser.parse_args())
    except (OSError, ValueError, subprocess.CalledProcessError) as error:
        parser.exit(1, f"Could not prepare release: {error}\n")
