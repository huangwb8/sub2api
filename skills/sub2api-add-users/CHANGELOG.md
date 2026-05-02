# Changelog

格式基于 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/)。

## [Unreleased]

### Added（新增）
- 新增 `sub2api-add-users` 技能初版：支持真实 sub2api 站点只读运营数据采集、容量冗余/缺口分析和 `report.md` 输出。
- 新增 `collect_sub2api_add_users_data.py`：通过管理员只读鉴权调用 Dashboard 推荐、用量统计、分组容量、模型和分组聚合接口。
- 新增 `analyze_sub2api_add_users.py`：将账号容量池推荐换算为建议新增用户数，正值表示冗余，负值表示算力缺口。
- 新增用户向 `README.md`：提供触发场景、输入输出和 `WHICHMODEL` 指引，方便人类快速上手。

### Changed（变更）
- 基于 `auto-test-skill` A 轮自检收敛默认采集范围：移除用户榜、用户拆解和 usage 明细样本等非必要用户级接口，默认只采集容量与聚合用量数据。
- 强化分析安全边界：分析脚本现在会校验输入、输出路径必须位于 `tmp/sub2api-add-users` 工作根目录内，并在核心数据缺失或格式错误时直接中止。
- 调整总建议口径：不同容量池不再正负抵消，只要任一容量池存在用户容量缺口，总结论优先输出负值。
- 清理了 `page_size` 这一已不再使用的默认配置，避免配置项漂移。
