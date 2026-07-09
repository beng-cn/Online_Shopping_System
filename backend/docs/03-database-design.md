# 03 — 数据库设计与全场景故障处理

> **难度**：⭐⭐⭐ &emsp; **建议阅读时间**：75 分钟  
> **前置知识**：MySQL 基础（索引/事务/锁）、GORM 基础用法、已阅读 [01-architecture.md](./01-architecture.md)  
> **下一篇**：[04-cascading-failures.md](./04-cascading-failures.md) — 缓存+数据库连锁故障处置

---

## 📋 目录

- [3.1 数据库连接池配置与监控](#31-数据库连接池配置与监控)
- [3.2 Repository 模式与接口设计](#32-repository-模式与接口设计)
- [3.3 WithTx 事务传播模式](#33-withtx-事务传播模式)
- [3.4 乐观锁并发控制](#34-乐观锁并发控制)
- [3.5 N+1 问题规避](#35-n1-问题规避)
- [3.6 MySQL 全文搜索](#36-mysql-全文搜索)
- [3.7 关键词自动生成](#37-关键词自动生成)
- [3.8 慢查询监控（GORM Logger）](#38-慢查询监控gorm-logger)
- [3.9 熔断器（Circuit Breaker）](#39-熔断器circuit-breaker)
- [3.10 信号量限流（Semaphore）](#310-信号量限流semaphore)
- [3.11 故障处理速查表](#311-故障处理速查表)

---

## 3.1 数据库连接池配置与监控

### 连接池参数

[`internal/repository/mysql/db.go`](../internal/repository/mysql/db.go#L39-L42)：

```go
sqlDB.SetMaxOpenConns(50)                 // 最大打开连接数
sqlDB.SetMaxIdleConns(10)                 // 最大空闲连接数
sqlDB.SetConnMaxLifetime(1 * time.Hour)   // 连接最大存活时间
sqlDB.SetConnMaxIdleTime(10 * time.Minute) // 空闲连接最大存活时间
```

### 参数选取原则

| 参数 | 值 | 公式/依据 | 说明 |
|------|-----|----------|------|
| `MaxOpenConns` | 50 | `(CPU核心数×2) + 有效磁盘数` ≈ `8×2+4` ≈ 20，预留余量 | 太小→并发瓶颈；太大→MySQL 连接数打满 |
| `MaxIdleConns` | 10 | `MaxOpenConns 的 20-25%` | 保持一定预热连接，但不过多占用 MySQL 资源 |
| `ConnMaxLifetime` | 1h | 略小于 MySQL `wait_timeout`（默认 8h） | 定期回收避免 MySQL 侧主动断开导致 `broken pipe` |
| `ConnMaxIdleTime` | 10min | 空闲连接回收比 Lifetime 更激进 | 及时释放长时间不用的连接 |

### GORM Logger 插件

[`internal/pkg/logger/gorm_logger.go`](../internal/pkg/logger/gorm_logger.go) 实现 `gorm.io/gorm/logger.Interface`，作为 GORM 的 SQL 日志插件：

```go
// 创建数据库连接时注入自定义 Logger
gormLog := logger.NewSlowQueryLogger(slowThreshold)
db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
    Logger: gormLog, // 自定义慢查询日志记录器
})
```

### 环境差异化阈值

[`internal/pkg/logger/gorm_logger.go`](../internal/pkg/logger/gorm_logger.go#L144-L153)：

| 环境 | GO_ENV | 慢查询阈值 | 日志级别 |
|------|--------|-----------|---------|
| 生产 | `prod` | 200ms | Warn（只记录慢查询+错误） |
| 开发 | `dev` | 100ms | Info（记录所有 SQL） |
| 测试 | `test` | 500ms | Warn |

### 健康监控建议

```sql
-- 当前连接数（应与 MaxOpenConns 对比）
SHOW STATUS LIKE 'Threads_connected';

-- 活跃连接数
SHOW PROCESSLIST;

-- 连接历史峰值
SHOW STATUS LIKE 'Max_used_connections';

-- 慢查询累计数
SHOW STATUS LIKE 'Slow_queries';
```

---

## 3.2 Repository 模式与接口设计

### 接口定义规范

每个数据实体对应一个 Repository 接口，定义在独立文件中，方法按 CRUD 分组：

[`internal/repository/mysql/product_repository.go`](../internal/repository/mysql/product_repository.go#L35-L52)：

```go
type ProductRepository interface {
    // 基础 CRUD
    Create(product *entity.Product) error
    Update(product *entity.Product) error
    Delete(id uint) error
    GetByID(id uint) (*entity.Product, error)

    // 批量查询（防 N+1）
    GetByIDs(ids []uint) ([]*entity.Product, error)
    GetAllIDs() ([]uint, error)

    // 搜索与列表
    List(keyword string, categoryID string) ([]*entity.Product, error)
    ListPage(req *request.ProductListRequest) ([]*entity.Product, int64, error)

    // 乐观锁扣减库存
    DecreaseStockWithVersion(id uint, quantity int, version int) (bool, error)

    // 事务传播
    WithTx(tx *gorm.DB) ProductRepository

    // 销量管理
    ListHotProductsBySales(limit int) ([]*entity.Product, error)
    IncreaseSales(id uint, quantity int) error
    BatchUpdateSales(stats []SalesStat) error
}
```

### 实现类不导出

```go
// 结构体小写（package-private），外部只能通过接口访问
type productRepository struct {
    db *gorm.DB
}

// 构造函数返回接口类型（编译期保证实现完整）
func NewProductRepository(db *gorm.DB) ProductRepository {
    return &productRepository{db: db}
}
```

### 接口化的三大收益

| 收益 | 说明 |
|------|------|
| **可测试性** | 单元测试时可 Mock Repository 接口，不依赖真实数据库 |
| **可替换性** | 切换存储引擎（如 MySQL → PostgreSQL）仅需实现同一接口 |
| **依赖倒置** | Service 层依赖抽象接口而非具体实现，符合 SOLID 原则 |

---

## 3.3 WithTx 事务传播模式

### 问题场景

订单创建涉及多个 Repository：扣减库存（ProductRepo）、创建订单（OrderRepo）、创建订单项（OrderItemRepo）、清空购物车（CartRepo）。这些操作必须在**同一数据库事务**中完成——要么全部成功，要么全部回滚。

### 设计方案

每个 Repository 接口都包含 `WithTx(tx *gorm.DB) XxxRepository` 方法：

[`internal/repository/mysql/product_repository.go`](../internal/repository/mysql/product_repository.go#L159-L162)：

```go
// WithTx 返回绑定事务的新仓储实例（原仓储不受影响）
func (r *productRepository) WithTx(tx *gorm.DB) ProductRepository {
    return &productRepository{db: tx}
}
```

所有 Repository 都实现了此模式：
- [OrderRepository.WithTx](../internal/repository/mysql/order_repository.go)
- [OrderItemRepository.WithTx](../internal/repository/mysql/order_item_repository.go)
- [CartRepository.WithTx](../internal/repository/mysql/cart_repository.go)
- [CategoryRepository.WithTx](../internal/repository/mysql/category_repository.go)

### Service 层标准事务模板

```go
func (s *flashService) createFlashOrder(userID uint, flashSaleID uint, quantity int) error {
    // 1. 开启事务
    tx := s.db.Begin()
    if tx.Error != nil {
        return tx.Error
    }

    // 2. defer 回滚保护（panic 或 业务错误时自动回滚）
    defer func() {
        if r := recover(); r != nil {
            tx.Rollback()
            panic(r) // 重新抛出 panic 保持上层 recovery 可见
        }
    }()

    // 3. 获取事务绑定的 Repository
    productRepo := s.productRepo.WithTx(tx)
    orderRepo := s.orderRepo.WithTx(tx)
    orderItemRepo := s.orderItemRepo.WithTx(tx)
    cartRepo := s.cartRepo.WithTx(tx)

    // 4. 在事务中执行多个 Repository 操作
    ok, err := productRepo.DecreaseStockWithVersion(productID, quantity, version)
    if err != nil || !ok {
        tx.Rollback()
        return err
    }

    orderRepo.Create(order)
    orderItemRepo.BatchCreate(items)
    cartRepo.DeleteByUserAndProduct(userID, productID)

    // 5. 提交事务
    return tx.Commit().Error
}
```

### 关键安全措施

- **defer + recover**：防止 panic 导致事务悬挂（未提交也未回滚，锁住数据行）
- **先 Rollback 后返回**：业务错误也需显式回滚（GORM 不会自动回滚未提交的事务）
- **WithTx 模式 vs GORM Transaction 闭包**：显式控制比闭包更灵活，可以跨多个 Service 方法传递事务

---

## 3.4 乐观锁并发控制

### 为什么需要乐观锁

秒杀场景下，多个用户同时购买同一商品。如果不加并发控制：

```text
时间线：
  T1: 请求A 读取 stock=1
  T2: 请求B 读取 stock=1
  T3: 请求A 写入 stock=0 ✓
  T4: 请求B 写入 stock=0 ✗（超卖！应该失败但成功了）
```

### Version 字段设计

两个高频并发实体使用 `version` 字段：

- [Product.Version](../internal/model/entity/product.go#L21)：库存扣减的乐观锁
- [FlashSale.Version](../internal/model/entity/flash_sale.go#L22)：秒杀状态更新的乐观锁

```go
type Product struct {
    gorm.Model
    // ...
    Version int `gorm:"default:0" json:"-"` // 乐观锁版本号，不暴露给前端
}
```

### 扣减库存实现

[`internal/repository/mysql/product_repository.go`](../internal/repository/mysql/product_repository.go#L145-L157)：

```go
func (r *productRepository) DecreaseStockWithVersion(id uint, quantity int, version int) (bool, error) {
    result := r.db.Model(&entity.Product{}).
        Where("id = ? AND version = ?", id, version).     // 版本匹配
        Updates(map[string]interface{}{
            "stock":   gorm.Expr("stock - ?", quantity),   // 原子扣减
            "version": version + 1,                        // 版本号 +1
        })
    if result.RowsAffected == 0 {
        return false, nil  // 版本冲突：被其他请求抢先修改
    }
    return true, nil
}
```

生成的 SQL：

```sql
UPDATE products
SET stock = stock - 1, version = version + 1
WHERE id = 42 AND version = 5
-- RowsAffected = 0 → 版本冲突（其他事务已修改）
-- RowsAffected = 1 → 扣减成功
```

### Service 层重试策略

```go
func (s *flashService) deductStock(id uint, quantity int) error {
    maxRetries := 3
    for i := 0; i < maxRetries; i++ {
        // 1. 读取当前版本号
        product, err := s.productRepo.GetByID(id)
        if err != nil { return err }

        // 2. 检查库存是否充足
        if product.Stock < quantity {
            return errors.New(errors.CodeStockInsufficient, "库存不足")
        }

        // 3. 乐观锁扣减
        ok, err := s.productRepo.DecreaseStockWithVersion(id, quantity, product.Version)
        if err != nil { return err }

        if ok {
            return nil // 成功
        }
        // 4. 版本冲突 → 重试（最多 3 次）
    }
    return errors.New(errors.CodeStockInsufficient, "抢购失败，请重试")
}
```

### 乐观锁 vs 悲观锁

| 维度 | 乐观锁（本项目） | 悲观锁（SELECT FOR UPDATE） |
|------|----------------|--------------------------|
| 实现方式 | `WHERE version = ?` + 重试 | 事务中 `SELECT ... FOR UPDATE` 锁住行 |
| 冲突处理 | 检测冲突 → 重试 | 排队等待锁释放 |
| 性能 | 高并发下重试少时优秀 | 高并发下行锁排队严重 |
| 死锁风险 | 零 | 有（锁顺序不一致时） |
| 适用场景 | 冲突概率低（读多写少） | 冲突概率高或必须串行化 |
| 本项目使用 | 秒杀库存扣减 | 不使用 |

**秒杀场景为什么不用悲观锁？** 上千并发请求同时 `SELECT FOR UPDATE` 同一行，会在 MySQL 层排长队，大量连接被占用，迅速耗尽连接池。

---

## 3.5 N+1 问题规避

### 什么是 N+1 问题

```text
场景：查询 10 个秒杀订单，每单需要显示商品名称

错误做法：
  1. SELECT * FROM orders WHERE flash_sale_id = 7     → 返回 10 行
  2. SELECT * FROM products WHERE id = 101             → 第 1 次
  3. SELECT * FROM products WHERE id = 102             → 第 2 次
  ...
  11. SELECT * FROM products WHERE id = 110            → 第 10 次

  总计 1 + 10 = 11 次 SQL（N+1）
```

### 解决方案一：批量 IN 查询 + 内存组装

[`internal/repository/mysql/product_repository.go`](../internal/repository/mysql/product_repository.go#L103-L113)：

```go
// GetByIDs 一次 SQL 获取全部商品
func (r *productRepository) GetByIDs(ids []uint) ([]*entity.Product, error) {
    var products []*entity.Product
    err := r.db.Where("id IN ?", ids).Find(&products).Error
    // SELECT * FROM products WHERE id IN (101,102,...,110)
    return products, err
}
```

Service 层模式：

```go
// 1. 收集所有需要的 ID
productIDs := make([]uint, 0, len(orders))
for _, order := range orders {
    productIDs = append(productIDs, order.ProductID)
}

// 2. 批量查询
products, _ := s.productRepo.GetByIDs(productIDs)

// 3. 内存组装 map（O(1) 查找）
productMap := make(map[uint]*entity.Product, len(products))
for _, p := range products {
    productMap[p.ID] = p
}

// 4. 填充关联数据
for _, order := range orders {
    order.Product = productMap[order.ProductID]
}
```

### GORM Preload 的使用边界

```go
// ✅ 可用场景：单条查询
db.Preload("Product").First(&flashSale, id)  // 1+1=2 次 SQL，可接受

// ⚠️ 谨慎场景：列表查询
db.Preload("Product").Find(&flashSales)      // 1+N 次 SQL，N 条记录时需优化
```

### N+1 检测方法

通过 GORM Logger 的慢查询日志识别：同一请求内出现大量结构相同的 SQL，仅参数不同（如 `WHERE id = ?` 重复出现）。

---

## 3.6 MySQL 全文搜索

### 全文索引

数据库层在建表时对 `products` 表建立了 `FULLTEXT INDEX`，覆盖 `name` 和 `keywords` 两个字段。

### BOOLEAN MODE 搜索

[`internal/repository/mysql/product_repository.go`](../internal/repository/mysql/product_repository.go#L127-L143)：

```go
func (r *productRepository) List(keyword string, categoryID string) ([]*entity.Product, error) {
    db := r.db.Model(&entity.Product{})
    if keyword != "" {
        db = db.Where("MATCH(name, keywords) AGAINST(? IN BOOLEAN MODE)",
            escapeFulltextKeyword(keyword))
    }
    if categoryID != "" {
        db = db.Where("category_id = ?", categoryID)
    }
    // ...
}
```

生成的 SQL：

```sql
SELECT * FROM products
WHERE MATCH(name, keywords) AGAINST('手机 5g' IN BOOLEAN MODE)
  AND category_id = 3
```

### 特殊字符转义

[`internal/repository/mysql/product_repository.go`](../internal/repository/mysql/product_repository.go#L12-L28)：

```go
func escapeFulltextKeyword(kw string) string {
    replacer := strings.NewReplacer(
        "+", "\\+", "-", "\\-", ">", "\\>",
        "<", "\\<", "(", "\\(", ")", "\\)",
        "~", "\\~", "*", "\\*", "\"", "\\\"", "@", "\\@",
    )
    return replacer.Replace(kw)
}
```

**为什么需要转义？** MySQL 的 BOOLEAN MODE 中 `+` 表示"必须包含"、`-` 表示"必须不包含"、`*` 是通配符。用户输入 `C++入门` 时，未转义的 `+` 会导致语法错误；恶意的 `*` 可导致全表扫描。

---

## 3.7 关键词自动生成

### 为什么需要关键词

中文电商搜索的挑战：用户可能搜索"苹果手机"、"iPhone"、"水果手机"，但数据库 `name` 字段只有 `iPhone 15 Pro`。关键词字段作为搜索别名，大幅提升召回率。

### 四维度生成策略

[`internal/pkg/keywords/generator.go`](../internal/pkg/keywords/generator.go) 的 `Generate` 函数从四个维度提取关键词：

```text
输入：商品名称 "iPhone 15 Pro Max" + 分类路径 ["电子产品", "手机"]

维度1 → 品牌映射：
  "iphone" → "苹果手机,apple,智能手机,ios"

维度2 → 属性映射：
  "pro" → "专业版,pro,高端"
  "max" → "大屏版,max,顶配"

维度3 → 分类层级：
  从分类路径中提取层级关键词

维度4 → 名称拆解：
  按空格/-/_/括号拆分名称，每个部分作为独立关键词

输出（去重后）："苹果手机,apple,智能手机,ios,专业版,pro,高端,大屏版,max,顶配,电子产品,手机"
```

### 品牌映射表（部分）

```go
var brandMapping = map[string]string{
    "iphone":      "苹果手机,apple,智能手机,ios",
    "huawei":      "华为手机,huawei,智能手机,鸿蒙,harmonyos,国产手机",
    "macbook":     "苹果电脑,mac,苹果笔记本,笔记本电脑",
    "xiaomi":      "小米手机,小米,xiaomi,智能手机,性价比",
    // ... 共 20+ 品牌
}
```

### 属性映射表（部分）

```go
var attributeMapping = map[string]string{
    "pro":   "专业版,pro,高端",
    "max":   "大屏版,max,顶配",
    "ultra": "旗舰版,ultra,顶配",
    "air":   "轻薄版,air,便携",
    "5g":    "5g手机,5g网络",
    // ...
}
```

### 自动触发时机

[`internal/service/product/product_service.go`](../internal/service/product/product_service.go)：

- **创建商品时**：`keywords` 字段留空 → 自动生成
- **更新商品时**：名称或分类变更 → 重新生成
- **管理员手动触发**：`/admin/product/batch-keywords` 接口批量回填

---

## 3.8 慢查询监控（GORM Logger）

### 三层日志策略

[`internal/pkg/logger/gorm_logger.go`](../internal/pkg/logger/gorm_logger.go#L67-L108) 的 `Trace` 方法在每次 SQL 执行后触发：

```text
SQL 执行完成
    │
    ├─ 错误？→ 必记录 [GORM Error] SQL + elapsed + error + requestID
    │
    ├─ 未超过阈值？→ 跳过（生产环境不记录正常 SQL）
    │
    └─ 超过阈值？→ 记录 [GORM Warn] SQL + elapsed + rows + requestID
           │
           └─ 同请求 ≥3 条慢查询？→ 额外打印 [GORM WARNING]
```

### Request ID 关联追踪

[`internal/pkg/logger/gorm_logger.go`](../internal/pkg/logger/gorm_logger.go#L116-L125)：

```go
func extractRequestID(ctx context.Context) string {
    if ctx == nil { return "unknown" }
    if requestID, ok := ctx.Value("request_id").(string); ok {
        return requestID
    }
    return "unknown"
}
```

Request ID 由 Trace 中间件注入 `context.Context`，GORM Logger 从中提取。这使得每个 HTTP 请求关联的 SQL 可通过 Request ID 串联：

```text
[GORM Warn] /api/v1/product/list [request_id=a1b2c3d4] 耗时 350ms
  → 追踪到具体是哪个用户请求导致的慢查询
```

### 慢查询计数器

一个请求内如果有 ≥3 条慢查询，GORM Logger 会打印 `⚠️ 单次请求慢查询数量过多` 警告。这能快速发现 N+1 问题——循环中的多次 SQL 很容易触发此告警。

---

## 3.9 熔断器（Circuit Breaker）

> 详细状态机原理已在 [02-cache-design.md §2.4](./02-cache-design.md#第二层熔断器三态状态机) 中阐述，本节仅做数据库保护视角的补充。

### 配置参数

[`internal/pkg/breaker/circuit_breaker.go`](../internal/pkg/breaker/circuit_breaker.go)：

| 参数 | 默认值 | 含义 |
|------|--------|------|
| `maxErrors` | 5 | 连续 5 次 DB 错误 → 熔断 |
| `timeout` | 30s | 熔断 30 秒后试探恢复 |
| `windowSize` | 滑动窗口 | 窗口内错误计数 |

### 集成方式

[`internal/service/product/product_service.go`](../internal/service/product/product_service.go)：

```go
func (s *productService) GetProductByID(id uint) (*response.ProductResponse, error) {
    // ... 布隆 → 缓存 → 锁 ...

    // DB 查询前检查熔断器
    if !s.breaker.Allow() {
        return nil, errors.New(errors.CodeServerError, "服务暂时不可用，请稍后重试")
    }

    product, err := s.productRepo.GetByID(id)
    if err != nil {
        s.breaker.RecordFailure()  // 记录失败 → 可能触发熔断
        return nil, err
    }
    s.breaker.RecordSuccess()     // 记录成功 → 关闭熔断器
    return product, nil
}
```

### 熔断触发条件

熔断只在**连续 DB 错误**时触发（如连接超时、连接池耗尽），单次业务错误（如商品不存在）不计入。

---

## 3.10 信号量限流（Semaphore）

> 详细原理已在 [02-cache-design.md §2.4](./02-cache-design.md#第三层信号量限流) 中阐述，本节仅做参数配置补充。

### 配置参数

[`internal/pkg/semaphore/limiter.go`](../internal/pkg/semaphore/limiter.go)：

| 参数 | 默认值 | 含义 |
|------|--------|------|
| `maxConcurrent` | 30 | 最多同时 30 个 goroutine 查 DB |
| `timeout` | 2s | 排队超时 → 返回错误 |

### 为什么是 30（连接池的 60%）

```text
连接池 MaxOpenConns = 50
    ├── 30 个槽位给并发查询（信号量保护）
    └── 20 个槽位给其他操作（事务提交、缓存回写、后台任务）
```

如果信号量设为 50，缓存雪崩时 DB 查询占满所有连接，事务无法提交 → 锁等待 → 连锁超时。

### 使用规范

```go
// Acquire 必须在 defer Release 之前
if err := s.semaphore.Acquire(ctx); err != nil {
    return nil, err
}
defer s.semaphore.Release()  // ← 必须 defer，防止忘记释放

product, err := s.productRepo.GetByID(id)
```

---

## 3.11 故障处理速查表

### 数据库故障五大类

| 故障类型 | 故障现象 | 根因 | 处置步骤 | 预防措施 | 恢复验证 | 代码位置 |
|---------|---------|------|---------|---------|---------|---------|
| **主库宕机** | 全部写操作失败 | 硬件故障/网络分区/磁盘满 | ①切换从库为主库 ②更新连接串 ③重启服务 | MHA/Orchestrator 高可用 + 读写分离 | 写操作恢复 | [db.go](../internal/repository/mysql/db.go) |
| **连接池耗尽** | 请求超时/大量排队 | 慢查询堆积/并发突增/长事务未提交 | ①从 GORM Logger 定位耗时 SQL ②KILL 长事务 ③临时增大 MaxOpenConns | 信号量限流 + 合理 PoolSize + 熔断器 | Threads_connected < MaxOpenConns | [db.go](../internal/repository/mysql/db.go#L39-L42) [semaphore](../internal/pkg/semaphore/limiter.go) |
| **慢 SQL 拖垮** | 响应 P99 飙升/CPU 高 | 缺索引/SQL 写坏/数据量增长 | ①从 GORM Logger 抓取慢 SQL ②EXPLAIN 分析 ③加索引或重写 SQL | FULLTEXT 索引 + 慢查询监控 + 定期 EXPLAIN 巡检 | 查询耗时 < 200ms | [gorm_logger.go](../internal/pkg/logger/gorm_logger.go#L67-L108) |
| **死锁** | 事务报 Error 1213 回滚 | 多事务以不同顺序更新同一批行 | ①查看 SHOW ENGINE INNODB STATUS ②分析死锁日志 ③统一加锁顺序 | 乐观锁替代悲观锁 + 固定更新顺序 | 死锁消失 | [product_repository.go](../internal/repository/mysql/product_repository.go#L145-L157) |
| **数据误删** | 数据不完整/业务异常 | 软删除误操作/越权/SQL 写坏 | ①停止服务 ②从延迟从库/binlog 恢复 ③数据校验后重启 | GORM 软删除 + 权限管控 + 延迟从库 | 数据完整性校验通过 | Entity 层 GORM DeletedAt |

### 连接池故障排查命令

```sql
-- 查看当前所有连接
SHOW PROCESSLIST;

-- 查看连接数统计
SHOW STATUS LIKE 'Threads%';

-- 查看 InnoDB 状态（含死锁信息）
SHOW ENGINE INNODB STATUS\G

-- 查看慢查询日志配置
SHOW VARIABLES LIKE 'slow_query%';
SHOW VARIABLES LIKE 'long_query_time';

-- 杀死指定连接
KILL <connection_id>;
```

### 慢查询优化速查

| 问题类型 | 识别特征 | 优化手段 |
|---------|---------|---------|
| 全表扫描 | `type: ALL` in EXPLAIN | 加 WHERE 条件索引 |
| 索引失效 | `key: NULL` 但有索引 | 避免 WHERE 中对列做函数/运算 |
| 回表查询 | `Extra: Using index condition` | 覆盖索引（包含 SELECT 全部列） |
| 排序代价高 | `Extra: Using filesort` | 利用索引排序（ORDER BY 与索引一致） |
| N+1 查询 | 慢查询计数器 ≥3 | 改用批量 IN 查询 + 内存组装 |

---

> **下一篇**：[04-cascading-failures.md](./04-cascading-failures.md) — 连锁故障深度剖析，将缓存和数据库的故障模式串联为完整的故障链分析
