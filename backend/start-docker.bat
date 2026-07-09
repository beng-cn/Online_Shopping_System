@echo off
echo ========================================
echo 正在启动在线商城（Docker方式）...
echo ========================================

:: 设置Docker环境变量
setx GO_ENV docker > nul 2>&1
setx MYSQL_ROOT_PASSWORD docker123 > nul 2>&1
setx MYSQL_PASSWORD docker123 > nul 2>&1
setx REDIS_PASSWORD docker_redis > nul 2>&1
setx JWT_SECRET docker-super-secret-jwt-key-with-more-than-32-characters-long > nul 2>&1

:: 创建Docker环境文件
echo GO_ENV=docker > .env.docker
echo MYSQL_ROOT_PASSWORD=docker123 >> .env.docker
echo MYSQL_PASSWORD=docker123 >> .env.docker
echo REDIS_PASSWORD=docker_redis >> .env.docker
echo JWT_SECRET=docker-super-secret-jwt-key-with-more-than-32-characters-long >> .env.docker

echo.
echo [步骤1] 检查Docker是否运行...
docker --version > nul 2>&1
if errorlevel 1 (
    echo [错误] Docker未安装或未启动，请先安装并启动Docker
    goto end
)

echo.
echo [步骤2] 启动Docker容器...
echo 这将启动 MySQL、Redis 和后端服务
echo.

:: 启动Docker Compose
docker compose up -d

echo.
echo [步骤3] 等待服务启动...
timeout /t 10 /nobreak > nul

echo.
echo [步骤4] 检查服务状态...
docker compose ps

echo.
echo ========================================
echo 服务已启动！
echo.
echo 访问地址:
echo - API文档: http://localhost:8080/swagger/index.html
echo - 健康检查: http://localhost:8080/health
echo.
echo 管理员账号: admin / admin123
echo.
echo 查看日志: docker compose logs -f
echo 停止服务: docker compose down
echo ========================================

:end
pause