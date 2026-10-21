#!/usr/bin/env python3
"""下载英雄头像到本地数据目录（不进入仓库）。

数据源：DataDragon cdn/{library.version}/img/champion/{key}.png
输出：默认 data/champions/{key}.png

用法：
    python3 scripts/fetch-champion-icons.py
    python3 scripts/fetch-champion-icons.py --output /srv/lol/data/champions
    python3 scripts/fetch-champion-icons.py --force
"""

from __future__ import annotations

import argparse
import json
import time
import urllib.request
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
LIBRARY = ROOT / "internal" / "domain" / "champion" / "library.json"


def download(url: str, target: Path) -> None:
    last_error: Exception | None = None
    for attempt in range(3):
        try:
            with urllib.request.urlopen(url, timeout=30) as response:
                data = response.read()
            if len(data) < 1000:
                raise RuntimeError(f"下载内容过小：{len(data)} bytes")
            target.parent.mkdir(parents=True, exist_ok=True)
            tmp = target.with_suffix(".png.tmp")
            tmp.write_bytes(data)
            tmp.replace(target)
            return
        except Exception as error:  # noqa: BLE001 - 打印后重试
            last_error = error
            time.sleep(0.4 * (attempt + 1))
    raise RuntimeError(f"{url} 下载失败：{last_error}")


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--output", default=str(ROOT / "data" / "champions"))
    parser.add_argument("--force", action="store_true")
    args = parser.parse_args()

    output = Path(args.output)
    library = json.loads(LIBRARY.read_text(encoding="utf-8"))
    version = str(library.get("version") or "").strip()
    champions = library.get("champions") or []
    if not version or not champions:
        print("英雄资料库缺少 version 或 champions")
        return 1

    output.mkdir(parents=True, exist_ok=True)
    downloaded = skipped = 0
    failed: list[str] = []

    for champion in champions:
        key = str(champion.get("key") or "").strip()
        if not key:
            continue
        target = output / f"{key}.png"
        if target.exists() and target.stat().st_size > 1000 and not args.force:
            skipped += 1
            continue
        url = f"https://ddragon.leagueoflegends.com/cdn/{version}/img/champion/{key}.png"
        try:
            download(url, target)
            downloaded += 1
            print(f"[OK] {key}")
        except Exception as error:  # noqa: BLE001
            failed.append(key)
            print(f"[FAIL] {key}: {error}")

    print(f"完成：下载 {downloaded}，跳过 {skipped}，失败 {len(failed)}")
    print(f"输出目录：{output}")
    if failed:
        print("失败列表：" + ", ".join(failed))
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
