//go:build windows

package main

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"
)

const (
	quickClassName             = "JTSNRadialQuickLauncherWindow"
	quickFavoritesClassName    = "JTSNQuickFavoritesSettingsWindow"
	quickWMActivate            = 0x0006
	quickWAInactive            = 0
	quickWMMouseMove           = 0x0200
	quickWMLButtonUp           = 0x0202
	quickWMMouseLeave          = 0x02A3
	quickVKReturn              = 0x0D
	quickWSExTopmost           = 0x00000008
	quickWSExToolWindow        = 0x00000080
	quickOuterDiameter         = 430
	quickOuterRadius           = 205
	quickInnerRadius           = 74
	quickMaxFavorites          = 8
	quickFavoriteButtonBase    = 8100
	quickFavoriteSaveID        = 8180
	quickFavoriteResetID       = 8181
	quickFavoriteCloseID       = 8182
	ID_SETTINGS_QUICK_FAVORITES = 7650
	quickRGNDiff               = 4
)

var (
	quickHWND                     syscall.Handle
	quickClassRegistered          bool
	quickFavoritesClassRegistered bool
	quickHover                    = -1
	quickMenuTools                []int
	quickFavoritesHWND            syscall.Handle
	quickFavoriteDraft            []int
	quickFavoriteControls         []syscall.Handle
	quickFavoriteButtons          = map[int]syscall.Handle{}

	quickGDI32                   = syscall.NewLazyDLL("gdi32.dll")
	quickUser32                  = syscall.NewLazyDLL("user32.dll")
	procQuickPolygon             = quickGDI32.NewProc("Polygon")
	procQuickCreateEllipticRgn   = quickGDI32.NewProc("CreateEllipticRgn")
	procQuickCombineRgn          = quickGDI32.NewProc("CombineRgn")
	procQuickGetSystemMetrics    = quickUser32.NewProc("GetSystemMetrics")
)

func ensureQuickLauncherClass() bool {
	if quickClassRegistered {
		return true
	}
	hInst, _, _ := procGetModuleHandleW.Call(0)
	wc := WNDCLASSEX{
		CbSize:        uint32(unsafe.Sizeof(WNDCLASSEX{})),
		LpfnWndProc:   syscall.NewCallback(quickLauncherWndProc),
		HInstance:     syscall.Handle(hInst),
		HIcon:         appIconBig,
		HIconSm:       appIconSmall,
		HbrBackground: brushPanel,
		LpszClassName: p16(quickClassName),
	}
	procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	quickClassRegistered = true
	return true
}

func ensureQuickFavoritesClass() bool {
	if quickFavoritesClassRegistered {
		return true
	}
	hInst, _, _ := procGetModuleHandleW.Call(0)
	wc := WNDCLASSEX{
		CbSize:        uint32(unsafe.Sizeof(WNDCLASSEX{})),
		LpfnWndProc:   syscall.NewCallback(quickFavoritesWndProc),
		HInstance:     syscall.Handle(hInst),
		HIcon:         appIconBig,
		HIconSm:       appIconSmall,
		HbrBackground: brushPanel,
		LpszClassName: p16(quickFavoritesClassName),
	}
	procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	quickFavoritesClassRegistered = true
	return true
}

func quickFavoritesPath() string {
	base, err := os.UserCacheDir()
	if err != nil || base == "" {
		base = os.TempDir()
	}
	return filepath.Join(base, "JTSN", "quick_launcher_favorites.json")
}

func defaultQuickFavorites() []int {
	return []int{ID_NAV_OCR, ID_NAV_CLIP, ID_NAV_COLOR, ID_NAV_PDF, ID_NAV_DUP, ID_NAV_RENAME}
}

func normalizeQuickFavorites(ids []int) []int {
	seen := map[int]bool{}
	out := make([]int, 0, quickMaxFavorites)
	for _, id := range ids {
		if id < ID_NAV_PRINT || id > ID_NAV_OCR || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
		if len(out) >= quickMaxFavorites {
			break
		}
	}
	if len(out) == 0 {
		return defaultQuickFavorites()
	}
	return out
}

func loadQuickFavorites() []int {
	b, err := os.ReadFile(quickFavoritesPath())
	if err != nil {
		return defaultQuickFavorites()
	}
	var ids []int
	if json.Unmarshal(b, &ids) != nil {
		return defaultQuickFavorites()
	}
	return normalizeQuickFavorites(ids)
}

