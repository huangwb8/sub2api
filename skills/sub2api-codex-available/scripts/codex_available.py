#!/usr/bin/env python3
"""Diagnose whether a sub2api account can be used by local Codex clients.

The script is intentionally conservative: it redacts secrets in persisted
artifacts and only performs write fixes when explicitly requested.
"""

from __future__ import annotations

import argparse
import datetime as dt
import json
import os
import re
import shutil
import subprocess
import tempfile
import time
import urllib.error
import urllib.parse
import urllib.request
from pathlib import Path
from typing import Any


SECRET_PATTERNS = [
    re.compile(r"sk-[A-Za-z0-9_\-]{8,}"),
    re.compile(r"Bearer\s+[A-Za-z0-9_\-.=]+", re.IGNORECASE),
]


def now_iso() -> str:
    return dt.datetime.now(dt.timezone.utc).isoformat()


def normalize_site_base_url(base_url: str) -> str:
    """Return the sub2api site root, accepting either root or OpenAI /v1 URL."""
    trimmed = base_url.strip().rstrip("/")
    if trimmed.endswith("/v1"):
        return trimmed[:-3].rstrip("/")
    return trimmed


def openai_base_url(site_base_url: str) -> str:
    return normalize_site_base_url(site_base_url).rstrip("/") + "/v1"


def redact(value: Any) -> Any:
    if isinstance(value, dict):
        out = {}
        for key, item in value.items():
            lowered = str(key).lower()
            if lowered in {"api_key", "access_token", "refresh_token", "id_token", "authorization", "x-api-key"}:
                out[key] = "[REDACTED]"
            else:
                out[key] = redact(item)
        return out
    if isinstance(value, list):
        return [redact(item) for item in value]
    if isinstance(value, str):
        text = value
        for pattern in SECRET_PATTERNS:
            text = pattern.sub("[REDACTED]", text)
        return text
    return value


class JsonlLogger:
    def __init__(self, path: Path) -> None:
        self.path = path
        self.path.parent.mkdir(parents=True, exist_ok=True)

    def write(self, event: str, **fields: Any) -> None:
        record = {"ts": now_iso(), "event": event, **redact(fields)}
        with self.path.open("a", encoding="utf-8") as handle:
            handle.write(json.dumps(record, ensure_ascii=False, sort_keys=True) + "\n")


class Client:
    def __init__(self, base_url: str, admin_headers: dict[str, str], timeout: int, logger: JsonlLogger) -> None:
        self.base_url = base_url.rstrip("/")
        self.admin_headers = admin_headers
        self.timeout = timeout
        self.logger = logger

    def request(
        self,
        method: str,
        path: str,
        *,
        headers: dict[str, str] | None = None,
        body: Any = None,
        admin: bool = True,
    ) -> tuple[int, dict[str, str], bytes]:
        url = self.base_url + path
        req_headers = {
            "Accept": "application/json, text/event-stream;q=0.9, */*;q=0.8",
            "User-Agent": "Mozilla/5.0 sub2api-codex-available/0.1",
        }
        if admin:
            req_headers.update(self.admin_headers)
        if headers:
            req_headers.update(headers)

        data = None
        if body is not None:
            data = json.dumps(body).encode("utf-8")
            req_headers.setdefault("Content-Type", "application/json")

        started = time.time()
        req = urllib.request.Request(url, data=data, headers=req_headers, method=method)
        try:
            with urllib.request.urlopen(req, timeout=self.timeout) as resp:
                payload = resp.read()
                status = int(resp.status)
                resp_headers = dict(resp.headers.items())
        except urllib.error.HTTPError as exc:
            payload = exc.read()
            status = int(exc.code)
            resp_headers = dict(exc.headers.items())
        except urllib.error.URLError as exc:
            self.logger.write("http_error", method=method, path=path, error=str(exc))
            raise
        elapsed_ms = int((time.time() - started) * 1000)
        self.logger.write("http", method=method, path=path, status=status, elapsed_ms=elapsed_ms, bytes=len(payload))
        return status, resp_headers, payload

    def json(self, method: str, path: str, **kwargs: Any) -> Any:
        status, _, payload = self.request(method, path, **kwargs)
        if status >= 400:
            raise RuntimeError(f"{method} {path} failed with HTTP {status}: {payload[:500]!r}")
        if not payload:
            return None
        return json.loads(payload.decode("utf-8"))


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--account", action="append", required=True, help="Target account id/name. Repeatable.")
    parser.add_argument("--model", default=os.getenv("SUB2API_CODEX_MODEL", "gpt-5.1"))
    parser.add_argument("--prompt", default=os.getenv("SUB2API_CODEX_PROMPT", "Reply with ok."))
    parser.add_argument("--out-dir", required=True)
    parser.add_argument("--timeout", type=int, default=int(os.getenv("SUB2API_TIMEOUT_SECONDS", "45")))
    parser.add_argument("--usage-poll-seconds", type=int, default=int(os.getenv("SUB2API_USAGE_POLL_SECONDS", "10")))
    parser.add_argument("--usage-poll-interval", type=int, default=int(os.getenv("SUB2API_USAGE_POLL_INTERVAL_SECONDS", "2")))
    parser.add_argument("--apply-known-config-fixes", action="store_true")
    parser.add_argument("--run-codex-cli", action="store_true", help="Run a real local Codex CLI probe with an isolated HOME.")
    parser.add_argument("--codex-cli-timeout", type=int, default=int(os.getenv("SUB2API_CODEX_CLI_TIMEOUT_SECONDS", "90")))
    return parser.parse_args()


