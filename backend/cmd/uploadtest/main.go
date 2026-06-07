package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"path/filepath"
)

func main() {
	// 登录
	loginBody := `{"username":"admintest","password":"admin123"}`
	resp, _ := http.Post("http://localhost:8080/api/user/login", "application/json", bytes.NewBufferString(loginBody))
	defer resp.Body.Close()
	var r struct {
		Data struct{ Token string } `json:"data"`
	}
	json.NewDecoder(resp.Body).Decode(&r)
	token := r.Data.Token
	fmt.Printf("✅ 登录成功\n\n")

	// 找文件
	imgDir := `C:\Users\LENOVO\Pictures\Camera Roll`
	entries, _ := os.ReadDir(imgDir)
	var imgPath string
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".png" {
			imgPath = filepath.Join(imgDir, e.Name())
			break
		}
	}
	info, _ := os.Stat(imgPath)
	fmt.Printf("📁 %s (%d 字节)\n\n", filepath.Base(imgPath), info.Size())

	// ===== 测试1: 正常上传 =====
	fmt.Println("===== 测试1: 正常上传 PNG =====")
	doUpload(token, imgPath, "image/png")

	// ===== 测试2: 伪造 Content-Type（试图传 .go 文件伪装成 PNG） =====
	fmt.Println("\n===== 测试2: 伪装攻击（.go文件声明为image/png）=====")
	goFile, _ := os.CreateTemp("", "fake-*.go")
	goFile.WriteString("package main\nfunc main(){}\n")
	goFile.Close()
	defer os.Remove(goFile.Name())
	doUpload(token, goFile.Name(), "image/png")

	fmt.Println("\n✅ 全部测试完成")
}

func doUpload(token, filePath, contentType string) {
	file, _ := os.Open(filePath)
	defer file.Close()

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="image"; filename="%s"`, filepath.Base(filePath)))
	h.Set("Content-Type", contentType)
	part, _ := w.CreatePart(h)
	io.Copy(part, file)
	w.Close()

	req, _ := http.NewRequest("POST", "http://localhost:8080/api/admin/upload", &buf)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, _ := http.DefaultClient.Do(req)
	defer resp.Body.Close()
	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"message"`
		Data struct{ URL string } `json:"data"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	fmt.Printf("   结果: code=%d msg=%s url=%s\n", result.Code, result.Msg, result.Data.URL)
}
