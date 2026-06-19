package captcha

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestGenerateReturnsFourCharAnswer(t *testing.T) {
	c, err := Generate()
	if err != nil {
		t.Fatalf("Generate() 失败: %v", err)
	}
	if len(c.Answer) != 4 {
		t.Errorf("验证码答案应为 4 个字符，实际 %d: %s", len(c.Answer), c.Answer)
	}
}

func TestGenerateNoAmbiguousChars(t *testing.T) {
	ambiguous := "0O1Il2Z5S"
	for i := 0; i < 50; i++ {
		c, err := Generate()
		if err != nil {
			t.Fatalf("第 %d 次 Generate() 失败: %v", i, err)
		}
		for _, ch := range c.Answer {
			if strings.ContainsRune(ambiguous, ch) {
				t.Errorf("验证码 %q 包含混淆字符 %q", c.Answer, string(ch))
			}
		}
	}
}

func TestGenerateReturnsBase64Image(t *testing.T) {
	c, err := Generate()
	if err != nil {
		t.Fatalf("Generate() 失败: %v", err)
	}

	if !strings.HasPrefix(c.ImageB64, "data:image/png;base64,") {
		t.Error("ImageB64 应以 data:image/png;base64, 开头")
	}

	_, decodeErr := base64.StdEncoding.DecodeString(strings.TrimPrefix(c.ImageB64, "data:image/png;base64,"))
	if decodeErr != nil {
		t.Errorf("ImageB64 解码失败: %v", decodeErr)
	}
}

func TestGenerateUniqueIDs(t *testing.T) {
	ids := make(map[string]bool)
	for i := 0; i < 10; i++ {
		c, err := Generate()
		if err != nil {
			t.Fatalf("Generate() 失败: %v", err)
		}
		if ids[c.ID] {
			t.Errorf("验证码 ID 重复: %s", c.ID)
		}
		ids[c.ID] = true
	}
}
