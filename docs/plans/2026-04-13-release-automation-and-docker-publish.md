# Release Automation And Docker Publish Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 为 Sub2API 建立稳定的版本驱动发布流程，自动创建 GitHub Release，并为最新 release 自动构建与补发 Docker 镜像。

**Architecture:** 保留现有 `release.yml` + GoReleaser 作为主发布通道，在外层增加三类自动化能力：版本同步检查、从 `backend/cmd/server/VERSION` 创建 tag/release、按最新 GitHub Release 自动补发缺失镜像。文档与本地校验脚本同步更新，确保流程可追溯、可手动演练、可在镜像丢失时恢复。

**Tech Stack:** GitHub Actions、Docker Buildx、GoReleaser、Bash、Makefile、Keep a Changelog

---

### Task 1: 固化版本源与计划文档

**Files:**
- Modify: `CHANGELOG.md`
- Create: `docs/plans/2026-04-13-release-automation-and-docker-publish.md`

**Step 1: 写入变更草稿**

在 `CHANGELOG.md` 的 `[Unreleased]` 中补充自动化发布相关条目，覆盖 workflow、文档和校验脚本。

**Step 2: 保存实施计划**

把发布目标、文件范围、验证步骤和回滚思路写入计划文档。

**Step 3: 校验计划文件位置**

Run: `test -f docs/plans/2026-04-13-release-automation-and-docker-publish.md`
Expected: 命令成功退出。

### Task 2: 先写失败测试式校验条件

**Files:**
- Create: `tools/verify-release-automation.sh`
- Modify: `Makefile`

**Step 1: 定义可失败的发布前检查**

让脚本先检查这些条件，不满足时直接失败：
- `backend/cmd/server/VERSION` 必须是 `x.y.z`
- 核心 workflow 文件必须存在
- Docker 相关文档必须包含自动发布与版本驱动说明

**Step 2: 先运行脚本并观察失败点**

Run: `bash tools/verify-release-automation.sh`
Expected: 在 workflow 或文档未完成前返回非零退出码。

**Step 3: 后续实现以这些检查为验收条件**

所有 workflow 与文档补齐后重新运行，必须变为成功。

### Task 3: 实现版本同步检查

**Files:**
- Create: `.github/workflows/check-version-sync.yml`
- Modify: `CHANGELOG.md`

**Step 1: 写版本同步检查 workflow**

检查内容：
- 从 `backend/cmd/server/VERSION` 读取版本
- 查询最新 GitHub Release
- 对比版本状态
- 检查 `CHANGELOG.md` 是否有对应版本段落或有效的 `[Unreleased]` 内容
- 在 PR 上输出摘要

**Step 2: 让 workflow 对缺口有可操作反馈**

摘要里要明确告诉维护者是否需要创建 release、是否缺少 changelog 内容。

**Step 3: 用本地脚本验证文件已落地**

Run: `rg -n "Check Version Sync|backend/cmd/server/VERSION|CHANGELOG.md" .github/workflows/check-version-sync.yml`
Expected: 能看到版本源和 changelog 校验逻辑。

### Task 4: 实现从版本文件创建 release/tag

**Files:**
- Create: `.github/workflows/create-release.yml`

**Step 1: 写创建 release workflow**

要求：
- 从 `backend/cmd/server/VERSION` 读取版本
- 自动派生 `vX.Y.Z`
- tag 已存在时跳过
- release notes 优先取 `CHANGELOG.md` 的对应版本段落，缺失时回退到 `[Unreleased]`

**Step 2: 只做“创建 tag/release”，不重复做镜像构建**

让现有 `release.yml` 继续负责 tag 触发后的 GoReleaser 发布。

**Step 3: 本地静态检查**

Run: `rg -n "backend/cmd/server/VERSION|createRelease|CHANGELOG.md|git tag -a" .github/workflows/create-release.yml`
Expected: 能看到版本读取、tag 创建和 release notes 逻辑。

### Task 5: 实现最新 release 的 Docker 镜像自动补发

**Files:**
- Create: `.github/workflows/publish-release-images.yml`

**Step 1: 写自动补发 workflow**

要求：
- 按计划任务和手动触发运行
- 获取最新 GitHub Release
- 检查 GHCR / Docker Hub 是否已有对应 manifest
- 缺失时使用 release tag 对应源码构建并推送
- 已存在时跳过

**Step 2: 保持与现有镜像标签兼容**

继续产出 `latest`、版本号以及必要的 OCI labels，不破坏现有拉取方式。

**Step 3: 输出 summary**

清楚记录本次是否构建、缺了哪些镜像、用了哪个 release 源码版本。

### Task 6: 本地验证入口与部署文档

**Files:**
- Create: `tools/verify-release-automation.sh`
- Modify: `Makefile`
- Modify: `deploy/README.md`
- Modify: `deploy/DOCKER.md`

**Step 1: 完成本地校验脚本**

脚本应支持快速验证：
- 版本号格式
- workflow 文件存在
- 关键文档包含自动发布说明

**Step 2: 增加 Makefile 入口**

加入便于维护者使用的目标，例如 `verify-release-automation`。

**Step 3: 更新部署文档**

补充：
- 版本文件是发布源
- 如何触发 create release
- 镜像何时自动发布 / 自动补发
- 需要配置哪些 secrets 和 vars

### Task 7: 运行验证并收尾审查

**Files:**
- Review: `.github/workflows/release.yml`
- Review: `.github/workflows/*.yml`
- Review: `tools/verify-release-automation.sh`

**Step 1: 运行本地校验脚本**

Run: `bash tools/verify-release-automation.sh`
Expected: 成功退出，并打印版本与 workflow 检查结果。

**Step 2: 运行 YAML 与 shell 基础校验**

Run: `bash -n tools/verify-release-automation.sh`
Expected: 成功退出。

**Step 3: 复查兼容性**

确认新增 workflow 不会替代或破坏现有 `release.yml`、不会改变已有镜像名或部署命令。

**Step 4: 总结剩余人工动作**

明确需要维护者在 GitHub 仓库中配置的 secrets / vars 与推荐发布顺序。
