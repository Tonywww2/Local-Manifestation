# Keep the downloaded toolchain and dependency caches inside this checkout.
$ErrorActionPreference = 'Stop'
$taskRoot = Split-Path -Parent $PSScriptRoot
Push-Location $taskRoot
try {
    $env:GOMODCACHE = Join-Path $taskRoot '.cache/modules'
    $env:GOCACHE = Join-Path $taskRoot '.cache/build'
    $env:GOPATH = Join-Path $taskRoot '.cache/gopath'
    $env:GOARCH = 'amd64'
    $env:CGO_ENABLED = '0'
    if ($args -contains '-race') {
        $env:CGO_ENABLED = '1'
        if (Test-Path -LiteralPath 'C:/msys64/mingw64/bin/gcc.exe') { $env:CC = 'C:/msys64/mingw64/bin/gcc.exe' }
    }
    $taskTempDir = Join-Path $taskRoot '.cache/tmp'
    New-Item -ItemType Directory -Force -Path $taskTempDir | Out-Null
    $env:TEMP = $taskTempDir
    $env:TMP = $taskTempDir
    & go @args
    $taskExitCode = $LASTEXITCODE
} finally { Pop-Location }
exit $taskExitCode