def require_auth() -> tuple[str, dict[str, str], str | None]:
    base_url = os.getenv("SUB2API_BASE_URL", "").strip()
    if not base_url:
        raise SystemExit("SUB2API_BASE_URL is required")
    base_url = normalize_site_base_url(base_url)
    mode = os.getenv("SUB2API_AUTH_MODE", "admin-api-key").strip().lower()
    api_key = os.getenv("SUB2API_ADMIN_API_KEY", "").strip()
    bearer = os.getenv("SUB2API_AUTH_TOKEN", "").strip()
    if mode == "bearer":
        if not bearer:
            raise SystemExit("SUB2API_AUTH_TOKEN is required when SUB2API_AUTH_MODE=bearer")
        headers = {"Authorization": f"Bearer {bearer}"}
    else:
        if not api_key:
            raise SystemExit("SUB2API_ADMIN_API_KEY is required")
        headers = {"x-api-key": api_key}
    return base_url, headers, os.getenv("SUB2API_CODEX_API_KEY", "").strip() or None


def write_json(path: Path, payload: Any) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(redact(payload), ensure_ascii=False, indent=2, sort_keys=True), encoding="utf-8")


def validate_out_dir(out_dir: str) -> Path:
    path = Path(out_dir)
    resolved = path.resolve()
    allowed_root = (Path.cwd() / "tmp" / "sub2api-codex-available").resolve()
    allow_outside = os.getenv("SUB2API_ALLOW_OUTSIDE_WORKDIR", "").strip().lower() in {"1", "true", "yes"}
    if not allow_outside and resolved != allowed_root and not resolved.is_relative_to(allowed_root):
        raise SystemExit(
            "--out-dir must be inside ./tmp/sub2api-codex-available "
            "(set SUB2API_ALLOW_OUTSIDE_WORKDIR=true only for controlled maintenance tests)"
        )
    return resolved


def usage_marker(item: dict[str, Any]) -> str:
    stable = {
        "id": item.get("id"),
        "created_at": item.get("created_at") or item.get("createdAt") or item.get("timestamp"),
        "request_id": item.get("request_id") or item.get("requestID"),
        "account_id": item.get("account_id") or item.get("accountID"),
        "model": item.get("model"),
        "requested_model": item.get("requested_model"),
        "inbound_endpoint": item.get("inbound_endpoint"),
        "upstream_endpoint": item.get("upstream_endpoint"),
    }
    if stable.get("id") is not None:
        return f"id:{stable['id']}"
    if stable.get("request_id"):
        return f"request:{stable['request_id']}"
    return "hash:" + json.dumps(stable, ensure_ascii=False, sort_keys=True)


def usage_markers(snapshot: dict[str, Any] | None) -> set[str]:
    if not snapshot:
        return set()
    items = snapshot.get("items") or []
    return {usage_marker(item) for item in items if isinstance(item, dict)}


