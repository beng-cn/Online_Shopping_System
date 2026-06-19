// +build ignore

package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	dsn := os.Getenv("MYSQL_DSN")
	if dsn == "" {
		dsn = "root:CHANGE_ME@tcp(127.0.0.1:3306)/Online_Shopping_System?charset=utf8mb4&parseTime=true&loc=Local"
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("连接数据库失败: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("数据库 Ping 失败: %v", err)
	}
	fmt.Println("✅ 数据库已连接")

	// 1. 创建测试管理员用户
	adminHash, _ := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)

	var existingID int
	err = db.QueryRow("SELECT id FROM users WHERE username = ?", "admin").Scan(&existingID)
	if err == sql.ErrNoRows {
		_, err = db.Exec(`INSERT INTO users (username, password, nickname, email, phone, status, role_id, created_at, updated_at)
			VALUES (?, ?, '系统管理员', 'admin@test.com', '13800000000', 1, 1, NOW(), NOW())`,
			"admin", string(adminHash))
		if err != nil {
			log.Printf("创建管理员失败: %v (可能已存在)", err)
		} else {
			fmt.Println("✅ 管理员用户已创建: admin / admin123")
		}
	} else if err != nil {
		log.Printf("查询管理员失败: %v", err)
	} else {
		// 重置密码
		_, err = db.Exec("UPDATE users SET password = ?, status = 1, role_id = 1 WHERE id = ? AND role_id = 1",
			string(adminHash), existingID)
		if err == nil {
			fmt.Println("✅ 管理员密码已重置: admin / admin123")
		}
	}

	// 2. 确保有测试分类
	var catID int
	err = db.QueryRow("SELECT id FROM categories WHERE name = ? LIMIT 1", "手机数码").Scan(&catID)
	if err == sql.ErrNoRows {
		result, _ := db.Exec(`INSERT INTO categories (name, parent_id, status, created_at, updated_at)
			VALUES ('压测分类', 0, 1, NOW(), NOW())`)
		id, _ := result.LastInsertId()
		catID = int(id)
		fmt.Printf("✅ 测试分类已创建: ID=%d\n", catID)
	} else {
		fmt.Printf("✅ 使用已有分类: ID=%d\n", catID)
	}

	// 3. 清理旧的压测数据（先删子表再删主表，尊重外键约束）
	cleanTables := []string{"order_items", "orders", "flash_sales", "carts"}
	for _, table := range cleanTables {
		_, err := db.Exec("DELETE FROM " + table)
		if err != nil {
			log.Printf("清理 %s 失败: %v", table, err)
		}
	}
	// 重置自增ID
	db.Exec("ALTER TABLE order_items AUTO_INCREMENT = 1")
	db.Exec("ALTER TABLE orders AUTO_INCREMENT = 1")
	fmt.Println("✅ 旧订单/秒杀数据已清理")

	// 4. 清理旧的压测用户
	result, err := db.Exec("DELETE FROM users WHERE username LIKE 'stresstest_%' OR username LIKE 'perftest_%'")
	if err != nil {
		log.Printf("清理旧压测用户失败: %v", err)
	} else {
		n, _ := result.RowsAffected()
		if n > 0 {
			fmt.Printf("✅ 清理了 %d 个旧压测用户\n", n)
		}
	}

	// 5. 重置商品库存（防止之前测试消耗了库存）
	_, err = db.Exec("UPDATE products SET stock = 9999, sales = 0 WHERE name LIKE '哨兵测试商品%'")
	if err != nil {
		log.Printf("重置商品库存失败: %v", err)
	} else {
		fmt.Println("✅ 测试商品库存已重置")
	}

	// 6. 显示当前数据概览
	fmt.Println()
	fmt.Println(strings.Repeat("=", 50))
	fmt.Println("📋 初始化完成，数据概览:")
	fmt.Println(strings.Repeat("=", 50))

	var userCount int
	db.QueryRow("SELECT COUNT(*) FROM users").Scan(&userCount)
	fmt.Printf("  用户总数: %d\n", userCount)

	var productCount int
	db.QueryRow("SELECT COUNT(*) FROM products WHERE status = 1").Scan(&productCount)
	fmt.Printf("  上架商品: %d\n", productCount)

	var catCount int
	db.QueryRow("SELECT COUNT(*) FROM categories").Scan(&catCount)
	fmt.Printf("  分类总数: %d\n", catCount)

	var orderCount int
	db.QueryRow("SELECT COUNT(*) FROM orders").Scan(&orderCount)
	fmt.Printf("  订单总数: %d\n", orderCount)

	fmt.Println()
	fmt.Println("🔑 管理员账号: admin / admin123")
	fmt.Println("🚀 可以启动压测: go run ./stress_test/")
	fmt.Println()

}
