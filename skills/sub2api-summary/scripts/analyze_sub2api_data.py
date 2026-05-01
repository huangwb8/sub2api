#!/usr/bin/env python3
"""Generate a first-pass sub2api operation analysis plan from collected JSON files."""

from __future__ import annotations

import argparse
import json
from pathlib import Path
from typing import Any


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Analyze collected sub2api operation data.")
    parser.add_argument("--data-dir", required=True)
    parser.add_argument("--analysis-dir", required=True)
    parser.add_argument("--plan-path", required=True)
    parser.add_argument("--config", default=str(Path(__file__).resolve().parents[1] / "config.yaml"))
    return parser.parse_args()


def load_json(path: Path) -> Any:
    if not path.exists():
        return None
    return json.loads(path.read_text(encoding="utf-8"))


def number(value: Any) -> float:
    if isinstance(value, (int, float)):
        return float(value)
    return 0.0


def first_list(payload: Any, keys: list[str]) -> list[dict[str, Any]]:
    if isinstance(payload, list):
        return [x for x in payload if isinstance(x, dict)]
    if isinstance(payload, dict):
        for key in keys:
            value = payload.get(key)
            if isinstance(value, list):
                return [x for x in value if isinstance(x, dict)]
    return []


def pct(part: float, total: float) -> float:
    if total <= 0:
        return 0.0
    return part / total


def fmt_money(value: float) -> str:
    return f"${value:.4f}"