def list_accounts(client: Client) -> list[dict[str, Any]]:
    items: list[dict[str, Any]] = []
    for page in range(1, 21):
        query = urllib.parse.urlencode({"platform": "openai", "page": page, "page_size": 200, "lite": "true"})
        data = client.json("GET", f"/api/v1/admin/accounts?{query}")
        batch = (((data or {}).get("data") or {}).get("items") or [])
        items.extend(batch)
        total = ((data or {}).get("data") or {}).get("total")
        if not batch or (isinstance(total, int) and len(items) >= total):
            break
    return items


def match_accounts(accounts: list[dict[str, Any]], selectors: list[str]) -> list[dict[str, Any]]:
    matched: list[dict[str, Any]] = []
    seen: set[int] = set()
    for selector in selectors:
        selector_norm = selector.strip().lower()
        candidates = []
        for account in accounts:
            account_id = str(account.get("id", "")).lower()
            name = str(account.get("name", "")).lower()
            if selector_norm == account_id or selector_norm == name or selector_norm in name:
                candidates.append(account)
        if len(candidates) != 1:
            raise SystemExit(f"account selector {selector!r} matched {len(candidates)} accounts; use exact id/name")
        account = candidates[0]
        account_id_int = int(account["id"])
        if account_id_int not in seen:
            seen.add(account_id_int)
            matched.append(account)
    return matched


def get_account_detail(client: Client, account_id: int) -> dict[str, Any]:
    data = client.json("GET", f"/api/v1/admin/accounts/{account_id}")
    return (data or {}).get("data") or {}


def summarize_account(account: dict[str, Any]) -> dict[str, Any]:
    credentials = account.get("credentials") or {}
    extra = account.get("extra") or {}
    groups = account.get("groups") or []
    return {
        "id": account.get("id"),
        "name": account.get("name"),
        "platform": account.get("platform"),
        "type": account.get("type"),
        "status": account.get("status"),
        "schedulable": account.get("schedulable"),
        "priority": account.get("priority"),
        "concurrency": account.get("concurrency"),
        "load_factor": account.get("load_factor"),
        "last_used_at": account.get("last_used_at"),
        "rate_limit_reset_at": account.get("rate_limit_reset_at"),
        "overload_until": account.get("overload_until"),
        "temp_unschedulable_until": account.get("temp_unschedulable_until"),
        "error_message": account.get("error_message"),
        "groups": [{"id": g.get("id"), "name": g.get("name")} for g in groups],
        "credentials": {
            "base_url": credentials.get("base_url"),
            "api_key_present": bool(credentials.get("api_key")),
            "model_capability_strategy": credentials.get("model_capability_strategy"),
            "model_mapping": credentials.get("model_mapping"),
        },
        "extra": {
            "chatapi_responses_enabled": extra.get("chatapi_responses_enabled"),
            "openai_ws_enabled": extra.get("openai_ws_enabled"),
            "responses_websockets_v2_enabled": extra.get("responses_websockets_v2_enabled"),
        },
    }


def account_preflight(summary: dict[str, Any], model: str) -> list[dict[str, str]]:
    issues: list[dict[str, str]] = []
    if summary.get("platform") != "openai":
        issues.append({"severity": "P0", "code": "platform_mismatch", "message": "Codex path requires OpenAI platform account."})
    if summary.get("status") != "active":
        issues.append({"severity": "P0", "code": "inactive", "message": "Account status is not active."})
    if not summary.get("schedulable"):
        issues.append({"severity": "P0", "code": "not_schedulable", "message": "Account schedulable is false."})
    if summary.get("rate_limit_reset_at"):
        issues.append({"severity": "P1", "code": "rate_limited", "message": "Account has a rate_limit_reset_at value; verify whether it is still in the future."})
    if summary.get("overload_until"):
        issues.append({"severity": "P1", "code": "overloaded", "message": "Account has overload_until; verify whether it is still in the future."})
    if summary.get("type") == "chatapi":
        if summary.get("extra", {}).get("chatapi_responses_enabled") is not True:
            issues.append({"severity": "P0", "code": "chatapi_not_responses_compatible", "message": "Local Codex normally uses /v1/responses; chatapi accounts need Responses compatibility or source-code conversion support."})
    strategy = summary.get("credentials", {}).get("model_capability_strategy")
    mapping = summary.get("credentials", {}).get("model_mapping")
    if strategy == "inherit_default":
        issues.append({"severity": "P1", "code": "inherit_default_may_filter_model", "message": f"Model {model!r} must be in the project's OpenAI default model list or scheduler may filter this account."})
    if mapping and isinstance(mapping, dict) and model not in mapping and "*" not in mapping:
        issues.append({"severity": "P1", "code": "mapping_may_filter_model", "message": f"Model {model!r} is not explicitly present in model_mapping."})
    return issues