func saveQuickFavorites(ids []int) {
	ids = normalizeQuickFavorites(ids)
	p := quickFavoritesPath()
	_ = os.MkdirAll(filepath.Dir(p), 0755)
	b, _ := json.Marshal(ids)
	_ = os.WriteFile(p, b, 0644)
}

func quickApplyDonutRegion(hwnd syscall.Handle) {
	outer, _, _ := procQuickCreateEllipticRgn.Call(0, 0, quickOuterDiameter+1, quickOuterDiameter+1)
	innerLeft := quickOuterDiameter/2 - quickInnerRadius
	innerTop := quickOuterDiameter/2 - quickInnerRadius
	innerRight := quickOuterDiameter/2 + quickInnerRadius + 1
	innerBottom := quickOuterDiameter/2 + quickInnerRadius + 1
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

func showQuickLauncher() {
	quickMenuTools = loadQuickFavorites()
	if quickHWND != 0 {
		quickHover = -1
		procInvalidateRect.Call(uintptr(quickHWND), 0, 0)
		procShowWindow.Call(uintptr(quickHWND), SW_SHOW)
		procSetForegroundWindow.Call(uintptr(quickHWND))
		return
	}
	if !ensureQuickLauncherClass() {
		showLauncherFromTray()
		return
	}

	var pt POINT
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	x := int(pt.X) - quickOuterDiameter/2
	y := int(pt.Y) - quickOuterDiameter/2

	// Keep the whole wheel visible when invoked close to a screen edge.
	sw, _, _ := procQuickGetSystemMetrics.Call(0)
	sh, _, _ := procQuickGetSystemMetrics.Call(1)
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	if x+quickOuterDiameter > int(sw) {
		x = int(sw) - quickOuterDiameter
	}
	if y+quickOuterDiameter > int(sh) {
		y = int(sh) - quickOuterDiameter
	}

	quickHover = -1
	quickHWND = createWindow(quickWSExTopmost|quickWSExToolWindow, quickClassName, "JTSN 퀵런처", WS_POPUP|WS_VISIBLE|WS_CLIPCHILDREN, x, y, quickOuterDiameter, quickOuterDiameter, 0, 0)
	if quickHWND == 0 {
		showLauncherFromTray()
		return
	}
	quickApplyDonutRegion(quickHWND)
	enableNativeWindowShadow(quickHWND)
	procShowWindow.Call(uintptr(quickHWND), SW_SHOW)
	procSetForegroundWindow.Call(uintptr(quickHWND))
	procSetFocus.Call(uintptr(quickHWND))
}

func closeQuickLauncher() {
	if quickHWND != 0 {
		procDestroyWindow.Call(uintptr(quickHWND))
	}
}

func quickSegmentAt(x, y int32) int {
	if len(quickMenuTools) == 0 {
		return -1
	}
	cx := float64(quickOuterDiameter) / 2
	cy := float64(quickOuterDiameter) / 2
	dx := float64(x) - cx
	dy := float64(y) - cy
	d := math.Hypot(dx, dy)
	if d < float64(quickInnerRadius) || d > float64(quickOuterRadius) {
		return -1
	}
	// 0 degrees is the top; indices progress clockwise.
	deg := math.Atan2(dx, -dy) * 180 / math.Pi
	if deg < 0 {
		deg += 360
	}
	span := 360.0 / float64(len(quickMenuTools))
	idx := int((deg + span/2) / span)
	return idx % len(quickMenuTools)
}

func quickExecuteSegment(idx int) {
	if idx < 0 || idx >= len(quickMenuTools) {
		return
	}
	id := quickMenuTools[idx]
	rememberLauncherRecent(id)
	closeQuickLauncher()
	launchTool(id)
}

func quickRingPolygon(cx, cy float64, inner, outer float64, start, end float64) []POINT {
	steps := 18
	pts := make([]POINT, 0, (steps+1)*2)
	for i := 0; i <= steps; i++ {
		a := start + (end-start)*float64(i)/float64(steps)
		r := (a - 90) * math.Pi / 180
		pts = append(pts, POINT{X: int32(math.Round(cx + outer*math.Cos(r))), Y: int32(math.Round(cy + outer*math.Sin(r)))})
	}
	for i := steps; i >= 0; i-- {
		a := start + (end-start)*float64(i)/float64(steps)
		r := (a - 90) * math.Pi / 180
		pts = append(pts, POINT{X: int32(math.Round(cx + inner*math.Cos(r))), Y: int32(math.Round(cy + inner*math.Sin(r)))})
	}
	return pts
}

func quickDrawCenteredText(hdc syscall.Handle, text string, cx, cy, w, h int32, font syscall.Handle, color uintptr) {
	rc := RECT{Left: cx - w/2, Top: cy - h/2, Right: cx + w/2, Bottom: cy + h/2}
	procSetBkMode.Call(uintptr(hdc), TRANSPARENT)
	procSetTextColor.Call(uintptr(hdc), color)
	old, _, _ := procSelectObject.Call(uintptr(hdc), uintptr(font))
	u16 := syscall.StringToUTF16(text)
	procDrawTextW.Call(uintptr(hdc), uintptr(unsafe.Pointer(&u16[0])), uintptr(len(u16)-1), uintptr(unsafe.Pointer(&rc)), DT_CENTER|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS)
	procSelectObject.Call(uintptr(hdc), old)
}

func paintQuickLauncher(hwnd syscall.Handle) {
	var ps PAINTSTRUCT
	hdc, _, _ := procBeginPaint.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&ps)))
	if hdc == 0 {
		return
	}
	defer procEndPaint.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&ps)))

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
		pts := quickRingPolygon(cx, cy, float64(quickInnerRadius), float64(quickOuterRadius), start, end)
		if len(pts) == 0 {
			continue
		}
		fill := solidBrush(248, 250, 252)
		textColor := rgb(30, 41, 59)
		if i == quickHover {
			fill = solidBrush(219, 234, 254)
			textColor = rgb(30, 64, 175)
		}
		pen, _, _ := procCreatePen.Call(PS_SOLID, 1, rgb(203, 213, 225))
		oldBrush, _, _ := procSelectObject.Call(uintptr(hdc), uintptr(fill))
		oldPen, _, _ := procSelectObject.Call(uintptr(hdc), pen)
		procQuickPolygon.Call(uintptr(hdc), uintptr(unsafe.Pointer(&pts[0])), uintptr(len(pts)))
		procSelectObject.Call(uintptr(hdc), oldPen)
		procSelectObject.Call(uintptr(hdc), oldBrush)
		procDeleteObject.Call(pen)
		procDeleteObject.Call(uintptr(fill))

		mid := float64(i) * span
		r := (mid - 90) * math.Pi / 180
		iconR := 121.0
		textR := 158.0
		ix := int32(math.Round(cx + iconR*math.Cos(r)))
		iy := int32(math.Round(cy + iconR*math.Sin(r)))
		tx := int32(math.Round(cx + textR*math.Cos(r)))
		ty := int32(math.Round(cy + textR*math.Sin(r)))
		drawToolBitmap(syscall.Handle(hdc), id, ix-16, iy-16, 32)
		quickDrawCenteredText(syscall.Handle(hdc), toolName(id), tx, ty, 118, 28, fontSmall, textColor)
	}
}

