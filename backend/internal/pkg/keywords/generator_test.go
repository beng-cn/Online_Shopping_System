package keywords

import (
	"strings"
	"testing"
)

func TestGenerateIPhone(t *testing.T) {
	result := Generate("iPhone 15 Pro Max", []string{"电子产品", "手机"})
	t.Logf("iPhone 15 Pro Max → %s", result)

	expected := []string{"苹果手机", "apple", "iphone", "智能手机", "ios", "pro", "max", "电子产品", "手机"}
	for _, kw := range expected {
		if !strings.Contains(result, kw) {
			t.Errorf("关键词中应包含 %q", kw)
		}
	}
}

func TestGenerateMacBook(t *testing.T) {
	result := Generate("MacBook Pro M3", []string{"电子产品", "电脑办公"})
	t.Logf("MacBook Pro M3 → %s", result)

	expected := []string{"苹果电脑", "macbook", "mac", "苹果笔记本", "笔记本电脑", "m3芯片", "苹果芯片", "pro"}
	for _, kw := range expected {
		if !strings.Contains(result, kw) {
			t.Errorf("关键词中应包含 %q", kw)
		}
	}
}

func TestGenerateWithEnglishBrandName(t *testing.T) {
	// 品牌映射靠英文名匹配，所以需要名字里包含英文品牌词
	result := Generate("Huawei Mate 60 Pro", []string{"电子产品", "手机"})
	t.Logf("Huawei Mate 60 Pro → %s", result)

	expected := []string{"华为手机", "huawei", "鸿蒙", "harmonyos", "国产手机"}
	for _, kw := range expected {
		if !strings.Contains(result, kw) {
			t.Errorf("关键词中应包含 %q", kw)
		}
	}
}

func TestGenerateXiaomi(t *testing.T) {
	result := Generate("Xiaomi 14 Ultra", []string{"电子产品", "手机"})
	t.Logf("Xiaomi 14 Ultra → %s", result)

	expected := []string{"小米手机", "小米", "xiaomi", "智能手机", "性价比", "ultra"}
	for _, kw := range expected {
		if !strings.Contains(result, kw) {
			t.Errorf("关键词中应包含 %q", kw)
		}
	}
}

func TestGenerateForProduct(t *testing.T) {
	result := GenerateForProduct("Samsung Galaxy S24", "手机", "电子产品")
	t.Logf("Samsung Galaxy S24 → %s", result)

	if !strings.Contains(result, "三星手机") || !strings.Contains(result, "samsung") {
		t.Error("应包含三星相关关键词")
	}
	if !strings.Contains(result, "电子产品") || !strings.Contains(result, "手机") {
		t.Error("应包含分类层级关键词")
	}
}

func TestNoPureDigitsInResult(t *testing.T) {
	result := Generate("iPhone 15", []string{"手机"})
	for _, kw := range strings.Split(result, ",") {
		kw = strings.TrimSpace(kw)
		isAllDigits := true
		for _, c := range kw {
			if c < '0' || c > '9' {
				isAllDigits = false
				break
			}
		}
		if isAllDigits {
			t.Errorf("结果不应包含纯数字: %q", kw)
		}
	}
}
