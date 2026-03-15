# CLAUDE.md

本文件为 Claude Code (claude.ai/code) 在此仓库中工作提供指导。

## 项目概述

Fenfa 是一个自托管的多平台应用分发平台，支持 iOS、Android、macOS、Windows、Linux。技术栈：Go 1.23+ 后端 + SQLite，Vue 3 + Vite 前端。

## 开发命令

### 后端 (Go)
```bash
go run ./cmd/server                    # 启动开发服务器 (端口 8000)
go build -o bin/fenfa ./cmd/server     # 构建二进制文件
```

### 前端
```bash
# 公开下载页面
cd web/front && npm install && npm run dev    # 开发: http://localhost:5173
cd web/front && npm run build                 # 构建到 internal/web/dist/front

# 管理后台
cd web/admin && npm install && npm run dev    # 开发: http://localhost:5174
cd web/admin && npm run build                 # 构建到 internal/web/dist/admin
```

### Docker
```bash
./scripts/docker-build.sh    # 多架构构建
./scripts/docker-run.sh      # 运行容器
```

### 本地开发热重载
在 config.json 中设置 `dev_proxy_front` 和 `dev_proxy_admin` 为 Vite 开发服务器地址，后端会代理请求到开发服务器。

## 架构

### 后端结构
- `cmd/server/main.go` - 入口，初始化数据库和路由
- `internal/config/` - 从 config.json 加载配置
- `internal/server/router.go` - 所有路由注册
- `internal/server/handlers/` - HTTP 处理器（上传、下载、manifest、管理 API）
- `internal/server/middleware/auth.go` - Token 认证（X-Auth-Token 请求头）
- `internal/store/` - GORM 模型和 SQLite 迁移
- `internal/web/` - 通过 go:embed 嵌入的前端资源

### 前端结构
- `web/front/` - 公开下载页面 (Vue 3 SPA)
- `web/admin/` - 管理后台 (Vue 3 SPA)
- 两个应用构建到 `internal/web/dist/`，嵌入 Go 二进制文件

### 数据流
```
请求 → Gin 路由 → 认证中间件 → Handler → GORM/Store → SQLite
```

### 文件存储
文件存储路径：`uploads/{product_id}/{variant_id}/{release_id}/app.{ext}`

## 代码规范

### Handler 模式
```go
func HandlerName(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
    return func(c *gin.Context) { ... }
}
```

### API 响应格式
```json
{"ok": true, "data": {...}}
{"ok": false, "error": {"code": "...", "message": "..."}}
```

### 认证
配置中有两种 token 权限：`upload_tokens`（应用上传）、`admin_tokens`（完整管理权限）。通过 `X-Auth-Token` 请求头传递。

### ID 生成
ID 格式为 `{前缀}_{随机字符串}`，如 `app_7f3xn`、`rel_b1cqa`、`prd_abc123`、`var_def456`。

## 关键文件

- `config.json` - 服务器配置（参考 `docs/config.example.json`）
- `docs/api-spec.md` - API 规范
- `docs/architecture.md` - 架构设计
- `docs/multi-platform.md` - 多平台架构说明
- `docs/dev/` - 开发参考（i18n 指南、组件结构、设计文档）

## 数据模型

核心表：`Product`、`Variant`、`Release`、`App`（legacy）、`Event`、`IOSDevice`、`SystemSettings`。Schema 见 `internal/store/models.go`。

## API 端点

公开：`/products/:slug`、`/d/:releaseID`（下载）、`/udid/*`（iOS 设备绑定）
需认证：`/upload`、`/admin/api/*`（需要 auth token）
