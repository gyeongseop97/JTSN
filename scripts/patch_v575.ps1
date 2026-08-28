$ErrorActionPreference = 'Stop'

function Replace-One([string]$text, [string]$pattern, [string]$replacement, [string]$label) {
    $count = [regex]::Matches($text, $pattern).Count
    if ($count -ne 1) { throw "$label expected exactly one match, got $count" }
    return [regex]::Replace($text, $pattern, $replacement, 1)
}

$appPath = 'app/main.go'
$app = Get-Content $appPath -Raw
$app = Replace-One $app 'const appVersion = "5\.74"' 'const appVersion = "5.75"' 'app version'

$latestHeader = "const latestPatchNotes = ``v5.75`r`n`r`n• 퀵런처의 사각 팝업 배경과 네이티브 사각 그림자 완전 제거`r`n• 실제 윈도우 영역을 도넛 외곽/내곽 반지름과 정확히 일치하도록 수정`r`n• 도넛 크기와 두께를 다듬고 조각 경계를 더 얇고 부드럽게 조정`r`n• 기능 아이콘과 짧은 기능명을 각 조각 중앙에 세로 정렬`r`n• 화면 OCR·클립보드·스포이드·PDF·중복파일·파일명 변경 등 퀵런처용 축약명 적용`r`n`r`nv5.74"
$app = Replace-One $app 'const latestPatchNotes = `v5\.74' $latestHeader 'latest patch notes'

$allHeader = "const allPatchNotes = ``잡툴사니 · JTSN 패치노트`r`n`r`nv5.75`r`n• 퀵런처 사각 배경과 네이티브 사각 그림자 제거`r`n• 실제 클릭/표시 영역을 도넛 형태와 정확히 일치하도록 수정`r`n• 조각 그래픽·아이콘·텍스트 정렬 전면 개선`r`n• 퀵런처용 짧은 기능명 적용`r`n`r`nv5.74"
$app = Replace-One $app 'const allPatchNotes = `잡툴사니 · JTSN 패치노트\r?\n\r?\nv5\.74' $allHeader 'all patch notes'
Set-Content $appPath -Value $app -Encoding utf8

$quickPath = 'app/quick_launcher.go'
$quick = Get-Content $quickPath -Raw
$quick = Replace-One $quick 'quickOuterDiameter\s+= 430\r?\n\s*quickOuterRadius\s+= 205\r?\n\s*quickInnerRadius\s+= 74' "quickOuterDiameter          = 390`r`n`tquickOuterRadius            = 190`r`n`tquickInnerRadius            = 70" 'quick launcher dimensions'
$quick = Replace-One $quick 'HbrBackground: brushPanel,\r?\n\s*LpszClassName: p16\(quickClassName\),' "HbrBackground: 0,`r`n`t`tLpszClassName: p16(quickClassName)," 'quick launcher background'

$regionFunction = @'
func quickApplyDonutRegion(hwnd syscall.Handle) {
	center := quickOuterDiameter / 2
	outerLeft := center - quickOuterRadius
	outerTop := center - quickOuterRadius
	outerRight := center + quickOuterRadius + 1
	outerBottom := center + quickOuterRadius + 1
	outer, _, _ := procQuickCreateEllipticRgn.Call(uintptr(outerLeft), uintptr(outerTop), uintptr(outerRight), uintptr(outerBottom))
	innerLeft := center - quickInnerRadius
	innerTop := center - quickInnerRadius
	innerRight := center + quickInnerRadius + 1
	innerBottom := center + quickInnerRadius + 1
	inner, _, _ := procQuickCreateEllipticRgn.Call(uintptr(innerLeft), uintptr(innerTop), uintptr(innerRight), uintptr(innerBottom))
	if outer == 0 || inner == 0 {
		if outer != 0 {
			procDeleteObject.Call(outer)
		}
		if inner != 0 {
			procDeleteObject.Call(inner)
		}
		return
	}
	procQuickCombineRgn.Call(outer, outer, inner, quickRGNDiff)
	procDeleteObject.Call(inner)
	procSetWindowRgn.Call(uintptr(hwnd), outer, 1)
}
'@
$quick = Replace-One $quick '(?s)func quickApplyDonutRegion\(hwnd syscall\.Handle\) \{.*?\r?\n\}' $regionFunction 'donut region'
$quick = Replace-One $quick '\r?\n\s*enableNativeWindowShadow\(quickHWND\)' '' 'remove rectangular shadow'
$quick = Replace-One $quick 'steps := 18' 'steps := 48' 'smooth donut segments'

