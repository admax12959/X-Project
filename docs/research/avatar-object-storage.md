# 用户头像上传 + 可配置对象存储 + 防盗刷 调研

- 查阅日期:2026-10-14(云厂商文档和 SDK 行为可能变化,一律以当时官方页面为准)
- 背景:内部 Web 工具,Go 后端 + React SPA,MySQL 8,Docker 部署在内网。头像存储后端由管理员配置:阿里云 OSS / S3 兼容 / MinIO / 腾讯云 COS / 本地磁盘。
- 依据:只用官方文档、RFC、官方 SDK 仓库和 pkg.go.dev。网页内容经抓取工具摘要,其中的数值另读原页面复核过的标"已复核";只来自搜索摘要、没读原页面的标"未核实";没有出处的结论标"常识 / 未引用"。
- 引用:`[Sx]` 指向第 7 节的来源。

---

## 1. 结论(推荐方案)

1. **上传走后端代理,不走浏览器直传。** 头像只有几百 KB,而且必须解码、去 EXIF、裁剪、重编码(第 5 节)。浏览器直传会把**未处理的原始字节**直接写进桶,后端还得再拉回来处理一遍,反而多一步。后端代理还能顺便免掉内网 Host / CORS / 本地盘无法预签名这些麻烦(第 3 节)。
2. **下载走后端代理:鉴权 + 带版本号的 URL + 强缓存。** 桶设为私有并开启阻止公共访问;浏览器访问 `/api/avatars/{userId}?v={hash}`,后端校验会话后从存储读出来返回,响应头带 `Cache-Control: private, max-age=31536000, immutable`、`X-Content-Type-Options: nosniff` 和准确的 `Content-Type`。桶没有公网 URL,也就没有可被盗刷的公网流量。签名 URL 只作为可选优化,并且要按固定时间窗取整(第 4 节)。
3. **存储抽象:一个小接口 + 两个实现。** 接口只包含 `Put / Open / Delete / List`,预签名作为可选能力。`S3Store` 一个实现覆盖 AWS S3、阿里云 OSS、COS、MinIO;另有 `LocalStore`。管理员需要能配:endpoint、region、bucket、AK/SK、寻址方式(path-style 或 virtual-hosted)、校验和兼容开关(第 2 节)。
4. **Referer 白名单只能当补充,不能当防线**(第 4.3 节)。
5. **防止上传刷爆存储:** 每个用户只保留一个当前头像对象(换头像后删掉旧对象)、对上传接口按用户限流、请求体上限、定期清理孤儿对象;云上再加流量和存储告警。

---

## 2. 存储抽象

### 2.1 S3 兼容覆盖面

| 后端 | S3 兼容情况 | 注意点 |
|---|---|---|
| AWS S3 | 原生 | aws-sdk-go-v2 的 `service/s3` 从 v1.73.0(2025-01-15)起默认在 Put 上附加 CRC32 校验和、在 Get 上做校验,可能和第三方 S3 实现不兼容;可把 `RequestChecksumCalculation` / `ResponseChecksumValidation` 设为 `WhenRequired` 关闭 [S8] |
| 阿里云 OSS | 兼容 PutObject / GetObject / HeadObject / DeleteObject(s) / PostObject / ListObjects(V2) / Multipart / Lifecycle 等 [S5] | **只支持 virtual-hosted 寻址**,path-style 可能失败 [S5];ACL 只有 private / public-read / public-read-write 三种 [S5];PUT 返回的 ETag 是大写 [S5] |
| 腾讯云 COS | 官方提供"使用 AWS S3 SDK 访问 COS",endpoint 形如 `https://cos.<region>.myqcloud.com` [S9] | aws-sdk-go-v2 需要 `UsePathStyle=false`,并把 `RequestChecksumCalculation` 设为 WhenRequired(用来关闭 x-amz-trailer)[S9];桶名带 APPID 后缀,如 `examplebucket-1250000000` [S9];来源是搜索摘要,**未核实**原页面 |
| MinIO | S3 兼容 | 自建时通常用 path-style(常识 / 未引用);生命周期语法沿用 S3 [S12] |

