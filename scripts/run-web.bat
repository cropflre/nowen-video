@echo off
setlocal EnableDelayedExpansion
chcp 65001 >nul

REM ============================================================
REM  nowen-video 前端 Vite 本地开发启动脚本
REM
REM  使用方法：
REM    scripts\run-web.bat [前端优先端口] [后端代理端口]
REM
REM  默认前端优先端口 28889，默认代理后端 28888。
REM  前端端口冲突时会自动向上寻找空闲端口。
REM ============================================================

set "SCRIPT_DIR=%~dp0"
set "PORT_HELPER=%SCRIPT_DIR%find-free-port.ps1"

REM 优先级：命令行参数 > 环境变量 > 默认值
if not "%~1"=="" set "WEB_PORT=%~1"
if not "%~2"=="" set "SERVER_PORT=%~2"
if "%WEB_PORT%"=="" set "WEB_PORT=28889"
if "%SERVER_PORT%"=="" set "SERVER_PORT=28888"

REM 前端构建版本：优先使用环境变量，其次使用最新 Git tag
if "%VITE_APP_VERSION%"=="" (
    for /f "usebackq delims=" %%v in (`git -C "%SCRIPT_DIR%\.." describe --tags --abbrev^=0 --match "v[0-9]*" 2^>nul`) do set "VITE_APP_VERSION=%%v"
    if defined VITE_APP_VERSION if "!VITE_APP_VERSION:~0,1!"=="v" set "VITE_APP_VERSION=!VITE_APP_VERSION:~1!"
)
if "%VITE_APP_VERSION%"=="" set "VITE_APP_VERSION=0.1.0"

REM 由一键脚本预先解析过端口时直接复用；单独运行时也自动处理冲突。
if not "%NOWEN_PORT_RESOLVED%"=="1" (
    if not exist "%PORT_HELPER%" (
        echo [error] 端口探测脚本不存在: %PORT_HELPER%
        exit /b 1
    )

    set "REQUESTED_WEB_PORT=%WEB_PORT%"
    set "RESOLVED_WEB_PORT="
    for /f "usebackq delims=" %%p in (`powershell.exe -NoLogo -NoProfile -NonInteractive -ExecutionPolicy Bypass -File "%PORT_HELPER%" -PreferredPort !WEB_PORT! -ExcludePort !SERVER_PORT!`) do (
        if not defined RESOLVED_WEB_PORT set "RESOLVED_WEB_PORT=%%p"
    )
    if not defined RESOLVED_WEB_PORT (
        echo [error] 无法为前端找到可用端口。
        exit /b 1
    )
    set "WEB_PORT=!RESOLVED_WEB_PORT!"
    if not "!REQUESTED_WEB_PORT!"=="!WEB_PORT!" (
        echo [warn] 前端优先端口 !REQUESTED_WEB_PORT! 已占用，自动切换到 !WEB_PORT!。
    )
)

pushd "%SCRIPT_DIR%\..\web"

REM 通过环境变量传给 vite.config.ts，使代理目标可动态调整
set "VITE_API_PROXY_TARGET=http://localhost:%SERVER_PORT%"

echo.
echo ============================================================
echo  启动 nowen-video 前端 (Vite Dev Server)
echo  前端端口   : %WEB_PORT%
echo  应用版本   : %VITE_APP_VERSION%
echo  后端代理至 : %VITE_API_PROXY_TARGET%
echo  工作目录   : %CD%
echo ============================================================
echo.

if not exist "node_modules" (
    echo [info] 未检测到 node_modules，正在执行 npm install ...
    call npm install
    if errorlevel 1 (
        echo [error] npm install 失败
        popd
        exit /b 1
    )
)

call npm run dev -- --port %WEB_PORT% --host --strictPort

set "EXIT_CODE=%ERRORLEVEL%"
popd
endlocal & exit /b %EXIT_CODE%
