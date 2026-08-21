//go:build windows

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"unicode/utf16"
	"unsafe"
)

const (
	WM_APP_TRAY           = WM_APP + 30
	NIM_ADD               = 0x00000000
	NIM_DELETE            = 0x00000002
	NIF_MESSAGE           = 0x00000001
	NIF_ICON              = 0x00000002
	NIF_TIP               = 0x00000004
	WM_LBUTTONDBLCLK_TRAY = 0x0203
	WM_RBUTTONUP_TRAY     = 0x0205

	ID_TRAY_OPEN             = 7301
	ID_TRAY_CLIP             = 7302
	ID_TRAY_TOGGLE           = 7303
	ID_TRAY_EXIT             = 7304
	ID_HOTKEY_PRIMARY        = 7401
	ID_HOTKEY_SECONDARY      = 7402
	ID_TIMER_HOTKEY_SEQUENCE = 7403
	ID_HOTKEY_OCR            = 7404
)

type notifyIconDataW struct {
	CbSize           uint32
	HWnd             syscall.Handle
	UID              uint32
	UFlags           uint32
	UCallbackMessage uint32
	HIcon            syscall.Handle
	SzTip            [128]uint16
	DwState          uint32
	DwStateMask      uint32
	SzInfo           [256]uint16
	UVersion         uint32
	SzInfoTitle      [64]uint16
	DwInfoFlags      uint32
	GuidItem         [16]byte
	HBalloonIcon     syscall.Handle
}

var (
	trayShell32              = syscall.NewLazyDLL("shell32.dll")
	procShellNotifyIconW     = trayShell32.NewProc("Shell_NotifyIconW")
	procRegisterHotKeyTray   = user32.NewProc("RegisterHotKey")
	procUnregisterHotKeyTray = user32.NewProc("UnregisterHotKey")
	trayData                 notifyIconDataW
	trayExitRequested        bool
	launcherHotkey           = "Ctrl+Shift+J"
	settingsHotkeyEdit       syscall.Handle
	settingsHotkeyOldProc    uintptr
	settingsHotkeyBeforeEdit string
	settingsHotkeyCaptured   string
	hotkeySecondaryVK        uint32
	hotkeySequenceActive     bool
	launcherInstanceMutex    syscall.Handle
)

func ensureSingleLauncherInstance() bool {
	h, _, err := procCreateMutexW.Call(0, 1, uintptr(unsafe.Pointer(p16("Local\\JTSN_Jobtoolsani_Main_Instance_v1"))))
	launcherInstanceMutex = syscall.Handle(h)
	if err != syscall.Errno(183) {
		return true
	}
	existing, _, _ := procFindWindowW.Call(0, uintptr(unsafe.Pointer(p16("JTSN · 잡툴사니"))))
	if existing != 0 {
		procShowWindow.Call(existing, SW_RESTORE)
		procSetForegroundWindow.Call(existing)
		procUpdateWindow.Call(existing)
	}
	if launcherInstanceMutex != 0 {
		procCloseHandleMain.Call(uintptr(launcherInstanceMutex))
		launcherInstanceMutex = 0
	}
	return false
}

func installHotkeyCapture(hwnd syscall.Handle) {
	settingsHotkeyOldProc, _, _ = procSetWindowLongPtrW.Call(uintptr(hwnd), ^uintptr(3), syscall.NewCallback(hotkeyCaptureWndProc))
}

func keyDown(vk int) bool {
	r, _, _ := procGetAsyncKeyState.Call(uintptr(vk))
	return r&0x8000 != 0
}

func capturedKeyName(vk uint32) string {
	if (vk >= 'A' && vk <= 'Z') || (vk >= '0' && vk <= '9') {
		return string(rune(vk))
	}
	if vk >= 0x70 && vk <= 0x7B {
		return "F" + strconv.Itoa(int(vk-0x70+1))
	}
	return ""
}

func hotkeyCaptureWndProc(hwnd syscall.Handle, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case 0x0007: // WM_SETFOCUS
		settingsHotkeyBeforeEdit = launcherHotkey
		settingsHotkeyCaptured = ""
		setText(hwnd, "키 조합을 누르세요...")
		return 0
	case WM_GETDLGCODE:
		return 0x0004 // DLGC_WANTALLKEYS
	case WM_KEYDOWN:
		vk := uint32(wParam)
		if vk == VK_ESCAPE {
			settingsHotkeyCaptured = settingsHotkeyBeforeEdit
			setText(hwnd, settingsHotkeyCaptured)
			return 0
		}
		if vk == VK_BACK || vk == VK_DELETE {
			settingsHotkeyCaptured = ""
			setText(hwnd, "키 조합을 누르세요...")
			return 0
		}
		if vk == VK_CONTROL || vk == VK_SHIFT || vk == VK_MENU {
			return 0
		}
		key := capturedKeyName(vk)
		if key == "" {
			return 0
		}
		if settingsHotkeyCaptured != "" {
			// A second ordinary key is treated as a sequential third key (Ctrl+J,T).
			if !strings.Contains(settingsHotkeyCaptured, ",") {
				settingsHotkeyCaptured += "," + key
				setText(hwnd, settingsHotkeyCaptured)
			}
			return 0
		}
		parts := []string{}
		if keyDown(VK_CONTROL) {
			parts = append(parts, "Ctrl")
		}
		if keyDown(VK_SHIFT) {
			parts = append(parts, "Shift")
		}
		if keyDown(VK_MENU) {
			parts = append(parts, "Alt")
		}
		parts = append(parts, key)
		if len(parts) > 3 {
			return 0
		}
		settingsHotkeyCaptured = strings.Join(parts, "+")
		setText(hwnd, settingsHotkeyCaptured)
		return 0
	}
	r, _, _ := procCallWindowProcW.Call(settingsHotkeyOldProc, uintptr(hwnd), uintptr(msg), wParam, lParam)
	return r
}

