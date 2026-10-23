#!/usr/bin/env python3
"""更新英雄库与本地头像（DataDragon）。

用法：
    python3 scripts/update-champions.py --version 16.20.1
    make champions CHAMPION_VERSION=16.20.1

行为：
  1. 拉取 cdn/{version}/data/zh_CN/champion.json；
  2. 更新 internal/domain/champion/library.json 的版本、名称、称号；
  3. 新增英雄追加到库中（positions/aliases 留空，后续可手工补）；
  4. 下载头像到 data/champions/{key}.png（可用 --output 指定）。
"""

from __future__ import annotations

import argparse
import datetime as dt
import json
import os
import re
import time
import urllib.request
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
DEFAULT_LIBRARY = ROOT / "internal" / "domain" / "champion" / "library.json"
DEFAULT_OUTPUT = ROOT / "data" / "champions"
VERSION_RE = re.compile(r"^\d{1,3}\.\d{1,3}\.\d{1,3}$")


def fetch(url: str, timeout: int = 30) -> bytes:
    last_error: Exception | None = None
    for attempt in range(3):
        try:
            request = urllib.request.Request(url, headers={"User-Agent": "lol-civil-war/1.0"})
            with urllib.request.urlopen(request, timeout=timeout) as response:
                data = response.read()
            if len(data) < 100:
                raise RuntimeError(f"内容过小：{len(data)} bytes")
            return data
        except Exception as error:  # noqa: BLE001
            last_error = error
            time.sleep(0.5 * (attempt + 1))
    raise RuntimeError(f"{url} 请求失败：{last_error}")


def download_icon(url: str, target: Path) -> None:
    data = fetch(url)
    if len(data) < 1000:
        raise RuntimeError(f"头像内容过小：{len(data)} bytes")
    target.parent.mkdir(parents=True, exist_ok=True)
    tmp = target.with_suffix(".png.tmp")
    tmp.write_bytes(data)
    tmp.replace(target)


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--version", default="", help="DataDragon 版本，例如 16.20.1")
    parser.add_argument("--library", default=str(DEFAULT_LIBRARY))
    parser.add_argument("--output", default=str(DEFAULT_OUTPUT))
    parser.add_argument("--force", action="store_true", help="强制重下已有头像")
    parser.add_argument("--no-icons", action="store_true", help="只更新英雄列表，不下载头像")
    args = parser.parse_args()

    library_path = Path(args.library)
    library = json.loads(library_path.read_text(encoding="utf-8"))
    version = (args.version or os.environ.get("LCW_DDRAGON_VERSION") or library.get("version") or "").strip()
    if not VERSION_RE.match(version):
        print(f"版本号不合法：{version!r}（需要 x.y.z，例如 16.20.1）")
        return 1

    url = f"https://ddragon.leagueoflegends.com/cdn/{version}/data/zh_CN/champion.json"
    payload = json.loads(fetch(url))
    data = payload.get("data")
    if not isinstance(data, dict) or not data:
        print(f"DataDragon 没有返回英雄数据：{url}")
        return 1

    champions = library.get("champions") or []
    by_key = {str(item.get("key") or ""): item for item in champions}
    updated = added = 0
    new_champions: list[dict] = []
    for key, item in data.items():
        champion_key = str(item.get("id") or key)
        existing = by_key.get(champion_key)
        if existing is None:
            existing = {
                "key": champion_key,
                "en": str(item.get("id") or champion_key),
                # DataDragon 的 zh_CN 里 name/title 与英文语义相反：
                # name=称号（暗黑元首），title=英雄名（辛德拉）。
                "cn": str(item.get("title") or champion_key),
                "title": str(item.get("name") or ""),
                "positions": [],
                "aliases": [],
            }
            champions.append(existing)
            by_key[champion_key] = existing
            new_champions.append(existing)
            added += 1
            continue
        changed = False
        # en 在旧库里是人工维护的可读名（如 "Xin Zhao"），这里不要用 DataDragon 的 id 覆盖。
        for field, value in (("cn", item.get("title")), ("title", item.get("name"))):
            if value and existing.get(field) != value:
                existing[field] = value
                changed = True
        if changed:
            updated += 1

    # 不删除旧英雄：历史对局仍可能引用旧 key。
    sources = [value for value in (library.get("sources") or []) if not str(value).startswith("ddragon ")]
    sources.insert(0, f"ddragon {version} zh_CN champion.json")
    library["version"] = version
    library["updatedAt"] = dt.date.today().isoformat()
    library["sources"] = sources
    library["champions"] = champions
    library_path.write_text(json.dumps(library, ensure_ascii=False, indent=4) + "\n", encoding="utf-8")

    print(f"英雄库：{len(data)} 个 DataDragon 条目，更新 {updated}，新增 {added}，库内共 {len(champions)}")
    if new_champions:
        print("新增英雄（positions/aliases 为空，可手工补充）：")
        for item in new_champions:
            print(f"  {item['key']} / {item['cn']} / {item['title']}")

    if args.no_icons:
        print("已跳过头像下载")
        return 0

    output = Path(args.output)
    output.mkdir(parents=True, exist_ok=True)
    downloaded = skipped = 0
    failed: list[str] = []
    for item in champions:
        key = str(item.get("key") or "").strip()
        if not key:
            continue
        target = output / f"{key}.png"
        if target.exists() and target.stat().st_size > 1000 and not args.force:
            skipped += 1
            continue
        icon_url = f"https://ddragon.leagueoflegends.com/cdn/{version}/img/champion/{key}.png"
        try:
            download_icon(icon_url, target)
            downloaded += 1
        except Exception as error:  # noqa: BLE001
            failed.append(key)
            print(f"[FAIL] {key}: {error}")

    print(f"头像：下载 {downloaded}，跳过 {skipped}，失败 {len(failed)}，目录 {output}")
    if failed:
        print("失败列表：" + ", ".join(failed))
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