func quickLauncherWndProc(hwnd syscall.Handle, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case quickWMMouseMove:
		x := int32(int16(lParam & 0xffff))
		y := int32(int16((lParam >> 16) & 0xffff))
		next := quickSegmentAt(x, y)
		if next != quickHover {
			quickHover = next
			procInvalidateRect.Call(uintptr(hwnd), 0, 0)
		}
		return 0
	case quickWMLButtonUp:
		x := int32(int16(lParam & 0xffff))
		y := int32(int16((lParam >> 16) & 0xffff))
		quickExecuteSegment(quickSegmentAt(x, y))
		return 0
	case WM_KEYDOWN:
		if uint32(wParam) == VK_ESCAPE {
			closeQuickLauncher()
			return 0
		}
		if uint32(wParam) == quickVKReturn && quickHover >= 0 {
			quickExecuteSegment(quickHover)
			return 0
		}
	case WM_PAINT:
		paintQuickLauncher(hwnd)
		return 0
	case WM_ERASEBKGND:
		return 1
	case quickWMMouseLeave:
		if quickHover != -1 {
			quickHover = -1
			procInvalidateRect.Call(uintptr(hwnd), 0, 0)
		}
		return 0
	case quickWMActivate:
		if uint16(wParam&0xffff) == quickWAInactive {
			closeQuickLauncher()
			return 0
		}
	case WM_CLOSE:
		closeQuickLauncher()
		return 0
	case WM_DESTROY:
		quickHWND = 0
		quickHover = -1
		quickMenuTools = nil
		return 0
	}
	r, _, _ := procDefWindowProcW.Call(uintptr(hwnd), uintptr(msg), wParam, lParam)
	return r
}

