@echo off
echo ========================================
echo 正在启动在线商城前端...
echo ========================================

:: 检查是否在frontend目录
if not exist "package.json" (
    echo [错误] 请在frontend目录下运行此脚本
    echo 当前目录: %CD%
    goto end
)

echo.
echo [步骤1] 检查Node.js...
node --version > nul 2>&1
if errorlevel 1 (
    echo [错误] 未找到Node.js，请先安装Node.js
    goto end
)

echo.
echo [步骤2] 检查依赖...
if not exist "node_modules" (
    echo [信息] 安装依赖...
    npm install
    if errorlevel 1 (
        echo [错误] 依赖安装失败
        goto end
    )
) else (
    echo [✓] 依赖已安装
)

echo.
echo [步骤3] 配置Vite代理...
:: 检查是否存在vite.config.js
if not exist "vite.config.js" (
    echo [警告] 未找到vite.config.js，请确保已配置API代理
    goto start
)

:: 添加代理配置到vite.config.js（如果还没有）
findstr "proxy" vite.config.js > nul
if errorlevel 1 (
    echo [信息] 配置API代理...
    echo. >> vite.config.js
    echo export default defineConfig({ >> vite.config.js
    echo   plugins: [vue()], >> vite.config.js
    echo   server: { >> vite.config.js
    echo     proxy: { >> vite.config.js
    echo       '/api': { >> vite.config.js
    echo         target: 'http://localhost:8080', >> vite.config.js
    echo         changeOrigin: true >> vite.config.js
    echo       } >> vite.config.js
    echo     } >> vite.config.js
    echo   } >> vite.config.js
    echo }) >> vite.config.js
)

:start
echo.
echo [步骤4] 启动前端开发服务器...
echo 前端将启动在 http://localhost:5173
echo 按Ctrl+C停止服务
echo.

:: 启动开发服务器
npm run dev

:end
echo.
echo 前端已停止
pause