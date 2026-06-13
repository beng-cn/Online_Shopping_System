# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 常用命令

```bash
npm create vite@latest . -- --template vue    # 初始化 Vue 3 项目（首次）
npm install                                     # 安装依赖
npm run dev                                     # 启动开发服务器（默认端口5173）
npm run build                                   # 生产构建
npm run preview                                 # 预览构建产物
```

## 技术栈

- **Vue 3** (Composition API + `<script setup>` 语法糖)
- **Element Plus** (管理后台 UI 组件库)
- **Vue Router 4** (前端路由，对应后端三组：公开 / 认证 / 管理)
- **Pinia** (状态管理，存储 JWT Token 和用户信息)
- **Axios** (HTTP 请求，封装 JWT 拦截器)
- **Vite** (构建工具)

## 架构

### 路由设计（对应后端 `internal/router/router.go`）

| 前端路径 | 后端 API 组 | 鉴权 |
|---|---|---|
| `/` | 商城首页/商品列表 | 无 |
| `/flash` | 秒杀活动页 | 无 |
| `/login` | 登录 | 无 |
| `/cart` | 购物车 | 需 JWT |
| `/orders` | 我的订单 | 需 JWT |
| `/admin/*` | 管理后台 | 需 JWT + role_id=1 |

### API 封装（Axios 拦截器）

请求拦截器自动从 Pinia 读 Token 注入 `Authorization: Bearer <token>`。
响应拦截器统一处理 `code=401` 跳转登录页。

### 后端接口速查（`../backend/internal/router/router.go`）

```
公开: /api/user/register|login, /api/product/list|:id, /api/flash/list|:id
认证: /api/auth/user/info, /api/auth/cart/*, /api/auth/order/*, /api/auth/flash/*
管理: /api/admin/product, /api/admin/flash/*, /api/admin/category/*, /api/admin/user/*
```

### 关键业务模块

- **秒杀页面** (`views/shop/FlashSale.vue`)：倒计时组件 → 排队入场 → 抢购按钮 → 结果展示
- **管理后台** (`views/admin/`)：商品管理、秒杀管理（创建/预热/结束）、用户管理、分类管理

## 后端对接要点

- 后端运行在 `localhost:8080`，Vite 开发服务器在 `localhost:5173`
- 需配置 Vite 代理：`/api` → `http://localhost:8080`
- 上传图片路径：`http://localhost:8080/uploads/xxx.png`
- JWT 过期默认 24 小时，过期后自动跳转登录页
