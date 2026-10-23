# Build one release target. The workflow runs each target on its own OS.
[CmdletBinding()]
param(
    [Parameter(Mandatory)]
    [ValidatePattern('^[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?$')]
    [string]$Version,
    [Parameter(Mandatory)]
    [ValidateSet('windows', 'linux')]
    [string]$Target
)

$ErrorActionPreference = 'Stop'
$taskRoot = Split-Path -Parent $PSScriptRoot
$taskManifest = Get-Content -LiteralPath (Join-Path $taskRoot 'content/base/manifest.json') -Raw | ConvertFrom-Json
if ($Version -cne $taskManifest.version) { throw "Release $Version differs from content version $($taskManifest.version)" }
if ($Target -eq 'linux' -and [Environment]::OSVersion.Platform -ne [PlatformID]::Unix) {
    throw 'Package Linux releases on Linux so the archive preserves executable permissions'
}

$taskPreviousGOOS = $env:GOOS
$taskStage = $null
Push-Location $taskRoot
try {
    $env:GOMODCACHE = Join-Path $taskRoot '.cache/modules'
    $env:GOCACHE = Join-Path $taskRoot '.cache/build'
    $env:GOPATH = Join-Path $taskRoot '.cache/gopath'
    $env:GOOS, $env:GOARCH, $env:CGO_ENABLED = $Target, 'amd64', '0'
    New-Item -ItemType Directory -Force -Path 'bin', 'dist', '.cache/tmp' | Out-Null
    $env:TEMP = Join-Path $taskRoot '.cache/tmp'
    $env:TMP = $env:TEMP

    if ($Target -eq 'windows') {
        foreach ($taskProgram in @('client', 'server', 'modtool')) {
            & go build -mod=readonly -trimpath -o "bin/$taskProgram.exe" "./cmd/$taskProgram"
            if ($LASTEXITCODE -ne 0) { throw "Build failed: $taskProgram" }
        }
        & (Join-Path $PSScriptRoot 'package.ps1') -Version $Version -OutputDirectory 'dist'
        return
    }

    & go build -mod=readonly -trimpath -o 'bin/server-linux-amd64' './cmd/server'
    if ($LASTEXITCODE -ne 0) { throw 'Linux server build failed' }
    $taskStage = Join-Path $taskRoot ('.cache/release-' + [guid]::NewGuid().ToString('N'))
    $taskFolder = "LocalManifestation-$Version-server-linux-amd64"
    $taskPackage = Join-Path $taskStage $taskFolder
    New-Item -ItemType Directory -Path $taskPackage | Out-Null
    Copy-Item -LiteralPath 'bin/server-linux-amd64' -Destination (Join-Path $taskPackage 'server')
    & chmod 755 (Join-Path $taskPackage 'server')
    if ($LASTEXITCODE -ne 0) { throw 'Cannot set Linux server executable permissions' }
    Copy-Item -LiteralPath 'docs/releases/server-linux.md' -Destination (Join-Path $taskPackage 'README.md')
    Copy-Item -LiteralPath 'README.md' -Destination (Join-Path $taskPackage 'PROJECT.md')
    $taskRevision = (& git rev-parse HEAD).Trim()
    if ($LASTEXITCODE -ne 0) { throw 'Cannot identify source commit' }
    @{ product = 'Local Manifestation'; version = $Version; commit = $taskRevision; platform = 'linux-amd64'; go = (& go env GOVERSION).Trim() } |
        ConvertTo-Json | Set-Content -LiteralPath (Join-Path $taskPackage 'BUILDINFO.json') -Encoding utf8

    $taskModules = & go list -deps -f '{{if .Module}}{{.Module.Path}}|{{.Module.Version}}|{{.Module.Dir}}{{end}}' ./cmd/server
    if ($LASTEXITCODE -ne 0) { throw 'Dependency inventory failed' }
    foreach ($taskLine in ($taskModules | Where-Object { $_ -ne '' } | Sort-Object -Unique)) {
        $taskParts = $taskLine.Split('|')
        if ($taskParts[0] -eq 'hsmultiplayer') { continue }
        $taskLicenseRoot = Join-Path $taskPackage ('third-party/' + $taskParts[0].Replace('/', '_') + '@' + $taskParts[1])
        foreach ($taskLicense in (Get-ChildItem -LiteralPath $taskParts[2] -Recurse -File | Where-Object { $_.Name -match '^(LICENSE|LICENCE|COPYING|NOTICE)' })) {
            $taskRelative = $taskLicense.FullName.Substring($taskParts[2].Length).TrimStart([char[]]'\/')
            $taskDestination = Join-Path $taskLicenseRoot $taskRelative
            New-Item -ItemType Directory -Force -Path (Split-Path -Parent $taskDestination) | Out-Null
            Copy-Item -LiteralPath $taskLicense.FullName -Destination $taskDestination
        }
    }
    $taskGoRoot = (& go env GOROOT).Trim()
    New-Item -ItemType Directory -Force -Path (Join-Path $taskPackage 'third-party/go') | Out-Null
    Copy-Item -LiteralPath (Join-Path $taskGoRoot 'LICENSE') -Destination (Join-Path $taskPackage 'third-party/go/LICENSE')
    $taskArchive = Join-Path $taskRoot "dist/$taskFolder.tar.gz"
    & tar -czf $taskArchive -C $taskStage $taskFolder
    if ($LASTEXITCODE -ne 0) { throw 'Linux server archive failed' }
    $taskHash = (Get-FileHash -LiteralPath $taskArchive -Algorithm SHA256).Hash.ToLowerInvariant()
    Set-Content -LiteralPath ($taskArchive + '.sha256') -Value ($taskHash + '  ' + [IO.Path]::GetFileName($taskArchive)) -Encoding ascii
    Get-Item -LiteralPath $taskArchive | Select-Object FullName, Length
} finally {
    try {
        if ($taskStage -and (Test-Path -LiteralPath $taskStage)) {
            $taskResolvedStage = (Resolve-Path -LiteralPath $taskStage).ProviderPath
            $taskResolvedCache = (Resolve-Path -LiteralPath (Join-Path $taskRoot '.cache')).ProviderPath
            if ((Split-Path -Parent $taskResolvedStage) -ne $taskResolvedCache -or (Split-Path -Leaf $taskResolvedStage) -notmatch '^release-[0-9a-f]{32}$') {
                throw "Unexpected release staging path: $taskResolvedStage"
            }
            Remove-Item -LiteralPath $taskResolvedStage -Recurse -Force
        }
    } finally {
        if ($null -eq $taskPreviousGOOS) { Remove-Item Env:GOOS -ErrorAction SilentlyContinue } else { $env:GOOS = $taskPreviousGOOS }
        Pop-Location
    }
}
