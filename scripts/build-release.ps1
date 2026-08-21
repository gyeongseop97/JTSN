param([string]$Version = "5.61")

$ErrorActionPreference = "Stop"
$repo = Split-Path -Parent $PSScriptRoot
$app = Join-Path $repo "app"
$installer = Join-Path $repo "installer"
$out = Join-Path $repo "outputs"
$core = Join-Path $installer ("core\JTSN_v{0}.exe" -f $Version)
$setup = Join-Path $out ("JTSN_Setup_v{0}.exe" -f $Version)

New-Item -ItemType Directory -Path (Split-Path $core) -Force | Out-Null
New-Item -ItemType Directory -Path $out -Force | Out-Null

Push-Location $app
try {
    $goFiles = Get-ChildItem -LiteralPath $app -Filter *.go | ForEach-Object FullName
    & gofmt -w $goFiles
    if ($LASTEXITCODE -ne 0) { throw "App formatting failed" }
    & go build -buildvcs=false -trimpath -ldflags "-s -w -H windowsgui" -o $core .
    if ($LASTEXITCODE -ne 0) { throw "App build failed" }
} finally { Pop-Location }

Push-Location $installer
try {
    $goFiles = Get-ChildItem -LiteralPath $installer -Filter *.go | ForEach-Object FullName
    & gofmt -w $goFiles
    if ($LASTEXITCODE -ne 0) { throw "Installer formatting failed" }
    & go build -buildvcs=false -trimpath -ldflags "-s -w -H windowsgui" -o $setup .
    if ($LASTEXITCODE -ne 0) { throw "Installer build failed" }
} finally { Pop-Location }

& (Join-Path $PSScriptRoot "package-release.ps1") -Version $Version
