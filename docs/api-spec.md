
一、目标与范围
	•	面向 iOS/Android 的内测分发：上传 IPA/APK → 生成安装页面（含二维码）→ iOS 走 itms-services，Android 直链下载。
	•	极简、安全、可自托管；单一上传密钥或多密钥；本地文件存储为默认；DB 支持 SQLite（默认）与 PostgreSQL。
	•	每次上传即一个版本（Release），可填写更新说明（changelog）；每个 App 支持多版本历史。

二、术语与对象
	•	App：同一应用的逻辑实体；以 bundle_id（iOS）或 application_id（Android）识别；字段包含名称、平台等。
	•	Release：一次上传（一个版本/构建）；含 version（如 1.2.3）、build（整型）、changelog、制品与元信息。
	•	Artifact：物理文件（IPA/APK）及派生文件（icon、二维码、manifest.plist）。

⸻

三、接口设计（REST）

所有 JSON 响应统一结构：

{ "ok": true, "data": {...} }

错误：

{ "ok": false, "error": { "code": "BAD_REQUEST", "message": "..." } }

1) 上传

POST /upload
Header：X-Auth-Token: <token>
Content-Type：multipart/form-data

表单字段：
	•	app_file：必填，ipa/apk 文件
	•	platform：必填，ios | android
	•	bundle_id：iOS 必填（如 com.example.foo）
	•	application_id：Android 必填（同上）
	•	app_name：可选（显示名）
	•	version：可选（如 1.2.3）
	•	build：可选（整型；用于排序与覆盖策略）
	•	min_os：可选（如 iOS 13.0 / Android 8.0）
	•	changelog：可选（更新说明，Markdown/纯文本）
	•	channel：可选（如 internal/beta）

行为：
	1.	校验 Token → 解析基本元信息（可从文件内读取兜底，不强制）。
	2.	若 bundle_id/application_id 对应的 App 不存在则创建。
	3.	新建 Release，保存元数据与文件（见存储方案）。
	4.	生成可访问的安装页与直链；iOS 同时生成 manifest.plist。
	5.	记录哈希（SHA256）、大小、MIME、上传者、IP、UA 等。

响应（示例）：

{
  "ok": true,
  "data": {
    "app": {
      "id": "app_7f3...", "name": "Foo", "platform": "ios",
      "bundle_id": "com.example.foo"
    },
    "release": {
      "id": "rel_b1c...",
      "version": "1.2.3",
      "build": 1020300,
      "changelog": "修复问题…",
      "created_at": "2025-10-11T10:20:30Z"
    },
    "urls": {
      "page": "https://dist.example.com/apps/app_7f3",
      "release_page": "https://dist.example.com/apps/app_7f3?r=rel_b1c",
      "download": "https://dist.example.com/d/rel_b1c",
      "ios_manifest": "https://dist.example.com/ios/rel_b1c/manifest.plist"
    }
  }
}

2) App 列表（可选）

GET /apps
	•	查询参数：platform、bundle_id/application_id、q（名称模糊）、page、page_size
	•	返回各 App 的最新 Release 摘要（可配置返回数量）

3) App 详情页（HTML）

GET /apps/{app_id}
	•	展示：App 基本信息、最近 N 个 Release（默认 5）、每个 Release 的下载按钮、更新说明、二维码（指向 release_page 或 /d/{id}）。
	•	查询参数：r=<release_id> 指定默认高亮版本；未指定则展示最新。

4) iOS manifest

GET /ios/{release_id}/manifest.plist
	•	返回 application/xml；内容指向该 Release 的 /d/{release_id} 直链。
	•	用于安装页按钮：
itms-services://?action=download-manifest&url=https://dist.example.com/ios/{release_id}/manifest.plist

plist 关键字段：
	•	items[0].assets[0].url → 指向 IPA 下载直链
	•	items[0].metadata.bundle-identifier、bundle-version、title

