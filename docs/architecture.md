# fenfa 最小可用设计方案（MVP：简易版“蒲公英”）

本方案落实 01.md 的基础上，提供“仅绑定 UDID + 手工签名/手工替换 IPA”的最小实现；技术栈 Go + gin + SQLite；前端采用 Vite 构建并通过 go:embed 将 front/admin 的构建产物打包进二进制，区分前台与后台。

## 1. 范围与目标
- 平台：iOS/Android 内测分发
- 前台：响应式下载页，二维码，历史版本
- iOS 前置：需绑定 UDID（仅采集与标记，不做自动注册与重签）
- 后台：导出 CSV（releases、events、ios_devices），简易筛选
- 安全：上传鉴权 Token、HTTPS、基础限流
- 存储：本地 uploads 为默认，SQLite 为默认 DB

## 2. 技术栈与项目结构
- 后端：Go 1.22+、gin、GORM、SQLite（gorm.io/driver/sqlite）
- 前端：Vue 3 + Vite + TailwindCSS；构建产物嵌入
- 二进制内嵌：go:embed 提供 /static 与 HTML 模板

目录建议（含前后端分离与 go:embed 路径约定）：
- cmd/server/main.go
- internal/
  - http/
    - router.go
    - handlers/（upload、apps、download、manifest、udid、admin）
    - middleware/（auth、rate、recovery、cors）
  - store/（GORM）
    - db.go（初始化 gorm.DB，sqlite 驱动）
    - models.go（App、Release、AuthToken、Event、IOSDevice 等）
    - migrations.go（AutoMigrate + 兼容外部 SQL）
  - services/
    - qrcode.go、udid.go
  - web/
    - templates/（Go html/template：壳页，注入 window.__APP_DATA__）
    - dist/front/（front 构建产物，用于 go:embed）
    - dist/admin/（admin 构建产物，用于 go:embed）
- web/（前端源码）
  - front/（Vue 3 + Vite 前台应用）
  - admin/（Vue 3 + Vite 管理后台）
- docs/（01.md、02_design.md、app.sql）
- uploads/（运行时生成）
- Makefile、scripts/（一键 build 前后端）
- .env.example（环境变量示例）

## 3. 数据库模型
沿用 01.md（apps、releases、auth_tokens、events），新增 iOS 设备表与可选绑定表：

- ios_devices
  - id TEXT PRIMARY KEY（dev_*）
  - udid TEXT NOT NULL UNIQUE
  - device_name TEXT
  - model TEXT
  - os_version TEXT
  - created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
  - verified_at TIMESTAMP
  - last_ip TEXT

- device_bindings（可选，先用 cookie/session 即可，后续扩展）
  - id TEXT PRIMARY KEY（bind_*）
  - udid TEXT NOT NULL REFERENCES ios_devices(udid)
  - app_id TEXT REFERENCES apps(id)
  - created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
- App 详情数据（JSON）：GET /api/apps/{app_id}[?r=release_id]（前端 Vue 拉取）


迁移：优先使用 GORM AutoMigrate；同时在 docs/app.sql 维护 DDL 以便调试/导出；启动时 AutoMigrate，必要时执行外部 SQL 补齐。

## 4. 路由与接口
保持与 01.md 一致并新增 UDID 与导出接口（均返回统一 JSON 结构或 HTML）：

- 上传：POST /upload（鉴权 Token，multipart）
- App 详情页：GET /apps/{app_id}[?r=release_id]（HTML，响应式）
- iOS manifest：GET /ios/{release_id}/manifest.plist（XML）
- 直链下载：GET /d/{release_id}（二进制，支持 Range）
- 健康检查：GET /healthz

新增（UDID 最小实现）：
- GET /udid/profile.mobileconfig
  - 下发 mobileconfig，包含回调 URL（/udid/callback?nonce=...）
- POST /udid/callback（Content-Type: application/x-apple-aspen-config 或 application/xml）
  - 设备回传 plist，解析 UDID/PRODUCT/VERSION/DEVICE_NAME；校验一次性 nonce，写入 ios_devices；设置 Cookie：udid_bound=1
- GET /udid/status
  - 返回 { ok: true, data: { bound: true, udid: "..." } }

新增（后台导出，需 admin scope）：
- GET /admin/exports/releases.csv
- GET /admin/exports/events.csv
- GET /admin/exports/ios_devices.csv
- 筛选参数：app_id、platform、from、to、q、limit

## 5. 前台下载页（响应式）
- 内容：App 基本信息、最新版本、历史版本（N=5）、更新说明（可折叠）、二维码（指向 release_page 或 /apps/{app_id}?r=...）
- iOS 安装按钮：
  - 若 UA 命中 iOS 且未绑定（/udid/status.bound=false 或缺少 cookie），展示“先绑定设备”，跳转 /udid/profile.mobileconfig
  - 已绑定后启用 itms-services://?action=download-manifest&url=/ios/{release_id}/manifest.plist
