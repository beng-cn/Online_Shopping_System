// 秒杀系统完整压测工具 v2.0
// 策略改进：用户准备阶段走 DB 批量插入（绕过 API 限流），秒杀阶段走真实 API
package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-redis/redis/v8"
	_ "github.com/go-sql-driver/mysql"
	"golang.org/x/crypto/bcrypt"
)

const BaseURL = "http://localhost:8080/api/v1"

// 全局 Redis 客户端（用于读取验证码答案）
var captchaRdb *redis.Client
var captchaCtx = context.Background()

// =================== 数据结构 ===================

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token string `json:"token"`
}

type FlashEnterResponse struct {
	Admitted    bool   `json:"admitted"`
	QueueNumber int64  `json:"queue_number"`
	Message     string `json:"message"`
}

type FlashSnatchResponse struct {
	Success bool   `json:"success"`
	OrderNo string `json:"order_no"`
	Message string `json:"message"`
}

type FlashDetailResponse struct {
	ID         uint   `json:"id"`
	FlashStock int    `json:"flash_stock"`
	Remaining  int    `json:"remaining"`
	QueueCount int    `json:"queue_count"`
	Status     int    `json:"status"`
}

// =================== HTTP 工具 ===================

var httpClient = &http.Client{
	Timeout: 30 * time.Second,
	Transport: &http.Transport{
		MaxIdleConns:        2000,
		MaxIdleConnsPerHost: 2000,
		MaxConnsPerHost:     2000,
		IdleConnTimeout:     60 * time.Second,
		DisableCompression:  true,
	},
}

func apiPost(path string, body map[string]interface{}, token string) (map[string]interface{}, time.Duration, error) {
	data, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", BaseURL+path, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	start := time.Now()
	resp, err := httpClient.Do(req)
	latency := time.Since(start)
	if err != nil {
		return nil, latency, err
	}
	defer resp.Body.Close()

	respData, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(respData, &result)
	return result, latency, nil
}

