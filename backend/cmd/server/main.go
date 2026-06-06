package main

import (
	"fmt"
)

func main() {
	router, err := InitApp()
	if err != nil {
		panic(fmt.Sprintf("应用初始化失败: %v", err))
	}

	// 启动HTTP服务
	fmt.Printf("服务器启动成功，监听端口: %d\n", router.Config.Server.Port)
	fmt.Printf("访问地址: http://localhost:%d\n", router.Config.Server.Port)

	if err := router.Setup().Run(fmt.Sprintf(":%d", router.Config.Server.Port)); err != nil {
		panic(fmt.Sprintf("服务器启动失败: %v", err))
	}
}