### 2.2 Go 库选择

- **minio-go v7**(Apache-2.0):README 原文 "provides straightforward APIs to access any Amazon S3 compatible object storage" [S6];有 `PresignedGetObject / PresignedPutObject / PresignedHeadObject / PresignedPostPolicy` [S6],`PostPolicy` 有 `SetContentLengthRange / SetContentType / SetExpires` [S7]。寻址方式通过 `Options.BucketLookup` 设置(`BucketLookupAuto / DNS / Path`),**未核实**:godoc 页面被截断,没读到定义。
- **aws-sdk-go-v2**:`PresignClient` 有 `PresignPutObject` 和 `PresignPostObject` [S10](`PresignPostOptions` 的字段**未核实**);接第三方存储时需要设置 `BaseEndpoint`、`UsePathStyle` 和 checksum 开关 [S8][S9]。
- **建议**:两者都能覆盖全部 S3 兼容后端。aws-sdk-go-v2 接第三方存储时必须把 checksum 两项设为 `WhenRequired` [S8][S9];minio-go 的 API 更小,自带 PostPolicy,**倾向 minio-go**。minio-go 是否也有类似的默认 checksum 行为,**未核实**,两种库都要用 CI 对 MinIO 容器做集成测试。不管用哪个,都要在管理后台提供"测试连接"按钮:实际 Put、Get、Delete 一个探针对象。

### 2.3 本地磁盘兜底

- 存放在 Docker volume 里,不放在静态资源目录下;由后端通过 ID 映射读取文件,不让 Web 服务器直接按路径暴露 [S1](OWASP:存放在 webroot 之外,通过 handler 把 ID 映射到文件)。
- 文件名由服务端生成(UUID 或内容哈希),不使用用户提供的文件名 [S1]。
- 本地盘没有预签名能力,这也是把预签名设计成"可选能力"的原因。

### 2.4 建议接口(示意)

```go
type ObjectStore interface {
    Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) error
    Open(ctx context.Context, key string) (io.ReadCloser, ObjectInfo, error)
    Delete(ctx context.Context, key string) error
    List(ctx context.Context, prefix string, fn func(ObjectInfo) error) error // 孤儿清理用
}
// 可选:type Presigner interface { PresignGet(ctx, key string, ttl time.Duration) (string, error) }
```

---

## 3. 上传路径:浏览器直传 vs 后端代理

| | 浏览器直传(预签名 PUT / POST policy) | 后端代理(推荐) |
|---|---|---|
| 大小限制 | POST policy 可以带 `["content-length-range", min, max]`,也可以用 `starts-with $Content-Type image/` 和 `starts-with $key` 限定前缀 [S3][S13];**预签名 PUT 没有大小区间限制**(常识 / 未引用);同一 URL 在有效期内可以重复使用,会覆盖同一个 key [S2][S4] | `http.MaxBytesReader` 硬性截断 |
| 内容可信度 | 进桶的是原始字节;Content-Type 由客户端声明,可以伪造 [S1] | 先解码、重编码,再落盘 |
| 内网部署 | SigV4 签名包含 Host。后端用 `minio:9000` 访问,而浏览器用另一个域名时,签名会失效,需要单独配置"对外 endpoint"(常识 / 未引用);每种后端都要配 CORS;本地盘无法使用 | 浏览器只访问自家后端,与存储类型无关 |
| 带宽 / 性能 | 节省后端带宽,适合大文件 | 头像只有几百 KB,可以忽略 |
| 复杂度 | 预签名、CORS、上传完成回调,还要再拉回来处理 | 一个 multipart 接口 |