def load_thresholds(config_path: Path) -> dict[str, float]:
    required_keys = {
        "high_average_duration_ms",
        "high_top_model_share",
        "high_top_user_share",
        "high_unhealthy_account_ratio",
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


def analyze(data_dir: Path, thresholds: dict[str, float]) -> dict[str, Any]:
    metadata = load_json(data_dir / "_metadata.json") or {}
    errors = load_json(data_dir / "_errors.json") or {}
    snapshot = load_json(data_dir / "dashboard_snapshot_v2.json") or {}
    usage_stats = load_json(data_dir / "usage_stats.json") or {}
    models_payload = load_json(data_dir / "models.json")
    groups_payload = load_json(data_dir / "groups.json")
    ranking_payload = load_json(data_dir / "users_ranking.json")
    breakdown_payload = load_json(data_dir / "user_breakdown.json")

    stats = snapshot.get("stats") if isinstance(snapshot, dict) else {}
    if not isinstance(stats, dict):
        stats = {}

    total_accounts = number(stats.get("total_accounts"))
    unhealthy_accounts = (
        number(stats.get("error_accounts"))
        + number(stats.get("ratelimit_accounts"))
        + number(stats.get("overload_accounts"))
    )
    total_requests = number(usage_stats.get("total_requests") or stats.get("total_requests"))
    actual_cost = number(usage_stats.get("total_actual_cost") or stats.get("total_actual_cost"))
    standard_cost = number(usage_stats.get("total_cost") or stats.get("total_cost"))
    avg_duration = number(usage_stats.get("average_duration_ms") or stats.get("average_duration_ms"))

    models = first_list(models_payload, ["models"])
    groups = first_list(groups_payload, ["groups"])
    users = first_list(ranking_payload, ["users", "ranking", "items"])
    if not users:
        users = first_list(breakdown_payload, ["users", "items"])

    model_total_cost = sum(number(x.get("actual_cost") or x.get("cost") or x.get("total_actual_cost")) for x in models)
    top_model = max(
        models,
        key=lambda x: number(x.get("actual_cost") or x.get("cost") or x.get("total_actual_cost") or x.get("requests")),
        default={},
    )
    top_model_cost = number(top_model.get("actual_cost") or top_model.get("cost") or top_model.get("total_actual_cost"))
    top_model_share = pct(top_model_cost, model_total_cost or actual_cost)

    user_total_cost = sum(number(x.get("actual_cost") or x.get("total_actual_cost") or x.get("cost")) for x in users)
    top_user = max(
        users,
        key=lambda x: number(x.get("actual_cost") or x.get("total_actual_cost") or x.get("cost") or x.get("requests")),
        default={},
    )
    top_user_cost = number(top_user.get("actual_cost") or top_user.get("total_actual_cost") or top_user.get("cost"))
    top_user_share = pct(top_user_cost, user_total_cost or actual_cost)

    findings: list[dict[str, Any]] = []
    if errors:
        findings.append({
            "priority": "P1",
            "title": "部分只读运营接口采集失败",
            "evidence": f"data/_errors.json 记录 {len(errors)} 个失败接口",
            "impact": "分析覆盖面下降，可能漏掉 ops 或盈利趋势问题。",
            "recommendation": "先确认站点版本、功能开关和管理员 API Key 权限，再决定是否补齐只读诊断接口。",
        })
    if stats.get("stats_stale"):
        findings.append({
            "priority": "P1",
            "title": "Dashboard 预聚合统计可能滞后",
            "evidence": "dashboard_snapshot_v2.stats.stats_stale=true",
            "impact": "控制台判断容量、成本和趋势时可能基于旧数据。",
            "recommendation": "检查 Dashboard aggregation worker、保留期配置和聚合水位更新日志。",
        })
    if total_accounts > 0 and pct(unhealthy_accounts, total_accounts) >= thresholds["high_unhealthy_account_ratio"]:
        findings.append({
            "priority": "P0",
            "title": "不可用或受限账号比例偏高",
            "evidence": f"异常账号 {int(unhealthy_accounts)} / 总账号 {int(total_accounts)}",
            "impact": "调度池容量下降，易引发排队、上游 429/5xx 和用户侧失败。",
            "recommendation": "按平台/分组拆分账号状态，优先恢复 rate_limited 与 overload 账号，再补充容量。",
        })
    if avg_duration >= thresholds["high_average_duration_ms"]:
        findings.append({
            "priority": "P1",
            "title": "平均请求耗时偏高",
            "evidence": f"average_duration_ms={avg_duration:.0f}",
            "impact": "用户体验下降，也可能放大并发占用和队列压力。",
            "recommendation": "按模型、分组、账号和请求类型拆解耗时，结合代理质量与流式首 token 指标定位瓶颈。",
        })
    if top_model and top_model_share >= thresholds["high_top_model_share"]:
        findings.append({
            "priority": "P1",
            "title": "模型成本集中度偏高",
            "evidence": f"Top 模型占模型成本 {top_model_share:.1%}: {top_model.get('model') or top_model.get('name')}",
            "impact": "单模型定价、账号容量或上游异常会显著影响整体利润和稳定性。",
            "recommendation": "检查该模型套餐倍率、账号池容量、缓存命中与用户分布，必要时做差异化定价或限速。",
        })
    if top_user and top_user_share >= thresholds["high_top_user_share"]:
        findings.append({
            "priority": "P1",
            "title": "用户消耗集中度偏高",
            "evidence": f"Top 用户占用户榜成本 {top_user_share:.1%}",
            "impact": "少数高消耗用户可能主导成本、风控和账号池压力。",
            "recommendation": "核对该用户 API Key、UA/IP、请求类型和订阅权益，区分正常大客户与转售/异常流量。",
        })
    if standard_cost > 0 and actual_cost > 0:
        ratio = actual_cost / standard_cost
        if ratio < 0.7 or ratio > 1.3:
            findings.append({
                "priority": "P2",
                "title": "标准成本与实际扣费差异较大",
                "evidence": f"actual/standard={ratio:.2f}, actual={fmt_money(actual_cost)}, standard={fmt_money(standard_cost)}",
                "impact": "可能意味着分组倍率、闲时计费、折扣或成本展示口径需要重新解释。",
                "recommendation": "结合分组倍率和订阅套餐检查利润口径，避免管理员误读毛利。",
            })

    summary = {
        "metadata": metadata,
        "data_files": sorted(p.name for p in data_dir.glob("*.json")),
        "metrics": {
            "total_requests": total_requests,
            "total_actual_cost": actual_cost,
            "total_cost": standard_cost,
            "average_duration_ms": avg_duration,
            "total_accounts": total_accounts,
            "unhealthy_accounts": unhealthy_accounts,
            "top_model_share": top_model_share,
            "top_user_share": top_user_share,
            "endpoint_errors": float(len(errors)),
        },
        "thresholds": thresholds,
        "findings": findings,
        "groups_seen": len(groups),
        "models_seen": len(models),
        "users_seen": len(users),
    }
    return summary


def render_plan(summary: dict[str, Any]) -> str:
    metadata = summary.get("metadata", {})
    metrics = summary.get("metrics", {})
    findings = summary.get("findings", [])
    if not findings:
        findings = [{
            "priority": "P2",
            "title": "未发现明显异常，但需要结合源码和更多维度复核",
            "evidence": "确定性脚本未触发阈值型告警",
            "impact": "可能存在脚本阈值无法覆盖的结构性问题。",
            "recommendation": "继续人工核对模型、分组、用户、账号与 ops 数据。",
        }]

    lines = [
        "# Sub2API 运营优化计划",
        "",
        "## 概览",
        f"- 站点：{metadata.get('base_url', '未知')}",
        f"- 时间段：{metadata.get('requested_since', '未知')} 至 {metadata.get('requested_until', '未知')}",
        f"- API 日期范围：{metadata.get('api_start_date', '未知')} 至 {metadata.get('api_end_date', '未知')}",
        f"- 日期口径：{metadata.get('api_date_range_note', '按 sub2api 管理端日期参数采集')}",
        f"- 请求数：{int(metrics.get('total_requests', 0))}",
        f"- 实际扣费：{fmt_money(number(metrics.get('total_actual_cost')))}",
        f"- 平均耗时：{number(metrics.get('average_duration_ms')):.0f} ms",
        f"- 数据完整性：{int(number(metrics.get('endpoint_errors')))} 个接口采集失败，详见 `data/_errors.json`",
        f"- 数据文件：{', '.join(summary.get('data_files', []))}",
        "",
        "## 关键发现",
    ]
    for item in findings:
        lines.extend([
            f"- **{item['priority']} {item['title']}**",
            f"  - 证据：{item['evidence']}",
            f"  - 影响：{item['impact']}",
            f"  - 建议：{item['recommendation']}",
        ])

    lines.extend([
        "",
        "## 根因分析",
        "- 结合 `backend/internal/server/middleware/admin_auth.go` 确认本次采集使用管理员只读鉴权，Admin API Key 对应 `x-api-key`，JWT 对应 `Authorization: Bearer`。",
        "- 结合 `backend/internal/handler/admin/usage_handler.go` 复核 `start_date/end_date/timezone` 的半开或闭区间语义，避免误判 24h 数据。",
        "- 结合 `backend/internal/handler/admin/dashboard_handler.go` 与 `backend/internal/repository/usage_log_repo.go` 核对 Dashboard 聚合口径、usage log 明细口径和预聚合新鲜度。",
        "- 结合 `backend/ent/schema/usage_log.go` 检查成本、token、请求类型、账号、分组和住宅代理流量字段是否能支撑上面的结论。",
        "",
        "## 优化计划",
        "### P0",
    ])
    p0 = [x for x in findings if x["priority"] == "P0"]
    lines.extend([f"- {x['recommendation']}" for x in p0] or ["- 暂无脚本判定的 P0 项；人工复核账号池健康与 ops 错误后再确认。"])
    lines.append("")
    lines.append("### P1")
    p1 = [x for x in findings if x["priority"] == "P1"]
    lines.extend([f"- {x['recommendation']}" for x in p1] or ["- 暂无脚本判定的 P1 项。"])
    lines.append("")
    lines.append("### P2")
    p2 = [x for x in findings if x["priority"] == "P2"]
    lines.extend([f"- {x['recommendation']}" for x in p2] or ["- 补充更长时间窗对比，验证当前窗口是否代表常态。"])
    lines.extend([
        "",
        "## 验证方案",
        "- 重新运行同一时间窗采集，确认关键指标可复现。",
        "- 选择修复项对应指标做前后对比：错误率、平均耗时、账号可调度比例、Top 模型成本占比、Top 用户成本占比、实际扣费与标准成本比例。",
        "- 所有写操作只进入计划，执行前需由站点管理员确认。",
        "",
        "## 附录",
        "- 原始数据目录：`data/`",
        "- 分析产物目录：`analysis/`",
    ])
    return "\n".join(lines) + "\n"


def main() -> int:
    args = parse_args()
    data_dir = Path(args.data_dir).resolve()
    analysis_dir = Path(args.analysis_dir).resolve()
    plan_path = Path(args.plan_path).resolve()
    analysis_dir.mkdir(parents=True, exist_ok=True)
    plan_path.parent.mkdir(parents=True, exist_ok=True)

    thresholds = load_thresholds(Path(args.config).resolve())
    summary = analyze(data_dir, thresholds)
    (analysis_dir / "summary.json").write_text(
        json.dumps(summary, ensure_ascii=False, indent=2, sort_keys=True),
        encoding="utf-8",
    )
    plan_path.write_text(render_plan(summary), encoding="utf-8")
    print(f"Wrote analysis to {analysis_dir}")
    print(f"Wrote plan draft to {plan_path}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
