# TLS 指纹模板示例

本文档提供 3 个可直接通过管理 API 导入的 TLS 指纹模板配置。每个模板对应不同客户端的 TLS 握手特征，适用于不同的使用场景。

## 哪些上游需要 TLS 指纹

| 上游服务 | 是否需要 TLS 指纹 | 说明 |
|----------|-------------------|------|
| **Anthropic (Claude)** | **需要** | 仅限 OAuth 和 SetupToken 类型的账号 |
| OpenAI | 不需要 | — |
| Gemini | 不需要 | — |
| AWS Bedrock | 不需要 | — |

**原因**：Anthropic 对 Claude Code 的 OAuth / SetupToken 会话执行 TLS 指纹校验（JA3 / JA4 等），服务端会检查 ClientHello 中的加密套件顺序、扩展列表、曲线选择等特征是否与合法的 Node.js 客户端一致。如果指纹不匹配（例如使用 Go 标准库的默认 TLS 握手），请求会被拒绝或限流。

其他上游（OpenAI、Gemini、Bedrock）的 API 不做 TLS 指纹校验，使用标准 HTTP 客户端即可正常访问，无需配置 TLS 指纹。

因此，本页提供的 TLS 指纹模板**仅在对接 Anthropic (Claude) 时需要关注**。

## 模板字段说明

| 字段 | 类型 | 说明 |
|------|------|------|
| `name` | string | 模板唯一名称 |
| `description` | string | 模板描述 |
| `enable_grease` | bool | 是否插入 GREASE 值（Chrome 启用，Node.js 不启用） |
| `cipher_suites` | []uint16 | 加密套件列表，**顺序影响 JA3 指纹** |
| `curves` | []uint16 | 椭圆曲线/支持的组 |
| `point_formats` | []uint16 | EC 点格式 |
| `signature_algorithms` | []uint16 | 签名算法 |
| `alpn_protocols` | []string | ALPN 协议列表 |
| `supported_versions` | []uint16 | 支持的 TLS 版本 |
| `key_share_groups` | []uint16 | Key Share 中的曲线组 |
| `psk_modes` | []uint16 | PSK 密钥交换模式 |
| `extensions` | []uint16 | TLS 扩展 ID 列表，按发送顺序排列 |

所有数组字段留空（`null`）时，使用内置默认值（Node.js 24.x）。

## 导入方式

```bash
curl -X POST https://your-domain/api/v1/admin/tls-fingerprint-profiles \
  -H "Authorization: Bearer <ADMIN_API_KEY>" \
  -H "Content-Type: application/json" \
  -d @profile.json
```

---

## Node.js 24.x（Claude Code 默认）

AI API 网关的推荐默认模板。模拟 macOS ARM64 上 Node.js v24 的 TLS 握手，与 Claude Code 客户端特征一致。

- **JA3 Hash**: `44f88fca027f27bab4bb08d4af15f23e`
- **JA4**: `t13d1714h1_5b57614c22b0_7baf387fc6ff`
- **适用场景**: 对接 Anthropic Claude API（OAuth / SetupToken）时作为默认指纹使用
- **特征**: 不启用 GREASE；17 个加密套件；14 个扩展（含 ECH）；3 条曲线

```json
{
  "name": "macos_arm64_node_v24",
  "description": "Node.js v24.x (macOS ARM64) — Claude Code 默认指纹，JA3: 44f88fca027f27bab4bb08d4af15f23e",
  "enable_grease": false,
  "cipher_suites": [4865, 4866, 4867, 49195, 49199, 49196, 49200, 52393, 52392, 49161, 49171, 49162, 49172, 156, 157, 47, 53],
  "curves": [29, 23, 24],
  "point_formats": [0],
  "signature_algorithms": [1027, 2052, 1025, 1283, 2053, 1281, 2054, 1537, 513],
  "alpn_protocols": ["http/1.1"],
  "supported_versions": [772, 771],
  "key_share_groups": [29],
  "psk_modes": [1],
  "extensions": [0, 65037, 23, 65281, 10, 11, 35, 16, 5, 13, 18, 51, 45, 43]
}
```

**扩展顺序解读**:

| 序号 | ID | 扩展名称 |
|------|----|----------|
| 1 | 0 | server_name |
| 2 | 65037 | encrypted_client_hello (ECH) |
| 3 | 23 | extended_master_secret |
| 4 | 65281 | renegotiation_info |
| 5 | 10 | supported_groups |
| 6 | 11 | ec_point_formats |
| 7 | 35 | session_ticket |
| 8 | 16 | alpn |
| 9 | 5 | status_request (OCSP) |
| 10 | 13 | signature_algorithms |
| 11 | 18 | signed_certificate_timestamp |
| 12 | 51 | key_share |
| 13 | 45 | psk_key_exchange_modes |
| 14 | 43 | supported_versions |

---

## Node.js 22.x（Linux x64）

