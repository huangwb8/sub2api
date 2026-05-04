# sub2api-codex-available

这个 skill 用来判断并修复：某个 sub2api 账号是否能被本地 Codex 程序通过真实网关链路正常调用。

## 推荐用法

```text
使用 sub2api-codex-available。站点是 https://example.com，Admin API Key 是 xxx，用户侧 Codex API Key 是 yyy，请测试账号 test01 是否能被本地 Codex 调用；如果不能，请修好。
```

## 必需输入

- 站点基础地址
- 管理员鉴权信息
- 至少一个待测试账号名称或 ID

推荐额外提供用户侧 API Key。没有它时，skill 只能完成账号配置预检和账号直连探测，不能证明真实 Codex E2E 链路。

如果需要模拟本机真实 Codex CLI，可让执行者启用脚本参数 `--run-codex-cli`。该模式会创建临时 HOME，执行 `codex login --with-api-key` 和最小 `codex exec`，再用 usage log 验证是否命中目标账号。

报告会区分 HTTP E2E 与真实 Codex CLI 的 usage 证据：启用 `--run-codex-cli` 后，只有 CLI 执行之后新增的目标账号 usage 记录才算通过证明。

脚本默认只允许把运行产物写到 `tmp/sub2api-codex-available/` 下，避免误把脱敏诊断材料散落到其它目录。

## 输出

每次运行会生成：

```text
tmp/sub2api-codex-available/run-{timestamp}/report.md
tmp/sub2api-codex-available/run-{timestamp}/logs/
```

报告会说明是否真正命中目标账号、失败根因、做过的配置或代码修复，以及最终验证证据。
