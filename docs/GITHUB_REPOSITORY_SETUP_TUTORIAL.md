# GitHub 仓库发布自动化配置教程

这份教程教你把 Sub2API 的 GitHub 仓库配置成“版本文件驱动 release，并自动发布 Docker 镜像”的状态。

适用对象：
- 第一次接手这个仓库的维护者
- 想启用 `create-release.yml`、`release.yml`、`publish-release-images.yml` 的管理员
- 想确认 GitHub Actions、GHCR、Docker Hub 是否都配置正确的人

## 目标

完成本教程后，你的仓库会具备这些能力：

1. 用 `backend/cmd/server/VERSION` 作为版本源
2. 手动触发 `create-release.yml` 创建 annotated tag
3. 自动触发现有 `release.yml` 发布 GitHub Release / GoReleaser 产物
4. 自动把镜像推到 GHCR
5. 可选地把镜像推到 Docker Hub
6. 当镜像缺失时，用 `publish-release-images.yml` 自动补发

## 前置条件

开始前，请确认你具备：

1. 这个 GitHub 仓库的管理员权限
2. 一个可用的 Docker Hub 账号
3. 这个仓库已经推送了最新的自动化文件
4. GitHub Actions 没有被组织策略禁用

建议先在本地仓库执行一次：

```bash
make verify-release-automation
```

如果本地检查不通过，先不要配置 GitHub 仓库，先修代码和文档。

## Step 1：确认仓库里的关键文件已经存在

打开仓库，确认这些文件已经在默认分支里：

1. `.github/workflows/check-version-sync.yml`
2. `.github/workflows/create-release.yml`
3. `.github/workflows/release.yml`
4. `.github/workflows/publish-release-images.yml`
5. `backend/cmd/server/VERSION`

如果这些文件还没进仓库，先提交并推送代码，再继续。

## Step 2：开启 GitHub Actions 基本权限

进入 GitHub 仓库页面：

1. 打开 `Settings`
2. 点击左侧 `Actions`
3. 点击 `General`
4. 在 `Actions permissions` 中选择允许 Actions 运行
5. 在 `Workflow permissions` 中选择 `Read and write permissions`
6. 勾选允许 GitHub Actions 创建和批准 pull requests（如果组织要求）
7. 点击 `Save`

为什么这一步必须做：
- `create-release.yml` 需要推送 tag
- `release.yml` 需要创建 GitHub Release
- `publish-release-images.yml` 需要写入 GitHub Packages

## Step 3：确认 GitHub Container Registry 可用

Sub2API 默认一定会推 GHCR，即使你还没配置 Docker Hub。

你需要确认：

1. 仓库没有被组织策略禁止写入 GitHub Packages
2. 仓库 owner 有权限发布到 `ghcr.io/<owner>/sub2api`

最常见情况里，不需要额外手动创建 GHCR 仓库，首次推送时会自动生成。

## Step 4：配置 Docker Hub Secrets

如果你希望自动同步推送 Docker Hub，继续这一节；如果只用 GHCR，可以跳到 Step 6。

先准备：

1. Docker Hub 用户名
2. Docker Hub Access Token

### 4.1 获取 Docker Hub Access Token

根据 Docker 官方文档，当前推荐做法是使用 Docker Hub 的 Personal Access Token，而不是直接把账号密码放进 GitHub Secrets。

官方文档：
- https://docs.docker.com/docker-hub/access-tokens/

按这个顺序操作：

1. 打开 `https://app.docker.com/`
2. 登录你的 Docker 账号
3. 点击右上角头像
4. 进入 `Account settings`
5. 点击 `Personal access tokens`
6. 点击 `Generate new token`
7. 在描述里写一个容易识别的名字
   - 推荐：`sub2api-github-actions`
8. 选择过期时间
   - 如果你想更稳一点，可以选一个相对长但可轮换的周期
9. 设置访问权限
   - 对这个仓库，推荐至少使用 `Read & Write`
   - 原因：workflow 需要登录 Docker Hub 并推送镜像