func trayHotkeyPath() string {
	base, err := os.UserCacheDir()
	if err != nil {
		base = os.TempDir()
	}
	return filepath.Join(base, "JTSN", "hotkey.json")
}
func loadLauncherHotkey() {
	var c struct {
		Hotkey string `json:"hotkey"`
	}
	if b, e := os.ReadFile(trayHotkeyPath()); e == nil && json.Unmarshal(b, &c) == nil && strings.TrimSpace(c.Hotkey) != "" {
		launcherHotkey = c.Hotkey
	}
}
func saveLauncherHotkey() {
	_ = os.MkdirAll(filepath.Dir(trayHotkeyPath()), 0755)
	b, _ := json.Marshal(struct {
		Hotkey string `json:"hotkey"`
	}{launcherHotkey})
	_ = os.WriteFile(trayHotkeyPath(), b, 0644)
}

func initTray(hwnd syscall.Handle) {
	trayData = notifyIconDataW{CbSize: uint32(unsafe.Sizeof(notifyIconDataW{})), HWnd: hwnd, UID: 1, UFlags: NIF_MESSAGE | NIF_ICON | NIF_TIP, UCallbackMessage: WM_APP_TRAY, HIcon: appIconSmall}
	copy(trayData.SzTip[:], utf16.Encode([]rune("잡툴사니 · JTSN")))
	procShellNotifyIconW.Call(NIM_ADD, uintptr(unsafe.Pointer(&trayData)))
	loadLauncherHotkey()
	registerLauncherHotkey(hwnd)
	procRegisterHotKeyTray.Call(uintptr(hwnd), ID_HOTKEY_OCR, 0x0002|0x0004, 'O')
}
func shutdownTray(hwnd syscall.Handle) {
	procUnregisterHotKeyTray.Call(uintptr(hwnd), ID_HOTKEY_PRIMARY)
	procUnregisterHotKeyTray.Call(uintptr(hwnd), ID_HOTKEY_SECONDARY)
	procUnregisterHotKeyTray.Call(uintptr(hwnd), ID_HOTKEY_OCR)
	procShellNotifyIconW.Call(NIM_DELETE, uintptr(unsafe.Pointer(&trayData)))
}
func showLauncherFromTray() {
	procShowWindow.Call(uintptr(mainHWND), SW_SHOW)
	procSetForegroundWindow.Call(uintptr(mainHWND))
	procUpdateWindow.Call(uintptr(mainHWND))
}
func hideLauncherToTray() { procShowWindow.Call(uintptr(mainHWND), 0) }

func trayMessage(lParam uintptr) bool {
	switch uint32(lParam & 0xffff) {
	case WM_LBUTTONDBLCLK_TRAY:
		showLauncherFromTray()
		return true
	case WM_RBUTTONUP_TRAY:
		showTrayMenu()
		return true
	}
	return false
}
func showTrayMenu() {
	menu, _, _ := procCreatePopupMenu.Call()
	if menu == 0 {
		return
	}
	defer procDestroyMenu.Call(menu)
	procAppendMenuW.Call(menu, MF_STRING, ID_TRAY_OPEN, uintptr(unsafe.Pointer(p16("잡툴사니 열기"))))
	procAppendMenuW.Call(menu, MF_STRING, ID_TRAY_CLIP, uintptr(unsafe.Pointer(p16("고급 클립보드"))))
	clipMu.Lock()
	paused := clipConfig.Paused
	clipMu.Unlock()
	label := "클립보드 기록 끄기"
	if paused {
		label = "클립보드 기록 켜기"
	}
	procAppendMenuW.Call(menu, MF_STRING, ID_TRAY_TOGGLE, uintptr(unsafe.Pointer(p16(label))))
	procAppendMenuW.Call(menu, MF_STRING, ID_TRAY_EXIT, uintptr(unsafe.Pointer(p16("종료"))))
	var pt POINT
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	procSetForegroundWindow.Call(uintptr(mainHWND))
	cmd, _, _ := procTrackPopupMenu.Call(menu, TPM_RETURNCMD|TPM_NONOTIFY, uintptr(pt.X), uintptr(pt.Y), 0, uintptr(mainHWND), 0)
	switch cmd {
	case ID_TRAY_OPEN:
		showLauncherFromTray()
	case ID_TRAY_CLIP:
		launchTool(ID_NAV_CLIP)
	case ID_TRAY_TOGGLE:
		clipMu.Lock()
		clipConfig.Paused = !clipConfig.Paused
		clipMu.Unlock()
		saveClipboardConfig()
	case ID_TRAY_EXIT:
		trayExitRequested = true
		procDestroyWindow.Call(uintptr(mainHWND))
	}
}

