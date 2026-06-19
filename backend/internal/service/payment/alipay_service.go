package payment

import (
	"backend/internal/config"
	"backend/internal/model/entity"
	"backend/internal/pkg/errors"
	"fmt"
	"net/http"

	"github.com/smartwalle/alipay/v3"
)

// 支付宝支付服务接口
type AlipayService interface {
	GeneratePayURL(order *entity.Order) (string, error)
	ParseNotify(req *http.Request) (*alipay.Notification, error)
	VerifyReturnSign(req *http.Request) error
}

// 支付宝支付服务实现
type alipayService struct {
	client *alipay.Client
	config *config.AlipayConfig
}

// NewAlipayService 创建支付宝支付服务实例，加载应用私钥和支付宝公钥
func NewAlipayService(cfg *config.AppConfig) (AlipayService, error) {
	aliConfig := &cfg.Alipay

	// 初始化客户端
	client, err := alipay.New(aliConfig.AppID, aliConfig.PrivateKey, false)
	if err != nil {
		fmt.Printf("❌ 初始化原始错误: %v\n", err)
		return nil, errors.Wrap(err, "支付宝客户端初始化失败")
	}

	// 加载支付宝公钥
	err = client.LoadAliPayPublicKey(aliConfig.PublicKey)
	if err != nil {
		fmt.Printf("❌ 加载公钥原始错误: %v\n", err)
		return nil, errors.Wrap(err, "加载支付宝公钥失败")
	}

	fmt.Println("✅ 支付宝客户端初始化成功")
	return &alipayService{
		client: client,
		config: aliConfig,
	}, nil
}

// 生成支付宝PC端支付链接
func (s *alipayService) GeneratePayURL(order *entity.Order) (string, error) {
	// 先创建空对象
	p := alipay.TradePagePay{}

	// 然后逐个赋值（嵌入结构体的字段可以直接访问）
	p.Subject = fmt.Sprintf("商城订单-%s", order.OrderNo)
	p.OutTradeNo = order.OrderNo
	p.TotalAmount = fmt.Sprintf("%.2f", order.Total)
	p.ProductCode = "FAST_INSTANT_TRADE_PAY"
	p.NotifyURL = s.config.NotifyURL
	p.ReturnURL = s.config.ReturnURL

	// 生成支付链接
	url, err := s.client.TradePagePay(p)
	if err != nil {
		return "", errors.Wrap(err, "生成支付宝支付链接失败")
	}

	return url.String(), nil
}

// 解析并验证支付宝异步回调请求
func (s *alipayService) ParseNotify(req *http.Request) (*alipay.Notification, error) {
	// 解析回调参数并自动验证签名
	noti, err := s.client.GetTradeNotification(req)
	if err != nil {
		return nil, errors.Wrap(err, "解析支付宝异步回调失败")
	}

	return noti, nil
}

// VerifyReturnSign 验证支付宝同步跳转的签名
func (s *alipayService) VerifyReturnSign(req *http.Request) error {
	// 验证同步跳转的签名
	err := s.client.VerifySign(req.Context(), req.URL.Query())
	if err != nil {
		return errors.Wrap(err, "支付宝同步回调签名验证失败")
	}

	return nil
}
