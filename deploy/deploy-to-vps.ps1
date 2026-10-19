# deploy-to-vps.ps1 — 在 Windows 上一键编译 + 上传 + 部署到 VPS
# 用法（PowerShell 7 / 5.1 均可）：
#   $env:VPS_USER="root"; $env:VPS_HOST="你的VPS_IP"
#   .\deploy\deploy-to-vps.ps1
# 可选：$env:VPS_SSH_PORT="22"
# 依赖：本机已安装 Go（本仓库里已有 C:\Users\LopezPC\go-sdk\go\bin\go.exe）、scp/ssh（Win10+ 内置）。

[CmdletBinding()]
param(
    [string]$VPSUser = $env:VPS_USER,
    [string]$VPSHost = $env:VPS_HOST,
    [string]$VPSPort = $env:VPS_SSH_PORT,
    [string]$AppDir  = "/opt/lopezpress"
)

$ErrorActionPreference = "Stop"

# ---- 定位 Go 编译器 ----
$goExe = if (Get-Command go -ErrorAction SilentlyContinue) { "go" }
         elseif (Test-Path "$env:USERPROFILE\go-sdk\go\bin\go.exe") { "$env:USERPROFILE\go-sdk\go\bin\go.exe" }
         else {
    Write-Error "找不到 Go 编译器。请安装 Go 或设置 $env:USERPROFILE\go-sdk\go\bin\go.exe"
    exit 1
  }

# ---- 参数检查 ----
if (-not $VPSUser -or -not $VPSHost) {
    Write-Host "用法：`$env:VPS_USER=`"root`"; `$env:VPS_HOST=`"你的VPS_IP`; .\deploy\deploy-to-vps.ps1" -ForegroundColor Yellow
    exit 1
}
if (-not $VPSPort) { $VPSPort = "22" }

$ProjectRoot = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
$ScriptDir   = Split-Path -Parent $MyInvocation.MyCommand.Path
$BinDir      = Join-Path $ProjectRoot "bin"
$WebDir      = Join-Path $ProjectRoot "web"
$WebDist     = Join-Path $WebDir "dist"

Write-Host "==> 交叉编译 Linux amd64 静态二进制..." -ForegroundColor Cyan
Push-Location $ProjectRoot
try {
    $env:CGO_ENABLED = "0"
    $env:GOOS = "linux"
    $env:GOARCH = "amd64"
    & $goExe build -ldflags="-s -w" -o "$BinDir\lopezpress" .\cmd\server
    if ($LASTEXITCODE -ne 0) { throw "go build 失败 (exit $LASTEXITCODE)" }
    $size = (Get-Item "$BinDir\lopezpress").Length / 1KB
    Write-Host "    产物: bin\lopezpress ($([math]::Round($size,1)) KB)"
} finally {
    Pop-Location
}

# ---- 构建前端 ----
Write-Host "==> 构建前端 (web/)..." -ForegroundColor Cyan
Push-Location $WebDir
try {
    if (-not (Test-Path "node_modules")) {
        Write-Host "    安装前端依赖 (npm install)..." -ForegroundColor Yellow
        & npm install
        if ($LASTEXITCODE -ne 0) { throw "npm install 失败" }
    }
    & npm run build
    if ($LASTEXITCODE -ne 0) { throw "npm run build 失败" }
    Write-Host "    前端产物: web\dist\index.html"
} finally {
    Pop-Location
}

# ---- 准备 .env ----
$envTemplate = Join-Path $ScriptDir "lopezpress.env"
$envOut      = Join-Path $ScriptDir ".env.vps"
Copy-Item -Path $envTemplate -Destination $envOut -Force
Write-Host "==> 已生成 $envOut（上传后会改名 .env；记得改 JWT_SECRET）" -ForegroundColor Cyan

# ---- 上传 ----
Write-Host "==> 上传到 ${VPSUser}@${VPSHost}:${AppDir} ..." -ForegroundColor Cyan
$sshBase = @("-P", $VPSPort, "${VPSUser}@${VPSHost}")

# 远端建目录
& ssh @sshBase "mkdir -p ${AppDir}/web"
if ($LASTEXITCODE -ne 0) { throw "ssh mkdir 失败" }

$files = @(
    "$BinDir\lopezpress",
    "$ScriptDir\install-systemd.sh",
    "$ScriptDir\lopezpress.service"
)
foreach ($f in $files) {
    Write-Host "    scp $(Split-Path $f -Leaf)"
    & scp @sshBase $f "${VPSUser}@${VPSHost}:${AppDir}/"
    if ($LASTEXITCODE -ne 0) { throw "scp 失败: $f" }
}

# 前端构建产物
Push-Location $WebDist
& scp @sshBase -r "." "${VPSUser}@${VPSHost}:${AppDir}/web/"
Pop-Location
if ($LASTEXITCODE -ne 0) { throw "scp web/dist 失败" }

& scp @sshBase $envOut "${VPSUser}@${VPSHost}:${AppDir}/.env"
if ($LASTEXITCODE -ne 0) { throw "scp .env 失败" }

# ---- 在 VPS 上安装/重启 systemd 服务 ----
Write-Host "==> 在 VPS 上安装/重启 systemd 服务..." -ForegroundColor Cyan
& ssh @sshBase "cd ${AppDir} && bash install-systemd.sh"
if ($LASTEXITCODE -ne 0) { throw "install-systemd.sh 执行失败" }

Write-Host ""
Write-Host "==> 部署完成！" -ForegroundColor Green
Write-Host "    健康检查：ssh -P ${VPSPort} ${VPSUser}@${VPSHost} 'curl -s http://localhost:8080/health'"
Write-Host "    记得在 ${AppDir}/.env 里把 JWT_SECRET 换成强随机值（openssl rand -hex 32）"
Write-Host "    改完执行：ssh -P ${VPSPort} ${VPSUser}@${VPSHost} 'systemctl restart lopezpress'"
