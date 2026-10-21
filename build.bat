@echo off
REM 一键构建脚本（Windows）。用法：build.bat [版本号，默认 1.0.0]
setlocal
set VERSION=%1
if "%VERSION%"=="" set VERSION=1.0.0
go build -trimpath -ldflags "-s -w -X main.version=%VERSION%" -o pkghub-%VERSION%".exe .
if errorlevel 1 (
  echo [失败] 构建未通过。
  exit /b 1
)
echo [完成] 已生成 pkghub.exe v%VERSION%
