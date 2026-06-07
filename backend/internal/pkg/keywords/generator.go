package keywords

import (
	"strings"
)

// 品牌中英文映射表 — 只需维护一次，自动应用到所有商品
var brandMapping = map[string]string{
	"iphone":      "苹果手机,apple,智能手机,ios",
	"apple":       "苹果,apple",
	"huawei":      "华为手机,huawei,智能手机,鸿蒙,harmonyos,国产手机",
	"mate":        "华为手机,mate系列,商务手机",
	"macbook":     "苹果电脑,mac,苹果笔记本,笔记本电脑",
	"ipad":        "苹果平板,ipad,平板电脑,ios",
	"samsung":     "三星手机,samsung,智能手机,安卓",
	"galaxy":      "三星手机,galaxy,安卓手机",
	"xiaomi":      "小米手机,小米,xiaomi,智能手机,性价比",
	"redmi":       "红米手机,redmi,小米,性价比手机",
	"oppo":        "oppo手机,拍照手机,安卓",
	"vivo":        "vivo手机,拍照手机,安卓",
	"thinkpad":    "联想笔记本,thinkpad,商务笔记本,笔记本电脑",
	"surface":     "微软平板,surface,二合一笔记本,windows",
	"airpods":     "苹果耳机,airpods,蓝牙耳机,无线耳机",
	"watch":       "智能手表,苹果手表,apple watch",
	"playstation": "ps5,索尼游戏机,游戏主机,playstation",
	"xbox":        "微软游戏机,xbox,游戏主机",
	"switch":      "任天堂,switch,游戏机,掌机",
	"kindle":      "电子书,kindle,亚马逊,阅读器",
}

// 通用属性中英文映射
var attributeMapping = map[string]string{
	"pro":   "专业版,pro,高端",
	"max":   "大屏版,max,顶配",
	"ultra": "旗舰版,ultra,顶配",
	"air":   "轻薄版,air,便携",
	"mini":  "迷你版,mini,小屏",
	"plus":  "增强版,plus,大屏",
	"se":    "入门版,se,性价比",
	"lite":  "青春版,lite,入门",
	"5g":    "5g手机,5g网络",
	"wifi":  "wifi,无线",
	"m1":    "m1芯片,苹果芯片",
	"m2":    "m2芯片,苹果芯片",
	"m3":    "m3芯片,苹果芯片,最新款",
	"m4":    "m4芯片,苹果芯片,最新款",
}

// Generate 根据商品名和分类层级自动生成搜索关键词
// name: 商品名称，categoryPath: 分类层级路径，如 []string{"电子产品","手机"}
func Generate(name string, categoryPath []string) string {
	nameLower := strings.ToLower(name)
	kwSet := make(map[string]bool)
	parts := strings.FieldsFunc(nameLower, func(r rune) bool {
		return r == ' ' || r == '-' || r == '_' || r == '/' || r == '(' || r == ')' || r == '（' || r == '）'
	})

	// 1. 品牌名映射：匹配名称中出现的品牌词
	for _, part := range parts {
		if mapping, ok := brandMapping[part]; ok {
			for _, kw := range strings.Split(mapping, ",") {
				kwSet[kw] = true
			}
		}
	}

	// 2. 属性词映射：Pro/Max/Ultra/5G 等
	for _, part := range parts {
		if mapping, ok := attributeMapping[part]; ok {
			for _, kw := range strings.Split(mapping, ",") {
				kwSet[kw] = true
			}
		}
	}

	// 3. 分类层级关键词：每个层级都加入
	for _, cat := range categoryPath {
		cat = strings.TrimSpace(cat)
		if cat != "" {
			kwSet[cat] = true
		}
	}

	// 4. 商品名称本身拆解作为关键词（空格/数字分隔的各部分）
	//    如 "iPhone 15 Pro Max 256GB" → "iphone,15,pro,max,256gb"
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if len(part) >= 2 {
			kwSet[part] = true
		}
	}

	// 5. 构建结果，过滤空值和纯数字
	var result []string
	for kw := range kwSet {
		kw = strings.TrimSpace(kw)
		if kw != "" && !isAllDigits(kw) {
			result = append(result, kw)
		}
	}

	return strings.Join(result, ",")
}

// GenerateForProduct 便捷方法：从分类名列表生成
func GenerateForProduct(name string, categoryName string, parentCategoryName string) string {
	var path []string
	if parentCategoryName != "" {
		path = append(path, parentCategoryName)
	}
	if categoryName != "" && categoryName != parentCategoryName {
		path = append(path, categoryName)
	}
	return Generate(name, path)
}

func isAllDigits(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