def admin_account_test(client: Client, account_id: int, model: str, prompt: str) -> dict[str, Any]:
    status, _, payload = client.request(
        "POST",
        f"/api/v1/admin/accounts/{account_id}/test",
        body={"model_id": model, "prompt": prompt},
    )
    text = payload.decode("utf-8", errors="replace")
    return {
        "status": status,
        "success": status < 400 and '"type":"test_complete"' in text and '"success":true' in text,
        "events": redact(text[-4000:]),
    }


def codex_e2e(client: Client, codex_api_key: str, model: str, prompt: str) -> dict[str, Any]:
    body = {
        "model": model,
        "instructions": "You are a concise test assistant.",
        "input": prompt,
        "stream": False,
    }
    headers = {
        "Authorization": f"Bearer {codex_api_key}",
        "Content-Type": "application/json",
        "Accept": "application/json, text/event-stream;q=0.9",
        "User-Agent": "codex_cli_rs/0.104.0",
        "OpenAI-Beta": "responses=experimental",
    }
    status, resp_headers, payload = client.request("POST", "/v1/responses", headers=headers, body=body, admin=False)
    text = payload.decode("utf-8", errors="replace")
    return {
        "status": status,
        "success": 200 <= status < 300,
        "content_type": resp_headers.get("Content-Type") or resp_headers.get("content-type"),
        "body_tail": redact(text[-4000:]),
    }


def codex_exec_help(codex_bin: str) -> str:
    try:
        result = subprocess.run(
            [codex_bin, "exec", "--help"],
            text=True,
            capture_output=True,
            check=False,
            timeout=15,
        )
    except Exception as exc:  # noqa: BLE001
        return str(exc)
    return (result.stdout or "") + "\n" + (result.stderr or "")


def build_codex_exec_command(codex_bin: str, model: str, prompt: str, help_text: str) -> list[str]:
    cmd = [codex_bin, "exec", "--skip-git-repo-check"]
    if "--dangerously-bypass-approvals-and-sandbox" in help_text:
        cmd.append("--dangerously-bypass-approvals-and-sandbox")
    else:
        if "--sandbox" in help_text:
            cmd.extend(["--sandbox", "danger-full-access"])
        if "--ask-for-approval" in help_text:
            cmd.extend(["--ask-for-approval", "never"])
        elif "--approval-policy" in help_text:
            cmd.extend(["--approval-policy", "never"])
    if model:
        cmd.extend(["--model", model])
    cmd.append(prompt)
    return cmd


def run_codex_cli_probe(base_url: str, codex_api_key: str, model: str, prompt: str, timeout: int, logger: JsonlLogger) -> dict[str, Any]:
    codex_bin = shutil.which("codex")
    if not codex_bin:
        return {"success": False, "skipped": True, "reason": "codex binary not found in PATH"}

    help_text = codex_exec_help(codex_bin)
    with tempfile.TemporaryDirectory(prefix="sub2api-codex-home-") as home_dir:
        env = os.environ.copy()
        env["HOME"] = home_dir
        env["OPENAI_API_KEY"] = codex_api_key
        env["OPENAI_BASE_URL"] = openai_base_url(base_url)
        env["OPENAI_API_BASE"] = env["OPENAI_BASE_URL"]

        login = subprocess.run(
            [codex_bin, "login", "--with-api-key"],
            input=codex_api_key + "\n",
            text=True,
            capture_output=True,
            env=env,
            check=False,
            timeout=min(timeout, 30),
        )
        logger.write("codex_cli_login", exit_code=login.returncode, stderr_tail=(login.stderr or "")[-1000:])

        cmd = build_codex_exec_command(codex_bin, model, prompt, help_text)
        started = time.time()
        try:
            result = subprocess.run(
                cmd,
                text=True,
                capture_output=True,
                env=env,
                check=False,
                timeout=timeout,
            )
            timed_out = False
        except subprocess.TimeoutExpired as exc:
            elapsed_ms = int((time.time() - started) * 1000)
            logger.write("codex_cli_timeout", elapsed_ms=elapsed_ms, cmd=cmd)
            return {
                "success": False,
                "skipped": False,
                "timed_out": True,
                "exit_code": None,
                "stdout_tail": redact((exc.stdout or "")[-4000:] if isinstance(exc.stdout, str) else ""),
                "stderr_tail": redact((exc.stderr or "")[-4000:] if isinstance(exc.stderr, str) else ""),
                "elapsed_ms": elapsed_ms,
            }
        elapsed_ms = int((time.time() - started) * 1000)
        logger.write("codex_cli_exec", exit_code=result.returncode, elapsed_ms=elapsed_ms)
        return {
            "success": result.returncode == 0,
            "skipped": False,
            "timed_out": timed_out,
            "exit_code": result.returncode,
            "stdout_tail": redact((result.stdout or "")[-4000:]),
            "stderr_tail": redact((result.stderr or "")[-4000:]),
            "elapsed_ms": elapsed_ms,
            "cmd_shape": [part if part not in {prompt} else "[PROMPT]" for part in cmd],
        }


