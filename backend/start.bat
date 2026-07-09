@echo off
echo ========================================
echo 正在启动在线商城后端服务...
echo ========================================

:: 检查配置文件
if not exist ".env" (
    echo [信息] 创建配置文件 .env
    copy .env.example .env > nul
    echo [提示] 已创建 .env 文件，已使用你的实际配置
)

:: 设置环境变量（如果还没有设置）
setx GO_ENV dev > nul 2>&1
setx MYSQL_ROOT_PASSWORD 181871ZX > nul 2>&1
setx MYSQL_PASSWORD 181871ZX > nul 2>&1

echo.
echo [步骤1] 检查数据库连接...
mysql -u root -p181871ZX -e "USE online_shopping_system;" > nul 2>&1
if errorlevel 1 (
    echo [警告] 无法连接到数据库，请确保MySQL已启动
    echo 数据库名: online_shopping_system
    echo 用户名: root
    echo 密码: 181871ZX
    goto end
) else (
    echo [✓] 数据库连接正常
)

echo.
echo [步骤2] 启动后端服务...
echo 服务将启动在 http://localhost:8080
echo 按Ctrl+C停止服务
echo.

:: 启动服务
go run ./cmd/server/

:end
echo.
echo 服务已停止
pause