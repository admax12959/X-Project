# IAM 模块调研：邮箱登录、SMTP、发信、二次验证（TOTP）

调研日期：2026-10-14。依据一手来源（RFC、NIST SP 800-63B 第 4 版、OWASP 速查表、库源码与 GitHub 元数据），见文末 Sources。

背景约束（来自 `GLOSSARY.md`、ADR-0002、ADR-0004、`docs/design/v1-scope.md`）：

- 一个用户挂多个登录身份（LDAP、本地密码），追加身份必须用新身份完成一次认证或由管理员绑定，禁止按邮箱自动合并。
- 绑定了 LDAP 的用户，邮箱以 LDAP 为准、登录时同步、本人不可改；当前没有"邮箱在用户间唯一"的规则。
- 首次启动按配置创建一个本地初始管理员。
- 界面中英双语，用户可切换。

---

## 1. TOTP

### 1.1 参数

| 项 | 建议 | 依据 |
|---|---|---|
| 算法 | HMAC-SHA1 | Google Authenticator Key URI 文档写明：GA 的实现会忽略 `algorithm`，Android/BlackBerry 版还会忽略 `digits`，并且忽略 `period`（文档较旧，但这是生态里最低的兼容基线）。SHA1 在 HMAC 里使用并不受 SHA1 碰撞问题影响。 |
| 位数 | 6 | NIST 允许截断到 6 位（800-63B §3.1.4.1 / §3.1.5.1）。 |
| 步长 | 30 秒 | RFC 6238 §5.2 推荐默认 30 秒；NIST 只要求至少每 2 分钟变一次。 |
| 密钥长度 | 20 字节（160 bit）随机数 | RFC 4226 R6 要求至少 128 bit，推荐 160 bit；RFC 6238 §5.1 建议与 HMAC 输出等长（SHA1 = 20 字节）；NIST 要求至少 112 bit。`pquerna/otp` 默认 `SecretSize = 20`。 |
| 时钟偏差窗口 | 前后各 1 步（±30 秒） | RFC 6238 §5.2："at most one time step is allowed as the network delay"；NIST 要求按预期时钟漂移定义有效期。服务器必须跑 NTP（Docker 宿主机同步即可）。 |

### 1.2 防重放（必须自己实现）

- RFC 6238 §5.2："The verifier MUST NOT accept the second attempt of the OTP after the successful validation"；NIST §3.1.5.2："Verifiers SHALL accept a given OTP only once while it is valid"。
- `pquerna/otp` v1.5.0 的 `totp.ValidateCustom` 只返回 `bool`，不告诉你命中的是哪个时间步；且 `ValidateOpts.Skew` 默认是 0，只有简化版 `totp.Validate()` 内置 `Skew: 1`。
- 做法：自己按 `t-1, t, t+1` 三个时间步循环调用 `totp.GenerateCodeCustom`，用常量时间比较（`crypto/subtle`），得到命中的时间步 `step`；然后对该用户的 TOTP 记录执行条件更新
  `UPDATE ... SET last_used_step = ? WHERE user_id = ? AND last_used_step < ?`，影响行数为 0 即视为重放。这样两个并发请求不会同时通过。
- 同时可以顺带拒绝"比上次更早的时间步"，避免窗口内回退。

### 1.3 失败限速

- NIST §3.1.5.2：多因素 OTP 的验证方 SHALL 实现限速；§3.2.2：连续失败不超过 100 次，超过就停用该认证器（可以更低）。
- OWASP：失败计数挂在用户上而不是 IP 上；可以用指数退避；注意防止被人恶意锁定他人。
- 建议：按用户计数，5 次失败后每次加指数延迟，累计例如 20 次停用 TOTP 并通知用户，只能走恢复码或管理员重置。
- 6 位码只有 10^6 种可能，只靠"每次登录重新输密码"不够，必须有用户级计数。

### 1.4 密钥存储（静态加密）

- NIST：对称密钥 SHALL 通过访问控制严格保护。TOTP 密钥必须能还原（验证时要用），所以不能哈希，只能加密。
- 建议：AES-256-GCM，主密钥来自环境变量或 Docker secret（不进数据库、不进镜像）；表里存 `key_version`，方便轮换；把行 ID / 用户 ID 作为 AEAD 的附加数据，防止密文被挪到别的行。
- SMTP 密码和其他服务令牌（例如代码平台只读令牌）复用同一套加密组件。

### 1.5 otpauth URI 与二维码