def latest_usage_for_account(client: Client, account_id: int) -> dict[str, Any]:
    query = urllib.parse.urlencode({"account_id": account_id, "page": 1, "page_size": 5})
    data = client.json("GET", f"/api/v1/admin/usage?{query}")
    items = (((data or {}).get("data") or {}).get("items") or [])
    return {"items": items, "total": ((data or {}).get("data") or {}).get("total")}


def poll_new_usage_for_account(
    client: Client,
    account_id: int,
    before_markers: set[str],
    seconds: int = 10,
    interval: int = 2,
) -> dict[str, Any]:
    deadline = time.time() + seconds
    latest: dict[str, Any] = {"items": [], "total": 0}
    new_items: list[dict[str, Any]] = []
    while time.time() <= deadline:
        latest = latest_usage_for_account(client, account_id)
        items = latest.get("items") or []
        new_items = [item for item in items if isinstance(item, dict) and usage_marker(item) not in before_markers]
        if new_items:
            break
        time.sleep(interval)
    return {"items": latest.get("items") or [], "new_items": new_items, "total": latest.get("total")}


def render_report(report: dict[str, Any]) -> str:
    lines = [
        "# sub2api Codex 可调用性诊断报告",
        "",
        "## 结论",
        f"- 站点：{report.get('base_url')}",
        f"- 测试模型：{report.get('model')}",
        f"- 用户侧 Codex API Key：{'已提供' if report.get('codex_api_key_present') else '未提供'}",
    ]
    results = report.get("results") or []
    for result in results:
        account = result.get("account") or {}
        label = f"{account.get('name')} (ID {account.get('id')})"
        proven = "是" if result.get("proven_codex_available") else "否"
        lines.append(f"- {label}：是否已证明本地 Codex 可调用：{proven}")

    lines.extend(["", "## 输入与安全", "- 敏感凭据仅从环境变量读取；写入文件前会做脱敏处理。"])
    if not report.get("codex_api_key_present"):
        lines.append("- 未提供用户侧 Codex API Key，因此无法证明真实用户链路，只能完成管理端预检和账号直连探测。")

    lines.extend(["", "## 测试过程"])
    for result in results:
        account = result.get("account") or {}
        account_id = account.get("id")
        lines.append(f"- 账号 {account.get('name')} (ID {account_id})：采集账号详情、可用性快照、临时不可调度状态、测试前 usage、管理端账号直连测试。")
        if result.get("codex_e2e") is not None:
            lines.append(f"- 账号 {account_id}：执行 Codex 风格 `/v1/responses` E2E 请求。")
        if result.get("codex_cli") is not None:
            lines.append(f"- 账号 {account_id}：执行真实 `codex login --with-api-key` 与 `codex exec` 强验证。")

    lines.extend(["", "## 关键日志"])
    lines.append("- 事件日志：`logs/events.jsonl`")
    lines.append("- 结构化诊断：`analysis/diagnosis.json`")
    for result in results:
        account = result.get("account") or {}
        account_id = account.get("id")
        lines.append(f"- 账号 {account_id} 摘要：`data/account_{account_id}.json`、`data/account_{account_id}_admin_test.json`")
        if result.get("codex_e2e") is not None:
            lines.append(f"- 账号 {account_id} HTTP E2E：`data/account_{account_id}_codex_e2e.json`、`data/account_{account_id}_usage_after_http.json`")
        if result.get("codex_cli") is not None:
            lines.append(f"- 账号 {account_id} Codex CLI：`data/account_{account_id}_codex_cli.json`、`data/account_{account_id}_usage_after_cli.json`")

    lines.extend(["", "## 根因分析"])
    for result in results:
        account = result.get("account") or {}
        account_id = account.get("id")
        issues = result.get("preflight_issues") or []
        if issues:
            for issue in issues:
                lines.append(f"- 账号 {account_id}：{issue.get('severity')} `{issue.get('code')}` - {issue.get('message')}")
        elif result.get("proven_codex_available"):
            lines.append(f"- 账号 {account_id}：未发现阻塞项，Codex E2E 且 usage 新记录均通过。")
        else:
            lines.append(f"- 账号 {account_id}：未发现明确预检阻塞，但未获得完整 E2E + 新 usage 命中证明。")

    lines.extend(["", "## 修复动作"])
    changed_any = False
    for result in results:
        account = result.get("account") or {}
        account_id = account.get("id")
        fix = result.get("fix_result") or {}
        if fix.get("changed"):
            changed_any = True
            lines.append(f"- 账号 {account_id}：执行已知安全配置修复，详情见 `analysis/diagnosis.json` 的脱敏 `fix_result`。")
    if not changed_any:
        lines.append("- 本次未执行远程配置写入；如需自动调整配置，请显式启用 `--apply-known-config-fixes`。")

    lines.extend(["", "## 最终验证"])
    for result in results:
        account = result.get("account") or {}
        account_id = account.get("id")
        direct = result.get("admin_account_test") or {}
        e2e = result.get("codex_e2e") or {}
        cli = result.get("codex_cli") or {}
        new_usage = bool((result.get("proof_usage") or {}).get("new_items"))
        lines.append(
            f"- 账号 {account_id}：直连测试={'通过' if direct.get('success') else '未通过'}；"
            f"HTTP E2E={'通过' if e2e.get('success') else '未通过/未执行'}；"
            f"Codex CLI={'通过' if cli.get('success') else '未通过/未执行'}；"
            f"本次新 usage 命中={'是' if new_usage else '否'}。"
        )

    lines.extend(["", "## 遗留风险"])
    if all(result.get("proven_codex_available") for result in results):
        lines.append("- 未发现影响本次待测账号的遗留风险。")
    else:
        lines.append("- 未通过账号需要结合 `analysis/diagnosis.json` 中的失败响应、usage 记录和预检项继续修复；不能把管理端账号直连测试视为 Codex 可调用证明。")
    lines.append("")
    return "\n".join(lines)


