$ErrorActionPreference = 'Stop'
$taskRoot = Split-Path -Parent $PSScriptRoot
$env:GOMODCACHE = Join-Path $taskRoot '.cache/modules'
$env:GOCACHE = Join-Path $taskRoot '.cache/build'
$env:GOPATH = Join-Path $taskRoot '.cache/gopath'
$env:GOARCH = 'amd64'
$env:CGO_ENABLED = '0'
$taskTempDir = Join-Path $taskRoot '.cache/tmp'
New-Item -ItemType Directory -Force -Path $taskTempDir | Out-Null
$env:TEMP = $taskTempDir
$env:TMP = $taskTempDir
Push-Location $taskRoot
try {
    New-Item -ItemType Directory -Force -Path 'bin' | Out-Null
    foreach ($taskTarget in @('server', 'client', 'modtool')) {
        & go build -trimpath -o "bin/$taskTarget.exe" "./cmd/$taskTarget"
        if ($LASTEXITCODE -ne 0) { throw "Build failed: $taskTarget" }
    }
    $env:GOOS = 'linux'
    & go build -trimpath -o 'bin/server-linux-amd64' './cmd/server'
    if ($LASTEXITCODE -ne 0) { throw 'Linux server build failed' }
} finally {
    Remove-Item Env:GOOS -ErrorAction SilentlyContinue
    Pop-Location
}
