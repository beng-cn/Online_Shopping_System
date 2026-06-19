-- ============================================
-- 商城数据库初始化：建表 + 种子数据
-- MySQL 容器首次启动时自动执行
-- ============================================

-- 用户表
CREATE TABLE `users` (
  `id` int unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` datetime DEFAULT NULL,
  `username` varchar(50) NOT NULL,
  `password` varchar(255) NOT NULL,
  `admin_pin` varchar(255) DEFAULT NULL COMMENT '管理员PIN码(bcrypt加密)，非管理员为NULL',
  `nickname` varchar(50) DEFAULT '',
  `email` varchar(100) DEFAULT '',
  `phone` char(11) DEFAULT '',
  `status` int NOT NULL DEFAULT '1',
  `role_id` int unsigned NOT NULL DEFAULT '2',
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_users_username` (`username`),
  KEY `idx_users_deleted_at` (`deleted_at`),
  KEY `idx_users_email` (`email`),
  KEY `idx_users_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 分类表
CREATE TABLE `categories` (
  `id` int unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` datetime DEFAULT NULL,
  `name` varchar(50) NOT NULL,
  `parent_id` int unsigned NOT NULL DEFAULT '0',
  `status` int NOT NULL DEFAULT '1',
  PRIMARY KEY (`id`),
  KEY `idx_categories_deleted_at` (`deleted_at`),
  KEY `idx_categories_parent_status` (`parent_id`,`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 商品表
CREATE TABLE `products` (
  `id` int unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` datetime DEFAULT NULL,
  `category_id` int unsigned NOT NULL,
  `name` varchar(100) NOT NULL,
  `keywords` varchar(500) DEFAULT '',
  `price` decimal(10,2) NOT NULL,
  `stock` int NOT NULL,
  `image` varchar(255) DEFAULT '',
  `status` int NOT NULL DEFAULT '1',
  `sales` int NOT NULL DEFAULT '0',
  `version` int NOT NULL DEFAULT '0',
  PRIMARY KEY (`id`),
  KEY `idx_products_deleted_at` (`deleted_at`),
  KEY `idx_products_cat_status_sales` (`category_id`,`status`,`sales`),
  KEY `idx_products_sales` (`sales` DESC),
  KEY `idx_products_name_like` (`name`(20)),
  FULLTEXT KEY `ft_product_search` (`name`,`keywords`),
  CONSTRAINT `fk_products_category_id` FOREIGN KEY (`category_id`) REFERENCES `categories` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 购物车表
CREATE TABLE `carts` (
  `id` int unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` datetime DEFAULT NULL,
  `user_id` int unsigned NOT NULL,
  `product_id` int unsigned NOT NULL,
  `quantity` int NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_carts_user_product` (`user_id`,`product_id`),
  KEY `idx_carts_deleted_at` (`deleted_at`),
  CONSTRAINT `fk_carts_user_id` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`),
  CONSTRAINT `fk_carts_product_id` FOREIGN KEY (`product_id`) REFERENCES `products` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 订单表
CREATE TABLE `orders` (
  `id` int unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` datetime DEFAULT NULL,
  `user_id` int unsigned NOT NULL,
  `flash_sale_id` int unsigned DEFAULT NULL COMMENT '秒杀活动ID（NULL=普通订单）',
  `order_no` varchar(64) NOT NULL,
  `total` decimal(10,2) NOT NULL,
  `status` int NOT NULL DEFAULT '0' COMMENT '0=待支付 1=已支付 2=已取消 3=待释放(秒杀冷却期)',
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_orders_order_no` (`order_no`),
  UNIQUE KEY `idx_orders_flash_user` (`flash_sale_id`,`user_id`) COMMENT '防止同一用户在同一秒杀活动中重复下单',
  KEY `idx_orders_deleted_at` (`deleted_at`),
  KEY `idx_orders_user_created` (`user_id`,`created_at` DESC),
  KEY `idx_orders_flash_status_time` (`flash_sale_id`,`status`,`created_at`),
  CONSTRAINT `fk_orders_user_id` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 订单项表
CREATE TABLE `order_items` (
  `id` int unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` datetime DEFAULT NULL,
  `order_id` int unsigned NOT NULL,
  `product_id` int unsigned NOT NULL,
  `quantity` int NOT NULL,
  `price` decimal(10,2) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_order_items_order_product` (`order_id`,`product_id`),
  KEY `idx_order_items_deleted_at` (`deleted_at`),
  CONSTRAINT `fk_order_items_order_id` FOREIGN KEY (`order_id`) REFERENCES `orders` (`id`),
  CONSTRAINT `fk_order_items_product_id` FOREIGN KEY (`product_id`) REFERENCES `products` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 秒杀活动表
CREATE TABLE `flash_sales` (
  `id` int unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` datetime DEFAULT NULL,
  `product_id` int unsigned NOT NULL COMMENT '关联商品ID',
  `flash_price` decimal(10,2) NOT NULL COMMENT '秒杀价格',
  `flash_stock` int NOT NULL COMMENT '秒杀总库存',
  `queue_cap` int NOT NULL DEFAULT '0' COMMENT '排队入场上限（0=按库存×10自动计算）',
  `start_time` datetime NOT NULL COMMENT '秒杀开始时间',
  `end_time` datetime NOT NULL COMMENT '秒杀结束时间',
  `status` tinyint NOT NULL DEFAULT '0' COMMENT '0=未开始 1=进行中 2=已结束 3=已取消',
  `version` int NOT NULL DEFAULT '0' COMMENT '乐观锁版本号',
  PRIMARY KEY (`id`),
  KEY `idx_flash_sales_deleted_at` (`deleted_at`),
  KEY `idx_flash_sales_time_status` (`start_time`,`end_time`,`status`),
  KEY `idx_flash_sales_product` (`product_id`),
  CONSTRAINT `fk_flash_sales_product_id` FOREIGN KEY (`product_id`) REFERENCES `products` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ============================================
-- 种子数据
-- ============================================

-- 管理员账号（密码: admin123）
INSERT INTO `users` VALUES (1, NOW(), NOW(), NULL, 'admin', '$2a$10$EAWs1fbtasK/CpDhMN4JxOgpLSMMoAf84WXXWvtpwOdLzX5b5i9Du', '系统管理员', 'admin@mall.com', '13800000000', 1, 1);

-- 分类
INSERT INTO `categories` VALUES (1, NOW(), NOW(), NULL, '电子产品', 0, 1);
INSERT INTO `categories` VALUES (2, NOW(), NOW(), NULL, '服装鞋帽', 0, 1);
INSERT INTO `categories` VALUES (3, NOW(), NOW(), NULL, '食品饮料', 0, 1);
INSERT INTO `categories` VALUES (4, NOW(), NOW(), NULL, '手机', 1, 1);
INSERT INTO `categories` VALUES (5, NOW(), NOW(), NULL, '电脑办公', 1, 1);
INSERT INTO `categories` VALUES (6, NOW(), NOW(), NULL, '男装', 2, 1);
INSERT INTO `categories` VALUES (7, NOW(), NOW(), NULL, '女装', 2, 1);
INSERT INTO `categories` VALUES (8, NOW(), NOW(), NULL, '休闲零食', 3, 1);
INSERT INTO `categories` VALUES (9, NOW(), NOW(), NULL, '饮料冲调', 3, 1);

-- 商品（含自动生成的关键词）
INSERT INTO `products` VALUES (1, NOW(), NOW(), NULL, 4, 'iPhone 15 Pro Max 256GB', '苹果手机,apple,iphone,手机,智能手机,5g手机,ios,pro,max,电子产品', 9999.00, 100, 'https://picsum.photos/400/400?random=1', 1, 28, 0);
INSERT INTO `products` VALUES (2, NOW(), NOW(), NULL, 4, '华为Mate 60 Pro 512GB', '华为手机,huawei,mate,手机,智能手机,国产手机,5g手机,harmonyos,鸿蒙,pro,电子产品', 6999.00, 80, 'https://picsum.photos/400/400?random=2', 1, 15, 0);
INSERT INTO `products` VALUES (3, NOW(), NOW(), NULL, 5, 'MacBook Pro 14英寸 M3', '苹果电脑,macbook,苹果笔记本,笔记本电脑,apple,m3芯片,苹果芯片,最新款,pro,电子产品,电脑办公', 14999.00, 50, 'https://picsum.photos/400/400?random=3', 1, 22, 0);
INSERT INTO `products` VALUES (4, NOW(), NOW(), NULL, 4, '小米14 Ultra 512GB', '小米手机,小米,xiaomi,智能手机,性价比,ultra,旗舰版,顶配,安卓,电子产品', 5999.00, 60, 'https://picsum.photos/400/400?random=4', 1, 10, 0);
INSERT INTO `products` VALUES (5, NOW(), NOW(), NULL, 5, 'ThinkPad X1 Carbon Gen 12', '联想笔记本,thinkpad,商务笔记本,笔记本电脑,电子产品,电脑办公', 9999.00, 30, 'https://picsum.photos/400/400?random=5', 1, 8, 0);
INSERT INTO `products` VALUES (6, NOW(), NOW(), NULL, 6, '夏季商务POLO衫 男士短袖', '男装,polo衫,商务,短袖,夏天,服装鞋帽', 299.00, 200, 'https://picsum.photos/400/400?random=6', 1, 45, 0);
INSERT INTO `products` VALUES (7, NOW(), NOW(), NULL, 7, '法式复古碎花连衣裙', '女装,连衣裙,碎花,法式,复古,夏天,服装鞋帽', 459.00, 150, 'https://picsum.photos/400/400?random=7', 1, 33, 0);
INSERT INTO `products` VALUES (8, NOW(), NOW(), NULL, 8, '三只松鼠坚果大礼包 1.5kg', '休闲零食,坚果,三只松鼠,零食,礼包,食品饮料', 129.00, 500, 'https://picsum.photos/400/400?random=8', 1, 120, 0);
INSERT INTO `products` VALUES (9, NOW(), NOW(), NULL, 9, '星巴克中度烘焙咖啡豆 1kg', '饮料冲调,咖啡,星巴克,咖啡豆,烘焙,食品饮料', 198.00, 300, 'https://picsum.photos/400/400?random=9', 1, 56, 0);
INSERT INTO `products` VALUES (10, NOW(), NOW(), NULL, 4, 'Samsung Galaxy S24 Ultra', '三星手机,samsung,galaxy,智能手机,安卓,安卓手机,ultra,旗舰版,顶配,电子产品', 8999.00, 40, 'https://picsum.photos/400/400?random=10', 1, 5, 0);