def apply_known_config_fixes(client: Client, summary: dict[str, Any], logger: JsonlLogger) -> dict[str, Any]:
    account_id = int(summary["id"])
    credentials_updates: dict[str, Any] = {}
    extra_updates: dict[str, Any] = {}
    if summary.get("credentials", {}).get("model_capability_strategy") == "inherit_default":
        credentials_updates["model_capability_strategy"] = None
    if summary.get("type") == "chatapi" and summary.get("extra", {}).get("chatapi_responses_enabled") is not True:
        extra_updates["chatapi_responses_enabled"] = True
    payload: dict[str, Any] = {"account_ids": [account_id]}
    if credentials_updates:
        payload["credentials"] = credentials_updates
    if extra_updates:
        payload["extra"] = extra_updates
    if len(payload) == 1:
        return {"changed": False, "updates": {}}
    logger.write("apply_known_config_fixes", account_id=account_id, updates=payload)
    result = client.json("POST", "/api/v1/admin/accounts/bulk-update", body=payload)
    return {"changed": True, "updates": payload, "result": result}


def main() -> int:
    args = parse_args()
    base_url, admin_headers, codex_api_key = require_auth()

    out_dir = validate_out_dir(args.out_dir)
    data_dir = out_dir / "data"
    logs_dir = out_dir / "logs"
    analysis_dir = out_dir / "analysis"
    for directory in (data_dir, logs_dir, analysis_dir):
        directory.mkdir(parents=True, exist_ok=True)

    logger = JsonlLogger(logs_dir / "events.jsonl")
    client = Client(base_url, admin_headers, args.timeout, logger)
    logger.write("start", base_url=base_url, accounts=args.account, model=args.model, codex_api_key_present=bool(codex_api_key))

    accounts = list_accounts(client)
    write_json(data_dir / "accounts_openai.json", {"items": accounts})
    targets = match_accounts(accounts, args.account)

    all_results = []
    for target in targets:
        account_id = int(target["id"])
        detail = get_account_detail(client, account_id)
        summary = summarize_account(detail)
        write_json(data_dir / f"account_{account_id}.json", summary)

        availability = None
        try:
            availability = client.json("GET", "/api/v1/admin/ops/account-availability?platform=openai")
            write_json(data_dir / "account_availability_openai.json", availability)
        except Exception as exc:  # noqa: BLE001
            logger.write("availability_failed", account_id=account_id, error=str(exc))

        temp_unsched = None
        try:
            temp_unsched = client.json("GET", f"/api/v1/admin/accounts/{account_id}/temp-unschedulable")
            write_json(data_dir / f"account_{account_id}_temp_unschedulable.json", temp_unsched)
        except Exception as exc:  # noqa: BLE001
            logger.write("temp_unsched_failed", account_id=account_id, error=str(exc))

        preflight_issues = account_preflight(summary, args.model)
        before_usage = latest_usage_for_account(client, account_id)
        write_json(data_dir / f"account_{account_id}_usage_before.json", before_usage)

        fix_result = None
        if args.apply_known_config_fixes and preflight_issues:
            fix_result = apply_known_config_fixes(client, summary, logger)
            if fix_result.get("changed"):
                detail = get_account_detail(client, account_id)
                summary = summarize_account(detail)
                write_json(data_dir / f"account_{account_id}_after_fix.json", summary)
                preflight_issues = account_preflight(summary, args.model)

        direct_test = admin_account_test(client, account_id, args.model, args.prompt)
        write_json(data_dir / f"account_{account_id}_admin_test.json", direct_test)

        e2e_result = None
        http_after_usage = None
        cli_after_usage = None
        codex_cli_result = None
        if codex_api_key:
            e2e_result = codex_e2e(client, codex_api_key, args.model, args.prompt)
            write_json(data_dir / f"account_{account_id}_codex_e2e.json", e2e_result)
            http_after_usage = poll_new_usage_for_account(
                client,
                account_id,
                usage_markers(before_usage),
                seconds=args.usage_poll_seconds,
                interval=args.usage_poll_interval,
            )
            write_json(data_dir / f"account_{account_id}_usage_after_http.json", http_after_usage)
            if args.run_codex_cli:
                before_cli_usage = latest_usage_for_account(client, account_id)
                write_json(data_dir / f"account_{account_id}_usage_before_cli.json", before_cli_usage)
                codex_cli_result = run_codex_cli_probe(base_url, codex_api_key, args.model, args.prompt, args.codex_cli_timeout, logger)
                write_json(data_dir / f"account_{account_id}_codex_cli.json", codex_cli_result)
                cli_after_usage = poll_new_usage_for_account(
                    client,
                    account_id,
                    usage_markers(before_cli_usage),
                    seconds=args.usage_poll_seconds,
                    interval=args.usage_poll_interval,
                )
                write_json(data_dir / f"account_{account_id}_usage_after_cli.json", cli_after_usage)

        probe_success = bool(e2e_result and e2e_result.get("success"))
        proof_usage = http_after_usage
        if args.run_codex_cli:
            probe_success = bool(codex_cli_result and codex_cli_result.get("success"))
            proof_usage = cli_after_usage
        proven = bool(probe_success and proof_usage and (proof_usage.get("new_items") or []))
        all_results.append({
            "account": summary,
            "preflight_issues": preflight_issues,
            "fix_result": fix_result,
            "admin_account_test": direct_test,
            "codex_e2e": e2e_result,
            "codex_cli": codex_cli_result,
            "usage_after_http": http_after_usage,
            "usage_after_cli": cli_after_usage,
            "proof_usage": proof_usage,
            "proven_codex_available": proven,
            "availability_snapshot_present": availability is not None,
            "temp_unschedulable": temp_unsched,
        })

    report = {
        "base_url": base_url,
        "model": args.model,
        "codex_api_key_present": bool(codex_api_key),
        "results": all_results,
        "generated_at": now_iso(),
    }
    write_json(analysis_dir / "diagnosis.json", report)
    (out_dir / "report.md").write_text(render_report(redact(report)), encoding="utf-8")
    logger.write("finish", result_count=len(all_results))
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except KeyboardInterrupt:
        raise SystemExit(130)
