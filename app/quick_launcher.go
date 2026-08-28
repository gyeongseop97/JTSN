//go:build windows

package main

import (
	"strings"
	"syscall"
	"unsafe"
)

const (
	quickClassName       = "JTSNQuickLauncherWindow"
	quickSearchID        = 7601
	quickWMActivate      = 0x0006
	quickWAInactive      = 0
	quickVKReturn        = 0x0D
	quickVKUp            = 0x26
	quickVKDown          = 0x28
	quickEMSetCueBanner  = 0x1501
	quickWSExTopmost     = 0x00000008
	quickWSExToolWindow  = 0x00000080
	quickMaxResults      = 6
)

var (
	quickHWND            syscall.Handle
	quickSearch          syscall.Handle
	quickSearchOldProc   uintptr
	quickClassRegistered bool
	quickControls        []syscall.Handle
	quickRows            []syscall.Handle
	quickResults         []int
	quickSelected        int
)

func ensureQuickLauncherClass() bool {
	if quickClassRegistered {
		return true
	}
	hInst, _, _ := procGetModuleHandleW.Call(0)
	className := p16(quickClassName)
	wc := WNDCLASSEX{
		CbSize:        uint32(unsafe.Sizeof(WNDCLASSEX{})),
		LpfnWndProc:   syscall.NewCallback(quickLauncherWndProc),
		HInstance:     syscall.Handle(hInst),
		HIcon:         appIconBig,
		HIconSm:       appIconSmall,
		HbrBackground: brushPanel,
		LpszClassName: className,
	}
	r, _, _ := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	if r == 0 {
		// ERROR_CLASS_ALREADY_EXISTS is harmless when a previous quick window
		// registered the class in the same process.
		quickClassRegistered = true
		return true
	}
	quickClassRegistered = true
	return true
}

func showQuickLauncher() {
	if quickHWND != 0 {
		procShowWindow.Call(uintptr(quickHWND), SW_SHOW)
		procSetForegroundWindow.Call(uintptr(quickHWND))
		if quickSearch != 0 {
			procSetFocus.Call(uintptr(quickSearch))
		}
		return
	}
	if !ensureQuickLauncherClass() {
		showLauncherFromTray()
		return
	}

	const w, h = 620, 452
	qUser32 := syscall.NewLazyDLL("user32.dll")
	getSystemMetrics := qUser32.NewProc("GetSystemMetrics")
	sw, _, _ := getSystemMetrics.Call(0)
	sh, _, _ := getSystemMetrics.Call(1)
	x := (int(sw) - w) / 2
	y := (int(sh) - h) / 4
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}

	quickHWND = createWindow(quickWSExTopmost|quickWSExToolWindow, quickClassName, "JTSN 퀵런처", WS_POPUP|WS_VISIBLE|WS_CLIPCHILDREN, x, y, w, h, 0, 0)
	if quickHWND == 0 {
		showLauncherFromTray()
		return
	}
	rgn, _, _ := procCreateRoundRectRgn.Call(0, 0, w+1, h+1, 22, 22)
	if rgn != 0 {
		procSetWindowRgn.Call(uintptr(quickHWND), rgn, 1)
	}
	enableNativeWindowShadow(quickHWND)
	procShowWindow.Call(uintptr(quickHWND), SW_SHOW)
	procSetForegroundWindow.Call(uintptr(quickHWND))
	if quickSearch != 0 {
		procSetFocus.Call(uintptr(quickSearch))
	}
}

func closeQuickLauncher() {
	if quickHWND != 0 {
		procDestroyWindow.Call(uintptr(quickHWND))
	}
}

func quickLabel(parent syscall.Handle, text string, x, y, w, h int, font syscall.Handle) syscall.Handle {
	h := createWindow(WS_EX_TRANSPARENT, "STATIC", text, WS_CHILD|WS_VISIBLE, x, y, w, h, parent, 0)
	sendFont(h, font)
	quickControls = append(quickControls, h)
	return h
}

