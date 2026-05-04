---
name: sub2api-codex-available
description: 当用户要求判断 sub2api 中某个 OpenAI/Chat Completions/Responses 账号是否能被本地 Codex 程序正常调用，或要求修复“Codex 不会调用某个账号/号池账号不可用/本地 Codex 连接 sub2api 失败”时必须使用。必须要求用户提供真实站点鉴权信息和至少一个待测账号；如果缺少这些输入，应立即中止。该 skill 会创建诊断工作目录、记录日志、执行 Codex 风格 E2E 测试，并在失败时通过账号配置调整或项目源码修改让账号可用。
metadata:
  author: Bensz Conan
  short-description: 诊断并修复 sub2api 账号的本地 Codex 可调用性
  keywords:
    - sub2api-codex-available
    - sub2api
    - Codex
    - Chat Completions
    - Responses
    - 账号调度
    - E2E 诊断
---

# sub2api-codex-available

## 目标

判断指定 sub2api 账号是否能被“本地 Codex 程序 -> sub2api -> 调度器 -> 目标账号 -> 上游”完整链路正常调用。若不能，优先通过站点配置修复；配置无解时，修改当前 sub2api 项目源码并补验证，直到目标账号可被本地 Codex 正常调用。

## 输入要求

必须输入：
- 站点基础地址，例如 `https://example.com`
- 管理员鉴权信息：Admin API Key（`x-api-key`）或管理员 JWT（`Authorization: Bearer`）
- 待测试账号：至少 1 个，支持账号名称、账号 ID 或可唯一匹配的名称片段

强烈建议输入：
- 一个真实用户侧 API Key，用于模拟本地 Codex 访问 sub2api。用环境变量 `SUB2API_CODEX_API_KEY` 传入，不要写入报告。
- 目标分组 ID 或名称。如果不提供，使用账号绑定分组逐个诊断。
- 测试模型。未提供时使用 `config.yaml:defaults.codex_model`。

如果缺少站点基础地址、管理员鉴权或待测试账号，立即中止，不要读取历史凭据、不要猜测、不要绕过鉴权。只有用户明确允许时，才可读取仓库根目录 `remote.env`。

## 工作目录

每次运行创建：

```text
./tmp/sub2api-codex-available/run-{时间戳}/
├── data/       # 脱敏后的账号、分组、usage 与探测响应摘要
├── logs/       # JSONL 日志、HTTP 摘要、失败根因记录
├── analysis/   # 中间诊断结论
└── report.md   # 最终报告
```

时间戳使用本地时间 `YYYYMMDDHHMMSS`。

确定性脚本默认拒绝写入该工作根目录之外的位置；只有受控维护测试需要例外时，才可设置 `SUB2API_ALLOW_OUTSIDE_WORKDIR=true`。

## 工作流程

### 阶段一：初始化与安全确认

1. 解析站点、管理员鉴权、待测账号、测试模型和可选用户侧 Codex API Key。
2. 创建工作目录与 `logs/`。
3. 将密钥仅放在环境变量中传给脚本；不要把完整密钥写入命令输出、日志或 `report.md`。
4. 运行确定性诊断脚本的只读阶段。`--run-codex-cli` 是强验证，会调用本机真实 Codex CLI；如果本机没有 `codex` 或不希望产生 CLI 调用成本，可先不加：

```bash
export SUB2API_BASE_URL="https://example.com"
export SUB2API_ADMIN_API_KEY="..."
export SUB2API_CODEX_API_KEY="sk-..."   # 可选但推荐
python3 skills/sub2api-codex-available/scripts/codex_available.py \
  --account "test01" \
  --model "gpt-5.1" \
  --out-dir "./tmp/sub2api-codex-available/run-{timestamp}"
```

使用管理员 JWT 时：

```bash
export SUB2API_BASE_URL="https://example.com"
export SUB2API_AUTH_MODE="bearer"
export SUB2API_AUTH_TOKEN="..."
python3 skills/sub2api-codex-available/scripts/codex_available.py \
  --account "test01" \
  --out-dir "./tmp/sub2api-codex-available/run-{timestamp}"
```

### 阶段二：了解账号设置

必须采集并脱敏保存：
- 账号详情：平台、类型、状态、可调度、优先级、并发、分组、模型能力、API format 相关 extra
- 分组容量和账号可用性：`/api/v1/admin/ops/account-availability?platform=openai`
- 临时不可调度状态：`/api/v1/admin/accounts/{id}/temp-unschedulable`
- 最近 usage log：`/api/v1/admin/usage?account_id={id}`

