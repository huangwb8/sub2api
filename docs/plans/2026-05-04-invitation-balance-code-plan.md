# 邀请码（余额）实施计划

## Goal

新增 `邀请码（余额）` 类型。管理员生成该类邀请码时可配置注册赠送余额，默认 `0`；用户使用后完成注册，初始余额应等于站点默认余额加该邀请码面值。

## Scope

- 后端新增兑换码类型常量 `invitation_balance`，并纳入邀请码校验范围。
- 注册与 OAuth 首次注册链路读取邀请码 `value`，按非负金额增加新用户初始余额。
- 管理员生成兑换码接口允许 `invitation_balance` 携带非负 `value`。
- 前端管理页支持选择、筛选和配置该类型，并补齐中英文文案。
- 增加单元测试覆盖普通注册、OAuth 注册和管理员生成行为。

## Verification

- `cd backend && go test -tags=unit ./internal/service -run 'TestAuthService_.*InvitationBalance|TestAdminService_GenerateRedeemCodes_InvitationBalance'`
- `cd frontend && pnpm run typecheck`