- 格式：`otpauth://totp/{Issuer}:{username}?secret={BASE32 无填充}&issuer={Issuer}&algorithm=SHA1&digits=6&period=30`。label 前缀和 `issuer` 参数都写，两者一致（Key URI 文档推荐）。label 用用户名，不用邮箱（LDAP 用户的邮箱会变）。
- 二维码在服务端生成 PNG（`key.Image()`）或前端用 JS 库渲染都可以；同时显示 Base32 文本供手动输入。URI 和二维码只在启用流程里返回一次，不写日志、不缓存（`Cache-Control: no-store`）。

### 1.6 启用确认流程

1. 用户在个人设置点"启用两步验证"，先重新认证（输当前密码，或 LDAP 用户用 LDAP 密码），这是 OWASP 对敏感操作的要求。
2. 服务端生成密钥，存为"待确认"状态（加密，带过期时间如 10 分钟），返回 URI 和二维码。
3. 用户输入当前 6 位码，服务端校验通过后才把状态改为"已启用"，并记录 `last_used_step`。
4. 同时生成并展示恢复码（只展示一次），要求用户确认已保存。
5. 通过独立渠道通知用户（邮件）："你的用户已启用两步验证"（NIST §4.1.2.1 / §4.6 要求绑定新认证器时通知）。
6. 写审计日志。

### 1.7 恢复码

- NIST §4.2.1.1（saved recovery codes）：每个码至少 64 bit 熵，来自批准的随机数生成器；用批准的单向函数以哈希形式存储；校验受 §3.2.2 限速约束；用掉后作废。
- 建议：一次生成 10 个，每个 16 个 Base32 字符（80 bit，显示成 `XXXX-XXXX-XXXX-XXXX`）；存储用 SHA-256 即可（熵足够高，不需要慢哈希），也可以直接复用密码哈希组件。用一个少一个，剩余不多时提示重新生成；重新生成会作废全部旧码并发通知。
- 使用恢复码登录后提示用户检查两步验证设置并通知用户（NIST §4.2 要求恢复时通知）。

### 1.8 Go 库

| 库 | 状态（2026-10-14 查询 GitHub） | 结论 |
|---|---|---|
| `github.com/pquerna/otp` | v1.5.0（2025-05-16），最后推送 2025-08-07，未归档，Apache-2.0，约 3k star | 推荐。更新慢，但 TOTP/HOTP 规范已冻结，够用。需要自己补防重放（见 1.2）。 |
| `github.com/xlzd/gotp` | 最后推送 2022-10 | 不推荐，基本停更。 |
| 自己实现 | RFC 4226/6238 核心约 50 行 HMAC 代码，RFC 附录有测试向量 | 备选；如果 pquerna 出问题可以替换，接口隔离在一个小模块里即可。 |

---

## 2. 邮箱作为登录方式

### 2.1 三种方式对比

| 方式 | 说明 | 评价 |
|---|---|---|
| 邮箱 + 密码 | 邮箱只是本地密码身份的另一个标识符（可以用用户名或邮箱登录） | 最简单，和现有本地密码身份一致。OWASP 认可把已验证的邮箱当用户名。 |
| 魔法链接 | 邮件里发一次性登录链接 | 安全性完全等于邮箱的安全性；链接会被邮件安全网关预取（点击扫描）导致失效或被消费；跨设备体验差。内网工具不推荐。 |
| 邮箱验证码登录 | 邮件发 6 位码，输入即登录 | 同样等于邮箱安全性；NIST 不承认它是认证器（见下）。 |

### 2.2 NIST 对邮件验证码的立场（重点）

- 800-63B 第 4 版 §3.1.3.1："Email SHALL NOT be used for out-of-band authentication"，理由：邮箱往往只靠密码保护、传输和邮件服务器上可能被截获、可能被 DNS 欺骗等重定向。
- 例外：用于**验证邮箱地址**或作为**发放的恢复码**（issued recovery codes）的邮件验证码"不是认证过程"，不受此禁令约束。
- 结论：**邮件验证码 / 魔法链接不能算作第二因素，也不应作为 MFA 选项提供。** 可以用于：绑定或修改邮箱时的验证、忘记密码、认证恢复（NIST 称 account recovery）中的"发放恢复码"（NIST §4.2.1.2：发到邮箱的码最长有效 24 小时，至少 6 位）。
- OWASP MFA 速查表同样说邮件"是否算 MFA 有争议"，只是为完整性列出。

### 2.3 与现有 ADR 的冲突点（必须先决定）

