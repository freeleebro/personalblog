"""本机免 Docker 启动一个便携版 PostgreSQL（仅用于开发）。

服务器启动方式请使用根目录的单容器 `docker compose up -d`——它会在一个容器内启动 PostgreSQL、Go API
和 Next.js。这个脚本只用于本地开发，给「机器上装不了 / 起不来 Docker」
的情况兜底，让你依然能跑起完整的真实数据库，而不是退化成假数据。

用法::

    python scripts/dev-postgres.py            # 前台启动，Ctrl+C 停止
    python scripts/dev-postgres.py --init     # 只做初始化（下载 + 建库目录）

随后在另一个终端启动后端::

    cd backend
    DATABASE_URL='postgres://blog:blog@127.0.0.1:5433/beichuang?sslmode=disable' go run ./cmd/server

后端启动时会自动创建数据库并执行迁移；文章内容由管理后台维护。
"""

from __future__ import annotations

import argparse
import lzma
import os
import subprocess
import sys
import tarfile
import urllib.request
import zipfile
from io import BytesIO
from pathlib import Path

# 便携版二进制来自 Maven Central 上的 zonky embedded-postgres 项目，
# 是 PostgreSQL 官方 Windows 构建的再打包，仅 23MB。
PG_VERSION = "17.11.0"
PG_URL = (
    "https://repo1.maven.org/maven2/io/zonky/test/postgres/"
    f"embedded-postgres-binaries-windows-amd64/{PG_VERSION}/"
    f"embedded-postgres-binaries-windows-amd64-{PG_VERSION}.jar"
)

ROOT = Path(__file__).resolve().parent.parent
DEVDB = ROOT / ".devdb"
JAR = DEVDB / "postgres-binaries.jar"
PG_HOME = DEVDB / "pg"
DATA = DEVDB / "data"

PORT = int(os.environ.get("POSTGRES_PORT", "5433"))
DB_USER = os.environ.get("POSTGRES_USER", "blog")
DB_NAME = os.environ.get("POSTGRES_DB", "beichuang")


def platform_dir() -> str:
    """不同平台对应不同的可执行文件目录布局。"""
    return "bin"


def exe(name: str) -> Path:
    suffix = ".exe" if os.name == "nt" else ""
    return PG_HOME / platform_dir() / f"{name}{suffix}"


def download(url: str, dest: Path) -> None:
    print(f"→ 下载 PostgreSQL {PG_VERSION}（约 23MB）…")
    dest.parent.mkdir(parents=True, exist_ok=True)
    tmp = dest.with_suffix(".part")

    def hook(block: int, size: int, total: int) -> None:
        if total > 0:
            pct = min(100, block * size * 100 // total)
            print(f"\r  {pct:3d}%", end="", flush=True)

    urllib.request.urlretrieve(url, tmp, reporthook=hook)
    print()
    tmp.replace(dest)


def extract() -> None:
    print("→ 解压二进制…")
    with zipfile.ZipFile(JAR) as zf:
        inner = next((n for n in zf.namelist() if n.endswith(".txz")), None)
        if inner is None:
            raise SystemExit("jar 内没有找到 .txz 载荷")
        payload = zf.read(inner)

    raw = lzma.decompress(payload)
    PG_HOME.mkdir(parents=True, exist_ok=True)
    with tarfile.open(fileobj=BytesIO(raw)) as tf:
        tf.extractall(PG_HOME, filter="fully_trusted")


def init() -> None:
    DEVDB.mkdir(parents=True, exist_ok=True)

    if not JAR.exists():
        download(PG_URL, JAR)
    if not PG_HOME.exists():
        extract()

    if (DATA / "PG_VERSION").exists():
        print("✓ 数据目录已存在，跳过 initdb")
        return

    print(f"→ 初始化数据目录（超级用户 {DB_USER}，本地 trust 认证）…")
    subprocess.run(
        [
            str(exe("initdb")),
            "-D", str(DATA),
            "-U", DB_USER,
            "--auth=trust",
            "--encoding=UTF8",
            "--locale=C",
        ],
        check=True,
    )
    print("✓ 初始化完成")


def run() -> None:
    print(f"→ PostgreSQL 监听 127.0.0.1:{PORT}，数据目录 {DATA}")
    print(f"  连接串：postgres://{DB_USER}:{DB_USER}@127.0.0.1:{PORT}/{DB_NAME}?sslmode=disable")
    print("  数据库由后端首次启动时自动创建；按 Ctrl+C 停止。\n")

    # 前台运行，方便用 Ctrl+C 直接停掉
    subprocess.run(
        [
            str(exe("postgres")),
            "-D", str(DATA),
            "-p", str(PORT),
            "-c", "listen_addresses=127.0.0.1",
        ],
        check=False,
    )


def main() -> int:
    parser = argparse.ArgumentParser(description="便携版 PostgreSQL 开发实例")
    parser.add_argument("--init", action="store_true", help="只做初始化，不启动服务")
    args = parser.parse_args()

    if os.name != "nt":
        print(
            "提示：这个脚本按 Windows 便携包编写。\n"
            "服务器上直接运行单容器 `docker compose up -d`。",
            file=sys.stderr,
        )

    init()
    if not args.init:
        run()
    return 0


if __name__ == "__main__":
    sys.exit(main())