func quickFavoriteIndex(id int) int {
	for i, v := range quickFavoriteDraft {
		if v == id {
			return i
		}
	}
	return -1
}

func quickFavoriteToggle(id int) {
	idx := quickFavoriteIndex(id)
	if idx >= 0 {
		quickFavoriteDraft = append(quickFavoriteDraft[:idx], quickFavoriteDraft[idx+1:]...)
	} else if len(quickFavoriteDraft) < quickMaxFavorites {
		quickFavoriteDraft = append(quickFavoriteDraft, id)
	} else {
		info("퀵런처 즐겨찾기는 최대 8개까지 선택할 수 있습니다.")
	}
	refreshQuickFavoriteButtons()
}

func refreshQuickFavoriteButtons() {
	for _, id := range allLauncherTools() {
		h := quickFavoriteButtons[id]
		if h == 0 {
			continue
		}
		idx := quickFavoriteIndex(id)
		if idx >= 0 {
			buttonKinds[h] = BTN_PRIMARY
			setText(h, fmtQuickFavoriteButton(idx+1, toolName(id)))
		} else {
			buttonKinds[h] = BTN_SECONDARY
			setText(h, toolName(id))
		}
		procInvalidateRect.Call(uintptr(h), 0, 0)
	}
}

func fmtQuickFavoriteButton(order int, name string) string {
	return strconvItoaQuick(order) + "  ·  " + name
}

func strconvItoaQuick(n int) string {
	if n <= 0 {
		return ""
	}
	if n < 10 {
		return string(rune('0' + n))
	}
	return "8"
}

func openQuickFavoritesSettings() {
	if quickFavoritesHWND != 0 {
		procShowWindow.Call(uintptr(quickFavoritesHWND), SW_SHOW)
		procSetForegroundWindow.Call(uintptr(quickFavoritesHWND))
		return
	}
	ensureQuickFavoritesClass()
	quickFavoriteDraft = append([]int(nil), loadQuickFavorites()...)
	owner := settingsHWND
	if owner == 0 {
		owner = mainHWND
	}
	var wr RECT
	procGetWindowRect.Call(uintptr(owner), uintptr(unsafe.Pointer(&wr)))
	w, h := int32(620), int32(530)
	x := wr.Left + (wr.Right-wr.Left-w)/2
	y := wr.Top + (wr.Bottom-wr.Top-h)/2
	quickFavoritesHWND = createWindow(quickWSExToolWindow, quickFavoritesClassName, "퀵런처 즐겨찾기 설정", WS_POPUP|WS_VISIBLE|WS_CLIPCHILDREN, int(x), int(y), int(w), int(h), owner, 0)
	if quickFavoritesHWND == 0 {
		return
	}
	rgn, _, _ := procCreateRoundRectRgn.Call(0, 0, uintptr(w+1), uintptr(h+1), 20, 20)
	if rgn != 0 {
		procSetWindowRgn.Call(uintptr(quickFavoritesHWND), rgn, 1)
	}
	enableNativeWindowShadow(quickFavoritesHWND)
	procShowWindow.Call(uintptr(quickFavoritesHWND), SW_SHOW)
	procSetForegroundWindow.Call(uintptr(quickFavoritesHWND))
}

func clearQuickFavoriteSettingsControls() {
	for _, h := range quickFavoriteControls {
		delete(buttonKinds, h)
		delete(buttonIDs, h)
		delete(hoveredButtons, h)
		delete(buttonOldProcs, h)
		procDestroyWindow.Call(uintptr(h))
	}
	quickFavoriteControls = nil
	quickFavoriteButtons = map[int]syscall.Handle{}
}

