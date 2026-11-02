// Package ctxutil 提供 context 的兜底工具。
//
// 背景：为了把 HTTP 请求的 Request ID 一路带到 GORM 的 SQL 日志里，
// 仓储层的方法都接收 context.Context。但后台定时任务、单元测试等调用方
// 可能不传 context（nil），而 database/sql 不接受 nil context，
// 因此统一在这里兜底成 context.Background()。
package ctxutil

import "context"

// Or 返回 ctx；若 ctx 为 nil 则返回 context.Background()
func Or(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}