$labelFunction = @'
func quickToolLabel(id int) string {
	switch id {
	case ID_NAV_PRINT:
		return "프린터"
	case ID_NAV_PDF:
		return "PDF"
	case ID_NAV_RENAME:
		return "파일명 변경"
	case ID_NAV_FOLDERS:
		return "폴더 도구"
	case ID_NAV_DUP:
		return "중복파일"
	case ID_NAV_IMAGE:
		return "이미지"
	case ID_NAV_COLOR:
		return "스포이드"
	case ID_NAV_TEXT:
		return "텍스트"
	case ID_NAV_CLIP:
		return "클립보드"
	case ID_NAV_BUNDLE:
		return "새 폴더"
	case ID_NAV_OCR:
		return "화면 OCR"
	default:
		return toolName(id)
	}
}

'@
$quick = Replace-One $quick 'func quickDrawCenteredText\(' ($labelFunction + 'func quickDrawCenteredText(') 'quick labels'

$paintFunction = @'
func paintQuickLauncher(hwnd syscall.Handle) {
	var ps PAINTSTRUCT
	hdc, _, _ := procBeginPaint.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&ps)))
	if hdc == 0 {
		return
	}
	defer procEndPaint.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&ps)))

	var client RECT
	procGetClientRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&client)))
	base := solidBrush(250, 252, 255)
	procFillRect.Call(hdc, uintptr(unsafe.Pointer(&client)), uintptr(base))
	procDeleteObject.Call(uintptr(base))

	cx := float64(quickOuterDiameter) / 2
	cy := float64(quickOuterDiameter) / 2
	n := len(quickMenuTools)
	if n == 0 {
		return
	}
	span := 360.0 / float64(n)
	for i, id := range quickMenuTools {
		start := float64(i)*span - span/2
		end := start + span
		inner := float64(quickInnerRadius) + 1
		outer := float64(quickOuterRadius) - 1
		pts := quickRingPolygon(cx, cy, inner, outer, start, end)
		if len(pts) == 0 {
			continue
		}

		fill := solidBrush(250, 252, 255)
		textColor := rgb(51, 65, 85)
		lineColor := rgb(226, 232, 240)
		if i == quickHover {
			fill = solidBrush(232, 240, 254)
			textColor = rgb(30, 64, 175)
			lineColor = rgb(191, 219, 254)
		}
		pen, _, _ := procCreatePen.Call(PS_SOLID, 1, lineColor)
		oldBrush, _, _ := procSelectObject.Call(uintptr(hdc), uintptr(fill))
		oldPen, _, _ := procSelectObject.Call(uintptr(hdc), pen)
		procQuickPolygon.Call(uintptr(hdc), uintptr(unsafe.Pointer(&pts[0])), uintptr(len(pts)))
		procSelectObject.Call(uintptr(hdc), oldPen)
		procSelectObject.Call(uintptr(hdc), oldBrush)
		procDeleteObject.Call(pen)
		procDeleteObject.Call(uintptr(fill))

		mid := float64(i) * span
		r := (mid - 90) * math.Pi / 180
		anchorR := (float64(quickInnerRadius) + float64(quickOuterRadius)) / 2
		ax := int32(math.Round(cx + anchorR*math.Cos(r)))
		ay := int32(math.Round(cy + anchorR*math.Sin(r)))

		iconSize := int32(28)
		drawToolBitmap(syscall.Handle(hdc), id, ax-iconSize/2, ay-31, iconSize)
		quickDrawCenteredText(syscall.Handle(hdc), quickToolLabel(id), ax, ay+24, 102, 24, fontSmall, textColor)
	}
}
'@
$quick = Replace-One $quick '(?s)func paintQuickLauncher\(hwnd syscall\.Handle\) \{.*?\r?\n\}\r?\n\r?\nfunc quickLauncherWndProc' ($paintFunction + "`r`nfunc quickLauncherWndProc") 'paint quick launcher'
Set-Content $quickPath -Value $quick -Encoding utf8

$installerPath = 'installer/main.go'
$installer = Get-Content $installerPath -Raw
$installer = Replace-One $installer 'launcherVersion = "5\.74"' 'launcherVersion = "5.75"' 'launcher version'
Set-Content $installerPath -Value $installer -Encoding utf8

gofmt -w app/main.go app/quick_launcher.go installer/main.go

$check = Get-Content app/quick_launcher.go -Raw
if ($check -match 'enableNativeWindowShadow\(quickHWND\)') { throw 'rectangular shadow still present' }
if ($check -notmatch 'return "화면 OCR"') { throw 'compact quick labels missing' }
if ($check -notmatch 'quickOuterDiameter\s+= 390') { throw 'quick launcher sizing patch failed' }