1. **邮箱唯一性**：如果邮箱能用来登录，就必须在"可登录的邮箱"范围内唯一。当前用户字段里的邮箱没有唯一约束，LDAP 同步也可能把两个用户同步成同一个邮箱。
2. **LDAP 同步改邮箱**：LDAP 用户的邮箱登录时会被覆盖。如果它被用作登录标识，LDAP 侧改邮箱就等于改了登录名；如果同步来的邮箱和另一个用户的登录邮箱冲突，需要规定行为（建议与 ADR-0004 同样处理：拒绝同步该字段或拒绝登录，交给管理员）。
3. **建模选择**：
   - 方案 A（推荐）：邮箱不是新的登录身份类型，而是本地密码身份的另一个登录标识。只有"已验证"且唯一的邮箱可以用来登录。LDAP 用户继续走 LDAP。
   - 方案 B：邮箱验证码作为一种新的登录身份。它等于一个单因素、可被邮箱接管的身份，而且与 ADR-0002"不按邮箱合并"的精神有张力；不推荐。
4. **绑定 / 修改邮箱必须验证**：向新邮箱发验证码，输入后才生效。这正好符合 ADR-0002"追加身份必须用新身份完成一次认证"。OWASP 建议修改邮箱时：先重新认证（有 MFA 则要求 MFA），新邮箱存为待生效，给新邮箱发确认、给旧邮箱发通知。

### 2.4 防枚举与限速

- OWASP Authentication：登录、找回密码、注册统一返回通用提示（如"如果该邮箱已登记，我们会发送邮件"），HTTP 状态码也要一致；响应时间一致（用户不存在时也走一次哈希；发邮件异步化，不让"是否真的发了"影响耗时）。
- OWASP Forgot Password：令牌用 CSPRNG 生成、足够长、绑定用户、哈希存储、一次性、短时有效；按用户限制发信频率（防止邮箱被轰炸），按令牌限制猜测次数；**在出示有效令牌之前不改变用户状态**（不锁定）；重置后不自动登录、使已有会话失效、发确认邮件。
- **链接里的域名不要用请求的 Host 头拼接**（Host 头注入），用管理员配置的"站点外部地址"。重置页设置 `Referrer-Policy: no-referrer`。
- 建议数值：邮件验证码 6 位、10 分钟有效、最多 5 次尝试；同一用户 60 秒内只能发一封、每小时上限约 5 封；同一 IP 再加一个宽一些的上限。

---

## 3. MFA 策略

### 3.1 谁来强制

- 建议三层：用户自愿启用 → 管理员全局强制（所有用户）→ 按角色强制（至少"管理员"角色强制）。v1 没有"Owner/PE 之外的全局角色"，所以按角色只需要先支持"管理员必须启用"。
- 强制启用时：用户下次登录完成第一因素后，进入"必须设置两步验证"页面，完成前只允许访问启用流程（可选给一个宽限期，例如 N 次登录或 N 天）。
- NIST §2.2.2：AAL2 的验证方 SHALL 至少提供一种**防钓鱼**认证选项，TOTP 不算防钓鱼。这给第二阶段做 WebAuthn 提供了理由（见 3.5）。

### 3.2 MFA 挂在用户上，不挂在登录身份上

- 一个用户有 LDAP 和本地密码两个登录身份。如果 MFA 只挂在某个身份上，攻击者走另一个身份就绕过了。OWASP："A common area that is missed is if the application provides a separate API that can be used to login"——所有登录路径都要一样保护。
- 所以：**LDAP 登录也需要 MFA**。LDAP bind 只证明了"知道密码"这一个因素，第二因素由本工具在用户层面统一要求。
- 同理，任何 API 令牌 / 个人访问令牌（如果以后有）不能成为绕过 MFA 的后门。

### 3.3 登录流程中的中间状态

1. 第一因素（LDAP 或本地密码）通过后，如果用户启用了 MFA，只发一个短期（如 5 分钟）、只能调用 MFA 校验接口的"预认证令牌"，不发正式会话。
2. MFA 通过后再发正式会话，并**轮换会话 ID**（OWASP：重新认证后使会话失效并轮换令牌）。
3. MFA 失败计数按用户累计（见 1.3），不是按预认证令牌。

### 3.4 管理员重置 MFA 与破窗

- 管理员可以重置某个用户的 MFA（清除 TOTP 和恢复码），用户下次登录重新启用。这在 NIST 的框架里属于认证恢复（NIST 称 account recovery），应当：重置前管理员线下核实身份；重置后通知用户（NIST §4.6）；写审计日志。管理员不能重置自己（避免单人绕过），自己的需要另一个管理员处理。
- **初始管理员锁死风险**：v1 只有一个从配置创建的本地初始管理员。如果强制管理员 MFA 而他丢了手机，又没有其他管理员，就无人能解。建议提供破窗手段：容器内 CLI 命令（`docker exec <容器> app admin reset-mfa <username>`），只有能登录宿主机的人可用，执行时写审计日志并通知该用户。
- 修改 MFA 设置（关闭、重新绑定、重新生成恢复码）都要求先用已绑定的因素重新认证（OWASP："Do not rely solely on the active session"）。

