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
rem Known install layouts, checked in order of likelihood. `where go` above only
rem finds a PATH entry, and a toolchain unpacked from the official .zip lives in
rem %USERPROFILE%\go-sdk\go without touching PATH at all. Missing that case made
rem the launcher abort before the backend ever started, which surfaces in the
rem browser as "captcha failed to load" - Vite's proxy had nothing to forward to.
if exist "%USERPROFILE%\go-sdk\go\bin\go.exe" set "GO=%USERPROFILE%\go-sdk\go\bin\go.exe" & goto :got_go
if exist "%USERPROFILE%\go-install\go\bin\go.exe" set "GO=%USERPROFILE%\go-install\go\bin\go.exe" & goto :got_go
if exist "%LOCALAPPDATA%\Programs\Go\bin\go.exe" set "GO=%LOCALAPPDATA%\Programs\Go\bin\go.exe" & goto :got_go
if exist "%ProgramFiles%\Go\bin\go.exe" set "GO=%ProgramFiles%\Go\bin\go.exe" & goto :got_go
if exist "%SystemDrive%\go\bin\go.exe" set "GO=%SystemDrive%\go\bin\go.exe" & goto :got_go
echo [error] go.exe not found.
echo         Install Go, add it to PATH, or unpack the official .zip so that
echo         go.exe sits in %%USERPROFILE%%\go-sdk\go\bin\
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
if /i "%~1"=="frontend"  goto :frontend
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
    rem No parentheses in this echo: an unescaped ) would close the if-block
    rem early and make cmd run the else-branch too, printing both states.
    echo [frontend] :5173 LISTENING  http://localhost:5173
) else (
    echo [frontend] :5173 NOT running
)
goto :end

=== ========================================================= ===
:dev
rem "dev" means what a developer means by it: run the project. It used to start
rem Vite alone, which left every /api call failing with ECONNREFUSED against the
rem 8080 proxy target, so the UI looked broken while the frontend itself was
rem fine. Both halves now start; the frontend-only path is :frontend.
goto :start

=== ========================================================= ===
:frontend
echo [frontend] starting vite dev server on :5173 (frontend only)
echo [frontend] the API on :8080 will NOT be started - expect failing requests
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

rem VERSION is the single source of truth (same file deploy/build.sh reads), so
rem the local binary reports the same number as the deployed one instead of the
rem unhelpful 0.0.0-dev that `go run` without -ldflags produces.
set "VER=0.0.0-dev"
if exist "%ROOT%VERSION" for /f "usebackq delims=" %%v in ("%ROOT%VERSION") do set "VER=%%v"
echo Version  : !VER!

rem === Per-run log files (every test keeps its own log for later debugging) ===
if not exist "%ROOT%logs" mkdir "%ROOT%logs"
rem Per-run log names. powershell gives a readable timestamp but is not
rem guaranteed to be on PATH; when it is missing the for loop yields nothing and
rem every log collapsed to "lopezpress-.log", overwriting the previous run. The
rem random suffix keeps runs distinct without depending on an external binary.
set "TS="
for /f %%t in ('powershell -NoProfile -Command "Get-Date -Format yyyyMMdd-HHmmss" 2^>nul') do set "TS=%%t"
if not defined TS set "TS=run-!RANDOM!"
echo Logs     : %ROOT%logs\lopezpress-!TS!.log  ^&  %ROOT%logs\vite-!TS!.log

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
    set "BLOG=%ROOT%logs\lopezpress-!TS!.log"
    echo [backend]  compiling lopezpress.exe ^(version !VER!^) ...
    rem Build to a real exe instead of `go run`. A compile error inside a
    rem detached `go run` leaves no trace - no window stays open, no log is
    rem written - so a broken build and a missing backend look identical from
    rem the outside. `go build` is incremental: ~1-3s on an unchanged tree.
    pushd "%ROOT%"
    set "GOSUMDB=sum.golang.org"
    rem Go derives its default build cache from LOCALAPPDATA, which is NOT set in
    rem every Windows session (a launcher-inherited environment can be missing
    rem it entirely, and then the default resolves to a relative "\go-build" and
    rem the build dies with "GOCACHE is not an absolute path"). USERPROFILE is
    rem always defined, so the cache is anchored there instead.
    if not defined GOCACHE set "GOCACHE=!USERPROFILE!\AppData\Local\go-build"
    "!GO!" build -ldflags "-X lopezpress/internal/version.Version=!VER!" -o lopezpress.exe ./cmd/server > "!BLOG!" 2>&1
    set "BUILDFAIL=!errorlevel!"
    popd
    if not "!BUILDFAIL!"=="0" (
        echo.
        echo [error] backend build failed. Compiler output:
        type "!BLOG!"
        echo [error] full log: !BLOG!
        pause
        goto :end
    )
    rem >> appends: the compiler output above is worth keeping when the server
    rem then refuses to boot (bad port, corrupt database, missing secret).
    rem Relative paths inside the quoted /c payload on purpose: /D already sets
    rem the working directory, and nesting "%ROOT%..." quotes inside cmd /c
    rem breaks the moment the project path contains a space.
    start "lopezpress" /MIN /D "%ROOT%" cmd /c "lopezpress.exe >> logs\lopezpress-!TS!.log 2>&1"
    echo [backend]  log: !BLOG!
)

rem 3. Open browser
echo [open] launching browser at http://localhost:5173
start "" "http://localhost:5173"

echo.
echo [done]  backend  http://localhost:8080
echo         frontend http://localhost:5173
echo.
echo Quick commands:
echo   LopezPress.bat status    - check ports
echo   LopezPress.bat stop      - kill both services
echo   LopezPress.bat restart   - stop + start
goto :end

=== ========================================================= ===
:help
echo.
echo Usage:
echo   LopezPress.bat            - start backend + frontend (idempotent)
echo   LopezPress.bat dev        - same as above; the normal way to run the project
echo   LopezPress.bat frontend   - vite only, no API on :8080
echo   LopezPress.bat status     - show which ports are up
echo   LopezPress.bat stop       - stop both
echo   LopezPress.bat restart    - stop then start
echo   LopezPress.bat build      - build web dist
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
