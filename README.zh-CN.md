<p align="center">
  <strong>Fenfa</strong> &nbsp;·&nbsp; 自托管应用分发平台
</p>

<p align="center">
  <a href="https://github.com/openprx/fenfa/blob/main/LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue.svg" alt="License"></a>
  <a href="https://github.com/openprx"><img src="https://img.shields.io/badge/org-OpenPRX-8b5cf6" alt="OpenPRX"></a>
</p>

<p align="center">
  <a href="README.md">English</a>
</p>

---

Fenfa（分发）是一个自托管的多平台应用分发系统，支持 iOS、Android、macOS、Windows、Linux。上传安装包后自动生成下载页面和二维码，通过管理后台统一管理。

隶属于 [OpenPRX](https://github.com/openprx) 开源生态 — 面向开发团队的基础设施。

## 特性

- **多平台** — iOS (IPA)、Android (APK)、macOS、Windows、Linux
- **智能上传** — 自动从安装包解析 Bundle ID、版本号、图标
- **iOS UDID 绑定** — Ad-Hoc 分发的设备注册流程
- **产品页面** — 公开下载页，带二维码和智能平台检测
- **管理后台** — 产品、变体、版本、设备、系统设置一站式管理
- **S3/R2 存储** — 可选 S3 兼容对象存储（Cloudflare R2、AWS S3、MinIO）
- **Apple 开发者 API** — 自动注册设备到 Apple 开发者账号
- **单文件部署** — Go 二进制内嵌前端，开箱即用
- **SQLite** — 零外部依赖，数据存储在单个文件
- **多语言** — 中英文界面

## 快速开始

### Docker（推荐）

```bash
docker run -d --name fenfa -p 8000:8000 fenfa/fenfa:latest
```

访问 `http://localhost:8000/admin`，使用默认 Token `dev-admin-token` 登录。

**生产部署** — 自定义 Token + 持久化存储：

```bash
docker run -d \
  --name fenfa \
  --restart=unless-stopped \
  -p 127.0.0.1:8000:8000 \
  -e FENFA_ADMIN_TOKEN=你的管理密钥 \
  -e FENFA_UPLOAD_TOKEN=你的上传密钥 \
  -e FENFA_PRIMARY_DOMAIN=https://your-domain.com \
  -v ./data:/data \
  -v ./uploads:/app/uploads \
  fenfa/fenfa:latest
```

也可以挂载 [`config.json`](docs/config.example.json) 进行完整配置。

### 从源码构建

需要：Go 1.25+，Node.js 20+

```bash
make build   # 构建前端 + 后端
make run     # 启动服务
```

或手动：

```bash
cd web/front && npm ci && npm run build && cd ../..
cd web/admin && npm ci && npm run build && cd ../..
go build -o fenfa ./cmd/server
./fenfa
```

## 配置

完整配置参考 [`docs/config.example.json`](docs/config.example.json)，API 文档见 [`docs/api-spec.md`](docs/api-spec.md)。

### 环境变量

支持通过环境变量覆盖配置（无需 config.json）：

| 变量 | 说明 |
|------|------|
| `FENFA_PORT` | HTTP 端口 |
| `FENFA_DATA_DIR` | 数据库目录 |
| `FENFA_PRIMARY_DOMAIN` | 主域名 |
| `FENFA_ADMIN_TOKEN` | 管理员 Token |
| `FENFA_UPLOAD_TOKEN` | 上传 Token |

## 架构

```
请求 → Gin 路由 → 认证中间件 → Handler → GORM → SQLite
```

```
cmd/server/          程序入口
internal/server/     HTTP 处理器和路由
internal/store/      数据模型 (GORM + SQLite)
internal/web/        内嵌前端资源 (go:embed)
web/front/           公开下载页 (Vue 3 + Vite)
web/admin/           管理后台 (Vue 3 + Vite)
```

## 参与贡献

Fenfa 是 [OpenPRX](https://github.com/openprx) 的一部分，欢迎提交 Issue 和 Pull Request。

## 开源协议

[MIT](LICENSE)
