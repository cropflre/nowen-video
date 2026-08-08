@echo off
setlocal EnableDelayedExpansion
chcp 65001 >nul

REM ============================================================
REM  nowen-video 一键启动脚本（Server Lite + Vite）
REM
REM  使用方法：
REM    scripts\run-dev.bat
REM    scripts\run-dev.bat [后端优先端口] [前端优先端口]
REM
REM  默认优先端口：后端 28888，前端 28889。
REM  如果端口已被占用，会从优先端口开始自动向上寻找空闲端口。
REM ============================================================

set "SCRIPT_DIR=%~dp0"
set "PROJECT_DIR=%SCRIPT_DIR%.."
set "PORT_HELPER=%SCRIPT_DIR%find-free-port.ps1"

REM 优先级：命令行参数 > 已有环境变量 > 项目默认值
if not "%~1"=="" set "SERVER_PORT=%~1"
if not "%~2"=="" set "WEB_PORT=%~2"
if "%SERVER_PORT%"=="" set "SERVER_PORT=28888"
if "%WEB_PORT%"=="" set "WEB_PORT=28889"

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
set "SERVER_PORT=%RESOLVED_SERVER_PORT%"

set "REQUESTED_WEB_PORT=%WEB_PORT%"
set "RESOLVED_WEB_PORT="
for /f "usebackq delims=" %%p in (`powershell.exe -NoLogo -NoProfile -NonInteractive -ExecutionPolicy Bypass -File "%PORT_HELPER%" -PreferredPort !WEB_PORT! -ExcludePort !SERVER_PORT!`) do (
    if not defined RESOLVED_WEB_PORT set "RESOLVED_WEB_PORT=%%p"
)
if not defined RESOLVED_WEB_PORT (
    echo [error] 无法为前端找到可用端口。
    exit /b 1
)
set "WEB_PORT=%RESOLVED_WEB_PORT%"

if not "%REQUESTED_SERVER_PORT%"=="%SERVER_PORT%" (
    echo [warn] 后端优先端口 %REQUESTED_SERVER_PORT% 已占用，自动切换到 %SERVER_PORT%。
)
if not "%REQUESTED_WEB_PORT%"=="%WEB_PORT%" (
    echo [warn] 前端优先端口 %REQUESTED_WEB_PORT% 已占用，自动切换到 %WEB_PORT%。
)

echo.
echo ============================================================
echo  nowen-video 本地开发环境（Server Lite）
echo  后端端口: %SERVER_PORT%
echo  前端端口: %WEB_PORT%
echo  前端代理: http://localhost:%SERVER_PORT%
echo ============================================================
echo.

echo [1/2] 启动后端服务窗口 ...
start "nowen-video-server-lite (port %SERVER_PORT%)" /D "%PROJECT_DIR%" cmd /k "set SERVER_PORT=%SERVER_PORT%&& set NOWEN_DEBUG=%NOWEN_DEBUG%&& set NOWEN_SERVER_MODE=lite&& set NOWEN_PORT_RESOLVED=1&& call scripts\run-server.bat"

REM 稍等一下，让后端先开始初始化
timeout /t 2 /nobreak >nul

echo [2/2] 启动前端 Vite 窗口 ...
start "nowen-video-web (port %WEB_PORT%)" /D "%PROJECT_DIR%" cmd /k "set WEB_PORT=%WEB_PORT%&& set SERVER_PORT=%SERVER_PORT%&& set NOWEN_PORT_RESOLVED=1&& call scripts\run-web.bat"

echo.
echo 已分别启动后端和前端窗口，关闭对应窗口即可停止服务。
echo 浏览器访问: http://localhost:%WEB_PORT%
echo.

endlocal
exit /b 0
