$ErrorActionPreference = 'Stop'

function Replace-One([string]$Text, [string]$Pattern, [string]$Replacement, [string]$Label) {
    $count = [regex]::Matches($Text, $Pattern).Count
    if ($count -ne 1) {
        throw "$Label expected exactly one match, got $count"
    }
    return [regex]::Replace($Text, $Pattern, $Replacement, 1)
}

$appPath = 'app/main.go'
$app = Get-Content $appPath -Raw

$app = Replace-One $app 'const appVersion = "5\.76"' 'const appVersion = "5.77"' 'app version'

$latestOnly = @'
const latestPatchNotes = `v5.77

• 업데이트 후 자동으로 표시되는 패치노트에는 최신 버전 내용만 표시
• 과거 패치 이력은 정보/전체 패치노트 화면에서만 확인하도록 분리`
'@

$latestPattern = '(?s)const latestPatchNotes = `.*?`\r?\n\r?\n(?=const allPatchNotes =)'
$app = Replace-One $app $latestPattern ($latestOnly + "`r`n`r`n") 'latest patch notes block'

$allHeader = @'
const allPatchNotes = `잡툴사니 · JTSN 패치노트

v5.77
• 업데이트 직후 최신 패치노트 팝업에는 최신 버전 내용만 표시
• 과거 패치 이력은 정보 화면의 전체 패치노트에서 계속 확인 가능

v5.76
'@
$allPattern = 'const allPatchNotes = `잡툴사니 · JTSN 패치노트\r?\n\r?\nv5\.76\r?\n'
$app = Replace-One $app $allPattern $allHeader 'all patch notes header'

Set-Content $appPath -Value $app -Encoding utf8

$installerPath = 'installer/main.go'
$installer = Get-Content $installerPath -Raw
$installer = Replace-One $installer 'launcherVersion = "5\.76"' 'launcherVersion = "5.77"' 'launcher version'
Set-Content $installerPath -Value $installer -Encoding utf8

gofmt -w app/main.go installer/main.go

$check = Get-Content $appPath -Raw
$latestMatch = [regex]::Match($check, '(?s)const latestPatchNotes = `(.*?)`')
if (-not $latestMatch.Success) { throw 'latestPatchNotes block not found after patch' }
$latestBody = $latestMatch.Groups[1].Value
if ($latestBody -notmatch '^v5\.77') { throw 'latestPatchNotes does not start with v5.77' }
if ($latestBody -match 'v5\.76|v5\.75|v5\.74') { throw 'latestPatchNotes still contains old versions' }
if ($check -notmatch 'const allPatchNotes = `잡툴사니 · JTSN 패치노트\r?\n\r?\nv5\.77') { throw 'allPatchNotes v5.77 header missing' }