func apiGet(path string, token string) (map[string]interface{}, time.Duration, error) {
	req, _ := http.NewRequest("GET", BaseURL+path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	start := time.Now()
	resp, err := httpClient.Do(req)
	latency := time.Since(start)
	if err != nil {
		return nil, latency, err
	}
	defer resp.Body.Close()

	respData, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(respData, &result)
	return result, latency, nil
}

// =================== 用户准备（MySQL 批量插入） ===================

var letters = []rune("abcdefghijklmnopqrstuvwxyz0123456789")

func randomStr(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func batchCreateUsers(db *sql.DB, count int, baseID int) ([]string, []string, error) {
	usernames := make([]string, count)
	passwords := make([]string, count)

	hash, _ := bcrypt.GenerateFromPassword([]byte("test123456"), bcrypt.DefaultCost)
	passwordHash := string(hash)

	fmt.Printf("  📝 通过 DB 批量创建 %d 个用户...", count)

	// 批量插入
	batchSize := 200
	var totalInserted int
	for batch := 0; batch < count; batch += batchSize {
		end := batch + batchSize
		if end > count {
			end = count
		}

		var values []string
		var args []interface{}
		for i := batch; i < end; i++ {
			uname := fmt.Sprintf("perftest_%s_%d", randomStr(4), baseID+i)
			usernames[i] = uname
			passwords[i] = "test123456"
			values = append(values, "(?, ?, '压测用户', ?, '13800000000', 1, 2, NOW(), NOW())")
			args = append(args, uname, passwordHash, uname+"@test.com")
		}

		query := fmt.Sprintf(`INSERT INTO users (username, password, nickname, email, phone, status, role_id, created_at, updated_at)
			VALUES %s ON DUPLICATE KEY UPDATE password=VALUES(password)`, strings.Join(values, ","))
		_, err := db.Exec(query, args...)
		if err != nil {
			return nil, nil, fmt.Errorf("批量插入用户失败 (batch %d): %w", batch, err)
		}
		totalInserted += (end - batch)
	}
	fmt.Printf("完成 %d 个\n", totalInserted)

	return usernames, passwords, nil
}

func batchLogin(usernames, passwords []string, concurrency int) []string {
	tokens := make([]string, len(usernames))
	var wg sync.WaitGroup
	sem := make(chan struct{}, concurrency)

	fmt.Printf("  🔑 批量登录 %d 个用户 (并发=%d)...", len(usernames), concurrency)
	startTime := time.Now()

	for i := range usernames {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			// 限流保护：每次请求后等待至少 10ms（相当于 100 QPS max）
			time.Sleep(10 * time.Millisecond)

			result, _, err := apiPost("/user/login", map[string]interface{}{
				"username": usernames[idx],
				"password": passwords[idx],
			}, "")
			if err != nil {
				return
			}

			if code, ok := result["code"].(float64); ok && code == 0 {
				if data, ok := result["data"].(map[string]interface{}); ok {
					if token, ok := data["token"].(string); ok {
						tokens[idx] = token
					}
				}
			}
		}(i)
	}
	wg.Wait()

	elapsed := time.Since(startTime)
	validCount := 0
	for _, t := range tokens {
		if t != "" {
			validCount++
		}
	}
	fmt.Printf("完成 %d/%d (耗时 %v)\n", validCount, len(usernames), elapsed.Round(time.Millisecond))
	return tokens
}

// =================== 秒杀压测结果 ===================

type StressResult struct {
	TotalUsers          int
	EnterAttempts       int64
	EnterSuccess        int64
	SnatchAttempts      int64
	SnatchSuccess       int64
	SnatchFail          int64
	ExpectedStock       int
	FinalRedisStock     int64
	Oversold            bool
	StockConsistent     bool
	EnterLatencies      []int64
	SnatchLatencies     []int64
	SnatchRecords       []SnatchRecord
	// 精细失败分类
	EnterFailReasons    sync.Map // map[string]int64 入场失败原因统计
	SnatchFailReasons   sync.Map // map[string]int64 抢购失败原因统计
	mu                  sync.Mutex
}

type SnatchRecord struct {
	UserID    int
	Success   bool
	OrderNo   string
	Message   string
	LatencyMs int64
}

func runSingleUser(flashSaleID uint, userIdx int, token string, result *StressResult) {
	// 0. 获取验证码（人机验证，每个用户抢购前必须获取）
	captchaResp, captchaLat, captchaErr := apiGet("/auth/flash/captcha", token)
	if captchaErr != nil || captchaResp == nil {
		incSyncMap(&result.EnterFailReasons, "验证码获取失败(限流)")
		return
	}
	var captchaID string
	if data, ok := captchaResp["data"].(map[string]interface{}); ok {
		if id, ok := data["captcha_id"].(string); ok {
			captchaID = id
		}
	}
	if captchaID == "" {
		incSyncMap(&result.EnterFailReasons, "验证码ID解析失败")
		return
	}
	// 从 Redis 读取答案（模拟 OCR 识别）
	captchaAnswer, _ := captchaRdb.Get(captchaCtx, "captcha:"+captchaID).Result()
	if captchaAnswer == "" {
		incSyncMap(&result.EnterFailReasons, "验证码答案已过期")
		return
	}

	// 1. 入场
	resp, lat, err := apiPost("/auth/flash/enter", map[string]interface{}{
		"flash_sale_id": flashSaleID,
	}, token)

	atomic.AddInt64(&result.EnterAttempts, 1)

	result.mu.Lock()
	result.EnterLatencies = append(result.EnterLatencies, lat.Milliseconds()+captchaLat.Milliseconds())
	result.mu.Unlock()

	if err != nil || resp == nil {
		incSyncMap(&result.EnterFailReasons, "网络错误/限流")
		return
	}

	code, _ := resp["code"].(float64)
	msg := ""
	if m, ok := resp["message"].(string); ok { msg = m }

	admitted := false
	var dataMsg string
	if data, ok := resp["data"].(map[string]interface{}); ok {
		if a, ok := data["admitted"].(bool); ok { admitted = a }
		if m, ok := data["message"].(string); ok { dataMsg = m }
	}

	if code != 0 || !admitted {
		reason := fmt.Sprintf("入场拒绝[code=%d]: %s", int(code), dataMsg)
		if dataMsg == "" {
			reason = fmt.Sprintf("入场拒绝[code=%d]: %s", int(code), msg)
		}
		incSyncMap(&result.EnterFailReasons, reason)
		return
	}
	atomic.AddInt64(&result.EnterSuccess, 1)

	// 2. 抢购（带验证码）
	resp2, lat2, err2 := apiPost("/auth/flash/snatch", map[string]interface{}{
		"flash_sale_id":  flashSaleID,
		"captcha_id":     captchaID,
		"captcha_answer": captchaAnswer,
	}, token)

	atomic.AddInt64(&result.SnatchAttempts, 1)

	result.mu.Lock()
	result.SnatchLatencies = append(result.SnatchLatencies, lat2.Milliseconds())
	result.mu.Unlock()

	if err2 != nil || resp2 == nil {
		atomic.AddInt64(&result.SnatchFail, 1)
		incSyncMap(&result.SnatchFailReasons, "网络错误/限流")
		return
	}

	// 解析抢购结果
	success := false
	orderNo := ""
	respMsg := ""
	respCode := int(codeFromResp(resp2))
	if data, ok := resp2["data"].(map[string]interface{}); ok {
		if s, ok := data["success"].(bool); ok { success = s }
		if o, ok := data["order_no"].(string); ok { orderNo = o }
		if m, ok := data["message"].(string); ok { respMsg = m }
	}
	if respMsg == "" {
		if m, ok := resp2["message"].(string); ok { respMsg = m }
	}

	record := SnatchRecord{UserID: userIdx, Success: success, OrderNo: orderNo, Message: respMsg, LatencyMs: lat2.Milliseconds()}

	if success {
		atomic.AddInt64(&result.SnatchSuccess, 1)
	} else {
		atomic.AddInt64(&result.SnatchFail, 1)
		failKey := fmt.Sprintf("抢购失败[code=%d]: %s", respCode, respMsg)
		incSyncMap(&result.SnatchFailReasons, failKey)
	}

	result.mu.Lock()
	if len(result.SnatchRecords) < 100 {
		result.SnatchRecords = append(result.SnatchRecords, record)
	}
	result.mu.Unlock()
}

func incSyncMap(m *sync.Map, key string) {
	for {
		val, _ := m.LoadOrStore(key, new(int64))
		atomic.AddInt64(val.(*int64), 1)
		return
	}
}

func codeFromResp(resp map[string]interface{}) int {
	if c, ok := resp["code"].(float64); ok { return int(c) }
	return -1
}

// =================== 压测引擎 ===================

func runStressTest(flashSaleID uint, tokens []string, flashStock int, startDelay time.Duration) *StressResult {
	result := &StressResult{
		TotalUsers:    len(tokens),
		ExpectedStock: flashStock,
	}

	validTokens := make([]string, 0, len(tokens))
	for _, t := range tokens {
		if t != "" {
			validTokens = append(validTokens, t)
		}
	}
	result.TotalUsers = len(validTokens)

	fmt.Printf("\n%s\n", strings.Repeat("=", 62))
	fmt.Printf("  ⚡ 并发抢购: %d 用户 × %d 库存\n", len(validTokens), flashStock)
	fmt.Printf("%s\n\n", strings.Repeat("=", 62))

	if len(validTokens) == 0 {
		fmt.Println("  ❌ 无可用用户，压测终止")
		return result
	}

	// 等待活动开始
	if startDelay > 0 {
		fmt.Printf("  ⏳ 等待活动开始... (%v)\n", startDelay.Round(time.Second))
		time.Sleep(startDelay)
	}

	startTime := time.Now()

	// 分批并发：控制瞬时压力
	batchSize := 500
	// 控制每个用户请求间隔，避免触发 per-user 限流
	for batchStart := 0; batchStart < len(validTokens); batchStart += batchSize {
		batchEnd := batchStart + batchSize
		if batchEnd > len(validTokens) {
			batchEnd = len(validTokens)
		}
		batch := validTokens[batchStart:batchEnd]
		var batchWG sync.WaitGroup

		for j, token := range batch {
			batchWG.Add(1)
			go func(userIdx int, tok string) {
				defer batchWG.Done()
				runSingleUser(flashSaleID, batchStart+userIdx, tok, result)
			}(j, token)
		}
		batchWG.Wait()

		entered := atomic.LoadInt64(&result.EnterSuccess)
		snatched := atomic.LoadInt64(&result.SnatchSuccess)
		fmt.Printf("  [%d/%d] 已入场=%d 已抢到=%d\n",
			batchEnd, len(validTokens), entered, snatched)
	}

	elapsed := time.Since(startTime)
	qps := float64(len(validTokens)) / elapsed.Seconds()
	fmt.Printf("  ⏱ 总耗时: %v | 吞吐: %.0f req/s\n", elapsed.Round(time.Millisecond), qps)

	return result
}

// =================== 结果分析 ===================

func analyze(result *StressResult, flashSaleID uint) {
	fmt.Printf("\n%s\n", strings.Repeat("=", 62))
	fmt.Printf("  📊 压测结果报告\n")
	fmt.Printf("%s\n\n", strings.Repeat("=", 62))

	entered := atomic.LoadInt64(&result.EnterSuccess)
	snatched := atomic.LoadInt64(&result.SnatchSuccess)
	failed := atomic.LoadInt64(&result.SnatchFail)
	attempts := atomic.LoadInt64(&result.SnatchAttempts)

	// ============ 表格 ============
	fmt.Println("  ┌──────────────────────────────────────┐")
	fmt.Println("  │          📋 请求统计                 │")
	fmt.Println("  ├──────────────────────────────────────┤")
	fmt.Printf("  │ 总用户数:            %6d         │\n", result.TotalUsers)
	fmt.Printf("  │ 入场尝试:            %6d         │\n", atomic.LoadInt64(&result.EnterAttempts))
	fmt.Printf("  │ 入场成功:            %6d         │\n", entered)
	fmt.Printf("  │ 抢购尝试:            %6d         │\n", attempts)
	fmt.Printf("  │ 抢购成功:            %6d         │\n", snatched)
	fmt.Printf("  │ 抢购失败:            %6d         │\n", failed)
	if entered > 0 {
		fmt.Printf("  │ 成功率(对入场):      %6.1f%%       │\n",
			float64(snatched)/float64(entered)*100)
	}
	fmt.Println("  ├──────────────────────────────────────┤")

	// 延迟统计
	if len(result.EnterLatencies) > 0 {
		sort.Slice(result.EnterLatencies, func(i, j int) bool {
			return result.EnterLatencies[i] < result.EnterLatencies[j]
		})
		n := len(result.EnterLatencies)
		fmt.Println("  │         ⏱ 延迟 (ms)                │")
		fmt.Printf("  │ 入场 avg: %6.1f  p99: %6d         │\n",
			avg(result.EnterLatencies), result.EnterLatencies[int(float64(n)*0.99)])
	}

	if len(result.SnatchLatencies) > 0 {
		sort.Slice(result.SnatchLatencies, func(i, j int) bool {
			return result.SnatchLatencies[i] < result.SnatchLatencies[j]
		})
		n := len(result.SnatchLatencies)
		fmt.Printf("  │ 抢购 avg: %6.1f  p99: %6d         │\n",
			avg(result.SnatchLatencies), result.SnatchLatencies[int(float64(n)*0.99)])
		fmt.Printf("  │ 抢购 max: %6d                   │\n",
			result.SnatchLatencies[n-1])
	}
	fmt.Println("  ├──────────────────────────────────────┤")

	// 库存一致性
	fmt.Println("  │         📦 库存校验                 │")
	detail, err := getFlashDetail(flashSaleID)
	if err != nil {
		fmt.Printf("  │ ⚠️ 无法获取详情: %v\n", err)
	} else {
		expectedRemaining := result.ExpectedStock - int(snatched)
		fmt.Printf("  │ 原始库存:            %6d         │\n", result.ExpectedStock)
		fmt.Printf("  │ 抢购成功:            %6d         │\n", snatched)
		fmt.Printf("  │ 理论剩余:            %6d         │\n", expectedRemaining)
		fmt.Printf("  │ 实际剩余(Redis):     %6d         │\n", detail.Remaining)
		fmt.Printf("  │ 排队总计:            %6d         │\n", detail.QueueCount)

		if detail.Remaining == expectedRemaining {
			fmt.Println("  │ ✅ 库存完全一致，无超卖！         │")
			result.StockConsistent = true
			result.Oversold = false
		} else if detail.Remaining < 0 {
			fmt.Printf("  │ ❌ 超卖！超出 %d 件！             │\n", -detail.Remaining)
			result.Oversold = true
		} else if detail.Remaining > expectedRemaining {
			fmt.Printf("  │ ⚠️ 库存盈余 %d 件（可能有回滚）  │\n",
				detail.Remaining-expectedRemaining)
		} else {
			fmt.Printf("  │ ❌ 库存偏差: %d                   │\n",
				detail.Remaining-expectedRemaining)
			result.Oversold = true
		}
	}
	fmt.Println("  └──────────────────────────────────────┘")

	// 抢购成功样例
	successCount := 0
	fmt.Printf("\n  📝 成功记录样例:\n")
	for _, r := range result.SnatchRecords {
		if r.Success && successCount < 10 {
			fmt.Printf("     ✅ #%d | %s | %dms\n", r.UserID, r.OrderNo, r.LatencyMs)
			successCount++
		}
	}
	if successCount == 0 {
		fmt.Println("     （无成功记录）")
	}

	// 入场失败原因分类
	fmt.Printf("\n  📝 入场失败原因分布:\n")
	printReasonMap(&result.EnterFailReasons)

	// 抢购失败原因分类
	fmt.Printf("\n  📝 抢购失败原因分布:\n")
	printReasonMap(&result.SnatchFailReasons)
}

func printReasonMap(m *sync.Map) {
	type kv struct {
		k string
		v int64
	}
	var sorted []kv
	m.Range(func(key, value interface{}) bool {
		ptr := value.(*int64)
		count := atomic.LoadInt64(ptr)
		if count > 0 {
			sorted = append(sorted, kv{key.(string), count})
		}
		return true
	})
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].v > sorted[j].v })
	if len(sorted) == 0 {
		fmt.Println("     （无记录）")
		return
	}
	for _, item := range sorted {
		if len(item.k) > 70 {
			item.k = item.k[:70] + "..."
		}
		fmt.Printf("     %4d 次 | %s\n", item.v, item.k)
	}
}