阿里云官方的直传方案(服务端生成 PostObject policy 和签名,推荐使用 V4 签名)也依赖 policy 里的 `content-length-range` 和 `in $content-type` 来限制大小和类型;不设置时 PostObject 最大允许 5 GB [S13](搜索摘要,**未核实**原页面)。

**结论**:头像场景用后端代理。直传留给以后的大附件功能。

---

## 4. 下载与防盗刷

### 4.1 威胁模型
"盗刷"有两种:① 公网流量被刷,产生高额账单;② 上传接口被刷,把存储写满。阿里云明确说明恶意流量产生的费用需要用户**自行承担**,并且按量计费无法在余额为 0 时立即停机 [S11]。

### 4.2 方案对比

| 方案 | 效果 | 缺点 |
|---|---|---|
| **私有桶 + 后端代理 + 会话鉴权**(推荐) | 桶不暴露公网,没有可盗刷的公网 URL;配合带版本号的 URL 加 `immutable`,浏览器基本只会请求一次 | 流量经过后端;对内网工具来说可以接受 |
| 私有桶 + 短时签名 URL | 链接会过期。AWS 说明签名 URL 是 bearer token,有效期内谁拿到都能用,可以多次使用 [S4];SDK 最长 7 天 [S4];阿里云控制台默认 3600 s、最长 32400 s(已复核),SDK 可以签更长 [S16] | URL 每次渲染都会变化,`<img>` 缓存失效;需要把过期时间按固定时间窗(比如按小时)取整才能复用缓存。需要公网 endpoint 时才有意义 |
| Referer 白名单 | 能挡住一般的第三方网页嵌入 | 见 4.3 |
| CDN URL 鉴权 | 适合公网大流量 | 内网工具一般不需要;开启私有桶回源后 CDN 缓存默认是公开的,需要在 CDN 层另配鉴权和 Referer [S17](搜索摘要,**未核实**) |

### 4.3 为什么 Referer 不可靠
- 阿里云文档自己的测试命令就是用 `curl -e "http://www.aliyun.com"` 伪造 Referer [S14]。浏览器禁止脚本修改 Referer [S15],但非浏览器客户端可以随意填写。
- 带 `Authorization` 头的请求**不受**防盗链规则约束 [S14];CDN 缓存命中时会绕过 OSS 的检查,需要在 CDN 层再配一遍 [S14]。
- 空 Referer 的处理左右为难:不允许空 Referer 会拦掉书签、邮件链接和 `no-referrer` 策略的正常访问;允许的话,curl 不带 Referer 就能直接通过 [S14]。

### 4.4 云上兜底(只在使用公有云 OSS / COS 时需要)
- 开启"阻止公共访问"并把 ACL 设为 private [S11]。
- 用云监控对公网流出流量设阈值告警(官方示例:1 分钟内 ≥ 100 MB 即告警)[S11],并设置费用预警。
- 对象名不要用可遍历的顺序 ID,改用哈希前缀 [S11]。
- 在内网通过 VPC / PrivateLink 访问 OSS,不走公网 [S11]。
- 存储侧硬上限(纵深防御,应用层已经保证每人只有一个对象):MinIO 的 bucket quota(`mc quota set`)、OSS 的单链接限速 `x-oss-traffic-limit` 和 Bucket 级 QoS 限流,**未核实**,上线前要查对应文档。

### 4.5 应用层限流
- 上传:按用户限流(例如每分钟 5 次、每天 50 次),外加全局并发上限。OWASP 要求上传者经过鉴权、上传接口有 CSRF 防护 [S1]。
- 下载:OWASP 建议对下载也做限流以防 DoS [S1];因为有 `immutable` 缓存,正常流量很小,可以把阈值设得宽松一些。

---

## 5. 图片安全处理流水线(Go)

按顺序:

1. **限制请求体**:`http.MaxBytesReader`,例如 5 MB。OWASP 要求设置上传大小上限 [S1]。
2. **嗅探魔数**:用 `http.DetectContentType` 或 `image.DecodeConfig` 返回的 format 判断,只允许 `jpeg / png / webp`。可以解码哪些格式,由空导入了哪些解码器决定(`image/jpeg`、`image/png`、`x/image/webp`)[S18];多导入 gif / bmp / tiff 就会扩大攻击面。客户端传的 Content-Type 和扩展名都不可信,魔数检查也不能单独使用 [S1]。
3. **防解压炸弹**:先 `image.DecodeConfig`(只读文件头,不分配完整像素缓冲)[S18],用 int64 计算 `w*h`,超过上限(例如 4096×4096,约 1600 万像素)就拒绝,然后才调用 `Decode`。Go 官方说明 `Decode` 可能按文件头声明的宽高分配内存,对不可信输入应先检查 [S18]。
4. **方向校正**:重编码会丢掉 EXIF,所以要先读取 EXIF Orientation 并旋转,否则手机竖拍的照片会横过来(常识 / 未引用;标准库不解析 EXIF,需要自己写一个小解析器,或者另选库,选型前先确认维护状态)。
5. **裁剪缩放**:居中裁成正方形,用 `golang.org/x/image/draw` 的 `CatmullRom`(慢但质量最好)或 `ApproxBiLinear`(速度和质量折中)缩放到固定尺寸,例如 256 和 64 [S19]。
6. **重编码**:用 `image/jpeg`(quality 约 85)或 `image/png` 输出。从 `image.Image` 重新编码天然不会带上 EXIF 和 GPS 等元数据。`x/image/webp` **只能解码**,不能编码 [S20],所以输出格式用 JPEG 或 PNG。OWASP 提醒:重编码不能保证清除所有恶意内容,图片库本身也在处理不可信输入,要及时升级 [S1]。
7. **拒绝 SVG**:SVG 是 XML,可以内嵌脚本;如果以 `image/svg+xml` 同源返回,就是存储型 XSS。OWASP 列出了 XML 炸弹、XXE 和客户端活动内容的风险 [S1](SVG 内嵌脚本这一点属于常识,OWASP 没有专门的 SVG 章节)。
8. **返回响应时**:设置固定的 `Content-Type: image/jpeg`、`X-Content-Type-Options: nosniff`;对象 key 由服务端生成 [S1]。

---

## 6. 生命周期

- **key 设计**:`avatars/{userId}/{sha256前16位}.jpg`。内容变化 key 就变化,URL 天然带版本,可以安全地使用 `immutable` 缓存。
- **替换顺序**:① Put 新对象 → ② 在事务里更新 MySQL 的 `users.avatar_key` → ③ 尽力删除旧对象,失败只记日志。顺序不能反过来,否则数据库可能指向一个不存在的对象。
- **孤儿清理**:定时任务对 `avatars/` 前缀执行 List,和数据库里正在引用的 key 做差集,只删除修改时间超过宽限期(例如 24 小时)的对象,避免误删正在上传的对象。也可以给 `tmp/` 前缀配置生命周期过期规则作为兜底:OSS 兼容 S3 的 Lifecycle API [S5],MinIO 语法沿用 S3 lifecycle,过期由低优先级扫描器执行,可能会有延迟 [S12]。本地盘需要自己实现同样的清理逻辑。
- **用户删除或停用**:删除该用户前缀下的所有对象。
- **LDAP 头像作为来源**:
  - AD 的 `thumbnailPhoto`:单值,上限 102400 字节(100 KB),微软建议使用 JPEG 或 GIF 等节省空间的格式 [S21]。
  - inetOrgPerson 的 `jpegPhoto`:JFIF 格式,**多值**(没有 SINGLE-VALUE)[S22],取第一个值即可。
  - 建议:管理员可以选择"头像来源 = 上传 / LDAP / 上传优先、LDAP 兜底"。登录同步时拿到的 LDAP 字节**同样走第 5 节的完整流水线**(它也是不可信输入,可能是损坏数据)。微软建议 thumbnailPhoto 用 JPEG 或 GIF [S21];为了不扩大攻击面,LDAP 里的非 JPEG / PNG 数据直接跳过,改用默认头像或用户上传的头像;用内容哈希判断是否变化,没变就不重写对象。

