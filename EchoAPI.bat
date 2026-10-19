@echo off
setlocal EnableDelayedExpansion
chcp 65001 >nul
title Echo Content API launcher

rem === Auto-detect paths (no hardcoding required) ===
for %%I in ("%~dp0") do set "ROOT=%%~dpI"
if not defined ROOT set "ROOT=%CD%\"

rem === Locate the Go toolchain ===
where go >nul 2>&1
if %errorlevel%==0 (
    for /f "tokens=*" %%i in ('where go') do (
        set "GO=%%i"
        goto :got_go
    )
)
if exist "%USERPROFILE%\go-install\go\bin\go.exe" set "GO=%USERPROFILE%\go-install\go\bin\go.exe" & goto :got_go
if exist "%SystemDrive%\go\bin\go.exe" set "GO=%SystemDrive%\go\bin\go.exe" & goto :got_go
echo [error] go.exe not found. Install Go and ensure it is on PATH, or place it under %%USERPROFILE%%\go-install\go\bin\
pause
exit /b 1
:got_go

rem === Ensure Node.js is on PATH for the frontend (fallback to WorkBuddy managed) ===
where node >nul 2>&1
if errorlevel 1 (
    for /d %%d in ("%USERPROFILE%\.workbuddy\binaries\node\versions\*") do set "NODEDIR=%%d"
    if defined NODEDIR set "PATH=%PATH%;!NODEDIR!"
)

rem === Subcommands ===
if /i "%~1"=="stop"      goto :stop
if /i "%~1"=="restart"   goto :restart
if /i "%~1"=="status"    goto :status
if /i "%~1"=="dev"       goto :dev
if /i "%~1"=="build"     goto :build
if /i "%~1"==""          goto :start
if /i "%~1"=="-h" goto :help
if /i "%~1"=="--help" goto :help
goto :help

=== ========================================================= ===
:stop
echo Stopping Echo services...
for /f "tokens=5" %%P in ('netstat -ano ^| findstr ":5173" ^| findstr "LISTENING"') do taskkill /PID %%P /F >nul 2>&1
for /f "tokens=5" %%P in ('netstat -ano ^| findstr ":8080" ^| findstr "LISTENING"') do taskkill /PID %%P /F >nul 2>&1
echo [ok] stop issued
goto :end

=== ========================================================= ===
:restart
call :stop
timeout /t 1 >nul
goto :start

=== ========================================================= ===
:status
echo === Echo services status ===
netstat -ano | findstr ":8080" | findstr "LISTENING" >nul 2>&1
if %errorlevel%==0 (
    echo [backend]  :8080 LISTENING
) else (
    echo [backend]  :8080 NOT running
)
netstat -ano | findstr ":5173" | findstr "LISTENING" >nul 2>&1
if %errorlevel%==0 (
    echo [frontend] :5173 LISTENING  (http://localhost:5173)
) else (
    echo [frontend] :5173 NOT running
)
goto :end

=== ========================================================= ===
:dev
echo [dev] starting vite dev server on :5173 (background)
start "vite-dev" /MIN cmd /c "cd /d %ROOT%web && npx vite --port 5173 --host"
goto :end

=== ========================================================= ===
:build
echo [build] building web dist...
call "%ROOT%web\node_modules\.bin\vite.cmd" build
if errorlevel 1 (echo [error] web build failed) & goto :end
echo [ok] build done
goto :end

=== ========================================================= ===
:start
echo === Echo Content API one-click start ===
echo Root dir : %ROOT%
echo Go tool  : %GO%

rem === Per-run log files (every test keeps its own log for later debugging) ===
if not exist "%ROOT%logs" mkdir "%ROOT%logs"
for /f %%t in ('powershell -NoProfile -Command "Get-Date -Format yyyyMMdd-HHmmss"') do set "TS=%%t"
echo Logs     : %ROOT%logs\echo-api-!TS!.log  ^&  %ROOT%logs\vite-!TS!.log

rem 1. Check frontend port
netstat -ano | findstr ":5173" | findstr "LISTENING" >nul 2>&1
if %errorlevel%==0 (
    echo [frontend] :5173 already running, skipping start
) else (
    echo [frontend] starting vite dev server on :5173 ...
    start "vite-dev" /MIN cmd /c "cd /d %ROOT%web && node node_modules\vite\bin\vite.js --port 5173 --host > %ROOT%logs\vite-!TS!.log 2>&1"
    echo [frontend] log: %ROOT%logs\vite-!TS!.log
    rem Give it a few seconds to bind the port
    timeout /t 3 >nul
)

rem 2. Check backend port
netstat -ano | findstr ":8080" | findstr "LISTENING" >nul 2>&1
if %errorlevel%==0 (
    echo [backend]  :8080 already running, skipping start
) else (
    echo [backend]  starting echo-api on :8080 ...
    if exist "%ROOT%echo-api.exe" (
        start "echo-api" /MIN /D "%ROOT%" cmd /c "%ROOT%echo-api.exe > %ROOT%logs\echo-api-!TS!.log 2>&1"
        echo [backend]  log: %ROOT%logs\echo-api-!TS!.log
    ) else (
        rem No compiled binary - build with Go on the fly
        start "echo-api" /MIN cmd /c "cd /d %ROOT% && %GO% run ./cmd/server"
    )
)

rem 3. Open browser
echo [open] launching browser at http://localhost:5173
start "" "http://localhost:5173"

echo.
echo [done]  backend  http://localhost:8080
echo         frontend http://localhost:5173
echo.
echo Quick commands:
echo   EchoAPI.bat status    - check ports
echo   EchoAPI.bat stop      - kill both services
echo   EchoAPI.bat restart   - stop + start
goto :end

=== ========================================================= ===
:help
echo.
echo Usage:
echo   EchoAPI.bat            - start (idempotent; skips already-running services)
echo   EchoAPI.bat dev        - start vite only
echo   EchoAPI.bat status     - show which ports are up
echo   EchoAPI.bat stop       - stop both
echo   EchoAPI.bat restart    - stop then start
echo   EchoAPI.bat build      - build web dist
echo.
goto :end

:end
if /i "%~1"=="status" (
    endlocal
    exit /b 0
)
if /i "%~1"=="stop" (
    endlocal
    exit /b 0
)
endlocal
pause
