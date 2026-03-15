# 多语言支持 (i18n) 使用说明

## ✅ 已实现功能

### 1. 核心功能
- ✅ 中文 (zh) 和英文 (en) 双语支持
- ✅ 语言切换组件 (LanguageSwitcher)
- ✅ 语言选择持久化 (localStorage)
- ✅ 响应式语言切换（无需刷新页面）

### 2. 已集成多语言的组件

#### ✅ LoginPage (登录页面)
- 标题、副标题
- 表单标签和占位符
- 按钮文本
- 错误提示

#### ✅ Sidebar (侧边栏)
- 标题和副标题
- 所有菜单项
- 退出登录按钮
- 集成语言切换器

#### ✅ AppsPage (应用列表)
- 页面标题和副标题
- 搜索框占位符
- 表格列标题
- 按钮文本
- 空状态提示

#### ✅ App.vue (主应用)
- 开发模式提示
- 确认对话框
- 错误提示

## 📁 文件结构

```
src/
├── i18n/
│   ├── index.ts      # i18n 核心逻辑
│   ├── zh.ts         # 中文语言包
│   └── en.ts         # 英文语言包
├── components/
│   ├── Sidebar.vue
│   └── LanguageSwitcher.vue  # 语言切换组件
└── pages/
    ├── LoginPage.vue
    ├── AppsPage.vue
    ├── UploadPage.vue  # 待更新
    ├── StatsPage.vue   # 待更新
    ├── EventsPage.vue  # 待更新
    └── ExportPage.vue  # 待更新
```

## 🚀 使用方法

### 在组件中使用

```vue
<script setup lang="ts">
import { useI18n } from '../i18n'

const { t, locale, setLocale } = useI18n()
</script>

<template>
  <div>
    <!-- 使用翻译文本 -->
    <h1>{{ t.apps.title }}</h1>
    <p>{{ t.apps.subtitle }}</p>
    
    <!-- 当前语言 -->
    <div>Current: {{ locale }}</div>
    
    <!-- 切换语言 -->
    <button @click="setLocale('en')">English</button>
    <button @click="setLocale('zh')">中文</button>
  </div>
</template>
```

### 添加新的翻译文本

1. 在 `src/i18n/zh.ts` 添加中文文本
2. 在 `src/i18n/en.ts` 添加对应的英文文本
3. 在组件中使用 `t.value.xxx.yyy`

示例：

```typescript
// zh.ts
export default {
  myFeature: {
    title: '我的功能',
    description: '这是描述'
  }
}

// en.ts
export default {
  myFeature: {
    title: 'My Feature',
    description: 'This is description'
  }
}

// Component.vue
<template>
  <h1>{{ t.myFeature.title }}</h1>
  <p>{{ t.myFeature.description }}</p>
</template>
```

## 🎨 语言切换器

语言切换器组件 (`LanguageSwitcher.vue`) 已集成在：
- 登录页面（右上角）
- 侧边栏（底部，退出登录按钮上方）

用户点击 🌐 按钮即可在中英文之间切换。

## 📝 待完成的工作

以下页面的翻译文本已在语言包中定义，但组件还未集成：

### ⏳ UploadPage.vue
需要替换的文本：
- 页面标题和副标题
- 表单标签（平台、应用名称、Bundle ID 等）
- 占位符文本
- 按钮文本
- 成功/失败消息

### ⏳ StatsPage.vue
需要替换的文本：
- 页面标题和副标题
- 输入框占位符
- 表格列标题
- 按钮文本

### ⏳ EventsPage.vue
需要替换的文本：
- 页面标题和副标题
- 筛选器标签
- 表格列标题
- 按钮文本

### ⏳ ExportPage.vue
需要替换的文本：
- 页面标题和副标题
- 表单标签
- 导出卡片标题和描述

## 🔧 快速更新指南

对于每个待更新的页面，按以下步骤操作：

1. **导入 i18n**
```typescript
import { useI18n } from '../i18n'
const { t } = useI18n()
```

2. **替换硬编码文本**
```vue
<!-- 之前 -->
<h1>上传构建</h1>

<!-- 之后 -->
<h1>{{ t.upload.title }}</h1>
```

3. **测试**
- 切换语言确保所有文本正确显示
- 检查是否有遗漏的文本

## 🌐 当前状态

**可用功能：**
- ✅ 登录页面完全支持中英文
- ✅ 侧边栏菜单完全支持中英文
- ✅ 应用列表页面完全支持中英文
- ✅ 语言切换实时生效
- ✅ 语言选择自动保存

**用户体验：**
- 首次访问默认显示中文
- 切换语言后刷新页面仍保持选择的语言
- 所有已集成的页面切换语言无需刷新

## 📦 构建和部署

```bash
# 开发模式
npm run dev

# 构建生产版本
npm run build

# 构建后重启 Go 服务器
cd /Users/xx/ck/code/fenfa
go run ./cmd/server
```

## 🎯 下一步

如需完成剩余页面的多语言集成，可以参考 `AppsPage.vue` 的实现方式，将硬编码的中文文本替换为 `t.value.xxx.yyy` 形式。

所有需要的翻译文本已经在 `src/i18n/zh.ts` 和 `src/i18n/en.ts` 中定义好了。