---

## 7. Sources

- [S1] OWASP File Upload Cheat Sheet — https://cheatsheetseries.owasp.org/cheatsheets/File_Upload_Cheat_Sheet.html
- [S2] AWS S3: Uploading objects with presigned URLs — https://docs.aws.amazon.com/AmazonS3/latest/userguide/PresignedUrlUploadObject.html
- [S3] AWS S3: Creating a POST policy (conditions, content-length-range) — https://docs.aws.amazon.com/AmazonS3/latest/API/sigv4-HTTPPOSTConstraints.html (抓取只拿到标题,条件写法以页面为准,**未逐字复核**)
- [S4] AWS S3: Download and upload objects with presigned URLs — https://docs.aws.amazon.com/AmazonS3/latest/userguide/using-presigned-url.html
- [S5] 阿里云 OSS 与 Amazon S3 的兼容性 — https://help.aliyun.com/zh/oss/developer-reference/compatibility-with-amazon-s3
- [S6] minio-go README — https://github.com/minio/minio-go
- [S7] minio-go v7 godoc — https://pkg.go.dev/github.com/minio/minio-go/v7
- [S8] aws-sdk-go-v2 公告:S3 默认数据完整性保护(v1.73.0)— https://github.com/aws/aws-sdk-go-v2/discussions/2960
- [S9] 腾讯云 COS:使用 AWS S3 SDK 访问 COS — https://cloud.tencent.com/document/product/436/37421
- [S10] aws-sdk-go-v2 service/s3 godoc — https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/s3
- [S11] 阿里云 OSS:降低因恶意访问流量带来的资金损失风险 — https://help.aliyun.com/zh/oss/user-guide/reduce-the-risks-of-unexpectedly-high-fees-caused-by-malicious-access-traffic
- [S12] MinIO 对象生命周期管理 — https://docs.min.io/community/minio-object-store/administration/object-management/object-lifecycle-management.html
- [S13] 阿里云 OSS:服务端签名直传 / 限制上传类型与大小 — https://help.aliyun.com/zh/oss/user-guide/obtain-signature-information-from-the-server-and-upload-data-to-oss ;https://help.aliyun.com/zh/oss/how-do-i-limit-object-formats-and-sizes-when-i-upload-objects-to-oss (搜索摘要,**未核实**)
- [S14] 阿里云 OSS:配置 Referer 防盗链 — https://help.aliyun.com/zh/oss/user-guide/hotlink-protection
- [S15] MDN: Referer — https://developer.mozilla.org/en-US/docs/Web/HTTP/Reference/Headers/Referer
- [S16] 阿里云 OSS:指定时间段内访问私有图片 — https://help.aliyun.com/zh/oss/how-to-access-a-private-image-object-within-a-specific-period-of-time
- [S17] 阿里云 CDN:防范流量盗刷 — https://help.aliyun.com/zh/cdn/use-cases/best-practices-for-preventing-traffic-theft (搜索摘要,**未核实**)
- [S18] Go `image` 包(DecodeConfig、安全注意事项)— https://pkg.go.dev/image
- [S19] golang.org/x/image/draw — https://pkg.go.dev/golang.org/x/image/draw
- [S20] golang.org/x/image/webp — https://pkg.go.dev/golang.org/x/image/webp
- [S21] Microsoft: thumbnailPhoto(Picture)属性 — https://learn.microsoft.com/en-us/windows/win32/adschema/a-thumbnailphoto (Range-Upper 102400,已复核)
- [S22] RFC 2798 inetOrgPerson(jpegPhoto)— https://www.rfc-editor.org/rfc/rfc2798
