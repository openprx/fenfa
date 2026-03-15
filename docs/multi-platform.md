# 多平台产品分发架构说明

## 1. 背景

本次改造的目标，是把项目从“移动应用分发系统”升级为“多平台产品分发系统”。

现在一个产品可以在同一个页面中同时展示并分发以下平台版本：

- iOS
- Android
- macOS
- Windows
- Linux

同时，每个平台仍然保持独立的版本号、独立的构建号、独立的 changelog、独立的安装包。

---

## 2. 改造前的架构

### 2.1 旧模型

原来系统的核心模型是：

- `App`
- `Release`

其中：

- 一条 `App` 记录只代表一个平台应用，典型场景是 `ios` 或 `android`
- 一条 `Release` 记录隶属于一个 `App`

旧模型的特点：

- 业务中心是“单个平台应用”
- 页面中心是 `/apps/:appID`
- 上传中心是“给某个 app 上传一个 ipa/apk”
- iOS/Android 是一等公民，桌面平台基本没有正式建模

### 2.2 旧模型的限制

这个模型在只做 `iOS / Android` 时还能工作，但一旦要支持 `macOS / Windows / Linux`，问题会很明显：

- 一个产品的多个平台版本无法自然归在同一页
- `BundleID / ApplicationID` 这种字段明显偏移动端
- 前台展示逻辑只能做“iOS 安装”与“其他平台下载”的二分
- 后台统计、事件、UDID、导出等能力都默认围绕 `app_id`
- 新增桌面平台只能靠补丁式兼容，模型会越来越别扭

---

## 3. 改造后的新架构

### 3.1 新模型

现在系统的核心模型变成：

- `Product`
- `Variant`
- `Release`

含义如下：

#### `Product`

表示一个产品页，是用户最终访问和分享的主体。

例如：

- 金语
- 企业内测客户端
- 某桌面工具

主要字段：

- `id`
- `slug`
- `name`
- `description`
- `icon_path`
- `published`

#### `Variant`

表示产品下的一个平台变体。

例如同一个产品下可以有：

- iOS 版
- Android 版
- macOS 版
- Windows 版
- Linux 版

主要字段：

- `product_id`
- `platform`
- `identifier`
- `display_name`
- `arch`
- `installer_type`
- `min_os`
- `published`
- `sort_order`

说明：

- `identifier` 是统一标识字段
- 对 iOS 可对应 `bundle id`
- 对 Android 可对应 `package name`
- 对桌面平台可用产品定义的唯一标识

#### `Release`

表示某个变体下的一次具体发布，也就是一个实际上传的安装包。

主要字段：

- `variant_id`
- `version`
- `build`
- `changelog`
- `min_os`
- `storage_path`
- `file_name`
- `file_ext`
- `mime_type`
- `sha256`
- `size_bytes`
- `download_count`
- `channel`

说明：

- 每个平台独立维护自己的 `version/build/changelog`
- `Release` 才是真正对应磁盘或对象存储中制品文件的记录

---

## 4. 新架构的关系

关系可以理解为：

```text
Product
  ├─ Variant (iOS)
  │    ├─ Release 1
  │    └─ Release 2
  ├─ Variant (Android)
  │    └─ Release 1
  ├─ Variant (macOS)
  │    └─ Release 1
  ├─ Variant (Windows)
  │    └─ Release 1
  └─ Variant (Linux)
       └─ Release 1
```

也就是说：

- 一个 `Product` 对外对应一个统一产品页
- 一个 `Product` 下有多个 `Variant`
- 一个 `Variant` 下有多个 `Release`

---

## 5. 当前实现中的关键原则

### 5.1 页面以 Product 为中心

用户访问的是：

- `/products/:productID`
- 或 `/products/:slug`

而不是旧的 `/apps/:appID`

产品页会一次性返回该产品下所有已发布变体及其版本信息。

### 5.2 上传以 Variant 为中心

上传不再是“给某个 app 上传”，而是：

- 先选 `Product`
- 再选 `Variant`
- 最后上传对应平台的安装包

也就是说，一个上传动作的核心目标是“给某个变体新增一个 release”。

### 5.3 统计与事件以 Variant 为中心

当前事件、后台统计、导出、UDID 绑定等都已经切换到 `variant_id` 维度。

这意味着：

- 同一个产品下不同平台可以独立统计
- iOS 与 Android 的行为不会混在一起
- 新建变体不再依赖旧 `app_id`

### 5.4 iOS 继续保留特殊安装链路

iOS 仍然有自己的特殊逻辑：

- 设备 UDID 绑定
- `manifest.plist`
- `itms-services` 安装入口
- provisioning profile 信息展示

但这些能力现在绑定在 `Variant` 上，而不是旧的 `App` 上。

### 5.5 桌面平台走标准下载链路

`macOS / Windows / Linux / Android` 默认走标准下载：

- 用户点击下载
- 访问 `/d/:releaseID`
- 服务端返回文件，或跳转到对象存储公共地址

---

## 6. 当前保留的兼容字段

为了迁移旧数据，当前模型里还保留了部分 legacy 字段：

- `Product.LegacyAppID`
- `Variant.LegacyAppID`
- `Release.AppID`
- `Event.AppID`
- `DeviceAppBinding.AppID`
- `UDIDNonce.AppID`

这些字段的定位是：

- 兼容旧数据
- 兼容历史迁移路径
- 便于老记录和新模型对齐

但新的业务和新代码应优先使用：

- `product_id`
- `variant_id`
- `release_id`

不要再把旧 `app_id` 当作主模型使用。