10. 点击 `Generate`
11. 立刻复制 token 并保存在安全的密码管理器里

注意：

1. Docker 只会在创建成功的当下展示一次 token
2. 关闭弹窗后，通常无法再次查看原文
3. 如果你丢失了 token，正确做法是删除旧 token，再重新生成一个新的

### 4.2 这个仓库推荐的 token 命名与权限

推荐这样配：

1. Token name：`sub2api-github-actions`
2. Expiration：按你们团队的轮换周期来定
3. Permission：`Read & Write`

为什么不是更低权限：

1. 只读权限不能推送镜像
2. 这个仓库的 GitHub Actions 需要执行 `docker login` 和 `docker push`

为什么不建议直接用密码：

1. 官方推荐 PAT 替代密码
2. 更容易单独吊销
3. 更适合 CI/CD 自动化
4. 开了 2FA 时，CLI/自动化场景通常也应使用 token

然后进入 GitHub 仓库：

1. 打开 `Settings`
2. 点击左侧 `Secrets and variables`
3. 点击 `Actions`
4. 进入 `Secrets` 标签
5. 点击 `New repository secret`

依次创建：

1. `DOCKERHUB_USERNAME`
   - Value：你的 Docker Hub 用户名
2. `DOCKERHUB_TOKEN`
   - Value：上一步刚生成并复制的 Docker Hub Personal Access Token

建议：
- 不要用 Docker Hub 登录密码，优先用 Access Token
- Token 最好只授予这个仓库所需的最小权限

## Step 5：配置 Docker Hub Variables

继续在 GitHub 仓库：

1. 打开 `Settings`
2. 点击左侧 `Secrets and variables`
3. 点击 `Actions`
4. 进入 `Variables` 标签

你有两种配置方式，二选一即可。

### 方式 A：直接指定完整镜像名

新建：

1. `DOCKERHUB_REPOSITORY`
   - 例子：`weishaw/sub2api`

这是最直接、最推荐的方式。

### 方式 B：只指定 namespace

新建：

1. `DOCKERHUB_NAMESPACE`
   - 例子：`weishaw`

这种方式下，workflow 会自动拼成：

```text
<namespace>/<repo-name>
```

也就是：

```text
weishaw/sub2api
```

## Step 6：检查默认分支和 workflow 文件触发条件

确保你的发布分支是仓库默认分支，通常是 `main`。

你需要确认：

1. `check-version-sync.yml` 会在修改 `backend/cmd/server/VERSION` 或 `CHANGELOG.md` 时运行
2. `create-release.yml` 通过手动触发运行
3. `release.yml` 会在推送 `v*` tag 后自动运行
4. `publish-release-images.yml` 会每日定时运行，也支持手动指定 tag 运行

如果你修改了默认分支名，比如改成 `master`，要记得同步调整相关 workflow 的分支条件。

## Step 7：做一次最小化验证

推荐用一个真实但可控的小版本号做演练。

### 7.1 修改版本文件

编辑：

```text
backend/cmd/server/VERSION
```

把版本改成一个新的 semver，比如：

```text
0.1.112
```

### 7.2 更新变更记录

编辑：

```text
CHANGELOG.md
```

确保：

1. 有对应版本段落，或
2. `[Unreleased]` 里有可发布内容

### 7.3 本地验证

运行：

```bash
make verify-release-automation
```

预期：

1. 版本格式检查通过
2. workflow 存在检查通过
3. 文档检查通过

### 7.4 推送到默认分支

提交并推送你的改动。

推送后到 GitHub 的 `Actions` 页面，确认：

1. `Check Version Sync` 运行成功
2. 摘要里提示版本比最新 release 新，或者提示当前同步状态正常

## Step 8：手动触发创建 release tag

进入 GitHub 仓库：

1. 点击 `Actions`
2. 选择 `Create Release Tag`
3. 点击 `Run workflow`
4. 选择默认分支
5. 点击确认运行

预期结果：

