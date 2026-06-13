package errors

import "fmt"

// 错误码定义
const (
	CodeSuccess      = 0   // 成功
	CodeParamError   = 400 // 参数错误
	CodeUnauthorized = 401 // 未授权
	CodeForbidden    = 403 // 禁止访问
	CodeNotFound     = 404 // 资源不存在
	CodeServerError  = 500 // 服务器错误

	// 业务错误码
	CodeProductNotFound    = 1001 // 商品不存在
	CodeStockInsufficient  = 1002 // 库存不足
	CodeOrderNotFound      = 1003 // 订单不存在
	CodeOrderAlreadyPaid   = 1004 // 订单已支付
	CodeOrderCancelled     = 1005 // 订单已取消
	CodeUserAlreadyExists  = 1006 // 用户名已存在
	CodeUserNotFound       = 1007 // 用户不存在
	CodePasswordError      = 1008 // 密码错误
	CodeUserDisabled       = 1009 // 用户已禁用
	CodeCategoryHasProduct = 1010 // 分类下有商品
	CodeCategoryHasChild   = 1011 // 分类下有子分类
	CodeAdminPinError      = 1012 // 管理员PIN码错误
	CodeAdminPinNotSet     = 1013 // 管理员PIN码未设置

	// 秒杀业务错误码
	CodeFlashSaleNotFound    = 2001 // 秒杀活动不存在
	CodeFlashSaleNotStarted  = 2002 // 秒杀活动未开始
	CodeFlashSaleEnded       = 2003 // 秒杀活动已结束
	CodeFlashSaleSoldOut     = 2004 // 秒杀库存已售罄
	CodeFlashSaleAlreadyDone = 2005 // 已参与过该秒杀
	CodeFlashSaleCancelled   = 2006 // 秒杀活动已取消
	CodeFlashQueueFull       = 2007 // 排队人数已满
	CodeFlashNotEntered      = 2008 // 未获取入场资格
)

// 自定义错误类型
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Err     error  `json:"-"` // 原始错误，不返回给前端
}

func (e *Error) Error() string {
	return e.Message
}

// 构造函数
func New(code int, message string) *Error {
	return &Error{Code: code, Message: message}
}

func NewParamError(message string) *Error {
	return &Error{Code: CodeParamError, Message: message}
}

func Wrap(err error, message string) *Error {
	return &Error{Code: CodeServerError, Message: message, Err: err}
}

func Wrapf(err error, format string, args ...interface{}) *Error {
	return &Error{Code: CodeServerError, Message: fmt.Sprintf(format, args...), Err: err}
}

func Errorf(code int, format string, args ...interface{}) *Error {
	return &Error{Code: code, Message: fmt.Sprintf(format, args...)}
}

// IsCode 检查错误是否为指定错误码的自定义错误
// 用于替代无法直接比较 gorm.ErrRecordNotFound 的场景
func IsCode(err error, code int) bool {
	if e, ok := err.(*Error); ok {
		return e.Code == code
	}
	return false
}