func quickCandidateTools(query string) []int {
	query = strings.TrimSpace(query)
	if query != "" {
		items := filterLauncherTools(query)
		if len(items) > quickMaxResults {
			items = items[:quickMaxResults]
		}
		return items
	}

	loadLauncherRecent()
	seen := map[int]bool{}
	out := make([]int, 0, quickMaxResults)
	add := func(id int) {
		if len(out) >= quickMaxResults || id < ID_NAV_PRINT || id > ID_NAV_OCR || seen[id] {
			return
		}
		seen[id] = true
		out = append(out, id)
	}
	for _, id := range launcherRecent {
		add(id)
	}
	for _, id := range loadLauncherFavorites() {
		add(id)
	}
	for _, id := range allLauncherTools() {
		add(id)
	}
	return out
}

func clearQuickRows() {
	for _, h := range quickRows {
		delete(buttonKinds, h)
		delete(buttonIDs, h)
		delete(hoveredButtons, h)
		delete(buttonOldProcs, h)
		procDestroyWindow.Call(uintptr(h))
	}
	quickRows = nil
}

func rebuildQuickResults() {
	if quickHWND == 0 {
		return
	}
	clearQuickRows()
	query := ""
	if quickSearch != 0 {
		query = getText(quickSearch)
	}
	quickResults = quickCandidateTools(query)
	quickSelected = 0
	if len(quickResults) == 0 {
		h := quickLabel(quickHWND, "검색 결과가 없습니다. 다른 검색어를 입력해 보세요.", 34, 150, 550, 34, fontNormal)
		quickRows = append(quickRows, h)
		return
	}
	for i, id := range quickResults {
		kind := BTN_RECENT
		if i == quickSelected {
			kind = BTN_PRIMARY
		}
		b := createOwnerButton(quickHWND, toolName(id), 24, 126+i*48, 572, 42, id, kind)
		quickRows = append(quickRows, b)
	}
}

func quickSetSelection(next int) {
	if len(quickResults) == 0 {
		return
	}
	if next < 0 {
		next = len(quickResults) - 1
	}
	if next >= len(quickResults) {
		next = 0
	}
	quickSelected = next
	for i, h := range quickRows {
		if _, ok := buttonKinds[h]; !ok {
			continue
		}
		if i == quickSelected {
			buttonKinds[h] = BTN_PRIMARY
		} else {
			buttonKinds[h] = BTN_RECENT
		}
		procInvalidateRect.Call(uintptr(h), 0, 0)
	}
}

func quickExecuteSelected() {
	if quickSelected < 0 || quickSelected >= len(quickResults) {
		return
	}
	quickExecuteTool(quickResults[quickSelected])
}

func quickExecuteTool(id int) {
	if id < ID_NAV_PRINT || id > ID_NAV_OCR {
		return
	}
	rememberLauncherRecent(id)
	closeQuickLauncher()
	launchTool(id)
}

func quickSearchWndProc(hwnd syscall.Handle, msg uint32, wParam, lParam uintptr) uintptr {
	if msg == WM_KEYDOWN {
		switch uint32(wParam) {
		case quickVKReturn:
			quickExecuteSelected()
			return 0
		case quickVKDown:
			quickSetSelection(quickSelected + 1)
			return 0
		case quickVKUp:
			quickSetSelection(quickSelected - 1)
			return 0
		case VK_ESCAPE:
			closeQuickLauncher()
			return 0
		}
	}
	if quickSearchOldProc != 0 {
		r, _, _ := procCallWindowProcW.Call(quickSearchOldProc, uintptr(hwnd), uintptr(msg), wParam, lParam)
		return r
	}
	r, _, _ := procDefWindowProcW.Call(uintptr(hwnd), uintptr(msg), wParam, lParam)
	return r
}

