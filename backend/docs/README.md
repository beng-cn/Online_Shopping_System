# 电商秒杀系统 — 缓存与数据库故障处理生产级方案

> 基于 Gin + GORM + MySQL + Redis 主流技术栈，面向数十万用户并发的面试级落地方案

---

## 📖 文档导航

| 序号 | 文档 | 主题 | 难度 | 建议阅读时间 |
|------|------|------|------|-------------|
| 1 | [01-architecture.md](./01-architecture.md) | 代码架构设计 | ⭐⭐ | 45 min |
| 2 | [02-cache-design.md](./02-cache-design.md) | 缓存体系设计与全场景故障处理 | ⭐⭐⭐ | 90 min |
| 3 | [03-database-design.md](./03-database-design.md) | 数据库设计与全场景故障处理 | ⭐⭐⭐ | 75 min |
| 4 | [04-cascading-failures.md](./04-cascading-failures.md) | 缓存+数据库连锁故障处置 | ⭐⭐⭐⭐ | 60 min |

## 💻 示例代码

项目已实现以下核心功能模块，代码位于相应目录：

| 模块 | 文件路径 | 功能 |
|------|----------|------|
| 环境配置 | [internal/config/](../../internal/config/) | 敏感信息环境注入 + 启动门禁校验 |
| 布隆过滤器 | [internal/pkg/bloom/](../../internal/pkg/bloom/) | 防缓存穿透 |
| 分布式锁 | [internal/pkg/semaphore/](../../internal/pkg/semaphore/) | 信号量限流 |
| 本地缓存 | [internal/pkg/localcache/](../../internal/pkg/localcache/) | 内存缓存 |
| 请求追踪 | [internal/middleware/trace.go](../../internal/middleware/trace.go) | 请求追踪中间件 |
| 慢查询日志 | [internal/pkg/logger/gorm_logger.go](../../internal/pkg/logger/gorm_logger.go) | 慢SQL自动埋点插件 |
| 断路器 | [internal/pkg/breaker/](../../internal/pkg/breaker/) | 数据库断路器 |
| 秒杀限流 | [internal/pkg/flashlimiter/](../../internal/pkg/flashlimiter/) | 秒杀限流器 |

> 注：示例代码目录已集成到项目源码中，无需额外下载。

## 🎯 阅读路线

### 面试突击路线（3小时）
```
01-architecture.md → 02-cache-design.md（重点读故障表）→ 03-database-design.md（重点读故障表）
```

### 深度学习路线（8小时）
```
01-architecture.md → 02-cache-design.md → 03-database-design.md → 04-cascading-failures.md → assets/code/*
```

### 按岗位推荐

| 岗位 | 必读 | 选读 |
|------|------|------|
| 初级 Go 后端 | 01-architecture.md（三层架构部分） | — |
| 中级 Go 后端 | 01 全篇 + 02 全篇 | assets/code/ |
| 高级 Go 后端 | 全部 4 篇 + 全部代码 | 结合项目源码阅读 |

## 📋 术语表

| 术语 | 英文 | 说明 |
|------|------|------|
| 缓存穿透 | Cache Penetration | 大量查询不存在的数据，绕过缓存直击数据库 |
| 缓存击穿 | Hotspot Invalid / Cache Breakdown | 热点 Key 过期瞬间大量并发请求涌入数据库 |
| 缓存雪崩 | Cache Avalanche | 大量 Key 同时过期导致数据库压力骤增 |
| 布隆过滤器 | Bloom Filter | 概率型数据结构，用于快速判断元素是否可能存在于集合中 |
| 乐观锁 | Optimistic Locking | 通过版本号（version 字段）检测并发冲突的并发控制机制 |
| 读写分离 | Read/Write Splitting | 写操作走主库，读操作走从库的数据库架构模式 |
| 熔断器 | Circuit Breaker | 在服务不可用时自动切断请求，防止故障扩散的保护机制 |
| 依赖倒置 | Dependency Inversion | 高层模块不依赖低层模块，两者都依赖抽象（接口） |
| DI | Dependency Injection | 依赖注入——将组件依赖的外部实现通过构造函数/Setter 传入，编译期 DI 是该方案首选 |

## 🏗 对应项目

本方案基于 [d:\Online_Shopping_System\backend](../) 项目实战经验编写。该项目实现了：

- 用户注册/登录（JWT + Redis 黑名单）
- 商品管理（MySQL 全文搜索、关键词自动生成）
- 购物车与订单
- **秒杀系统**（三级防超卖：本地内存 → Redis Lua → DB 唯一约束）
- 支付宝支付集成
- 缓存预热 + 自动续期 + 缓存降级
- 崩溃恢复与库存对账

技术栈：Go 1.25 + Gin v1.12 + GORM v1.31 + MySQL 8.0 + Redis 7 + Google Wire

---

## ⚠️ 注意事项

1. 所有代码片段与项目当前依赖版本兼容（Gin v1.12、GORM v1.31、go-redis v8.11）
2. 故障命令/配置基于 MySQL 8.0 和 Redis 7.x 编写
3. 部分增强方案（如布隆过滤器、两级缓存）需要引入额外依赖，已在各文档中注明
4. 所有配置示例中的密码/密钥均为占位符，生产环境必须通过环境变量注入