### 3.5 "记住此设备"

- NIST §5.1："Cookies and similar 'remember my browser' features SHALL NOT be used instead of authentication"，唯一例外是 AAL2 会话的再认证（§2.2.3：AAL2 总时长建议不超过 24 小时、空闲超时建议不超过 1 小时，空闲超时后可以只用密码加会话密钥再认证）。
- OWASP 只把"新设备 / 新位置时要求 MFA"作为基于风险的认证提及，位置、IP 只是风险信号，不是因素。
- 建议：v1 **不做**"记住设备 30 天免 MFA"。如果业务上一定要做，把它明确记录为对 NIST 的有意偏离（写 ADR），并且：期限有界（如 ≤ 30 天）、服务端存储可撤销、用户可在设置里看到并撤销、密码变更 / MFA 变更 / 管理员重置时全部失效；管理员角色不允许使用。
- 更符合 NIST 的折中：会话本身最长 24 小时，空闲超时内续期，不额外"记住设备"。

### 3.6 WebAuthn / 通行密钥（后续）

- 唯一主流的防钓鱼方式；NIST 要求 AAL2 至少提供一种防钓鱼选项。可同步的通行密钥可以满足 AAL2，但不满足 AAL3（§3.2.13：可同步密钥本质上可导出）。
- Go 库：`github.com/go-webauthn/webauthn`，v0.18.2（2026-09-19），2026-10 仍在活跃提交。
- 数据模型现在就预留：一个用户可以有多个"第二因素"（TOTP 至多一个，WebAuthn 凭据多个，恢复码一组），而不是在用户表上加一个 `totp_secret` 字段。

---

## 4. SMTP 配置与发信

### 4.1 配置项与加密模式

| 端口 / 场景 | 模式 | `go-mail` 设置 |
|---|---|---|
| 465 | 隐式 TLS（SMTPS） | `mail.WithSSL()`（或 `WithSSLPort(false)`） |
| 587 | STARTTLS，必须加密 | `mail.WithTLSPortPolicy(mail.TLSMandatory)` |
| 25，内网中继，无认证 | 明文或尽力加密 | `mail.TLSOpportunistic` 或 `mail.NoTLS`；只有管理员显式勾选"允许不加密"时才能用 |

- 管理员可配：主机、端口、加密模式（SSL/TLS、STARTTLS、无）、是否认证、用户名、密码、认证方式（默认自动协商 `SMTPAuthAutoDiscover`，可手选 PLAIN / LOGIN / CRAM-MD5）、发件人地址与名称、超时、可选的自定义 CA 证书（内网自签证书常见）。
- **不要提供"跳过证书校验"**作为常规选项；内网自签就让管理员上传 CA。
- `go-mail` 的 `PLAIN` / `LOGIN` 默认拒绝在未加密连接上发送凭据，另有 `PLAIN-NOENC` / `LOGIN-NOENC` 允许明文；这两个也应该只在管理员显式选择"无加密"时使用，界面要有明确警告。
- 另需"站点外部地址"配置项（邮件中的链接用它拼接，见 2.4）。

### 4.2 SMTP 密码

- 用 1.4 的同一套 AES-256-GCM 加密存储。
- API 只写不读：GET 配置时返回 `password_set: true`，不返回密码；PUT 时字段为空表示"不修改"。
- 修改 SMTP 配置写审计日志（不记录密码）。

### 4.3 测试发送

- 配置页提供"发送测试邮件"按钮，收件人默认是当前管理员的邮箱，可改。
- 允许"用表单里尚未保存的配置测试"，以便先测后存；测试时带短超时（如 15 秒），用 `DialAndSendWithContext`。
- 把 SMTP 错误原样（去掉凭据）返回给管理员，常见问题如证书不受信任、认证失败、端口与加密模式不匹配要能看出来。打开 `go-mail` 的调试日志时注意不要把 AUTH 内容写进日志。
- 测试接口本身限速，防止被用来扫描内网端口（SSRF 风险：只有管理员可调用即可接受）。

### 4.4 发送

