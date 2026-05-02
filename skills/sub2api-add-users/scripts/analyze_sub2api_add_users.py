#!/usr/bin/env python3
"""Convert collected sub2api capacity data into an add-user recommendation report."""

from __future__ import annotations

import argparse
import csv
import json
import math
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


def parse_args() -> argparse.Namespace:
    config_path = Path(__file__).resolve().parents[1] / "config.yaml"
    config_directories = load_config_section(config_path, "directories")
    parser = argparse.ArgumentParser(description="Analyze sub2api capacity headroom for adding users.")
    parser.add_argument("--data-dir", required=True)
    parser.add_argument("--analysis-dir", required=True)
    parser.add_argument("--report-path", required=True)
    parser.add_argument("--config", default=str(config_path))
    parser.add_argument(
        "--work-root",
        default=str(config_directories["work_root"]),
        help="Refuse to read or write outside this work root.",
    )
    return parser.parse_args()


def load_json(path: Path) -> Any:
    if not path.exists():
        return None
    return json.loads(path.read_text(encoding="utf-8"))


def number(value: Any, default: float = 0.0) -> float:
    if isinstance(value, (int, float)):
        return float(value)
    if isinstance(value, str):
        try:
            return float(value)
        except ValueError:
            return default
    return default


def integer(value: Any, default: int = 0) -> int:
    return int(number(value, float(default)))


def load_thresholds(config_path: Path) -> dict[str, float]:
    required_keys = {
        "reserve_account_ratio",
        "min_account_reserve",
        "max_redundant_utilization",
        "high_utilization",
        "low_confidence_score",
        "medium_confidence_score",
    }
    thresholds: dict[str, float] = {}
    if not config_path.exists():
        raise SystemExit(f"Missing config file: {config_path}")
    in_section = False
    for raw_line in config_path.read_text(encoding="utf-8").splitlines():
        line = raw_line.rstrip()
        if line.startswith("analysis_thresholds:"):
            in_section = True
            continue
        if in_section and line and not line.startswith(" "):
            break
        if not in_section or ":" not in line:
            continue
        key, value = [part.strip() for part in line.split(":", 1)]
        if key in required_keys:
            try:
                thresholds[key] = float(value)
            except ValueError as exc:
                raise SystemExit(f"Invalid analysis_thresholds.{key} in config.yaml") from exc
    missing = sorted(required_keys - set(thresholds))
    if missing:
        raise SystemExit(f"Missing analysis thresholds in config.yaml: {', '.join(missing)}")
    return thresholds


def normalize_pools(payload: Any) -> tuple[dict[str, Any], list[dict[str, Any]], str]:
    if not isinstance(payload, dict):
        return {}, [], "missing"
    if isinstance(payload.get("pools"), list):
        return payload.get("summary") or {}, [p for p in payload["pools"] if isinstance(p, dict)], "pools"
    items = payload.get("items")
    if isinstance(items, list):
        pools: list[dict[str, Any]] = []
        for item in items:
            if not isinstance(item, dict):
                continue
            current_total = integer(item.get("current_total_accounts"))
            current_schedulable = integer(item.get("current_schedulable_accounts"))
            additional = integer(item.get("recommended_additional_accounts"))
            pools.append({
                "pool_key": f"legacy-group-{item.get('group_id', 'unknown')}",
                "platform": item.get("platform", ""),
                "group_names": [item.get("group_name", "")],
                "plan_names": item.get("plan_names") or [],
                "recommended_account_type": item.get("recommended_account_type", "schedulable"),
                "status": item.get("status", "unknown"),
                "confidence_score": number(item.get("confidence_score"), 0.35),
                "current_total_accounts": current_total,
                "current_schedulable_accounts": current_schedulable,
                "current_unschedulable_accounts": max(current_total - current_schedulable, 0),
                "recommended_schedulable_accounts": max(current_schedulable + additional, current_schedulable),
                "recommended_additional_schedulable_accounts": additional,
                "recoverable_unschedulable_accounts": min(max(current_total - current_schedulable, 0), additional),
                "new_accounts_required": max(additional - max(current_total - current_schedulable, 0), 0),
                "reason": item.get("reason", ""),
                "metrics": item.get("metrics") or {},
            })
        return payload.get("summary") or {}, pools, "legacy_items"
    return payload.get("summary") or {}, [], "empty"