Linux x64 平台上 Node.js v22 的指纹。相比 v24，加密套件更多（OpenSSL 完整列表），扩展不含 ECH，适用于需要模拟 Linux 服务器端 Node.js 客户端的场景。

- **JA4 cipher hash**: `a33745022dd6`
- **适用场景**: 需要模拟 Linux 服务器上的 Node.js 客户端；部分上游对 ECH 扩展敏感时可作为降级选项
- **特征**: 不启用 GREASE；56 个加密套件（OpenSSL 完整列表）；11 个扩展（无 ECH）；10 条曲线

```json
{
  "name": "linux_x64_node_v22",
  "description": "Node.js v22.17.1 (Linux x64) — OpenSSL 完整加密套件列表，无 ECH 扩展，JA4 cipher: a33745022dd6",
  "enable_grease": false,
  "cipher_suites": [4866, 4867, 4865, 49199, 49195, 49200, 49196, 158, 49191, 103, 49192, 107, 163, 159, 52393, 52392, 52394, 49327, 49325, 49315, 49311, 49245, 49249, 49239, 49235, 162, 49326, 49324, 49314, 49310, 49244, 49248, 49238, 49234, 49188, 106, 49187, 64, 49162, 49172, 57, 56, 49161, 49171, 51, 50, 157, 49313, 49309, 49233, 156, 49312, 49308, 49232, 61, 60, 53, 47, 255],
  "curves": [29, 23, 30, 25, 24, 256, 257, 258, 259, 260],
  "point_formats": [0, 1, 2],
  "signature_algorithms": [1027, 1283, 1539, 2055, 2056, 2057, 2058, 2059, 2052, 2053, 2054, 1025, 1281, 1537, 771, 769, 770, 1058, 1282, 1538],
  "alpn_protocols": ["http/1.1"],
  "supported_versions": [772, 771],
  "key_share_groups": [29],
  "psk_modes": [1],
  "extensions": [0, 11, 10, 35, 16, 22, 23, 13, 43, 45, 51]
}
```

**与 Node.js 24.x 的关键差异**:

| 特征 | Node.js 24.x | Node.js 22.x |
|------|-------------|-------------|
| 加密套件数量 | 17 | 56 |
| 曲线数量 | 3 | 10 |
| 扩展数量 | 14 | 11 |
| ECH 扩展 | 有 (65037) | 无 |
| encrypt_then_mac (22) | 无 | 有 |
| 签名算法数量 | 9 | 20 |
| EC 点格式 | [0] | [0, 1, 2] |

---

## Chrome 131 Desktop（通用浏览器）

模拟 Chrome 131 桌面浏览器的 TLS 握手特征。启用 GREASE，使用浏览器特有的扩展顺序和加密套件组合。当上游服务对非浏览器指纹有限制或风控时使用。

- **适用场景**: 上游服务要求浏览器级别的 TLS 指纹；需要最大化与普通用户流量的一致性
- **特征**: 启用 GREASE；10 个核心加密套件；支持 h2 和 http/1.1 双协议；含 compress_certificate 扩展

```json
{
  "name": "chrome_131_desktop",
  "description": "Chrome 131 Desktop — 通用浏览器指纹，启用 GREASE，支持 h2/http1.1，适用于上游要求浏览器指纹的场景",
  "enable_grease": true,
  "cipher_suites": [4865, 4866, 4867, 49195, 49199, 52393, 52392, 49196, 49200, 49162, 49161, 49172, 49171, 157, 156, 47, 53],
  "curves": [29, 23, 24],
  "point_formats": [0],
  "signature_algorithms": [1027, 2052, 1025, 1283, 2053, 1281, 2054, 1537, 513],
  "alpn_protocols": ["h2", "http/1.1"],
  "supported_versions": [772, 771],
  "key_share_groups": [29],
  "psk_modes": [1],
  "extensions": [0, 65037, 23, 65281, 10, 11, 35, 16, 5, 13, 18, 27, 51, 45, 43]
}
```

**Chrome 特有配置说明**:

- **enable_grease: true**: Chrome 会在扩展列表首尾和加密套件中插入随机 GREASE 值，这是 Chrome 指纹的标志性特征
- **alpn_protocols**: Chrome 同时支持 h2 和 http/1.1，而 Node.js 只用 http/1.1
- **compress_certificate (27)**: Chrome 支持 certificate compression，这是现代浏览器的标志扩展
- **加密套件精简**: Chrome 只保留 10 个常用套件，比 Node.js 的 OpenSSL 更精练

**与 Node.js 24.x 的关键差异**:

| 特征 | Node.js 24.x | Chrome 131 |
|------|-------------|------------|
| GREASE | 关闭 | 开启 |
| ALPN | http/1.1 | h2, http/1.1 |
| 加密套件数量 | 17 | 10 |
| compress_certificate (27) | 无 | 有 |
| 指纹风格 | 服务端/CLI 工具 | 桌面浏览器 |
