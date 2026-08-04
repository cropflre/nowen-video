@echo off
REM 项目根目录一键启动：自动选择空闲的前后端端口。
call "%~dp0scripts\run-dev.bat" %*
exit /b %ERRORLEVEL%