func getFlashDetail(flashID uint) (*FlashDetailResponse, error) {
	result, _, err := apiGet(fmt.Sprintf("/flash/%d", flashID), "")
	if err != nil {
		return nil, err
	}
	if data, ok := result["data"].(map[string]interface{}); ok {
		d := &FlashDetailResponse{}
		if v, ok := data["id"].(float64); ok { d.ID = uint(v) }
		if v, ok := data["flash_stock"].(float64); ok { d.FlashStock = int(v) }
		if v, ok := data["remaining"].(float64); ok { d.Remaining = int(v) }
		if v, ok := data["queue_count"].(float64); ok { d.QueueCount = int(v) }
		if v, ok := data["status"].(float64); ok { d.Status = int(v) }
		return d, nil
	}
	return nil, fmt.Errorf("解析失败")
}

func avg(data []int64) float64 {
	if len(data) == 0 { return 0 }
	var sum int64
	for _, v := range data { sum += v }
	return float64(sum) / float64(len(data))
}

// =================== 主流程 ===================

func main() {
	fmt.Println(strings.Repeat("=", 62))
	fmt.Println("  🔥 秒杀系统压测工具 v2.0 (DB批量准备 + API压测)")
	fmt.Println(strings.Repeat("=", 62))
	fmt.Println()

	// 检查服务器健康
	httpResp, err := http.Get("http://localhost:8080/health")
	if err != nil || httpResp.StatusCode != 200 {
		fmt.Printf("❌ 服务器未就绪: %v\n请先执行: go run ./cmd/server/\n", err)
		os.Exit(1)
	}
	httpResp.Body.Close()
	fmt.Println("✅ 服务器已连接")

	// 连接数据库（DSN 从环境变量读取，避免明文密码进 Git）
	dsn := os.Getenv("MYSQL_DSN")
	if dsn == "" {
		dsn = "root:CHANGE_ME@tcp(127.0.0.1:3306)/Online_Shopping_System?charset=utf8mb4&parseTime=true&loc=Local"
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		fmt.Printf("❌ 连接数据库失败: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()
	fmt.Println("✅ 数据库已连接")

	// 连接 Redis（用于读取验证码答案）
	captchaRdb = redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379", DB: 0})
	fmt.Println("✅ Redis 已连接")
	fmt.Println()

	// ====== 读取参数 ======
	flashStock := 100
	numUsers := 1000

	if len(os.Args) > 1 {
		fmt.Sscanf(os.Args[1], "%d", &flashStock)
	}
	if len(os.Args) > 2 {
		fmt.Sscanf(os.Args[2], "%d", &numUsers)
	}

	fmt.Printf("⚙ 压测参数: 库存=%d, 并发用户=%d\n", flashStock, numUsers)
	fmt.Println()

	// ====== 步骤1: 管理员登录 & 创建秒杀 ======
	fmt.Println("━━━ 步骤1: 创建秒杀活动 ━━━")

	adminToken := ""
	if resp, _, err := apiPost("/user/login", map[string]interface{}{
		"username": "admin", "password": "admin123",
	}, ""); err == nil {
		if data, ok := resp["data"].(map[string]interface{}); ok {
			if t, ok := data["token"].(string); ok {
				adminToken = t
			}
		}
	}
	if adminToken == "" {
		fmt.Println("❌ 管理员登录失败")
		os.Exit(1)
	}
	fmt.Println("  ✅ 管理员登录成功")

	// 获取商品
	productID := uint(0)
	if resp, _, err := apiPost("/product/list", map[string]interface{}{
		"page_num": 1, "page_size": 1,
	}, ""); err == nil {
		if data, ok := resp["data"].(map[string]interface{}); ok {
			if list, ok := data["list"].([]interface{}); ok && len(list) > 0 {
				if p, ok := list[0].(map[string]interface{}); ok {
					if id, ok := p["id"].(float64); ok {
						productID = uint(id)
						fmt.Printf("  📦 使用商品: ID=%d\n", productID)
					}
				}
			}
		}
	}
	if productID == 0 {
		fmt.Println("❌ 无可用商品")
		os.Exit(1)
	}

	var apiResp map[string]interface{}

	// 创建秒杀活动（服务端用 time.Local 解析，直接发本地时间即可）
	startTime := time.Now().Add(30 * time.Second).Format("2006-01-02 15:04:05")
	endTime := time.Now().Add(10 * time.Minute).Format("2006-01-02 15:04:05")

	apiResp, _, err = apiPost("/admin/flash", map[string]interface{}{
		"product_id":  productID,
		"flash_price": 5.00,
		"flash_stock": flashStock,
		"start_time":  startTime,
		"end_time":    endTime,
	}, adminToken)
	if err != nil || getCode(apiResp) != 0 {
		fmt.Printf("❌ 创建秒杀失败: %v %v\n", err, apiResp)
		os.Exit(1)
	}
	flashSaleID := uint(getDataField(apiResp, "id"))
	fmt.Printf("  ✅ 秒杀活动创建成功: ID=%d\n", flashSaleID)

	// 预热
	apiResp, _, err = apiPost(fmt.Sprintf("/admin/flash/%d/warmup", flashSaleID), nil, adminToken)
	if err != nil || getCode(apiResp) != 0 {
		fmt.Printf("❌ 预热失败: %v %v\n", err, apiResp)
		os.Exit(1)
	}
	fmt.Println("  ✅ 预热完成")

	// 验证状态
	detail, _ := getFlashDetail(flashSaleID)
	if detail != nil {
		fmt.Printf("  📋 状态: stock=%d remaining=%d status=%d\n",
			detail.FlashStock, detail.Remaining, detail.Status)
	}
	fmt.Println()

	// ====== 步骤2: 批量准备用户 ======
	fmt.Println("━━━ 步骤2: 准备测试用户 ━━━")

	// 清理旧数据
	db.Exec("DELETE FROM carts")
	db.Exec("DELETE FROM order_items")
	db.Exec("DELETE FROM orders")
	db.Exec("DELETE FROM flash_sales WHERE id != ?", flashSaleID)
	db.Exec("DELETE FROM users WHERE username LIKE 'perftest_%' OR username LIKE 'stresstest_%'")
	fmt.Println("  ✅ 旧数据已清理")

	usernames, passwords, err := batchCreateUsers(db, numUsers, 0)
	if err != nil {
		fmt.Printf("❌ 创建用户失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Println()

	// ====== 步骤3: 批量登录获取 Token ======
	fmt.Println("━━━ 步骤3: 获取用户 Token ━━━")
	tokens := batchLogin(usernames, passwords, 20) // 控制并发数
	fmt.Println()

	// ====== 步骤4: 执行秒杀压测 ======
	fmt.Println("━━━ 步骤4: 秒杀压测 ━━━")

	// 计算活动开始前的等待时间
	startTimeParsed, _ := time.ParseInLocation("2006-01-02 15:04:05", startTime, time.Local)
	waitTime := time.Until(startTimeParsed)
	if waitTime < 0 {
		waitTime = 3 * time.Second // 活动已开始，等3秒确保预热生效
	}

	result := runStressTest(flashSaleID, tokens, flashStock, waitTime)

	// ====== 步骤5: 分析结果 ======
	analyze(result, flashSaleID)

	// ====== 步骤6: DB 层面的最终验证 ======
	fmt.Println("\n━━━ 数据库层面最终验证 ━━━")

	var dbOrderCount int
	db.QueryRow("SELECT COUNT(*) FROM orders WHERE flash_sale_id = ? AND status IN (0,1,3)",
		flashSaleID).Scan(&dbOrderCount)
	fmt.Printf("  DB 有效订单数: %d (期望 ≤ %d)\n", dbOrderCount, flashStock)
	if dbOrderCount > flashStock {
		fmt.Printf("  ❌ 超卖！数据库多出 %d 个订单！\n", dbOrderCount-flashStock)
	} else if dbOrderCount == int(atomic.LoadInt64(&result.SnatchSuccess)) {
		fmt.Println("  ✅ DB 订单数 与 Redis 抢购成功数一致")
	} else {
		fmt.Printf("  ⚠️ DB 订单数(%d) ≠ Redis 抢购成功数(%d)\n",
			dbOrderCount, atomic.LoadInt64(&result.SnatchSuccess))
	}

	// 检查 Duplicate 情况
	var dupUserCount int
	db.QueryRow(`SELECT COUNT(*) FROM (
		SELECT user_id, COUNT(*) cnt FROM orders
		WHERE flash_sale_id = ? AND status IN (0,1,3)
		GROUP BY user_id HAVING cnt > 1
	) t`, flashSaleID).Scan(&dupUserCount)
	if dupUserCount > 0 {
		fmt.Printf("  ❌ 重复抢购：%d 个用户重复下单！\n", dupUserCount)
	} else {
		fmt.Println("  ✅ 无用户重复抢购")
	}

	fmt.Println()
	fmt.Println(strings.Repeat("=", 62))
	if result.StockConsistent && !result.Oversold && dbOrderCount <= flashStock && dupUserCount == 0 {
		fmt.Println("  🎉 全部校验通过！无超卖、无重复、库存一致")
	} else {
		fmt.Println("  ⚠️ 存在数据不一致，需要排查")
	}
	fmt.Println(strings.Repeat("=", 62))
	fmt.Println()
	fmt.Printf("  验证 SQL:\n")
	fmt.Printf("  SELECT status, COUNT(*) FROM orders WHERE flash_sale_id=%d GROUP BY status;\n", flashSaleID)
	fmt.Println()
}

func getCode(resp map[string]interface{}) int {
	if c, ok := resp["code"].(float64); ok {
		return int(c)
	}
	return -1
}

func getDataField(resp map[string]interface{}, field string) float64 {
	if data, ok := resp["data"].(map[string]interface{}); ok {
		if v, ok := data[field].(float64); ok {
			return v
		}
	}
	return 0
}
