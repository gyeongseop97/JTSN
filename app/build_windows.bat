@echo off
setlocal
where go >nul 2>nul
if errorlevel 1 (
  echo Go compiler was not found.
  echo Install Go from https://go.dev/dl/ and run this file again.
  pause
  exit /b 1
)
gofmt -w main.go pdf_tool.go
if not exist build mkdir build
set GOOS=windows
set GOARCH=amd64
set CGO_ENABLED=0
go build -trimpath -ldflags="-s -w -H windowsgui" -o build\JTSN.exe .
if errorlevel 1 (
  echo Build failed.
  pause
  exit /b 1
)
copy /y README_v5.2.txt build\README.txt >nul
copy /y CHANGELOG_v5.2.txt build\CHANGELOG.txt >nul
copy /y THIRD_PARTY.txt build\THIRD_PARTY.txt >nul
echo.
echo Build complete: build\JTSN.exe
pause
