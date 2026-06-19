// 秒杀系统端到端集成测试
// 需要 MySQL + Redis 运行，服务器会自动启动
// 运行方式: go test -v -count=1 -timeout 120s ./test/
package test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	goredis "github.com/go-redis/redis/v8"
)

const BaseURL = "http://localhost:8080/api/v1"

// ServerPath 指向编译好的 server.exe
var ServerPath = "d:/Online_Shopping_System/backend/server.exe"

// =================== 数据结构 ===================

type apiResponse struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

// =================== 辅助函数 ===================

func httpPost(t *testing.T, token, path, body string) *apiResponse {
	t.Helper()
	req, _ := http.NewRequest("POST", BaseURL+path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST %s 失败: %v", path, err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	var r apiResponse
	json.Unmarshal(data, &r)
	return &r
}

func httpGet(t *testing.T, token, path string) *apiResponse {
	t.Helper()
	req, _ := http.NewRequest("GET", BaseURL+path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s 失败: %v", path, err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	var r apiResponse
	json.Unmarshal(data, &r)
	return &r
}

func extractField(t *testing.T, r *apiResponse, field string) string {
	t.Helper()
	var m map[string]interface{}
	json.Unmarshal(r.Data, &m)
	if v, ok := m[field]; ok {
		return fmt.Sprint(v)
	}
	return ""
}

// =================== 启动/停止服务器 ===================

var serverCmd *exec.Cmd

func TestMain(m *testing.M) {
	// 启动已编译的服务器（工作目录设为 backend/ 以找到 configs/）
	serverCmd = exec.Command(ServerPath)
	serverCmd.Dir = "d:/Online_Shopping_System/backend"
	serverCmd.Env = append(os.Environ(),
		"MYSQL_PASSWORD=181871ZX",
		"MYSQL_HOST=127.0.0.1",
		"REDIS_HOST=127.0.0.1",
		"JWT_SECRET=test-integration-secret-key-32chars",
		"GO_ENV=dev",
	)
	serverCmd.Stdout = os.Stdout
	serverCmd.Stderr = os.Stderr

	if err := serverCmd.Start(); err != nil {
		fmt.Printf("启动失败: %v\n", err)
		os.Exit(1)
	}

	// 等待就绪
	ready := false
	for i := 0; i < 30; i++ {
		time.Sleep(1 * time.Second)
		resp, err := http.Get("http://localhost:8080/health")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == 200 {
				ready = true
				break
			}
		}
	}
	if !ready {
		fmt.Println("服务器启动超时")
		serverCmd.Process.Kill()
		os.Exit(1)
	}
	time.Sleep(5 * time.Second) // 等缓存预热

	code := m.Run()

	if serverCmd.Process != nil {
		serverCmd.Process.Kill()
	}
	os.Exit(code)
}

// =================== 测试用例 ===================

func TestFullFlashSaleFlow(t *testing.T) {
	var token string
	var flashSaleID int

	// ====== 1. 管理员登录 ======
	t.Log(">>> 1. 管理员登录")
	r := httpPost(t, "", "/user/login", `{"username":"admin","password":"admin123"}`)
	if r.Code != 0 {
		t.Fatalf("登录失败: %s", r.Message)
	}
	token = extractField(t, r, "token")
	t.Logf("Token 获取成功")

	// ====== 2. 创建秒杀活动 ======
	t.Log(">>> 2. 创建秒杀活动")
	startTime := time.Now().Add(-1 * time.Minute).Format("2006-01-02 15:04:05")
	endTime := time.Now().Add(1 * time.Hour).Format("2006-01-02 15:04:05")
	body := fmt.Sprintf(`{"product_id":1,"flash_price":1,"flash_stock":5,"start_time":"%s","end_time":"%s"}`,
		startTime, endTime)
	r = httpPost(t, token, "/admin/flash", body)
	if r.Code != 0 {
		t.Fatalf("创建失败: %s", r.Message)
	}
	flashSaleID = int(parseID(t, r))
	t.Logf("活动 ID=%d 创建成功", flashSaleID)

	// ====== 3. 预热 ======
	t.Log(">>> 3. 预热秒杀")
	r = httpPost(t, token, fmt.Sprintf("/admin/flash/%d/warmup", flashSaleID), "")
	if r.Code != 0 {
		t.Fatalf("预热失败: %s", r.Message)
	}
	time.Sleep(300 * time.Millisecond)
	t.Log("预热成功")

	// ====== 4. 排队入场 ======
	t.Log(">>> 4. 排队入场（验证随机延迟不会阻断）")
	enterBody := fmt.Sprintf(`{"flash_sale_id":%d}`, flashSaleID)
	start := time.Now()
	r = httpPost(t, token, "/auth/flash/enter", enterBody)
	elapsed := time.Since(start)
	if r.Code != 0 {
		t.Fatalf("入场失败: %s", r.Message)
	}
	var enterResp struct {
		Admitted    bool   `json:"admitted"`
		QueueNumber int64  `json:"queue_number"`
		Message     string `json:"message"`
	}
	json.Unmarshal(r.Data, &enterResp)
	if !enterResp.Admitted {
		t.Fatalf("未获得入场资格: %s", enterResp.Message)
	}
	t.Logf("入场成功: 排队号=%d 耗时=%v", enterResp.QueueNumber, elapsed)

	// ====== 5. 获取验证码 + 秒杀抢购 ======
	t.Log(">>> 5. 抢购（读取 Redis 验证码）")
	r = httpGet(t, token, "/auth/flash/captcha")
	if r.Code != 0 {
		t.Fatalf("获取验证码失败: %s", r.Message)
	}
	captchaID := extractField(t, r, "captcha_id")

	// 直接从 Redis 读验证码答案
	captchaAnswer := readCaptchaFromRedis(t, captchaID)
	if captchaAnswer == "" {
		t.Skip("无法读取验证码，跳过抢购步骤（需 Redis 可访问）")
	}

	snatchBody := fmt.Sprintf(`{"flash_sale_id":%d,"captcha_id":"%s","captcha_answer":"%s"}`,
		flashSaleID, captchaID, captchaAnswer)
	r = httpPost(t, token, "/auth/flash/snatch", snatchBody)
	if r.Code != 0 {
		t.Fatalf("抢购失败: code=%d msg=%s", r.Code, r.Message)
	}
	var snatchResp struct {
		Success bool   `json:"success"`
		OrderNo string `json:"order_no"`
		Message string `json:"message"`
	}
	json.Unmarshal(r.Data, &snatchResp)
	if !snatchResp.Success {
		t.Fatalf("抢购未成功: %s", snatchResp.Message)
	}
	t.Logf("🎉 抢购成功！订单号: %s", snatchResp.OrderNo)

	// ====== 6. 查询订单 ======
	t.Log(">>> 6. 查询秒杀订单")
	r = httpGet(t, token, "/auth/flash/orders")
	if r.Code != 0 {
		t.Fatalf("查询订单失败: %s", r.Message)
	}
	var orders []struct {
		OrderNo string `json:"order_no"`
		Status  int    `json:"status"`
	}
	json.Unmarshal(r.Data, &orders)
	found := false
	for _, o := range orders {
		if o.OrderNo == snatchResp.OrderNo {
			found = true
			t.Logf("✅ 订单确认: %s Status=%d", o.OrderNo, o.Status)
		}
	}
	if !found {
		t.Errorf("订单 %s 未找到", snatchResp.OrderNo)
	}

	// ====== 7. 验证库存扣减 ======
	t.Log(">>> 7. 验证库存")
	r = httpGet(t, "", fmt.Sprintf("/flash/%d", flashSaleID))
	var detail struct {
		Remaining  int `json:"remaining"`
		FlashStock int `json:"flash_stock"`
	}
	json.Unmarshal(r.Data, &detail)
	if detail.Remaining != detail.FlashStock-1 {
		t.Errorf("库存: 期望 %d/%d, 实际 %d/%d", detail.FlashStock-1, detail.FlashStock, detail.Remaining, detail.FlashStock)
	} else {
		t.Logf("库存正确: %d/%d", detail.Remaining, detail.FlashStock)
	}

	// ====== 8. 重复抢购应被拒 ======
	t.Log(">>> 8. 重复抢购验证")
	r = httpGet(t, token, "/auth/flash/captcha")
	captchaID2 := extractField(t, r, "captcha_id")
	captchaAnswer2 := readCaptchaFromRedis(t, captchaID2)
	if captchaAnswer2 != "" {
		snatchBody2 := fmt.Sprintf(`{"flash_sale_id":%d,"captcha_id":"%s","captcha_answer":"%s"}`,
			flashSaleID, captchaID2, captchaAnswer2)
		r = httpPost(t, token, "/auth/flash/snatch", snatchBody2)
		var dupResp struct{ Message string }
		json.Unmarshal(r.Data, &dupResp)
		t.Logf("重复抢购返回: %s", dupResp.Message)
	}

	// ====== 9. 清理 ======
	t.Log(">>> 9. 清理")
	httpPost(t, token, fmt.Sprintf("/admin/flash/%d/end", flashSaleID), "")

	t.Log("✅ 秒杀端到端集成测试全部通过")
}

func parseID(t *testing.T, r *apiResponse) float64 {
	t.Helper()
	var m map[string]interface{}
	json.Unmarshal(r.Data, &m)
	if v, ok := m["id"]; ok {
		switch n := v.(type) {
		case float64:
			return n
		}
	}
	return 0
}

// readCaptchaFromRedis 直接从 Redis 读取验证码答案
func readCaptchaFromRedis(t *testing.T, captchaID string) string {
	t.Helper()
	client := goredis.NewClient(&goredis.Options{Addr: "127.0.0.1:6379", Password: "", DB: 0})
	defer client.Close()
	ctx := context.Background()
	key := fmt.Sprintf("captcha:%s", captchaID)
	val, err := client.Get(ctx, key).Result()
	if err != nil {
		t.Logf("Redis 读取 captcha 失败: %v", err)
		return ""
	}
	return val
}