func quickFavoritesWndProc(hwnd syscall.Handle, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case WM_CREATE:
		title := createWindow(WS_EX_TRANSPARENT, "STATIC", "퀵런처 즐겨찾기", WS_CHILD|WS_VISIBLE, 28, 20, 380, 34, hwnd, 0)
		sendFont(title, fontLauncherTitle)
		desc := createWindow(WS_EX_TRANSPARENT, "STATIC", "최대 8개 · 선택 순서대로 12시 방향부터 시계방향 배치", WS_CHILD|WS_VISIBLE, 30, 55, 520, 24, hwnd, 0)
		sendFont(desc, fontSmall)
		quickFavoriteControls = append(quickFavoriteControls, title, desc)
		for i, id := range allLauncherTools() {
			col, row := i%2, i/2
			x := 30 + col*286
			y := 94 + row*52
			b := createOwnerButton(hwnd, toolName(id), x, y, 270, 42, quickFavoriteButtonBase+id, BTN_SECONDARY)
			quickFavoriteButtons[id] = b
			quickFavoriteControls = append(quickFavoriteControls, b)
		}
		reset := createOwnerButton(hwnd, "기본값", 30, 456, 110, 42, quickFavoriteResetID, BTN_SECONDARY)
		closeBtn := createOwnerButton(hwnd, "취소", 368, 456, 94, 42, quickFavoriteCloseID, BTN_SECONDARY)
		saveBtn := createOwnerButton(hwnd, "저장", 472, 456, 118, 42, quickFavoriteSaveID, BTN_PRIMARY)
		quickFavoriteControls = append(quickFavoriteControls, reset, closeBtn, saveBtn)
		refreshQuickFavoriteButtons()
		return 0
	case WM_NCHITTEST:
		var wr RECT
		procGetWindowRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&wr)))
		y := int32(int16((lParam >> 16) & 0xffff))
		if y-wr.Top < 56 {
			return HTCAPTION
		}
		return HTCLIENT
	case WM_PAINT:
		var ps PAINTSTRUCT
		hdc, _, _ := procBeginPaint.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&ps)))
		if hdc != 0 {
			var rc RECT
			procGetClientRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&rc)))
			procFillRect.Call(hdc, uintptr(unsafe.Pointer(&rc)), uintptr(brushPanel))
			drawSoftCard(syscall.Handle(hdc), RECT{0, 0, rc.Right, rc.Bottom}, 18, rgb(203, 213, 225), rgb(255, 255, 255))
			procEndPaint.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&ps)))
		}
		return 0
	case WM_ERASEBKGND:
		return 1
	case WM_CTLCOLORSTATIC:
		hdc := syscall.Handle(wParam)
		procSetBkMode.Call(uintptr(hdc), TRANSPARENT)
		procSetTextColor.Call(uintptr(hdc), rgb(31, 41, 55))
		return uintptr(brushPanel)
	case WM_DRAWITEM:
		dis := (*DRAWITEMSTRUCT)(unsafe.Pointer(lParam))
		if dis != nil {
			if kind, ok := buttonKinds[dis.HwndItem]; ok {
				drawOwnerButton(dis, kind)
				return 1
			}
		}
	case WM_COMMAND:
		id := int(wParam & 0xffff)
		switch {
		case id >= quickFavoriteButtonBase+ID_NAV_PRINT && id <= quickFavoriteButtonBase+ID_NAV_OCR:
			quickFavoriteToggle(id - quickFavoriteButtonBase)
		case id == quickFavoriteResetID:
			quickFavoriteDraft = defaultQuickFavorites()
			refreshQuickFavoriteButtons()
		case id == quickFavoriteSaveID:
			saveQuickFavorites(quickFavoriteDraft)
			procDestroyWindow.Call(uintptr(hwnd))
		case id == quickFavoriteCloseID:
			procDestroyWindow.Call(uintptr(hwnd))
		}
		return 0
	case WM_CLOSE:
		procDestroyWindow.Call(uintptr(hwnd))
		return 0
	case WM_DESTROY:
		clearQuickFavoriteSettingsControls()
		quickFavoriteDraft = nil
		quickFavoritesHWND = 0
		return 0
	}
	r, _, _ := procDefWindowProcW.Call(uintptr(hwnd), uintptr(msg), wParam, lParam)
	return r
}
