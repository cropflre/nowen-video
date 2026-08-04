@echo off
setlocal EnableDelayedExpansion
chcp 65001 >nul

REM ============================================================
REM  nowen-video 本地开发交互启动器
REM
REM  推荐直接运行项目根目录 dev.bat，一键启动 Server Lite + Vite。
REM  本脚本保留按需启动单个服务的交互入口。
REM ============================================================

set "SCRIPT_DIR=%~dp0"
REM 去掉末尾反斜杠，避免路径拼接时出现重复分隔符
set "SCRIPT_DIR=%SCRIPT_DIR:~0,-1%"

:menu
cls
echo.
echo ============================================================
echo            nowen-video local dev launcher
echo ============================================================
echo.
echo   [1] Start Backend  (Server Lite)
echo   [2] Start Frontend (Vite dev server)
echo   [3] Start ALL      (auto-select free ports)
echo   [0] Exit
echo.
echo ============================================================
set "CHOICE="
set /p "CHOICE=请输入选项 [0-3]: "

if "%CHOICE%"=="1" goto run_server
if "%CHOICE%"=="2" goto run_web
if "%CHOICE%"=="3" goto run_all
if "%CHOICE%"=="0" goto end

echo.
echo [warn] 无效输入，请重试...
timeout /t 2 /nobreak >nul
goto menu


REM ============================================================
REM  仅启动后端
REM ============================================================
:run_server
echo.
echo ----------- 启动后端服务 -----------
call :ask_server_port
echo.
echo 将从端口 %SERVER_PORT% 开始自动寻找可用端口。
echo.
start "nowen-video-server-lite preferred port %SERVER_PORT%" cmd /k "set SERVER_PORT=%SERVER_PORT%&& set NOWEN_SERVER_MODE=lite&& call %SCRIPT_DIR%\run-server.bat"
goto end


REM ============================================================
REM  仅启动前端
REM ============================================================
:run_web
echo.
echo ----------- 启动前端服务 -----------
call :ask_web_port
call :ask_server_port_for_proxy
echo.
echo 将从前端端口 %WEB_PORT% 开始自动寻找可用端口。
echo 后端代理目标: http://localhost:%SERVER_PORT%
echo.
start "nowen-video-web preferred port %WEB_PORT%" cmd /k "set WEB_PORT=%WEB_PORT%&& set SERVER_PORT=%SERVER_PORT%&& call %SCRIPT_DIR%\run-web.bat"
goto end


REM ============================================================
REM  全部启动：复用非交互一键脚本，确保前后端端口成对解析
REM ============================================================
:run_all
echo.
echo ----------- 启动全部服务 后端 + 前端 -----------
call :ask_server_port
call :ask_web_port
echo.
echo 将自动解析并启动:
echo   后端优先端口: %SERVER_PORT%
echo   前端优先端口: %WEB_PORT%
echo.
call "%SCRIPT_DIR%\run-dev.bat" %SERVER_PORT% %WEB_PORT%
goto end


REM ============================================================
REM  子例程：询问后端优先端口（默认 28888）
REM ============================================================
:ask_server_port
set "INPUT="
set /p "INPUT=请输入后端优先端口 [默认 28888，直接回车使用默认]: "
if "%INPUT%"=="" (
    set "SERVER_PORT=28888"
) else (
    set "SERVER_PORT=%INPUT%"
)
goto :eof


REM ============================================================
REM  子例程：询问前端优先端口（默认 28889）
REM ============================================================
:ask_web_port
set "INPUT="
set /p "INPUT=请输入前端优先端口 [默认 28889，直接回车使用默认]: "
if "%INPUT%"=="" (
    set "WEB_PORT=28889"
) else (
    set "WEB_PORT=%INPUT%"
)
goto :eof


REM ============================================================
REM  子例程：询问后端端口（用于仅启动前端时的代理目标）
REM ============================================================
:ask_server_port_for_proxy
set "INPUT="
set /p "INPUT=请输入要代理的后端端口 [默认 28888，直接回车使用默认]: "
if "%INPUT%"=="" (
    set "SERVER_PORT=28888"
) else (
    set "SERVER_PORT=%INPUT%"
)
goto :eof


:end
echo.
pause
endlocal
exit /b 0
