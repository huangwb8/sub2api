#!/usr/bin/env python3
"""Read-only collector for real sub2api admin operation data."""

from __future__ import annotations

import argparse
import datetime as dt
import json
import os
import sys
import urllib.error
import urllib.parse
import urllib.request
from pathlib import Path
from typing import Any


def load_config_section(config_path: Path, section: str) -> dict[str, Any]:
    if not config_path.exists():
        raise SystemExit(f"Missing config file: {config_path}")
    values: dict[str, Any] = {}
    in_section = False
    for raw_line in config_path.read_text(encoding="utf-8").splitlines():
        line = raw_line.rstrip()
        if line.startswith(f"{section}:"):
            in_section = True
            continue
        if in_section and line and not line.startswith(" "):
            break
        if not in_section or ":" not in line:
            continue
        key, value = [part.strip() for part in line.split(":", 1)]
        if not value:
            continue
        values[key] = value.strip('"')
    return values


def int_default(values: dict[str, Any], key: str) -> int:
    try:
        return int(str(values[key]))
    except (KeyError, ValueError) as exc:
        raise SystemExit(f"Invalid or missing defaults.{key} in config.yaml") from exc


def parse_args() -> argparse.Namespace:
    config_path = Path(__file__).resolve().parents[1] / "config.yaml"
    config_defaults = load_config_section(config_path, "defaults")
    config_directories = load_config_section(config_path, "directories")
    parser = argparse.ArgumentParser(description="Collect read-only sub2api admin operation data.")
    parser.add_argument("--base-url", default=os.getenv("SUB2API_BASE_URL", ""))
    parser.add_argument("--admin-api-key", default=os.getenv("SUB2API_ADMIN_API_KEY", ""))
    parser.add_argument("--auth-token", default=os.getenv("SUB2API_AUTH_TOKEN", ""))
    parser.add_argument(
        "--auth-mode",
        choices=("admin-api-key", "bearer"),
        default=os.getenv("SUB2API_AUTH_MODE", "admin-api-key"),
    )
    parser.add_argument("--since", default="")
    parser.add_argument("--until", default="")
    parser.add_argument("--timezone", default=os.getenv("SUB2API_TIMEZONE", str(config_defaults["timezone"])))
    parser.add_argument("--out-dir", required=True)
    parser.add_argument(
        "--work-root",
        default=os.getenv("SUB2API_SUMMARY_WORK_ROOT", str(config_directories["work_root"])),
    )
    parser.add_argument("--page-size", type=int, default=int_default(config_defaults, "page_size"))
    parser.add_argument("--user-limit", type=int, default=int_default(config_defaults, "user_limit"))
    parser.add_argument("--timeout", type=int, default=int_default(config_defaults, "request_timeout_seconds"))
    return parser.parse_args()


def parse_time(value: str, fallback: dt.datetime) -> dt.datetime:
    if not value:
        return fallback
    normalized = value.strip().replace("Z", "+00:00")
    try:
        parsed = dt.datetime.fromisoformat(normalized)
    except ValueError as exc:
        raise SystemExit(f"Invalid ISO time: {value}") from exc
    if parsed.tzinfo is None:
        parsed = parsed.replace(tzinfo=dt.timezone.utc)
    return parsed


def ensure_api_base(base_url: str) -> str:
    base = base_url.strip().rstrip("/")
    if not base:
        raise SystemExit("Missing required --base-url or SUB2API_BASE_URL.")
    if not (base.startswith("http://") or base.startswith("https://")):
        raise SystemExit("base-url must start with http:// or https://.")
    if not base.endswith("/api/v1"):
        base = base + "/api/v1"
    return base


def auth_headers(auth_mode: str, token: str) -> dict[str, str]:
    if auth_mode == "admin-api-key":
        return {"x-api-key": token}
    return {"Authorization": f"Bearer {token}"}


def request_headers(base_url: str, auth_mode: str, token: str) -> dict[str, str]:
    parsed = urllib.parse.urlparse(base_url)
    origin = f"{parsed.scheme}://{parsed.netloc}"
    headers = {
        "Accept": "application/json, text/plain, */*",
        "Accept-Language": "zh-CN,zh;q=0.9,en;q=0.8",
        "User-Agent": (
            "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) "
            "AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"
        ),
        "Origin": origin,
        "Referer": origin + "/",
    }
    headers.update(auth_headers(auth_mode, token))
    return headers


def request_json(
    api_base: str,
    headers: dict[str, str],
    path: str,
    params: dict[str, Any],
    timeout: int,
) -> Any:
    query = urllib.parse.urlencode({k: v for k, v in params.items() if v is not None})
    url = f"{api_base}{path}"
    if query:
        url = f"{url}?{query}"
    req = urllib.request.Request(url, method="GET")
    req.add_header("Accept", "application/json")
    for key, value in headers.items():
        req.add_header(key, value)
    with urllib.request.urlopen(req, timeout=timeout) as resp:
        body = resp.read().decode("utf-8")
    payload = json.loads(body)
    if isinstance(payload, dict) and "code" in payload:
        if payload.get("code") != 0:
            raise RuntimeError(f"API error code={payload.get('code')} message={payload.get('message')}")
        return payload.get("data")
    return payload