def users_per_account(metrics: dict[str, Any], current_schedulable: int) -> float:
    baseline = metrics.get("platform_baseline")
    if not isinstance(baseline, dict):
        baseline = {}
    candidates = [
        number(baseline.get("active_subscriptions_per_schedulable")),
        number(metrics.get("active_subscriptions")) / current_schedulable if current_schedulable > 0 else 0,
        number(baseline.get("active_users_per_schedulable")),
        number(metrics.get("active_users_30d")) / current_schedulable if current_schedulable > 0 else 0,
    ]
    positive = [value for value in candidates if value > 0]
    return max(max(positive or [1.0]), 1.0)


def confidence_multiplier(score: float, thresholds: dict[str, float]) -> float:
    if score < thresholds["low_confidence_score"]:
        return 0.5
    if score < thresholds["medium_confidence_score"]:
        return 0.75
    return 1.0


def analyze_pool(pool: dict[str, Any], thresholds: dict[str, float]) -> dict[str, Any]:
    metrics = pool.get("metrics") if isinstance(pool.get("metrics"), dict) else {}
    current_schedulable = integer(pool.get("current_schedulable_accounts"))
    current_total = integer(pool.get("current_total_accounts"))
    recommended_additional = integer(pool.get("recommended_additional_schedulable_accounts"))
    expected_by_subs = integer(metrics.get("expected_accounts_by_subscriptions"))
    expected_by_active_users = integer(metrics.get("expected_accounts_by_active_users"))
    expected_by_cost = integer(metrics.get("expected_accounts_by_cost"))
    estimated_required = max(expected_by_subs, expected_by_active_users, expected_by_cost)
    utilization = max(
        number(metrics.get("capacity_utilization")),
        number(metrics.get("concurrency_utilization")),
        number(metrics.get("sessions_utilization")),
        number(metrics.get("rpm_utilization")),
    )
    confidence = number(pool.get("confidence_score"), 0.35)
    per_account = users_per_account(metrics, current_schedulable)
    reserve_accounts = max(
        int(thresholds["min_account_reserve"]),
        math.ceil(current_schedulable * thresholds["reserve_account_ratio"]) if current_schedulable > 0 else 0,
    )

    reasons: list[str] = []
    spare_accounts = 0
    if recommended_additional > 0:
        user_delta = -math.ceil(recommended_additional * per_account)
        reasons.append(f"容量推荐接口已给出 {recommended_additional} 个可调度账号缺口")
    else:
        if utilization >= thresholds["high_utilization"]:
            reserve_accounts = max(reserve_accounts, 2)
            reasons.append(f"容量利用率 {utilization:.0%} 偏高，增加安全保留")
        if utilization >= thresholds["max_redundant_utilization"]:
            user_delta = 0
            reasons.append(f"容量利用率 {utilization:.0%} 已超过冗余阈值，不建议继续加用户")
        else:
            spare_accounts = max(current_schedulable - estimated_required - reserve_accounts, 0)
            user_delta = math.floor(spare_accounts * per_account * confidence_multiplier(confidence, thresholds))
            if spare_accounts <= 0:
                reasons.append("扣除估算所需账号和安全保留后没有富余可调度账号")
            else:
                reasons.append(f"扣除 {estimated_required} 个估算所需账号和 {reserve_accounts} 个安全保留后仍有 {spare_accounts} 个富余账号")

    if recommended_additional <= 0 and user_delta > 0:
        if confidence < thresholds["low_confidence_score"]:
            reasons.append(f"置信度 {confidence:.2f} 偏低，已按 50% 折减新增用户建议")
        elif confidence < thresholds["medium_confidence_score"]:
            reasons.append(f"置信度 {confidence:.2f} 中等，已按 75% 折减新增用户建议")

    return {
        "pool_key": str(pool.get("pool_key", "")),
        "platform": str(pool.get("platform", "")),
        "group_names": pool.get("group_names") or [],
        "plan_names": pool.get("plan_names") or [],
        "status": str(pool.get("status", "")),
        "confidence_score": confidence,
        "current_total_accounts": current_total,
        "current_schedulable_accounts": current_schedulable,
        "recommended_additional_schedulable_accounts": recommended_additional,
        "estimated_required_accounts": estimated_required,
        "reserve_accounts": reserve_accounts,
        "spare_accounts": spare_accounts,
        "users_per_schedulable_account": per_account,
        "capacity_utilization": utilization,
        "recommended_user_delta": user_delta,
        "source_reason": str(pool.get("reason", "")),
        "analysis_reason": "；".join(reasons),
    }