5) 直链下载

GET /d/{release_id}
	•	返回二进制文件（IPA/APK），支持 Range；记录下载计数与日志。
	•	可选：支持 ?token= 临时签名或过期时间（短期私有分发）。

6) 健康检查

GET /healthz
	•	检查：DB 连接、存储可写、磁盘余量 ≥ 阈值、最近 N 分钟无错误写入。

⸻

四、存储结构

默认本地（可挂载 NAS/对象存储网关）：

./uploads/
  └── <app_id>/
      └── <release_id>/
          ├── app.ipa | app.apk
          ├── meta.json
          ├── manifest.plist        (iOS)
          ├── icon.png              (可选：解包提取)
          └── qrcode.png            (可选：生成缓存)

meta.json（示例）：

{
  "app_id": "app_7f3",
  "release_id": "rel_b1c",
  "platform": "ios",
  "bundle_id": "com.example.foo",
  "version": "1.2.3",
  "build": 1020300,
  "min_os": "iOS 13.0",
  "size": 73400320,
  "sha256": "6f4a...",
  "changelog": "修复问题…",
  "uploaded_by": "ci-bot",
  "uploaded_at": "2025-10-11T10:20:30Z"
}


⸻

五、数据库模型（SQLite 默认；PostgreSQL 兼容）

SQLite 适合单机起步；未来换 PG 只需调整连接串与少量 SQL。

-- apps
CREATE TABLE apps (
  id            TEXT PRIMARY KEY,              -- app_*
  platform      TEXT NOT NULL CHECK (platform IN ('ios','android')),
  bundle_id     TEXT,                          -- iOS 唯一
  application_id TEXT,                         -- Android 唯一
  name          TEXT,
  icon_path     TEXT,
  created_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE(platform, bundle_id),
  UNIQUE(platform, application_id)
);

