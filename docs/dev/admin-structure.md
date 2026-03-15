# 后台管理项目结构

## 📁 目录结构

```
web/admin/
├── src/
│   ├── components/          # 可复用组件
│   │   └── Sidebar.vue      # 侧边栏导航组件
│   │
│   ├── pages/               # 页面组件
│   │   ├── LoginPage.vue    # 登录页面
│   │   ├── AppsPage.vue     # 应用列表页面
│   │   ├── UploadPage.vue   # 上传构建页面
│   │   ├── StatsPage.vue    # 下载统计页面
│   │   ├── EventsPage.vue   # 访问明细页面
│   │   └── ExportPage.vue   # 导出数据页面
│   │
│   ├── styles/              # 样式文件
│   │   └── common.css       # 公共样式
│   │
│   ├── App.vue              # 根组件（状态管理和路由）
│   ├── main.ts              # 入口文件
│   ├── style.css            # 全局基础样式
│   └── vite-env.d.ts        # TypeScript 类型声明
│
├── index.html               # HTML 模板
├── vite.config.ts           # Vite 配置
├── package.json             # 依赖配置
├── README.md                # 项目说明
└── STRUCTURE.md             # 项目结构说明（本文件）
```

## 🧩 组件说明

### 1. **App.vue** - 根组件
- 管理全局状态（token、currentPage、appId 等）
- 处理登录/登出逻辑
- 协调各个页面组件
- 调用 API 接口

### 2. **components/Sidebar.vue** - 侧边栏
- 导航菜单
- 页面切换
- 退出登录按钮
- 支持折叠（预留功能）

### 3. **pages/LoginPage.vue** - 登录页
- Token 输入
- 登录验证
- 渐变背景设计

### 4. **pages/AppsPage.vue** - 应用列表
- 显示所有应用
- 搜索功能
- 跳转到统计/事件页面

### 5. **pages/UploadPage.vue** - 上传构建
- 表单输入（平台、版本、Bundle ID 等）
- 文件上传
- 上传结果展示

### 6. **pages/StatsPage.vue** - 下载统计
- 按 App ID 查询
- 显示各版本下载数据
- 表格展示

### 7. **pages/EventsPage.vue** - 访问明细
- 多条件筛选（App ID、类型、时间范围）
- 事件列表展示
- UA 信息解析

### 8. **pages/ExportPage.vue** - 导出数据
- 时间范围选择
- 三种导出类型（Releases、Events、iOS Devices）
- 卡片式交互

## 🔄 数据流

```
App.vue (状态管理)
    ↓
各个 Page 组件 (通过 props 接收数据)
    ↓
用户交互 (通过 emit 发送事件)
    ↓
App.vue (处理事件，调用 API，更新状态)
```

## 🎨 样式组织

- **style.css** - 全局基础样式（body、滚动条等）
- **styles/common.css** - 公共组件样式（按钮、表单、表格等）
- **组件内 `<style scoped>`** - 组件特定样式

## 🔧 开发指南

### 添加新页面

1. 在 `src/pages/` 创建新的 `.vue` 文件
2. 在 `App.vue` 中导入组件
3. 在 `Sidebar.vue` 的 `menuItems` 添加菜单项
4. 在 `App.vue` 的模板中添加条件渲染

### 修改样式

- 全局样式 → `style.css` 或 `styles/common.css`
- 组件样式 → 组件内的 `<style scoped>`

### API 调用

所有 API 调用都在 `App.vue` 中统一管理，通过 props 和 events 与子组件通信。

## 📦 构建

```bash
# 开发
npm run dev

# 构建
npm run build

# 预览
npm run preview
```

## ✨ 优势

1. **模块化** - 每个页面独立，易于维护
2. **可复用** - 组件可以在不同页面复用
3. **类型安全** - TypeScript 支持
4. **清晰的职责** - App.vue 管理状态，Page 组件负责展示
5. **易于扩展** - 添加新功能只需创建新组件

