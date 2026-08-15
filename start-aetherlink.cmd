@echo off
setlocal

set "ROOT_DIR=%~dp0"
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%ROOT_DIR%start-aetherlink.ps1" -Open %*
set "EXIT_CODE=%ERRORLEVEL%"

echo.
if "%EXIT_CODE%"=="0" (
  echo AetherLink IoT starter finished. Open the URL shown above, then follow 接入第一台设备.
) else (
  echo AetherLink IoT did not finish successfully. See the messages above.
)

pause
exit /b %EXIT_CODE%
