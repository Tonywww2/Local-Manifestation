# Assemble Windows programs, SDK sources and docs without personal data.
$ErrorActionPreference = 'Stop'
$taskRoot = Split-Path -Parent $PSScriptRoot
$env:GOMODCACHE = Join-Path $taskRoot '.cache/modules'
$env:GOCACHE = Join-Path $taskRoot '.cache/build'
$env:GOPATH = Join-Path $taskRoot '.cache/gopath'
$env:GOARCH = 'amd64'
$env:CGO_ENABLED = '0'
Push-Location $taskRoot
$taskStage = $null
try {
    $taskStage = Join-Path $taskRoot ('.cache/package-' + [guid]::NewGuid().ToString('N'))
    New-Item -ItemType Directory -Path $taskStage | Out-Null
    foreach ($taskBinary in @('client.exe','server.exe','modtool.exe')) {
        Copy-Item -LiteralPath (Join-Path $taskRoot "bin/$taskBinary") -Destination $taskStage
    }
    Copy-Item -LiteralPath (Join-Path $taskRoot 'README.md') -Destination (Join-Path $taskStage 'README.md')
    Copy-Item -LiteralPath (Join-Path $taskRoot 'AGENTS.md') -Destination $taskStage
    Copy-Item -LiteralPath (Join-Path $taskRoot 'docs') -Destination $taskStage -Recurse
    Copy-Item -LiteralPath (Join-Path $taskRoot 'docs/player-guide.md') -Destination (Join-Path $taskStage 'PLAY.txt')
    Copy-Item -LiteralPath (Join-Path $taskRoot 'docs/art-assets.json') -Destination (Join-Path $taskStage 'art-assets.json')
    New-Item -ItemType Directory -Force -Path (Join-Path $taskStage 'locales'),(Join-Path $taskStage 'third-party/noto-cjk') | Out-Null
    Copy-Item -LiteralPath (Join-Path $taskRoot 'locales/en.json'),(Join-Path $taskRoot 'locales/zh-CN.json') -Destination (Join-Path $taskStage 'locales')
    Copy-Item -LiteralPath (Join-Path $taskRoot 'internal/client/fonts/LICENSE'),(Join-Path $taskRoot 'internal/client/fonts/source.json') -Destination (Join-Path $taskStage 'third-party/noto-cjk')
    New-Item -ItemType Directory -Force -Path (Join-Path $taskStage 'content') | Out-Null
    Copy-Item -LiteralPath (Join-Path $taskRoot 'content/base') -Destination (Join-Path $taskStage 'content') -Recurse
    $taskSDK = Join-Path $taskStage 'sdk-source'
    New-Item -ItemType Directory -Path $taskSDK | Out-Null
    foreach ($taskSourceDir in @('cmd','internal','pkg','sdk','content','locales')) {
        $taskOrigin = Join-Path $taskRoot $taskSourceDir
        foreach ($taskSource in (Get-ChildItem -LiteralPath $taskOrigin -Recurse -File)) {
            if (($taskSource.Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0) { throw "Source link is unsupported: $($taskSource.FullName)" }
            $taskRelative = $taskSource.FullName.Substring($taskRoot.Length).TrimStart('\','/')
            if ($taskSource.Name -like '*_test.go' -or $taskRelative.Replace('\','/') -eq 'internal/native/generated.go') { continue }
            $taskDestination = Join-Path $taskSDK $taskRelative
            New-Item -ItemType Directory -Force -Path (Split-Path -Parent $taskDestination) | Out-Null
            Copy-Item -LiteralPath $taskSource.FullName -Destination $taskDestination
        }
    }
    Copy-Item -LiteralPath (Join-Path $taskRoot 'go.mod'),(Join-Path $taskRoot 'go.sum') -Destination $taskSDK
    Copy-Item -LiteralPath (Join-Path $taskRoot 'examples') -Destination $taskStage -Recurse
    $taskModules = & go list -deps -f '{{if .Module}}{{.Module.Path}}|{{.Module.Version}}|{{.Module.Dir}}{{end}}' ./cmd/client ./cmd/server ./cmd/modtool
    if ($LASTEXITCODE -ne 0) { throw 'Dependency inventory failed' }
    foreach ($taskLine in ($taskModules | Where-Object { $_ -ne '' } | Sort-Object -Unique)) {
        $taskParts = $taskLine.Split('|')
        if ($taskParts[0] -eq 'hsmultiplayer') { continue }
        $taskLicenseRoot = Join-Path $taskStage ('third-party/' + $taskParts[0].Replace('/','_') + '@' + $taskParts[1])
        foreach ($taskLicense in (Get-ChildItem -LiteralPath $taskParts[2] -Recurse -File | Where-Object { $_.Name -match '^(LICENSE|LICENCE|COPYING|NOTICE)' })) {
            $taskRelative = $taskLicense.FullName.Substring($taskParts[2].Length).TrimStart('\','/')
            $taskDestination = Join-Path $taskLicenseRoot $taskRelative
            New-Item -ItemType Directory -Force -Path (Split-Path -Parent $taskDestination) | Out-Null
            Copy-Item -LiteralPath $taskLicense.FullName -Destination $taskDestination
        }
    }
    $taskGoRoot = (& go env GOROOT).Trim()
    New-Item -ItemType Directory -Force -Path (Join-Path $taskStage 'third-party/go') | Out-Null
    Copy-Item -LiteralPath (Join-Path $taskGoRoot 'LICENSE') -Destination (Join-Path $taskStage 'third-party/go/LICENSE')
    $taskArchive = Join-Path $taskRoot 'bin/LocalManifestation-dev-windows-amd64.zip'
    Compress-Archive -Path (Join-Path $taskStage '*') -DestinationPath $taskArchive -Force
    $taskHash = (Get-FileHash -LiteralPath $taskArchive -Algorithm SHA256).Hash.ToLowerInvariant()
    Set-Content -LiteralPath ($taskArchive + '.sha256') -Value $taskHash -Encoding ascii
    Get-Item -LiteralPath $taskArchive | Select-Object FullName,Length
    $taskHash
} finally {
    try {
        if ($taskStage -and (Test-Path -LiteralPath $taskStage)) {
            $taskResolvedStage = (Resolve-Path -LiteralPath $taskStage).ProviderPath
            $taskResolvedCache = (Resolve-Path -LiteralPath (Join-Path $taskRoot '.cache')).ProviderPath
            if ((Split-Path -Parent $taskResolvedStage) -ne $taskResolvedCache -or (Split-Path -Leaf $taskResolvedStage) -notmatch '^package-[0-9a-f]{32}$') {
                throw "Unexpected package staging path: $taskResolvedStage"
            }
            Remove-Item -LiteralPath $taskResolvedStage -Recurse -Force
        }
    } finally { Pop-Location }
}