def validate_core_data(data_dir: Path) -> None:
    missing = [name for name in ("recommendations.json", "usage_stats.json") if not (data_dir / name).exists()]
    if missing:
        raise SystemExit(f"Missing required collected data: {', '.join(missing)}")
    recommendations = load_json(data_dir / "recommendations.json")
    usage_stats = load_json(data_dir / "usage_stats.json")
    if not isinstance(recommendations, dict):
        raise SystemExit("Invalid recommendations.json: expected a JSON object.")
    if not isinstance(usage_stats, dict):
        raise SystemExit("Invalid usage_stats.json: expected a JSON object.")


def analyze(data_dir: Path, thresholds: dict[str, float]) -> dict[str, Any]:
    validate_core_data(data_dir)
    metadata = load_json(data_dir / "_metadata.json") or {}
    errors = load_json(data_dir / "_errors.json") or {}
    recommendations = load_json(data_dir / "recommendations.json") or {}
    usage_stats = load_json(data_dir / "usage_stats.json") or {}
    summary, pools, recommendation_shape = normalize_pools(recommendations)
    pool_results = [analyze_pool(pool, thresholds) for pool in pools]
    positive_capacity_user_delta = sum(max(integer(pool["recommended_user_delta"]), 0) for pool in pool_results)
    shortage_user_gap = sum(abs(min(integer(pool["recommended_user_delta"]), 0)) for pool in pool_results)
    net_user_delta = positive_capacity_user_delta - shortage_user_gap
    recommended_user_delta = -shortage_user_gap if shortage_user_gap > 0 else positive_capacity_user_delta
    shortage_pools = [pool for pool in pool_results if integer(pool["recommended_user_delta"]) < 0]
    addable_pools = [pool for pool in pool_results if integer(pool["recommended_user_delta"]) > 0]
    neutral_pools = [pool for pool in pool_results if integer(pool["recommended_user_delta"]) == 0]
    if recommended_user_delta > 0:
        verdict = "有冗余，适合谨慎新增用户"
    elif recommended_user_delta < 0:
        verdict = "有算力缺口，暂不适合新增用户"
    else:
        verdict = "当前不建议新增用户，继续观察"

    return {
        "metadata": metadata,
        "data_files": sorted(p.name for p in data_dir.glob("*.json")),
        "endpoint_errors": errors,
        "recommendation_shape": recommendation_shape,
        "raw_recommendation_summary": summary,
        "usage_stats": {
            "total_requests": number(usage_stats.get("total_requests")),
            "total_actual_cost": number(usage_stats.get("total_actual_cost")),
            "total_cost": number(usage_stats.get("total_cost")),
            "average_duration_ms": number(usage_stats.get("average_duration_ms")),
        },
        "thresholds": thresholds,
        "pool_results": pool_results,
        "totals": {
            "recommended_user_delta": recommended_user_delta,
            "net_user_delta": net_user_delta,
            "positive_capacity_user_delta": positive_capacity_user_delta,
            "shortage_user_gap": shortage_user_gap,
            "pool_count": len(pool_results),
            "shortage_pool_count": len(shortage_pools),
            "addable_pool_count": len(addable_pools),
            "neutral_pool_count": len(neutral_pools),
        },
        "verdict": verdict,
    }