func quickLauncherWndProc(hwnd syscall.Handle, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case WM_CREATE:
		quickLabel(hwnd, "무엇을 실행할까요?", 24, 20, 360, 34, fontLauncherTitle)
		quickLabel(hwnd, "도구 이름을 입력하고 Enter · ↑↓ 이동 · Esc 닫기", 26, 52, 480, 22, fontSmall)
		quickSearch = createWindow(0, "EDIT", "", WS_CHILD|WS_VISIBLE|WS_TABSTOP|ES_AUTOHSCROLL, 34, 82, 552, 34, hwnd, quickSearchID)
		sendFont(quickSearch, fontNormal)
		procSendMessageW.Call(uintptr(quickSearch), EM_SETMARGINS, EC_LEFTMARGIN|EC_RIGHTMARGIN, uintptr(8|(8<<16)))
		procSendMessageW.Call(uintptr(quickSearch), quickEMSetCueBanner, 1, uintptr(unsafe.Pointer(p16("예: OCR, PDF, 중복, 스포이드, rename ..."))))
		quickSearchOldProc, _, _ = procSetWindowLongPtrW.Call(uintptr(quickSearch), ^uintptr(3), syscall.NewCallback(quickSearchWndProc))
		quickControls = append(quickControls, quickSearch)
		rebuildQuickResults()
		procSetFocus.Call(uintptr(quickSearch))
		return 0
	case WM_COMMAND:
		id := int(wParam & 0xffff)
		notify := int((wParam >> 16) & 0xffff)
		if id == quickSearchID && notify == EN_CHANGE {
			rebuildQuickResults()
			return 0
		}
		if id >= ID_NAV_PRINT && id <= ID_NAV_OCR {
			quickExecuteTool(id)
			return 0
		}
	case WM_DRAWITEM:
		dis := (*DRAWITEMSTRUCT)(unsafe.Pointer(lParam))
		if dis != nil {
			if kind, ok := buttonKinds[dis.HwndItem]; ok {
				drawOwnerButton(dis, kind)
				return 1
			}
		}
	case WM_PAINT:
		var ps PAINTSTRUCT
		hdc, _, _ := procBeginPaint.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&ps)))
		if hdc != 0 {
			var rc RECT
			procGetClientRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&rc)))
			procFillRect.Call(hdc, uintptr(unsafe.Pointer(&rc)), uintptr(brushPanel))
			drawSoftCard(syscall.Handle(hdc), RECT{0, 0, rc.Right, rc.Bottom}, 20, rgb(203, 213, 225), rgb(255, 255, 255))
			drawSoftCard(syscall.Handle(hdc), RECT{24, 74, rc.Right - 24, 122}, 12, rgb(203, 213, 225), rgb(255, 255, 255))
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
	case WM_CTLCOLOREDIT:
		hdc := syscall.Handle(wParam)
		procSetBkMode.Call(uintptr(hdc), TRANSPARENT)
		procSetTextColor.Call(uintptr(hdc), rgb(17, 24, 39))
		return uintptr(brushPanel)
	case WM_NCHITTEST:
		var wr RECT
		procGetWindowRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&wr)))
		y := int32(int16((lParam >> 16) & 0xffff))
		if y-wr.Top < 62 {
			return HTCAPTION
		}
		return HTCLIENT
	case quickWMActivate:
		if uint16(wParam&0xffff) == quickWAInactive {
			closeQuickLauncher()
			return 0
		}
	case WM_CLOSE:
		closeQuickLauncher()
		return 0
	case WM_DESTROY:
		clearQuickRows()
		for _, h := range quickControls {
			delete(buttonKinds, h)
			delete(buttonIDs, h)
			delete(hoveredButtons, h)
			delete(buttonOldProcs, h)
		}
		quickControls = nil
		quickResults = nil
		quickSearch = 0
		quickSearchOldProc = 0
		quickHWND = 0
		return 0
	}
	r, _, _ := procDefWindowProcW.Call(uintptr(hwnd), uintptr(msg), wParam, lParam)
	return r
}
