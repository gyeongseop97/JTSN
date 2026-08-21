param([string]$Version = "5.61")

$ErrorActionPreference = "Stop"
$repo = Split-Path -Parent $PSScriptRoot
$out = Join-Path $repo "outputs"
$stage = Join-Path ([System.IO.Path]::GetTempPath()) ("JTSN_v{0}_FULL_SOURCE_{1}" -f $Version, [guid]::NewGuid().ToString("N"))
$sourceRoot = Join-Path $stage ("JTSN_v{0}_FULL_SOURCE" -f $Version)

New-Item -ItemType Directory -Path $sourceRoot -Force | Out-Null
New-Item -ItemType Directory -Path $out -Force | Out-Null

$items = @("app", "installer", ".github", "scripts", "README.md", ".gitignore")
foreach ($item in $items) {
    $path = Join-Path $repo $item
    if (Test-Path -LiteralPath $path) {
        Copy-Item -LiteralPath $path -Destination $sourceRoot -Recurse -Force
    }
}

Get-ChildItem -LiteralPath $sourceRoot -Recurse -File | Where-Object {
    $relative = $_.FullName.Substring($sourceRoot.Length).TrimStart("\\")
    $relative -like "installer\\core\\*.exe" -or
    $relative -like "app\\build\\*.exe" -or
    $_.Extension -eq ".zip" -or
    $_.Name -match "^(coverage|profile)\."
} | Remove-Item -Force

$zip = Join-Path $out ("JTSN_v{0}_FULL_SOURCE.zip" -f $Version)
if (Test-Path -LiteralPath $zip) { Remove-Item -LiteralPath $zip -Force }
Compress-Archive -LiteralPath $sourceRoot -DestinationPath $zip -CompressionLevel Optimal
Remove-Item -LiteralPath $stage -Recurse -Force

$setup = Join-Path $out ("JTSN_Setup_v{0}.exe" -f $Version)
if (-not (Test-Path -LiteralPath $setup)) { throw "Installer is missing: $setup" }

$hashFile = Join-Path $out ("JTSN_v{0}_SHA256SUMS.txt" -f $Version)
$lines = foreach ($file in @($setup, $zip)) {
    $hash = (Get-FileHash -LiteralPath $file -Algorithm SHA256).Hash.ToLowerInvariant()
    "$hash  $([System.IO.Path]::GetFileName($file))"
}
[System.IO.File]::WriteAllLines($hashFile, $lines, [System.Text.UTF8Encoding]::new($false))
Write-Host "Release package complete: $out"