重点判断：
- 账号是否在用户 Codex API Key 所属分组中；如果无法从 API Key 反查，报告中明确“缺少用户侧 API Key，无法证明完整链路”。
- 账号是否会被调度器过滤。
- 账号类型是否兼容 Codex 实际入口 `/v1/responses`。
- 模型是否会被 `model_capability_strategy` 或 `model_mapping` 过滤。

### 阶段三：本地 Codex 风格模拟测试

测试分三层，不要把第一层误当作成功闭环：

1. **账号直连探测**：调用 `/api/v1/admin/accounts/{id}/test`。它只证明账号凭据和上游可用。
2. **Codex 风格 E2E**：如果提供 `SUB2API_CODEX_API_KEY`，向真实网关发 `/v1/responses` 请求，带 Codex 风格 `User-Agent` 与响应头。
3. **真实 Codex CLI 强验证**：如果用户要求“本地 Codex 程序”级证明，或本机存在 `codex`，使用 `--run-codex-cli`。脚本会创建临时 HOME，先执行 `codex login --with-api-key`，再执行 `codex exec --skip-git-repo-check`，并根据 `codex exec --help` 自动选择非交互权限参数。
4. **usage log 核验**：轮询 `/api/v1/admin/usage?account_id={id}`，确认本次请求之后生成了新的 `account_id=目标账号` 记录，并记录 `inbound_endpoint` 与 `upstream_endpoint`。如果启用真实 Codex CLI，必须使用 CLI 执行前后的 usage 差异作为证明，不能复用 HTTP E2E 的 usage 记录。

只有轻量 E2E 或真实 Codex CLI 成功，且对应阶段的 usage log 新记录确认命中目标账号，才可判定“本地 Codex 可以正常调用该账号”。如果未提供用户侧 API Key，最多判定为“配置预检/账号直连通过，Codex E2E 未证明”。

### 阶段四：根因分析与修复

失败时按优先级处理：

1. **账号配置可修复**：例如 `schedulable=false`、模型能力策略误过滤、账号不在目标分组、`chatapi_responses_enabled` 与上游能力不匹配。可在用户授权写操作或任务明确要求修复时调用管理端更新接口。
2. **源码逻辑需要修复**：例如 Chat Completions API 账号需要支持 Codex `/v1/responses` 入站转换，但当前代码只会转发到上游 `/responses`。修改范围应收敛到 OpenAI/Codex/Chat Completions 账号调度与转发逻辑，不影响 Anthropic、Gemini、Antigravity 或正常 OpenAI OAuth/API Key 功能。
3. **无法自动修复**：例如缺少用户侧 API Key、上游凭据失效、目标上游不支持任何 Codex 所需能力。报告中给出阻塞项和下一步。

每次修复后重新执行阶段三，直到所有待测账号通过或出现无法绕过的外部阻塞。

### 阶段五：报告

最终写入 `report.md`：

```markdown
# sub2api Codex 可调用性诊断报告

## 结论
- 站点：
- 待测账号：
- 是否已证明本地 Codex 可调用：

## 输入与安全

## 测试过程

## 关键日志

## 根因分析

## 修复动作

## 最终验证

## 遗留风险
```

报告必须引用本次工作目录中的 `logs/`、`data/` 或 `analysis/` 文件名作为证据，但不得展示完整密钥。

## 写操作边界

- 默认先只读诊断。
- 只有用户明确要求“保证可用/可以调参数/可以修复”或显式传入 `--apply-known-config-fixes` 时，才允许改账号配置。
- 任何远程写操作前后都要保存脱敏快照。
- 修改源码时，优先补单元测试；变更影响管理端用户文档或 skill 源码地图时同步更新。

## 源码参考

需要代码判断或修复时，先阅读 `references/source-map.md`，再只打开相关源码文件。

## 来自 dudu 的实现洞见

历史项目 `/Volumes/2T01/winE/Starup/dudu` 的 CLI sandbox 表明，仅用 curl 或 SDK 请求 `/responses` 不能完全代表 Codex CLI：
- Codex CLI 可能需要先通过 `codex login --with-api-key` 写入临时 HOME 凭据。
- 不同 Codex CLI 版本的非交互参数不完全一致，应先读 `codex exec --help` 再决定使用 `--dangerously-bypass-approvals-and-sandbox` 或 `--sandbox` / approval 参数。
- 测试结果必须结合 sub2api usage log 的 `account_id` 判断，否则只能说明“某个账号响应了请求”，不能说明“目标账号被调用”。
