@echo off
setlocal

set "ROOT_DIR=%~dp0"
rem 仅在无参数时自动打开浏览器；带参数调用（如 -Doctor）由用户显式控制 -Open。
set "OPEN_ARG=-Open"
if not "%~1"=="" set "OPEN_ARG="
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%ROOT_DIR%start-aetherlink.ps1" %OPEN_ARG% %*
set "EXIT_CODE=%ERRORLEVEL%"

echo.
if "%EXIT_CODE%"=="0" (
  echo AetherLink IoT starter finished. Open the URL shown above, then follow 接入第一台设备.
) else (
  echo AetherLink IoT did not finish successfully. See the messages above.
)

pause
exit /b %EXIT_CODE%
