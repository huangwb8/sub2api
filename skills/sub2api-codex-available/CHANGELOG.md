# Changelog

格式基于 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/)。

## [Unreleased]

### Added（新增）
- 新增 `sub2api-codex-available` 技能初版：用于判断指定 sub2api 账号是否可被本地 Codex 程序通过真实网关链路调用，并指导配置或源码修复。
- 新增 `scripts/codex_available.py`：支持管理员鉴权预检、账号定位、配置诊断、可选 Codex 风格 E2E 请求和 usage log 校验。
- 新增 `references/source-map.md`：记录 Codex 兼容诊断相关后端路由、账号调度、Chat Completions/Responses 兼容逻辑与管理端接口。
- 吸收 dudu 项目 Codex CLI sandbox 经验：明确真实 CLI 强验证需要临时 HOME、`codex login --with-api-key`、`codex exec` 非交互参数兼容和 usage log 命中核验。
- 新增一轮 `auto-test-skill` A/B 自检产物：记录证明链、配置集中化、安全边界与脚本确定性验证。

### Changed（变更）
- 优化诊断脚本的证明链：支持站点根地址和 `/v1` 地址归一化，按测试前后 usage 差异证明本次命中，且真实 Codex CLI 模式必须使用 CLI 阶段新增 usage 作为通过依据。
- 优化诊断脚本输出：自动生成 `report.md`，并将 HTTP E2E 与 Codex CLI 的 usage 证据拆分保存。
- 收紧诊断脚本输出目录边界：默认只允许写入 `tmp/sub2api-codex-available/`，避免诊断产物散落到非约定位置。