-- releases
CREATE TABLE releases (
  id            TEXT PRIMARY KEY,              -- rel_*
  app_id        TEXT NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
  version       TEXT,
  build         INTEGER,
  changelog     TEXT,
  min_os        TEXT,
  size_bytes    INTEGER NOT NULL,
  sha256        TEXT NOT NULL,
  storage_path  TEXT NOT NULL,                 -- 相对路径：uploads/app_id/release_id/app.ipa
  download_count INTEGER NOT NULL DEFAULT 0,
  created_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_releases_app_created ON releases(app_id, created_at DESC);
CREATE INDEX idx_releases_app_build   ON releases(app_id, build DESC);

-- auth_tokens（多密钥/分渠道可选）
CREATE TABLE auth_tokens (
  token         TEXT PRIMARY KEY,
  label         TEXT,
  scopes        TEXT,                          -- 逗号分隔：upload,read,admin
  rate_limit    INTEGER DEFAULT 0,             -- 每小时上限（0=不限）
  created_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  revoked_at    TIMESTAMP
);

-- events（下载/上传日志，可选）
CREATE TABLE events (
  id            INTEGER PRIMARY KEY AUTOINCREMENT,
  ts            TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  type          TEXT,                          -- upload/download/cleanup
  app_id        TEXT,
  release_id    TEXT,
  ip            TEXT,
  ua            TEXT,
  extra         TEXT
);

ID 生成：短前缀 + 随机 base32/ULID（如 app_7f3xn...、rel_b1cqa...）。
幂等：可用 (app_id, version, build) 唯一约束避免重复。

⸻

六、安全与合规
	•	HTTPS 强制：对外仅暴露 443；后端监听 127.0.0.1，由反代接入。
	•	上传鉴权：X-Auth-Token 必填；支持多密钥与吊销；上传频次限流（令牌桶/简单计数 + Redis/SQLite）。
	•	大小限制：反代如 Nginx client_max_body_size 2g;；后端也做上限校验。
	•	MIME/签名校验：检测扩展名 + 魔术字节；可选 ClamAV 扫描。
	•	路径与文件名：统一重命名为 app.ipa|apk；拒绝原始文件名写入以杜绝目录穿越。
	•	清理策略：保留每 App 最近 N 个版本或总配额上限（GB）；超出则按时间/下载数回收。
	•	隐私：尽量不收集不必要 PII；下载日志可脱敏（IP hash）。

⸻

七、安装页（HTML 要点）
	•	展示：App 名称、Bundle ID、平台、最新版本、历史版本列表、更新说明（可折叠）。
	•	按钮：
	•	iOS：安装 → 跳 itms-services://?action=download-manifest&url=...
	•	Android：下载 APK → 直链 /d/{release_id}
	•	二维码：生成 PNG，内容指向当前 Release 页面（含 release_id）；移动端扫码直达。
	•	渠道标识：页面顶部显示 channel（若有）。

⸻

八、iOS manifest.plist 模板

（动态生成）

<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN"
 "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>items</key>
  <array>
    <dict>
      <key>assets</key>
      <array>
        <dict>
          <key>kind</key><string>software-package</string>
          <key>url</key><string>https://dist.example.com/d/rel_b1c</string>
        </dict>
        <!-- 可选：应用图标 -->
        <!--
        <dict>
          <key>kind</key><string>display-image</string>
          <key>needs-shine</key><true/>
          <key>url</key><string>https://dist.example.com/uploads/app_7f3/rel_b1c/icon.png</string>
        </dict>
        -->
      </array>
      <key>metadata</key>
      <dict>
        <key>bundle-identifier</key><string>com.example.foo</string>
        <key>bundle-version</key><string>1.2.3 (1020300)</string>
        <key>kind</key><string>software</string>
        <key>title</key><string>Foo</string>
      </dict>
    </dict>
  </array>
</dict>
</plist>


⸻

九、Nginx 反向代理（要点）

server {
  listen 443 ssl http2;
  server_name dist.example.com;

  # ...ssl 证书略
  client_max_body_size 2g;
  sendfile on; tcp_nopush on;

  # 静态文件（uploads）可直接由 Nginx 回源本地目录提高吞吐（可选）
  # location ^~ /uploads/ { root /srv/dist; expires 7d; }

  location / {
    proxy_pass http://127.0.0.1:8000;
    proxy_set_header Host $host;
    proxy_set_header X-Forwarded-Proto https;
    proxy_set_header X-Real-IP $remote_addr;
  }
}


⸻

十、最小实现顺序（一天可完成的里程碑）
	1.	项目脚手架（Go+gin）+ SQLite 连接
	2.	DB 建表（apps/releases/auth_tokens）
	3.	上传接口：鉴权 → 保存文件 → 计算 SHA256 → 入库 → 写 meta.json → 返回 URLs
	4.	直链下载接口 /d/{release_id}（记录下载次数）
	5.	iOS manifest.plist 生成接口
	6.	App 详情页（响应式HTML + 二维码）
	7.	健康检查
	8.	Nginx/HTTPS 打通

（随后迭代：App 列表、分页、清理策略、管理后台、临时签名 URL、统计图表等）

⸻

十一、扩展与集成
	•	CI 集成：构建后 curl 上传；响应里取 release_page 发到机器人/邮箱。
	•	UDID 流程：独立 /udid Profile Service（已在上一轮说明），与 Apple API 自动注册 + 自动重签对接。
	•	对象存储：抽象 Storage 接口（local / S3 / MinIO）；配置切换。
	•	鉴权升级：多 token、按渠道限速；下载也可要求短期 token。
	•	指标：Prometheus /metrics（下载总数、吞吐、失败率、磁盘剩余）。

⸻
    
	•	路由/控制器
	•	ORM/SQL（含迁移）
	•	manifest 生成
	•	上传与下载逻辑
	•	简易 HTML 页面与二维码
	•	Dockerfile + Compose（带 Nginx 反代）