def fmt_signed(value: int) -> str:
    if value > 0:
        return f"+{value}"
    return str(value)


def fmt_money(value: float) -> str:
    return f"${value:.4f}"


def render_report(summary: dict[str, Any]) -> str:
    metadata = summary.get("metadata", {})
    totals = summary.get("totals", {})
    usage = summary.get("usage_stats", {})
    endpoint_errors = summary.get("endpoint_errors", {})
    total_delta = integer(totals.get("recommended_user_delta"))
    lines = [
        "# Sub2API 加用户容量建议报告",
        "",
        "## 结论",
        f"- 建议新增用户数：**{fmt_signed(total_delta)}**",
        f"- 判断：{summary.get('verdict', '未知')}",
        f"- 站点：{metadata.get('base_url', '未知')}",
        f"- 时间段：{metadata.get('requested_since', '未知')} 至 {metadata.get('requested_until', '未知')}",
        f"- API 日期范围：{metadata.get('api_start_date', '未知')} 至 {metadata.get('api_end_date', '未知')}",
        f"- 数据完整性：{len(endpoint_errors)} 个非核心接口采集失败；核心容量与用量接口已进入分析",
        "",
        "## 计算口径",
        "- 以 `data/recommendations.json` 的容量池为主口径：该接口会把共享账号的订阅分组合并为同一容量池。",
        "- 负值来自 `recommended_additional_schedulable_accounts`：表示当前容量池已有可调度账号缺口，再按该池每个可调度账号承载的活跃订阅基线换算为用户缺口。",
        "- 正值只在没有账号缺口、容量利用率低于阈值时计算：当前可调度账号减去估算所需账号和安全保留账号，再乘以每账号承载用户基线。",
        "- 不同容量池不能默认互相支援；只要任一容量池有缺口，总结论优先呈现缺口，不用其它池的冗余抵消。",
        "",
        "## 分容量池建议",
        "| 容量池 | 平台 | 分组 | 当前可调度账号 | 估算所需账号 | 安全保留 | 容量利用率 | 每账号用户基线 | 建议用户数 | 原因 |",
        "|---|---|---|---:|---:|---:|---:|---:|---:|---|",
    ]
    pool_results = summary.get("pool_results", [])
    if not pool_results:
        lines.append("| 无 | - | - | 0 | 0 | 0 | 0% | 0 | 0 | 未拿到容量池数据，不能可靠评估 |")
    for pool in pool_results:
        group_names = ", ".join(str(x) for x in pool.get("group_names") or []) or "-"
        lines.append(
            "| {pool_key} | {platform} | {groups} | {current} | {required} | {reserve} | {util:.0%} | {per:.2f} | {delta} | {reason} |".format(
                pool_key=pool.get("pool_key", "-"),
                platform=pool.get("platform", "-"),
                groups=group_names,
                current=integer(pool.get("current_schedulable_accounts")),
                required=integer(pool.get("estimated_required_accounts")),
                reserve=integer(pool.get("reserve_accounts")),
                util=number(pool.get("capacity_utilization")),
                per=number(pool.get("users_per_schedulable_account")),
                delta=fmt_signed(integer(pool.get("recommended_user_delta"))),
                reason=str(pool.get("analysis_reason", "")).replace("|", "/"),
            )
        )

    lines.extend([
        "",
        "## 原因",
        f"- 本时间窗请求数：{int(number(usage.get('total_requests')))}；实际扣费：{fmt_money(number(usage.get('total_actual_cost')))}；平均耗时：{number(usage.get('average_duration_ms')):.0f} ms。",
        f"- 容量池数量：{integer(totals.get('pool_count'))}；有缺口容量池：{integer(totals.get('shortage_pool_count'))}；可新增容量池：{integer(totals.get('addable_pool_count'))}。",
        f"- 分池净值为 {fmt_signed(integer(totals.get('net_user_delta')))}，但安全口径下的总结论为 {fmt_signed(total_delta)}，因为不同容量池不能默认互相支援。",
        "- 源码口径显示，容量推荐默认基于 active 订阅分组、30d 活跃用户、7d 增长系数、可调度账号数和分组容量利用率；因此本报告把“新增用户数”视为同类订阅用户画像下的保守估算。",
        "",
        "## 风险与限制",
        "- `dashboard/recommendations` 的内部增长和基线窗口固定为 30d，本报告的用户指定时间段主要用于补充用量背景和数据完整性判断。",
        "- 如果新增用户画像明显重于既有用户，实际可新增人数会低于本报告。",
        "- 如果近期刚新增账号或刚恢复不可调度账号，建议至少观察一个完整高峰周期后再扩大销售。",
        "",
        "## 后续验证",
        "- 新增用户前后复跑同一技能，对比 `recommended_user_delta`、容量利用率、平均耗时和失败接口数。",
        "- 对建议为负的容量池，先恢复不可调度账号或补充账号，再复核是否仍为负值。",
        "- 对建议为正的容量池，按分池建议小批量新增，避免一次性把所有冗余卖完。",
        "",
        "## 附录",
        "- 原始数据目录：`data/`",
        "- 分析产物目录：`analysis/`",
        "- 主要证据文件：`data/recommendations.json`、`data/usage_stats.json`、`analysis/summary.json`、`analysis/pool_capacity.csv`",
    ])
    if endpoint_errors:
        lines.append(f"- 接口错误详情：`data/_errors.json`（{len(endpoint_errors)} 项）")
    return "\n".join(lines) + "\n"