- Android 下载按钮：直接 /d/{release_id}
- 实现：Vue 3 应用控制状态；Go 模板仅作 HTML 壳并注入初始数据（window.__APP_DATA__ 或 data-*）；样式由 Vite/Tailwind 产物提供

## 6. UDID 绑定流程（仅采集）
1) 生成 mobileconfig：
- Payload 包含 PayloadContent 指向 /udid/callback?nonce=<一次性随机>；描述用途与隐私说明
2) 设备安装后系统自动回传：
- 服务器接收 plist，提取字段（UDID、PRODUCT、VERSION、DEVICE_NAME 等），写 ios_devices；nonce 作废
3) 绑定状态：
- 服务端在 session/cookie 标记绑定（udid_bound=1），前端据此启用 iOS 安装按钮
4) 不做：Apple API 自动注册与自动重签（留待后续 M2）

## 7. 后台导出（CSV）
- 鉴权：X-Auth-Token（需含 admin 或 read+export 权限）
- 字段：
  - releases.csv：app_id, release_id, version, build, created_at, size_bytes, sha256, download_count, channel
  - events.csv：ts, type, app_id, release_id, ip(mask), ua, extra
  - ios_devices.csv：udid, device_name, model, os_version, created_at, verified_at
- 性能：流式写出 text/csv；大范围导出分页扫描

## 8. go:embed 与静态资源
- 约定：vite build 输出到 internal/web/dist/front 与 internal/web/dist/admin
- 嵌入：
  - //go:embed web/dist/front web/dist/admin web/templates
  - 使用 http.FS 提供 /static/front、/static/admin；模板用 html/template.ParseFS 解析
- 构建流水线：
  - 前端：npm ci && npm run build（分别在 front/ 与 admin/）
  - 后端：go build ./cmd/server（嵌入 dist 与模板）
- 开发模式：若检测到环境变量 DEV=1，可直接从磁盘读取未打包资源（便于热更）

## 9. 安全与合规
- 强制 HTTPS（前置 Nginx/ALB），仅 127.0.0.1 对后端
- 上传与导出均需 Token；上传频率限流；记录 events
- UDID 视为敏感：最小化收集，仅用于安装前置判断；提供隐私说明页面
- 下载日志可脱敏（IP hash）
- 文件名统一：app.ipa|app.apk，拒绝原名写入

## 10. 运行与部署
- 二进制 + SQLite 文件 + uploads 目录
- 配置：config.json（server.port、server.data_dir、server.db_path、server.base_url、auth.upload_tokens[]、auth.admin_tokens[]）
- Nginx：参考 01.md；可选静态目录直出 /uploads/


config.json  Example:
{
  "server": { "port": "8000", "base_url": "https://dist.example.com", "data_dir": "data", "db_path": "data/fenfa.db" },
  "auth":   { "upload_tokens": ["<token>"], "admin_tokens": ["<admin-token>"] }
}

## 11. 里程碑（M1：2–3 天）
1) 脚手架 + 路由 + SQLite 建表/迁移
2) 上传 /upload（鉴权、存储、SHA256）、直链下载 /d/{id}（Range + 计数）
3) iOS manifest.plist 生成
4) 前台下载页（响应式 + 二维码 + iOS 按钮 gating）
5) UDID：/udid/profile.mobileconfig → /udid/callback → /udid/status
6) 导出：/admin/exports/*.csv
7) 健康检查 /healthz

## 12. 验收标准
- iOS 未绑定时“安装”按钮不可点击且提示先绑定；绑定成功后可安装
- Android 直链可断点续传；下载计数与 events 正确
- releases/events/ios_devices 导出可按时间范围与 app_id 过滤
- go:embed 后二进制可独立运行，/static 与页面可用

## 13. 关键实现片段（示意）
- go:embed（示意）：
  - embed.FS: FrontFS=/web/dist/front, AdminFS=/web/dist/admin, TplFS=/web/templates
  - gin.StaticFS("/static/front", http.FS(FrontFS))
  - tmpl := template.ParseFS(TplFS, "*.html")

## 14. 多平台产品模型演进

当前系统已从单一 `App -> Release` 方向，演进到支持多平台聚合展示的目标模型：

- `Product`：公开下载页的主体，一个产品页可同时展示多个平台
- `Variant`：产品下的平台变体，如 `ios/android/macos/windows/linux`
- `Release`：某个平台变体下的具体安装包，独立维护版本号与 changelog

迁移原则：

- 保留旧 `App` / `Release.AppID` 作为兼容字段，避免旧链接立即失效
- 迁移时为每个旧 `App` 自动创建一个 `Product` 和一个同平台 `Variant`
- 后续公开 API 与后台 API 逐步切换到 `Product -> Variant -> Release`