func parseLauncherHotkey(s string) (mods, primary, secondary uint32, ok bool) {
	s = strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(s), " ", ""))
	if s == "" {
		return
	}
	parts := strings.FieldsFunc(s, func(r rune) bool { return r == '+' || r == ',' })
	normal := []uint32{}
	for _, p := range parts {
		switch p {
		case "CTRL", "CONTROL":
			mods |= 0x0002
		case "SHIFT":
			mods |= 0x0004
		case "ALT":
			mods |= 0x0001
		default:
			v := hotkeyVK(p)
			if v == 0 {
				return 0, 0, 0, false
			}
			normal = append(normal, v)
		}
	}
	if len(parts) > 3 || len(normal) < 1 || len(normal) > 2 {
		return 0, 0, 0, false
	}
	if len(normal) == 2 {
		secondary = normal[1]
	}
	return mods, normal[0], secondary, true
}
func hotkeyVK(s string) uint32 {
	if len(s) == 1 {
		c := s[0]
		if (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') {
			return uint32(c)
		}
	}
	if strings.HasPrefix(s, "F") {
		n := 0
		for _, c := range s[1:] {
			if c < '0' || c > '9' {
				return 0
			}
			n = n*10 + int(c-'0')
		}
		if n >= 1 && n <= 12 {
			return uint32(0x70 + n - 1)
		}
	}
	return 0
}
func registerLauncherHotkey(hwnd syscall.Handle) bool {
	procUnregisterHotKeyTray.Call(uintptr(hwnd), ID_HOTKEY_PRIMARY)
	procUnregisterHotKeyTray.Call(uintptr(hwnd), ID_HOTKEY_SECONDARY)
	hotkeySequenceActive = false
	mods, vk, second, ok := parseLauncherHotkey(launcherHotkey)
	if !ok {
		return false
	}
	r, _, _ := procRegisterHotKeyTray.Call(uintptr(hwnd), ID_HOTKEY_PRIMARY, uintptr(mods), uintptr(vk))
	if r == 0 {
		return false
	}
	hotkeySecondaryVK = second
	return true
}
func launcherHotkeyMessage(hwnd syscall.Handle, id uintptr) bool {
	if id == ID_HOTKEY_OCR {
		launchTool(ID_NAV_OCR)
		return true
	}
	if id == ID_HOTKEY_PRIMARY {
		if hotkeySecondaryVK == 0 {
			showLauncherFromTray()
		} else {
			procUnregisterHotKeyTray.Call(uintptr(hwnd), ID_HOTKEY_SECONDARY)
			procRegisterHotKeyTray.Call(uintptr(hwnd), ID_HOTKEY_SECONDARY, 0, uintptr(hotkeySecondaryVK))
			hotkeySequenceActive = true
			procSetTimer.Call(uintptr(hwnd), ID_TIMER_HOTKEY_SEQUENCE, 1800, 0)
		}
		return true
	}
	if id == ID_HOTKEY_SECONDARY && hotkeySequenceActive {
		finishHotkeySequence(hwnd)
		showLauncherFromTray()
		return true
	}
	return false
}
func finishHotkeySequence(hwnd syscall.Handle) {
	procKillTimer.Call(uintptr(hwnd), ID_TIMER_HOTKEY_SEQUENCE)
	procUnregisterHotKeyTray.Call(uintptr(hwnd), ID_HOTKEY_SECONDARY)
	hotkeySequenceActive = false
}
func hotkeyTimer(hwnd syscall.Handle, id uintptr) bool {
	if id == ID_TIMER_HOTKEY_SEQUENCE {
		finishHotkeySequence(hwnd)
		return true
	}
	return false
}

func applySettingsHotkey(hwnd syscall.Handle) bool {
	if settingsHotkeyEdit == 0 {
		return true
	}
	candidate := strings.TrimSpace(getText(settingsHotkeyEdit))
	old := launcherHotkey
	launcherHotkey = candidate
	if !registerLauncherHotkey(hwnd) {
		launcherHotkey = old
		registerLauncherHotkey(hwnd)
		errorBox("단축키 형식이 올바르지 않거나 다른 프로그램에서 사용 중입니다.\n\n예: Ctrl+J / Ctrl+Shift+J / Ctrl+J,T")
		return false
	}
	saveLauncherHotkey()
	return true
}