def write_json(path: Path, payload: Any) -> None:
    path.write_text(json.dumps(payload, ensure_ascii=False, indent=2, sort_keys=True), encoding="utf-8")


def safe_error(exc: BaseException) -> dict[str, str]:
    if isinstance(exc, urllib.error.HTTPError):
        return {"type": "HTTPError", "message": f"HTTP {exc.code}: {exc.reason}"}
    if isinstance(exc, urllib.error.URLError):
        return {"type": "URLError", "message": str(exc.reason)}
    return {"type": exc.__class__.__name__, "message": str(exc)}


def collect() -> int:
    args = parse_args()
    token = (args.auth_token or args.admin_api_key).strip()
    if not token:
        print(
            "Missing required credential: set SUB2API_ADMIN_API_KEY, SUB2API_AUTH_TOKEN, "
            "or pass --admin-api-key/--auth-token.",
            file=sys.stderr,
        )
        return 2

    api_base = ensure_api_base(args.base_url)
    headers = request_headers(args.base_url, args.auth_mode, token)
    out_dir = Path(args.out_dir).resolve()
    work_root = Path(args.work_root).resolve()
    try:
        out_dir.relative_to(work_root)
    except ValueError:
        print(f"--out-dir must be inside work root: {work_root}", file=sys.stderr)
        return 2
    out_dir.mkdir(parents=True, exist_ok=True)

    config_defaults = load_config_section(Path(__file__).resolve().parents[1] / "config.yaml", "defaults")
    now = dt.datetime.now(dt.timezone.utc)
    until = parse_time(args.until, now)
    since = parse_time(args.since, until - dt.timedelta(hours=int_default(config_defaults, "lookback_hours")))
    if since >= until:
        print("--since must be earlier than --until.", file=sys.stderr)
        return 2

    start_date = since.date().isoformat()
    end_date = until.date().isoformat()
    common = {"start_date": start_date, "end_date": end_date, "timezone": args.timezone}
    parsed = urllib.parse.urlparse(api_base)
    metadata = {
        "base_url": f"{parsed.scheme}://{parsed.netloc}",
        "api_base": f"{parsed.scheme}://{parsed.netloc}/api/v1",
        "requested_since": since.isoformat(),
        "requested_until": until.isoformat(),
        "api_start_date": start_date,
        "api_end_date": end_date,
        "api_date_range_note": "sub2api admin summary endpoints accept calendar dates; this is a date-bucket superset of the requested ISO range.",
        "timezone": args.timezone,
        "collected_at": dt.datetime.now(dt.timezone.utc).isoformat(),
        "read_only": True,
        "auth": {
            "mode": args.auth_mode,
            "header": "x-api-key" if args.auth_mode == "admin-api-key" else "Authorization",
            "stored": False,
        },
    }
    write_json(out_dir / "_metadata.json", metadata)

    endpoints: list[tuple[str, str, dict[str, Any], bool]] = [
        (
            "dashboard_snapshot_v2.json",
            "/admin/dashboard/snapshot-v2",
            {
                **common,
                "granularity": "hour",
                "include_stats": "true",
                "include_trend": "true",
                "include_model_stats": "true",
                "include_group_stats": "true",
                "include_users_trend": "true",
                "users_trend_limit": args.user_limit,
            },
            False,
        ),
        ("usage_stats.json", "/admin/usage/stats", common, True),
        ("models.json", "/admin/dashboard/models", common, False),
        ("groups.json", "/admin/dashboard/groups", common, False),
        ("users_ranking.json", "/admin/dashboard/users-ranking", {**common, "limit": args.user_limit}, False),
        ("user_breakdown.json", "/admin/dashboard/user-breakdown", {**common, "limit": args.user_limit}, False),
        ("profitability.json", "/admin/dashboard/profitability", {**common, "granularity": "hour"}, False),
        ("recommendations.json", "/admin/dashboard/recommendations", {}, False),
        (
            "usage_sample.json",
            "/admin/usage",
            {**common, "page": 1, "page_size": args.page_size, "sort_by": "created_at", "sort_order": "desc"},
            False,
        ),
        ("ops_snapshot_v2.json", "/admin/ops/dashboard/snapshot-v2", {}, False),
    ]

    errors: dict[str, Any] = {}
    for filename, path, params, required in endpoints:
        try:
            write_json(out_dir / filename, request_json(api_base, headers, path, params, args.timeout))
        except Exception as exc:  # noqa: BLE001 - preserve all endpoint failures in data/_errors.json.
            errors[filename] = safe_error(exc)
            if required:
                write_json(out_dir / "_errors.json", errors)
                print(f"Required endpoint failed: {filename}: {errors[filename]['message']}", file=sys.stderr)
                return 1

    metadata["finished_at"] = dt.datetime.now(dt.timezone.utc).isoformat()
    write_json(out_dir / "_metadata.json", metadata)
    write_json(out_dir / "_errors.json", errors)
    print(f"Collected sub2api data into {out_dir}")
    return 0


if __name__ == "__main__":
    raise SystemExit(collect())