- 发信不要放在请求路径上同步执行：写入 MySQL 的发件队列表，后台 worker 发送、失败重试（指数退避、上限次数），记录状态。这同时满足 2.4 的"响应时间一致"。
- 验证码、重置令牌只放在邮件正文，不写日志、不存明文（队列表里存渲染后的正文时，发送成功后清除正文或只存模板名和参数的脱敏版本）。

### 4.5 Go 库

| 库 | 状态 | 结论 |
|---|---|---|
| `github.com/wneessen/go-mail` | v0.8.1（2026-07-09），2026-09 仍有提交，MIT，依赖只有标准库和 `golang.org/x` | 推荐。支持隐式 SSL、STARTTLS 策略、多种 SASL（含 SCRAM、XOAUTH2、自动协商）、`context`、`html/template` / `text/template` 正文、DKIM / S/MIME。 |
| 标准库 `net/smtp` | 官方已冻结，不再加新功能 | 不推荐，需要自己处理隐式 TLS、MIME、编码。 |

### 4.6 模板与中英双语

- 模板用 Go `embed` 打进二进制，`html/template`（自动转义）生成 HTML 部分，`text/template` 生成纯文本部分，同时发送（multipart/alternative）。
- 每类邮件一对模板：`verify_email`、`password_reset`、`mfa_enabled`、`mfa_reset_by_admin`、`recovery_codes_regenerated`、`new_login_notice`（可选）、`smtp_test`；每个各有 `zh`、`en` 版本。
- 术语按 `GLOSSARY.md`（中英文名）。
- **依赖项**：要按收件人的语言发邮件，就必须在服务端保存用户的语言偏好。`v1-scope.md` 只说"用户可切换"，没写是否存到服务端；如果目前只存在浏览器里，需要在用户表加 `locale` 字段（缺省回退到中文或站点默认语言）。

---

## 5. 建议的数据模型草图（供设计参考，非定稿）

- `user_mfa_totp`：user_id（唯一）、secret_ciphertext、key_version、status（pending/active）、last_used_step、created_at、confirmed_at。
- `user_recovery_codes`：user_id、code_hash、used_at。
- `user_mfa_failures`（或放在用户表的计数字段）：失败计数、锁定截止时间。
- `email_verifications` / `password_resets`：user_id、purpose、token_hash、target_email、expires_at、attempts、used_at。
- `smtp_settings`：单行；password_ciphertext、key_version 等。
- `mail_outbox`：模板名、语言、收件人、状态、重试次数、错误信息。
- 用户表新增：`email_verified_at`、`locale`、`mfa_required`（或由策略计算）。

## 6. 候选术语（未写入 GLOSSARY，供 domain-modeling 讨论）

- **两步验证 / 第二因素 (Second Factor)**：在登录身份认证通过后，对用户额外要求的验证；挂在用户上，不挂在登录身份上。
- **恢复码 (Recovery Code)**：启用两步验证时生成的一次性码，丢失第二因素时代替它。
- **邮件服务器配置 (SMTP Settings)**：管理员配置的发信参数。
- 需要决定：邮箱登录是否算一种新的"登录身份"（本报告建议不算）。

---

## Sources

- RFC 6238, TOTP: Time-Based One-Time Password Algorithm — https://datatracker.ietf.org/doc/html/rfc6238
- RFC 4226, HOTP: An HMAC-Based One-Time Password Algorithm — https://datatracker.ietf.org/doc/html/rfc4226
- NIST SP 800-63B-4, Digital Identity Guidelines: Authentication and Authenticator Management — https://pages.nist.gov/800-63-4/sp800-63b.html
- Google Authenticator Key URI Format — https://github.com/google/google-authenticator/wiki/Key-Uri-Format
- OWASP Multifactor Authentication Cheat Sheet — https://cheatsheetseries.owasp.org/cheatsheets/Multifactor_Authentication_Cheat_Sheet.html
- OWASP Authentication Cheat Sheet — https://cheatsheetseries.owasp.org/cheatsheets/Authentication_Cheat_Sheet.html
- OWASP Forgot Password Cheat Sheet — https://cheatsheetseries.owasp.org/cheatsheets/Forgot_Password_Cheat_Sheet.html
- pquerna/otp — https://github.com/pquerna/otp （源码 `totp/totp.go`）, https://pkg.go.dev/github.com/pquerna/otp
- xlzd/gotp — https://github.com/xlzd/gotp
- wneessen/go-mail — https://github.com/wneessen/go-mail （源码 `tls.go`、`auth.go`、`client.go`）, https://pkg.go.dev/github.com/wneessen/go-mail
- go-webauthn/webauthn — https://github.com/go-webauthn/webauthn
- Go `net/smtp`（frozen）— https://pkg.go.dev/net/smtp