---

## 7. 当前公开访问方式

### 7.1 产品页

主访问入口：

- `/products/:slug`
- `/products/:productID`

公开接口：

- `GET /api/products/:productID`
- `GET /api/products/slug/:slug`

返回结构核心为：

- `product`
- `variants[]`
- 每个 `variant` 下有 `latest_release`
- 每个 `variant` 下有 `releases[]`

### 7.2 旧链接处理

旧的：

- `/apps/:appID`

现在会重定向到对应的产品页。

因此外部不应该再继续生成新的 `/apps/:appID` 分享链接。

### 7.3 下载

统一下载入口：

- `GET /d/:releaseID`

行为：

- 本地存储时直接下发文件
- S3/R2 场景下跳转到公共下载地址

注意：

- 只有在文件真实可用时才会增加 `download_count` 并记录 `download` 事件
- 如果 `storage_path` 指向的文件缺失，会返回 `404 file missing`

### 7.4 iOS 安装

iOS 版本的公开动作包含：

- `ios_manifest`
- `ios_install`

用户流程为：

1. 先进行设备绑定
2. 服务端保存 UDID 与变体绑定关系
3. 绑定成功后再显示安装入口

---

## 8. 当前后台使用方式

### 8.1 创建产品

后台先创建 `Product`，填写：

- 名称
- slug
- 描述
- 发布状态

一个产品就是一个对外分发页面。

### 8.2 创建变体

然后在产品下创建一个或多个 `Variant`。

每个变体至少需要明确：

- `platform`
- `identifier`

可选补充：

- `display_name`
- `arch`
- `installer_type`
- `min_os`
- `sort_order`
- `published`

示例：

- `platform=ios, identifier=com.example.app`
- `platform=windows, identifier=example-desktop, arch=x64, installer_type=msi`
- `platform=linux, identifier=example-desktop, arch=arm64, installer_type=appimage`

### 8.3 上传版本

上传时必须基于 `variant_id`。

上传接口：

- `POST /upload`

核心表单字段：

- `variant_id`
- `app_file`
- `version`
- `build`
- `channel`
- `min_os`
- `changelog`
- `icon_base64` 可选

智能上传接口：

- `POST /smart-upload`

当前智能上传主要适用于：

- iOS
- Android

桌面平台以普通上传为主。

### 8.4 发布控制

发布状态分两层：

- `Product.published`
- `Variant.published`

只有当：

- 产品已发布
- 变体已发布

该变体才会出现在公开产品页中。

### 8.5 统计与事件

后台统计和事件查询现在以 `variant_id` 为主。

含义是：

- 查看某个产品下某个平台的下载与事件
- 不再依赖旧 `app_id`

### 8.6 UDID 设备管理

iOS 设备绑定现在按 `variant_id` 关联。

这意味着：

- 同一个产品下多个 iOS 变体可以独立管理绑定设备
- 设备绑定、导出、筛选都应按变体维度理解

---

## 9. 当前支持的文件类型

### iOS

- `.ipa`

### Android

- `.apk`

### macOS

- `.dmg`
- `.pkg`
- `.zip`

### Windows

- `.exe`
- `.msi`
- `.zip`

### Linux

- `.AppImage`
- `.deb`
- `.rpm`
- `.zip`
- `.tar.gz`

---

## 10. 当前文件存储方式

现在本地存储的目录结构是：

```text
uploads/{productID}/{variantID}/{releaseID}/{filename}
```

例如：

```text
uploads/prd_xxx/var_xxx/rel_xxx/app.ipa
```

这和旧架构下的按 `appID` 分目录不同。

其优点是：

- 目录结构与新模型一致
- 多平台变体天然隔离
- 同一产品下多平台包不会互相混淆

---

## 11. 推荐的最新使用方式

### 11.1 面向运营/后台

推荐流程：

1. 先创建 `Product`
2. 在产品下创建各平台 `Variant`
3. 每个变体单独上传自己的 `Release`
4. 打开产品发布状态
5. 按需打开各变体发布状态
6. 对外分享产品页链接 `/products/:slug`

不要再使用：

- 直接围绕旧 `App` 思维管理分发
- 分享 `/apps/:appID`
- 把不同平台强行塞进同一个旧 app 记录

### 11.2 面向前台用户

推荐流程：

1. 打开产品页
2. 页面自动展示所有可用平台
3. 当前设备平台会被优先排序
4. iOS 用户先绑定设备再安装
5. 其他平台用户直接下载相应安装包

### 11.3 面向开发维护者

新增功能时，请遵循：

- 页面入口优先挂在 `Product`
- 平台差异优先挂在 `Variant`
- 版本与制品优先挂在 `Release`
- 统计、事件、导出、绑定优先使用 `variant_id`

如果要新增字段，优先判断它属于哪一层：

- 产品级信息：加到 `Product`
- 平台级信息：加到 `Variant`
- 版本级信息：加到 `Release`

---

## 12. 迁移后的现状结论

当前系统已经从“移动 App 分发”切换为“多平台产品分发”。

可以认为现在的主模型已经是：

- `Product -> Variant -> Release`

而旧 `App` 相关内容已经降为：

- 兼容
- 迁移辅助
- 历史数据过渡

后续新功能、新页面、新接口设计，都应围绕新架构继续演进。

---

## 13. 一句话总结

旧架构是“一个 app 对应一个平台，一个页面只服务一个 app”；  
新架构是“一个 product 对应一个统一产品页，product 下有多个 variant，每个 variant 下有独立 release，所有平台统一展示、独立分发、独立统计”。
