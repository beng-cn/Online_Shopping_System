// 验证码生成器：4位字母数字混合 + 干扰线 + 噪点
// 纯 Go 标准库 + x/image（无外部字体依赖）
package captcha

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math/big"
	"strings"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

// 排除易混淆字符：0/O、1/I/l、2/Z、5/S、8/B
const chars = "34679ABCDEFGHJKMNPQRTUVWXY"

// Captcha 验证码实例
type Captcha struct {
	ID       string // Redis Key，uuid
	Answer   string // 正确答案（大写）
	ImageB64 string // base64 PNG 图片
}

// Generate 生成一个4位字母数字验证码
func Generate() (*Captcha, error) {
	// 1. 随机选4个字符
	text := make([]byte, 4)
	for i := range text {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		if err != nil {
			return nil, err
		}
		text[i] = chars[n.Int64()]
	}
	answer := string(text)

	// 2. 生成 UUID
	uuid := make([]byte, 16)
	_, _ = rand.Read(uuid)
	id := strings.ToLower(captchaID(uuid))

	// 3. 绘制图片
	img := drawImage(answer)

	// 4. 编码为 base64
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}

	return &Captcha{
		ID:       id,
		Answer:   answer,
		ImageB64: "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes()),
	}, nil
}

// captchaID 将 16 字节随机数转为 hex 字符串
func captchaID(b []byte) string {
	const hex = "0123456789abcdef"
	result := make([]byte, 32)
	for i := range b {
		result[i*2] = hex[b[i]>>4]
		result[i*2+1] = hex[b[i]&0x0f]
	}
	return string(result)
}

// drawImage 绘制 200×70 的验证码图片
// 字符用 basicfont 7×13 位图字体，通过缩放绘制来放大
func drawImage(text string) *image.RGBA {
	width, height := 200, 70
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	// 白色背景
	draw.Draw(img, img.Bounds(), &image.Uniform{color.White}, image.Point{}, draw.Src)

	// 干扰线（2条，穿过文字区域）
	for i := 0; i < 2; i++ {
		x1, _ := randInt(width)
		y1, _ := randInt(height)
		x2, _ := randInt(width)
		y2, _ := randInt(height)
		drawLine(img, x1, y1, x2, y2, randomLightColor())
	}

	// 噪点（60个随机位置）
	for i := 0; i < 60; i++ {
		x, _ := randInt(width)
		y, _ := randInt(height)
		img.Set(x, y, randomDarkColor())
	}

	// 绘制4个字符，每个字符独立缩放和偏移
	face := basicfont.Face7x13
	charSpacing := 40                    // 字符间距
	startX := 20                         // 起始 X
	baseY := 48                          // 基线 Y

	for i, ch := range text {
		// 每个字符随机偏移
		offsetX := startX + i*charSpacing + randRange(-5, 5)
		offsetY := baseY + randRange(-8, 8)

		// 随机颜色（深色，每个字符不同深浅）
		charColor := randomDarkColor()

		// 用 font.Drawer 绘制字符，每个字符缩放 3 倍（7x13 → 21x39）
		scale := 3
		drawScaledChar(img, ch, offsetX, offsetY, scale, charColor, face)
	}

	return img
}

// drawScaledChar 按缩放比例绘制单个字符
// face = basicfont.Face7x13（原生 7×13），scale=3 → 实际渲染 21×39
func drawScaledChar(img *image.RGBA, ch rune, x, y int, scale int, clr color.Color, face font.Face) {
	// 获取字符的 glyph mask（位图掩码）
	dot := fixed.P(x, y)
	dr, mask, maskp, _, ok := face.Glyph(dot, ch)
	if !ok {
		return
	}

	// 对 mask 中每个非零像素，在目标图像上画 scale×scale 的色块
	for py := 0; py < dr.Dy(); py++ {
		for px := 0; px < dr.Dx(); px++ {
			// 检查 mask 中该像素是否被点亮（Alpha > 0）
			alpha := mask.At(maskp.X+px, maskp.Y+py)
			_, _, _, a := alpha.RGBA()
			if a == 0 {
				continue
			}

			// 放大绘制：1 个 mask 像素 → scale×scale 目标像素
			destX := dr.Min.X + px*scale
			destY := dr.Min.Y + py*scale
			for dy := 0; dy < scale; dy++ {
				for dx := 0; dx < scale; dx++ {
					img.Set(destX+dx, destY+dy, clr)
				}
			}
		}
	}
}

// drawLine Bresenham 画线
func drawLine(img *image.RGBA, x1, y1, x2, y2 int, clr color.Color) {
	dx := abs(x2 - x1)
	dy := -abs(y2 - y1)
	sx, sy := 1, 1
	if x1 > x2 {
		sx = -1
	}
	if y1 > y2 {
		sy = -1
	}
	err := dx + dy

	for {
		if x1 >= 0 && x1 < img.Bounds().Dx() && y1 >= 0 && y1 < img.Bounds().Dy() {
			img.Set(x1, y1, clr)
		}
		if x1 == x2 && y1 == y2 {
			break
		}
		e2 := 2 * err
		if e2 >= dy {
			err += dy
			x1 += sx
		}
		if e2 <= dx {
			err += dx
			y1 += sy
		}
	}
}

// randInt 返回 [0, max) 的随机整数
func randInt(max int) (int, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(max)))
	if err != nil {
		return 0, err
	}
	return int(n.Int64()), nil
}

// randRange 返回 [base-range, base+range] 的随机偏移
func randRange(low, high int) int {
	n, _ := randInt(high - low + 1)
	return low + n
}

// randomLightColor 随机浅色（干扰线用）
func randomLightColor() color.Color {
	r, _ := randInt(100)
	g, _ := randInt(100)
	b, _ := randInt(100)
	return color.RGBA{uint8(180 + r), uint8(180 + g), uint8(180 + b), 255}
}

// randomDarkColor 随机深色（文字用）
func randomDarkColor() color.Color {
	r, _ := randInt(150)
	g, _ := randInt(150)
	b, _ := randInt(150)
	return color.RGBA{uint8(r), uint8(g), uint8(b), 255}
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