def write_pool_csv(path: Path, pool_results: list[dict[str, Any]]) -> None:
    fieldnames = [
        "pool_key",
        "platform",
        "group_names",
        "status",
        "confidence_score",
        "current_total_accounts",
        "current_schedulable_accounts",
        "recommended_additional_schedulable_accounts",
        "estimated_required_accounts",
        "reserve_accounts",
        "spare_accounts",
        "users_per_schedulable_account",
        "capacity_utilization",
        "recommended_user_delta",
        "analysis_reason",
    ]
    with path.open("w", encoding="utf-8", newline="") as handle:
        writer = csv.DictWriter(handle, fieldnames=fieldnames)
        writer.writeheader()
        for pool in pool_results:
            row = {key: pool.get(key, "") for key in fieldnames}
            row["group_names"] = ", ".join(str(x) for x in pool.get("group_names") or [])
            writer.writerow(row)


def main() -> int:
    args = parse_args()
    data_dir = Path(args.data_dir).resolve()
    analysis_dir = Path(args.analysis_dir).resolve()
    report_path = Path(args.report_path).resolve()
    work_root = Path(args.work_root).resolve()
    for label, path in (("data-dir", data_dir), ("analysis-dir", analysis_dir), ("report-path", report_path)):
        try:
            path.relative_to(work_root)
        except ValueError:
            raise SystemExit(f"--{label} must be inside work root: {work_root}")
    analysis_dir.mkdir(parents=True, exist_ok=True)
    report_path.parent.mkdir(parents=True, exist_ok=True)

    thresholds = load_thresholds(Path(args.config).resolve())
    summary = analyze(data_dir, thresholds)
    (analysis_dir / "summary.json").write_text(
        json.dumps(summary, ensure_ascii=False, indent=2, sort_keys=True),
        encoding="utf-8",
    )
    write_pool_csv(analysis_dir / "pool_capacity.csv", summary.get("pool_results", []))
    report_path.write_text(render_report(summary), encoding="utf-8")
    print(f"Wrote analysis to {analysis_dir}")
    print(f"Wrote report draft to {report_path}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