1. workflow 读取 `backend/cmd/server/VERSION`
2. 自动创建 `vX.Y.Z` annotated tag
3. push tag 成功
4. 触发现有 `release.yml`

如果这个步骤提示 tag 已存在，说明这个版本已经发过，不需要再次创建。

## Step 9：观察主发布流程

继续在 `Actions` 页面查看 `Release` workflow。

你要确认这些关键步骤成功：

1. 前端构建成功
2. VERSION artifact 上传成功
3. GoReleaser 执行成功
4. GitHub Release 创建成功
5. GHCR 镜像发布成功
6. 如果配了 Docker Hub，Docker Hub 镜像也成功

发布成功后，你应该能在这些地方看到结果：

1. `Releases` 页面出现新版本
2. `Packages` 页面出现 GHCR 镜像
3. Docker Hub 上出现对应标签

## Step 10：验证镜像补发流程

这个步骤用于确认兜底机制真的可用。

进入 GitHub 仓库：

1. 点击 `Actions`
2. 选择 `Publish Release Images`
3. 点击 `Run workflow`
4. 如果要补指定版本，就填写 `tag`
5. 否则留空，默认处理最新 release

预期结果：

1. 如果镜像标签都已经存在，workflow 会直接跳过
2. 如果某些镜像标签缺失，workflow 会自动重建并推送
3. summary 会显示缺了哪些 tags、是否执行了补发

## Step 11：理解最终镜像标签规则

稳定版 release 会生成这些标签：

1. `x.y.z`
2. `latest`
3. `x.y`
4. `x`

预发布版本（例如带 `-rc.1`、`-beta.1`）只会保留精确版本标签，不会覆盖 `latest`。

这能避免测试版把稳定版标签冲掉。

## Step 12：常见问题排查

### 问题 1：`create-release.yml` 不能推 tag

排查顺序：

1. 检查 `Settings -> Actions -> General -> Workflow permissions` 是否是 `Read and write permissions`
2. 检查组织是否禁用了 tag push
3. 检查仓库分支保护规则是否附带限制

### 问题 2：GHCR 推送失败

排查顺序：

1. 检查仓库是否允许 GitHub Packages
2. 检查组织策略是否限制 package 发布
3. 检查 workflow 的 `packages: write` 权限是否还在

### 问题 3：Docker Hub 没有新镜像

排查顺序：

1. 检查 `DOCKERHUB_USERNAME` 和 `DOCKERHUB_TOKEN` 是否都已配置
2. 检查 `DOCKERHUB_REPOSITORY` 或 `DOCKERHUB_NAMESPACE` 是否配置正确
3. 检查 Docker Hub token 是否过期
4. 手动运行 `publish-release-images.yml` 看 summary 提示

### 问题 4：`Check Version Sync` 失败

排查顺序：

1. 检查 `backend/cmd/server/VERSION` 是否是合法 semver
2. 检查 `CHANGELOG.md` 是否有对应版本内容或有效 `[Unreleased]`
3. 检查你是不是把版本号改小了，导致落后于最新 release

### 问题 5：补发 workflow 一直跳过

这通常不是问题，表示镜像已经齐全。

如果你确定镜像缺失，却还在跳过：

1. 去镜像仓库确认真实 tag 名
2. 确认手动输入的 `tag` 是否带 `v`
3. 查看 workflow summary 里列出的 `required refs`

## 推荐的日常发布习惯

每次准备发版时，按这个顺序执行：

1. 修改 `backend/cmd/server/VERSION`
2. 更新 `CHANGELOG.md`
3. 本地运行 `make verify-release-automation`
4. 合并到默认分支
5. 手动运行 `create-release.yml`
6. 等待 `release.yml`
7. 必要时运行 `publish-release-images.yml`

## 相关文档

- 部署总说明：`deploy/README.md`
- Docker 镜像说明：`deploy/DOCKER.md`
- 发布自动化实施计划：`docs/plans/2026-04-13-release-automation-and-docker-publish.md`
