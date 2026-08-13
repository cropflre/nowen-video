@echo off
setlocal EnableDelayedExpansion
chcp 65001 >nul

REM ============================================================
REM  nowen-video 后端本地开发启动脚本
REM
REM  使用方法：
REM    scripts\run-server.bat [优先端口]
REM
REM  默认启动 Nowen Video 正式版，默认优先端口 28888。
REM  端口冲突时会自动向上寻找空闲端口。
REM  旧版完整服务仅用于迁移/回滚，可设置 NOWEN_SERVER_MODE=legacy。
REM  为兼容旧脚本，lite/full 仍作为 official/legacy 的别名接受。
REM ============================================================

set "SCRIPT_DIR=%~dp0"
set "PORT_HELPER=%SCRIPT_DIR%find-free-port.ps1"

REM 优先级：命令行参数 > 环境变量 SERVER_PORT > 默认 28888
if not "%~1"=="" set "SERVER_PORT=%~1"
if "%SERVER_PORT%"=="" set "SERVER_PORT=28888"

REM 是否启用调试模式（true / false）
if "%NOWEN_DEBUG%"=="" set "NOWEN_DEBUG=true"

REM 服务模式：official / legacy；lite / full 仅为兼容别名。
if "%NOWEN_SERVER_MODE%"=="" set "NOWEN_SERVER_MODE=official"
if /I "%NOWEN_SERVER_MODE%"=="official" (
    set "SERVER_ENTRY=./cmd/server-lite"
    set "SERVER_DISPLAY_MODE=正式版"
) else if /I "%NOWEN_SERVER_MODE%"=="lite" (
    set "SERVER_ENTRY=./cmd/server-lite"
    set "SERVER_DISPLAY_MODE=正式版"
) else if /I "%NOWEN_SERVER_MODE%"=="legacy" (
    set "SERVER_ENTRY=./cmd/server"
    set "SERVER_DISPLAY_MODE=旧版兼容"
) else if /I "%NOWEN_SERVER_MODE%"=="full" (
    set "SERVER_ENTRY=./cmd/server"
    set "SERVER_DISPLAY_MODE=旧版兼容"
) else (
    echo [error] NOWEN_SERVER_MODE 仅支持 official 或 legacy（兼容别名: lite/full），当前值: %NOWEN_SERVER_MODE%
    exit /b 1
)

REM 应用版本：优先使用环境变量，其次使用最新 Git tag
if "%NOWEN_VERSION%"=="" (
    for /f "usebackq delims=" %%v in (`git -C "%SCRIPT_DIR%\.." describe --tags --abbrev^=0 --match "v[0-9]*" 2^>nul`) do set "NOWEN_VERSION=%%v"
    if defined NOWEN_VERSION if "!NOWEN_VERSION:~0,1!"=="v" set "NOWEN_VERSION=!NOWEN_VERSION:~1!"
)
if "%NOWEN_VERSION%"=="" set "NOWEN_VERSION=0.1.0"

REM 由一键脚本预先解析过端口时直接复用；单独运行时也自动处理冲突。
if not "%NOWEN_PORT_RESOLVED%"=="1" (
    if not exist "%PORT_HELPER%" (
        echo [error] 端口探测脚本不存在: %PORT_HELPER%
        exit /b 1
    )

    set "REQUESTED_SERVER_PORT=%SERVER_PORT%"
    set "RESOLVED_SERVER_PORT="
    for /f "usebackq delims=" %%p in (`powershell.exe -NoLogo -NoProfile -NonInteractive -ExecutionPolicy Bypass -File "%PORT_HELPER%" -PreferredPort !SERVER_PORT!`) do (
        if not defined RESOLVED_SERVER_PORT set "RESOLVED_SERVER_PORT=%%p"
    )
    if not defined RESOLVED_SERVER_PORT (
        echo [error] 无法为后端找到可用端口。
        exit /b 1
    )
    set "SERVER_PORT=!RESOLVED_SERVER_PORT!"
    if not "!REQUESTED_SERVER_PORT!"=="!SERVER_PORT!" (
        echo [warn] 后端优先端口 !REQUESTED_SERVER_PORT! 已占用，自动切换到 !SERVER_PORT!。
    )
)

REM 切换到项目根目录（脚本父目录）
pushd "%SCRIPT_DIR%\.."

REM 通过 Viper 的环境变量机制覆盖 app.port
set "NOWEN_APP_PORT=%SERVER_PORT%"
set "CGO_ENABLED=1"

echo.
echo ============================================================
echo  启动 nowen-video 后端服务
echo  服务版本: %SERVER_DISPLAY_MODE%
echo  启动入口: %SERVER_ENTRY%
echo  监听端口: %SERVER_PORT%
echo  应用版本: %NOWEN_VERSION%
echo  调试模式: %NOWEN_DEBUG%
echo  工作目录: %CD%
echo ============================================================
echo.

go run %SERVER_ENTRY%

set "EXIT_CODE=%ERRORLEVEL%"
popd
endlocal & exit /b %EXIT_CODE%
