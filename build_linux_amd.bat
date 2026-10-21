@echo off
setlocal enabledelayedexpansion

set VERSION=%1
if "%VERSION%"=="" set VERSION=1.0.0
set OUTPUT=pkghub-linux-amd64-%VERSION%

set CGO_ENABLED=0
set GOOS=linux
set GOARCH=amd64

REM 清理旧产物
if exist "%OUTPUT%" del /q "%OUTPUT%"

go build -a -ldflags "-s -w -X main.version=%VERSION%" -o "%OUTPUT%" .
if errorlevel 1 (
  echo [失败] 构建未通过。
  exit /b 1
)

echo [完成] 已生成 %OUTPUT% (version=%VERSION%)