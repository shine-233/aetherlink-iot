@echo off
setlocal

set "SCRIPT_DIR=%~dp0"
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%SCRIPT_DIR%start-windows.ps1" -NoPause %*
set "EXIT_CODE=%ERRORLEVEL%"

echo.
if not "%EXIT_CODE%"=="0" (
  echo AetherLink IoT did not finish successfully. See the messages above.
)
pause
exit /b %EXIT_CODE%
