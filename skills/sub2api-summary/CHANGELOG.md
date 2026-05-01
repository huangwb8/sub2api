# Changelog

格式基于 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/)。

## [Unreleased]

### Added（新增）
- 新增 `sub2api-summary` 技能初版：支持真实 sub2api 站点只读运营数据采集、确定性分析和 `plan.md` 优化计划输出。
- 新增 `collect_sub2api_data.py`：通过 `x-api-key` 只读调用管理端 dashboard、usage、model、group、user 与 ops 相关 GET 接口。
- 新增 `analyze_sub2api_data.py`：读取采集结果生成分析摘要与优化计划初稿。

### Changed（变更）
- 增强采集鉴权：支持 Admin API Key 与管理员 JWT 两种模式，并推荐通过环境变量传入凭据，降低命令行泄密风险。
- 调整核心接口容错：只把 `usage_stats.json` 作为必需核心数据，旧站点缺少 `dashboard/snapshot-v2` 时不再直接中止。
- 采集默认值与分析阈值改为读取 `config.yaml`，减少脚本硬编码并强化配置单一来源。
- 增强输出路径安全：采集脚本默认只允许写入 `tmp/sub2api-summary` 工作根目录下的本次运行目录。
