//go:build windows

package main

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

const (
	launcherVersion = "5.74"
	releaseAPI      = "https://api.github.com/repos/gyeongseop97/JTSN/releases/latest"
	appFolderName   = "JTSN"
	installedName   = "JTSN.exe"
	expectedCoreSHA = "28a8cf11bf50ee7964eca562c790bd5eb3807ffd38b84c5d58d63f3fa9e3c5f5"
	wmAppUpdateExit = 0x8000 + 60
)

//go:embed core/*.exe
var embeddedCore embed.FS

//go:embed assets/JTSN.ico
var appIcon []byte

type asset struct {
	Name   string `json:"name"`
	URL    string `json:"browser_download_url"`
	Digest string `json:"digest"`
}

type release struct {
	Tag    string  `json:"tag_name"`
	Body   string  `json:"body"`
	Assets []asset `json:"assets"`
	EXE    string  `json:"-"`
	SUM    string  `json:"-"`
	HASH   string  `json:"-"`
}

var client = &http.Client{Timeout: 5 * time.Minute}

const (
	wmDestroy        = 0x0002
	wmCommand        = 0x0111
	wmClose          = 0x0010
	wmSetFont        = 0x0030
	wmCtlColorEdit   = 0x0133
	wmCtlColorStatic = 0x0138
	wmDrawItem       = 0x002B
	wmAppProgress    = 0x8001
	wmAppDone        = 0x8002
	wmUpdateStep     = 0x8003
	wmUpdateDone     = 0x8004
	pbmSetRange32    = 0x0406
	pbmSetPos        = 0x0402
	pbmSetBarColor   = 0x0409
	pbmSetBkColor    = 0x2001
	idInstall        = 1001
	idCancel         = 1002
	bsOwnerDraw      = 0x0000000B
	odsSelected      = 0x0001
	dtCenter         = 0x0001
	dtVCenter        = 0x0004
	dtSingleLine     = 0x0020
)

var (
	installing     bool
	installOK      bool
	installDone    bool
	installerHWND  uintptr
	progressHWND   uintptr
	statusHWND     uintptr
	installBtnHWND uintptr
	cancelBtnHWND  uintptr
	installerSelf  string
	installerWant  string
	updateHWND     uintptr
	updateBarHWND  uintptr
	updateTextHWND uintptr
	updatePctHWND  uintptr
	updateMu       sync.Mutex
	updateStatus   string
	updateErr      error
	updateCorePID  int
	installErrMu   sync.Mutex
	installErr     error
	modernButtons  = map[uintptr]bool{}
)

type drawItemStruct struct {
	CtlType, CtlID, ItemID, ItemAction, ItemState uint32
	HwndItem, HDC                                 uintptr
	Left, Top, Right, Bottom                      int32
	ItemData                                      uintptr
}

func drawModernButton(lParam uintptr) bool {
	d := (*drawItemStruct)(unsafe.Pointer(lParam))
	if d == nil || d.HDC == 0 {
		return false
	}
	primary := modernButtons[d.HwndItem]
	g := syscall.NewLazyDLL("gdi32.dll")
	u := syscall.NewLazyDLL("user32.dll")
	fill := uintptr(0x00FFFFFF)
	line := uintptr(0x00E4DED8)
	textColor := uintptr(0x00443A32)
	if primary {
		fill, line, textColor = 0x00EF6F2E, 0x00EF6F2E, 0x00FFFFFF
	}
	if d.ItemState&odsSelected != 0 {
		if primary {
			fill = 0x00D85B1D
			line = 0x00D85B1D
		} else {
			fill = 0x00F6F3F0
		}
	}
	brush, _, _ := g.NewProc("CreateSolidBrush").Call(fill)
	pen, _, _ := g.NewProc("CreatePen").Call(0, 1, line)
	oldBrush, _, _ := g.NewProc("SelectObject").Call(d.HDC, brush)
	oldPen, _, _ := g.NewProc("SelectObject").Call(d.HDC, pen)
	g.NewProc("RoundRect").Call(d.HDC, uintptr(d.Left), uintptr(d.Top), uintptr(d.Right), uintptr(d.Bottom), 18, 18)
	g.NewProc("SelectObject").Call(d.HDC, oldBrush)
	g.NewProc("SelectObject").Call(d.HDC, oldPen)
	g.NewProc("DeleteObject").Call(brush)
	g.NewProc("DeleteObject").Call(pen)
	g.NewProc("SetBkMode").Call(d.HDC, 1)
	g.NewProc("SetTextColor").Call(d.HDC, textColor)
	buf := make([]uint16, 128)
	n, _, _ := u.NewProc("GetWindowTextW").Call(d.HwndItem, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	r := struct{ Left, Top, Right, Bottom int32 }{d.Left, d.Top, d.Right, d.Bottom}
	u.NewProc("DrawTextW").Call(d.HDC, uintptr(unsafe.Pointer(&buf[0])), n, uintptr(unsafe.Pointer(&r)), dtCenter|dtVCenter|dtSingleLine)
	return true
}

func p16(s string) *uint16 { p, _ := syscall.UTF16PtrFromString(s); return p }

func loadInstallerBrandIcon(size int) uintptr {
	if len(appIcon) == 0 {
		return 0
	}
	dir := filepath.Join(os.TempDir(), "JTSN")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return 0
	}
	iconPath := filepath.Join(dir, "installer_brand.ico")
	if err := os.WriteFile(iconPath, appIcon, 0644); err != nil {
		return 0
	}
	u := syscall.NewLazyDLL("user32.dll")
	h, _, _ := u.NewProc("LoadImageW").Call(0, uintptr(unsafe.Pointer(p16(iconPath))), 1, uintptr(size), uintptr(size), 0x0010)
	return h
}

func message(text string, flags uintptr) uintptr {
	u := syscall.NewLazyDLL("user32.dll")
	r, _, _ := u.NewProc("MessageBoxW").Call(0, uintptr(unsafe.Pointer(p16(text))), uintptr(unsafe.Pointer(p16("JTSN · 잡툴사니 업데이트"))), flags)
	return r
}

func askUpdate(title, content string) bool {
	comctl32 := syscall.NewLazyDLL("comctl32.dll")
	button := int32(0)
	hr, _, _ := comctl32.NewProc("TaskDialog").Call(
		0, 0,
		uintptr(unsafe.Pointer(p16("JTSN · 잡툴사니"))),
		uintptr(unsafe.Pointer(p16(title))),
		uintptr(unsafe.Pointer(p16(content))),
		0x0002|0x0004, 0,
		uintptr(unsafe.Pointer(&button)),
	)
	if int32(hr) < 0 {
		return message(title+"\n\n"+content, 0x00000004|0x00000020) == 6
	}
	return button == 6
}

func main() {
	runtime.LockOSThread()
	if len(os.Args) >= 3 && os.Args[1] == "--background-update-check" {
		pid, _ := strconv.Atoi(os.Args[2])
		backgroundUpdateCheck(pid)
		return
	}
	if len(os.Args) >= 3 && os.Args[1] == "--manual-update-check" {
		pid, _ := strconv.Atoi(os.Args[2])
		manualUpdateCheck(pid)
		return
	}
	if len(os.Args) >= 4 && os.Args[1] == "--apply-update" {
		applyUpdate(os.Args[2], os.Args[3])
		return
	}
	if len(os.Args) >= 4 && os.Args[1] == "--cleanup-source" {
		cleanupSource(os.Args[2], os.Args[3])
		return
	}
	if len(os.Args) >= 2 && os.Args[1] == "--uninstall" {
		startUninstall()
		return
	}
	if len(os.Args) >= 4 && os.Args[1] == "--finish-uninstall" {
		finishUninstall(os.Args[2], os.Args[3])
		return
	}
	if len(os.Args) >= 3 && os.Args[1] == "--post-update" {
		cleanupUpdateBackup(os.Args[2])
		refreshBranding()
		os.Args = os.Args[:1]
	}
	if !ensureInstalled() {
		return
	}
	// Launch-first policy: JTSN must open even when GitHub/update checks fail.
	// The running core performs the update check shortly after startup.
	launchCore()
}

func updateSnoozePath() string {
	base, err := os.UserCacheDir()
	if err != nil {
		base = os.TempDir()
	}
	return filepath.Join(base, "JTSN", "update_snooze.txt")
}

func updateSnoozed(tag string) bool {
	b, err := os.ReadFile(updateSnoozePath())
	if err != nil {
		return false
	}
	parts := strings.SplitN(strings.TrimSpace(string(b)), "|", 2)
	if len(parts) != 2 || parts[0] != tag {
		return false
	}
	until, err := strconv.ParseInt(parts[1], 10, 64)
	return err == nil && time.Now().Unix() < until
}

func snoozeUpdate(tag string) {
	p := updateSnoozePath()
	_ = os.MkdirAll(filepath.Dir(p), 0755)
	_ = os.WriteFile(p, []byte(tag+"|"+strconv.FormatInt(time.Now().Add(6*time.Hour).Unix(), 10)), 0644)
}

func updateCheckFailurePath() string {
	base, err := os.UserCacheDir()
	if err != nil {
		base = os.TempDir()
	}
	return filepath.Join(base, "JTSN", "update_check_failure.txt")
}

func reportBackgroundUpdateError(err error) {
	if err == nil {
		return
	}
	p := updateCheckFailurePath()
	if b, readErr := os.ReadFile(p); readErr == nil {
		parts := strings.SplitN(strings.TrimSpace(string(b)), "|", 2)
		if len(parts) > 0 {
			if last, parseErr := strconv.ParseInt(parts[0], 10, 64); parseErr == nil && time.Now().Unix()-last < 24*60*60 {
				return
			}
		}
	}
	_ = os.MkdirAll(filepath.Dir(p), 0755)
	_ = os.WriteFile(p, []byte(strconv.FormatInt(time.Now().Unix(), 10)+"|"+err.Error()), 0644)
	message("자동 업데이트 확인에 실패했습니다.\n\n인터넷 연결 또는 GitHub 접속 상태를 확인해 주세요.\n설정의 '업데이트 확인' 버튼으로 즉시 다시 시도할 수 있습니다.\n\n"+err.Error(), 0x10)
}

func manualUpdateCheck(corePID int) {
	rel, err := latest()
	if err != nil {
		message("최신 버전을 확인하지 못했습니다.\n\n인터넷 연결 또는 GitHub 접속 상태를 확인해 주세요.\n\n"+err.Error(), 0x10)
		return
	}
	if !newer(rel.Tag, launcherVersion) {
		message(fmt.Sprintf("현재 v%s은 최신 버전입니다.\n\n설치된 JTSN을 그대로 사용하시면 됩니다.", launcherVersion), 0x40)
		return
	}
	body := strings.TrimSpace(rel.Body)
	if len([]rune(body)) > 480 {
		body = string([]rune(body)[:480]) + "…"
	}
	content := fmt.Sprintf("현재 버전 v%s  →  최신 버전 %s\n\n새 버전이 있습니다. 지금 업데이트하시겠습니까?", launcherVersion, rel.Tag)
	if body != "" {
		content += "\n\n" + body
	}
	if !askUpdate("새로운 JTSN을 사용할 수 있습니다", content) {
		return
	}
	updateCorePID = corePID
	if err := runUpdateProgress(rel); err != nil {
		message("업데이트에 실패했습니다. 실행 중인 버전은 그대로 유지됩니다.\n\n"+err.Error(), 0x10)
	}
}

func backgroundUpdateCheck(corePID int) {
	rel, err := latest()
	if err != nil {
		reportBackgroundUpdateError(err)
		return
	}
	if !newer(rel.Tag, launcherVersion) || updateSnoozed(rel.Tag) {
		return
	}
	body := strings.TrimSpace(rel.Body)
	if len([]rune(body)) > 320 {
		body = string([]rune(body)[:320]) + "…"
	}
	content := fmt.Sprintf("현재 버전 v%s  →  최신 버전 %s\n\n최신 버전이 있습니다. 지금 업데이트하시겠습니까?", launcherVersion, rel.Tag)
	if body != "" {
		content += "\n\n" + body
	}
	if !askUpdate("새로운 JTSN을 사용할 수 있습니다", content) {
		snoozeUpdate(rel.Tag)
		return
	}
	updateCorePID = corePID
	if err := runUpdateProgress(rel); err != nil {
		message("업데이트에 실패했습니다. 실행 중인 버전은 그대로 유지됩니다.\n\n"+err.Error(), 0x10)
	}
}

func cleanupUpdateBackup(path string) {
	for i := 0; i < 30; i++ {
		err := os.Remove(path)
		if err == nil || os.IsNotExist(err) {
			return
		}
		time.Sleep(300 * time.Millisecond)
	}
}

func installDir() string {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		base, _ = os.UserCacheDir()
	}
	return filepath.Join(base, "Programs", appFolderName)
}

func installedPath() string { return filepath.Join(installDir(), installedName) }

func setInstallError(err error) {
	installErrMu.Lock()
	installErr = err
	installErrMu.Unlock()
}

func getInstallError() error {
	installErrMu.Lock()
	defer installErrMu.Unlock()
	return installErr
}

func ensureInstalled() bool {
	self, err := os.Executable()
	if err != nil {
		return true
	}
	want := installedPath()
	a, _ := filepath.Abs(self)
	b, _ := filepath.Abs(want)
	if strings.EqualFold(a, b) {
		return true
	}
	if !runInstallWizard(self, want) {
		return false
	}
	refreshBranding()
	cmd := exec.Command(want, "--cleanup-source", self, strconv.Itoa(os.Getpid()))
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := cmd.Start(); err != nil {
		message("설치는 완료됐지만 자동 실행에 실패했습니다. 바탕화면의 JTSN 바로가기를 실행해 주세요.", 0x40)
	}
	return false
}

func runInstallWizard(self, want string) bool {
	installerSelf, installerWant = self, want
	installing, installOK, installDone = false, false, false
	setInstallError(nil)

	user32 := syscall.NewLazyDLL("user32.dll")
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	gdi32 := syscall.NewLazyDLL("gdi32.dll")
	comctl32 := syscall.NewLazyDLL("comctl32.dll")
	_ = comctl32.Load()
	comctl32.NewProc("InitCommonControls").Call()

	type wndClassEx struct {
		Size, Style         uint32
		WndProc             uintptr
		ClsExtra, WndExtra  int32
		Instance, Icon      uintptr
		Cursor, Background  uintptr
		MenuName, ClassName uintptr
		IconSm              uintptr
	}
	type point struct{ X, Y int32 }
	type msg struct {
		HWnd, Message, WParam, LParam uintptr
		Time                          uint32
		Pt                            point
		Private                       uint32
	}

	defWindowProc := user32.NewProc("DefWindowProcW")
	postQuit := user32.NewProc("PostQuitMessage")
	destroyWindow := user32.NewProc("DestroyWindow")
	setText := user32.NewProc("SetWindowTextW")
	enableWindow := user32.NewProc("EnableWindow")
	sendMessage := user32.NewProc("SendMessageW")
	postMessage := user32.NewProc("PostMessageW")
	createBrush := gdi32.NewProc("CreateSolidBrush")
	brandBrush, _, _ := createBrush.Call(0x00EF6F2E)
	headerBrush, _, _ := createBrush.Call(0x00FCFAF8)
	whiteBrush, _, _ := createBrush.Call(0x00FFFFFF)
	surfaceBrush, _, _ := createBrush.Call(0x00F8F6F3)
	setTextColor := gdi32.NewProc("SetTextColor")
	setBkColor := gdi32.NewProc("SetBkColor")
	var headerBand, headerTitle, headerSub, accentBar, logoText, logoIconCtl uintptr

	wndProc := syscall.NewCallback(func(hwnd uintptr, m uint32, wp, lp uintptr) uintptr {
		switch m {
		case wmDrawItem:
			if drawModernButton(lp) {
				return 1
			}
		case wmCtlColorStatic:
			if lp == accentBar {
				setBkColor.Call(wp, 0x00EF6F2E)
				return brandBrush
			}
			if lp == headerBand || lp == headerTitle || lp == logoText || lp == logoIconCtl {
				setTextColor.Call(wp, 0x0037291F)
				setBkColor.Call(wp, 0x00FCFAF8)
				if lp == logoText {
					setTextColor.Call(wp, 0x00EF6F2E)
				}
				return headerBrush
			}
			if lp == headerSub {
				setTextColor.Call(wp, 0x008B7464)
				setBkColor.Call(wp, 0x00FCFAF8)
				return headerBrush
			}
			setTextColor.Call(wp, 0x00352A20)
			setBkColor.Call(wp, 0x00FFFFFF)
			return whiteBrush
		case wmCtlColorEdit:
			setTextColor.Call(wp, 0x00443A32)
			setBkColor.Call(wp, 0x00FAF7F4)
			return surfaceBrush
		case wmCommand:
			id := int(wp & 0xffff)
			if id == idInstall && installDone {
				destroyWindow.Call(hwnd)
				return 0
			}
			if id == idInstall && !installing {
				installing = true
				enableWindow.Call(installBtnHWND, 0)
				enableWindow.Call(cancelBtnHWND, 0)
				setText.Call(statusHWND, uintptr(unsafe.Pointer(p16("설치 파일을 복사하고 있습니다..."))))
				go func() {
					err := copyFileProgress(installerSelf, installerWant, func(percent int) {
						postMessage.Call(installerHWND, wmAppProgress, uintptr(percent), 0)
					})
					setInstallError(err)
					if err != nil {
						postMessage.Call(installerHWND, wmAppDone, 0, 0)
						return
					}
					postMessage.Call(installerHWND, wmAppDone, 1, 0)
				}()
				return 0
			}
			if id == idCancel && !installing {
				destroyWindow.Call(hwnd)
				return 0
			}
		case wmAppProgress:
			sendMessage.Call(progressHWND, pbmSetPos, wp, 0)
			setText.Call(statusHWND, uintptr(unsafe.Pointer(p16(fmt.Sprintf("설치 중... %d%%", wp)))))
			return 0
		case wmAppDone:
			if wp == 1 {
				installOK = true
				installDone = true
				installing = false
				sendMessage.Call(progressHWND, pbmSetPos, 100, 0)
				setText.Call(statusHWND, uintptr(unsafe.Pointer(p16("설치가 완료되었습니다. [완료]를 누르면 JTSN이 실행됩니다."))))
				setText.Call(installBtnHWND, uintptr(unsafe.Pointer(p16("완료"))))
				enableWindow.Call(installBtnHWND, 1)
				user32.NewProc("ShowWindow").Call(cancelBtnHWND, 0)
			} else {
				installing = false
				enableWindow.Call(installBtnHWND, 1)
				enableWindow.Call(cancelBtnHWND, 1)
				setText.Call(statusHWND, uintptr(unsafe.Pointer(p16("설치하지 못했습니다. 다시 시도해 주세요."))))
				detail := "알 수 없는 오류"
				if err := getInstallError(); err != nil {
					detail = err.Error()
				}
				message("JTSN 설치에 실패했습니다.\n\n"+detail+"\n\n실행 중인 JTSN을 종료한 뒤 [설치 시작]을 다시 눌러 주세요.", 0x10)
			}
			return 0
		case wmClose:
			if !installing {
				destroyWindow.Call(hwnd)
			}
			return 0
		case wmDestroy:
			postQuit.Call(0)
			return 0
		}
		r, _, _ := defWindowProc.Call(hwnd, uintptr(m), wp, lp)
		return r
	})

	hInstance, _, _ := kernel32.NewProc("GetModuleHandleW").Call(0)
	brandIcon := loadInstallerBrandIcon(64)
	className := p16("JTSNInstallerWindow")
	wc := wndClassEx{Size: uint32(unsafe.Sizeof(wndClassEx{})), WndProc: wndProc, Instance: hInstance, Icon: brandIcon, Background: 6, ClassName: uintptr(unsafe.Pointer(className)), IconSm: brandIcon}
	user32.NewProc("RegisterClassExW").Call(uintptr(unsafe.Pointer(&wc)))

	create := user32.NewProc("CreateWindowExW")
	installerHWND, _, _ = create.Call(0, uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(p16("JTSN 설치"))), 0x00CA0000,
		uintptr(0x80000000), uintptr(0x80000000), 720, 500, 0, 0, hInstance, 0)
	if installerHWND == 0 {
		if message("JTSN을 다음 위치에 설치합니다.\n\n"+filepath.Dir(want)+"\n\n설치하시겠습니까?", 0x24) != 6 {
			return false
		}
		if err := os.MkdirAll(filepath.Dir(want), 0755); err != nil {
			return false
		}
		return copyFile(self, want) == nil
	}
	rgn, _, _ := gdi32.NewProc("CreateRoundRectRgn").Call(0, 0, 721, 501, 28, 28)
	user32.NewProc("SetWindowRgn").Call(installerHWND, rgn, 1)

	makeFont := func(height int, weight int) uintptr {
		font, _, _ := gdi32.NewProc("CreateFontW").Call(^uintptr(height-1), 0, 0, 0, uintptr(weight), 0, 0, 0, 1, 0, 0, 5, 0, uintptr(unsafe.Pointer(p16("Segoe UI"))))
		return font
	}
	bodyFont, titleFont, subFont, buttonFont, labelFont := makeFont(16, 400), makeFont(30, 700), makeFont(15, 400), makeFont(16, 600), makeFont(14, 600)
	add := func(class, text string, style uintptr, x, y, w, h int, id, font uintptr) uintptr {
		child, _, _ := create.Call(0, uintptr(unsafe.Pointer(p16(class))), uintptr(unsafe.Pointer(p16(text))), style|0x40000000|0x10000000,
			uintptr(x), uintptr(y), uintptr(w), uintptr(h), installerHWND, id, hInstance, 0)
		sendMessage.Call(child, wmSetFont, font, 1)
		return child
	}
	headerBand = add("STATIC", "", 0, 0, 0, 720, 122, 0, bodyFont)
	logoIconCtl = add("STATIC", "", 0x00000003, 40, 27, 64, 64, 0, bodyFont)
	if headerIcon := loadInstallerBrandIcon(56); headerIcon != 0 {
		sendMessage.Call(logoIconCtl, 0x0170, headerIcon, 0)
	}
	headerTitle = add("STATIC", "잡툴사니 설치", 0, 124, 25, 500, 38, 0, titleFont)
	headerSub = add("STATIC", "JTSN을 설치하고 자동 업데이트를 준비합니다", 0, 125, 68, 500, 24, 0, subFont)

	add("STATIC", "설치 경로", 0, 42, 150, 120, 22, 0, labelFont)
	add("EDIT", filepath.Dir(want), 0x00800800, 42, 177, 636, 38, 0, subFont)
	add("STATIC", "기본 설치 위치이며, 설치 후 바탕화면 바로가기가 자동으로 생성됩니다.", 0, 42, 222, 636, 24, 0, subFont)

	add("STATIC", "설치 상태", 0, 42, 270, 120, 22, 0, labelFont)
	progressHWND = add("msctls_progress32", "", 0, 42, 299, 636, 12, 0, bodyFont)
	sendMessage.Call(progressHWND, pbmSetRange32, 0, 100)
	sendMessage.Call(progressHWND, pbmSetBarColor, 0, 0x00EF6F2E)
	sendMessage.Call(progressHWND, pbmSetBkColor, 0, 0x00ECE8E4)
	statusHWND = add("STATIC", "설치 준비가 완료되었습니다. 아래 버튼을 눌러 시작하세요.", 0, 42, 323, 636, 28, 0, subFont)
	add("STATIC", "✓ 프로그램 파일 검증   ·   ✓ 자동 업데이트 지원   ·   ✓ 사용자 영역 설치", 0, 42, 361, 636, 24, 0, subFont)

	cancelBtnHWND = add("BUTTON", "취소", bsOwnerDraw|0x00008000, 426, 411, 106, 44, idCancel, buttonFont)
	installBtnHWND = add("BUTTON", "설치 시작", bsOwnerDraw|0x00008000, 544, 411, 134, 44, idInstall, buttonFont)
	modernButtons[installBtnHWND] = true
	modernButtons[cancelBtnHWND] = false

	user32.NewProc("ShowWindow").Call(installerHWND, 5)
	user32.NewProc("UpdateWindow").Call(installerHWND)
	var m msg
	getMessage := user32.NewProc("GetMessageW")
	translate := user32.NewProc("TranslateMessage")
	dispatch := user32.NewProc("DispatchMessageW")
	for {
		r, _, _ := getMessage.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		translate.Call(uintptr(unsafe.Pointer(&m)))
		dispatch.Call(uintptr(unsafe.Pointer(&m)))
	}
	return installOK
}

func setUpdateResult(status string, err error) {
	updateMu.Lock()
	updateStatus = status
	updateErr = err
	updateMu.Unlock()
}

func getUpdateResult() (string, error) {
	updateMu.Lock()
	defer updateMu.Unlock()
	return updateStatus, updateErr
}

func runUpdateProgress(r release) error {
	setUpdateResult("업데이트를 준비하고 있습니다...", nil)

	user32 := syscall.NewLazyDLL("user32.dll")
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	gdi32 := syscall.NewLazyDLL("gdi32.dll")
	comctl32 := syscall.NewLazyDLL("comctl32.dll")
	_ = comctl32.Load()
	comctl32.NewProc("InitCommonControls").Call()

	type wndClassEx struct {
		Size, Style         uint32
		WndProc             uintptr
		ClsExtra, WndExtra  int32
		Instance, Icon      uintptr
		Cursor, Background  uintptr
		MenuName, ClassName uintptr
		IconSm              uintptr
	}
	type point struct{ X, Y int32 }
	type msg struct {
		HWnd, Message, WParam, LParam uintptr
		Time                          uint32
		Pt                            point
		Private                       uint32
	}

	defWindowProc := user32.NewProc("DefWindowProcW")
	postQuit := user32.NewProc("PostQuitMessage")
	destroyWindow := user32.NewProc("DestroyWindow")
	setText := user32.NewProc("SetWindowTextW")
	sendMessage := user32.NewProc("SendMessageW")
	createBrush := gdi32.NewProc("CreateSolidBrush")
	brandBrush, _, _ := createBrush.Call(0x00EF6F2E)
	headerBrush, _, _ := createBrush.Call(0x00FCFAF8)
	whiteBrush, _, _ := createBrush.Call(0x00FFFFFF)
	setTextColor := gdi32.NewProc("SetTextColor")
	setBkColor := gdi32.NewProc("SetBkColor")
	var headerBand, headerTitle, headerSub, accentBar, logoText uintptr

	wndProc := syscall.NewCallback(func(hwnd uintptr, m uint32, wp, lp uintptr) uintptr {
		switch m {
		case wmCtlColorStatic:
			if lp == accentBar {
				setBkColor.Call(wp, 0x00EF6F2E)
				return brandBrush
			}
			if lp == headerBand || lp == headerTitle || lp == logoText {
				setTextColor.Call(wp, 0x0037291F)
				setBkColor.Call(wp, 0x00FCFAF8)
				if lp == logoText {
					setTextColor.Call(wp, 0x00EF6F2E)
				}
				return headerBrush
			}
			if lp == headerSub {
				setTextColor.Call(wp, 0x008B7464)
				setBkColor.Call(wp, 0x00FCFAF8)
				return headerBrush
			}
			setTextColor.Call(wp, 0x00352A20)
			setBkColor.Call(wp, 0x00FFFFFF)
			return whiteBrush
		case wmUpdateStep:
			status, _ := getUpdateResult()
			sendMessage.Call(updateBarHWND, pbmSetPos, wp, 0)
			setText.Call(updateTextHWND, uintptr(unsafe.Pointer(p16(status))))
			setText.Call(updatePctHWND, uintptr(unsafe.Pointer(p16(fmt.Sprintf("%d%%", wp)))))
			return 0
		case wmUpdateDone:
			if wp == 1 {
				sendMessage.Call(updateBarHWND, pbmSetPos, 100, 0)
				setText.Call(updatePctHWND, uintptr(unsafe.Pointer(p16("100%"))))
				setText.Call(updateTextHWND, uintptr(unsafe.Pointer(p16("업데이트 적용을 시작합니다. 잠시 후 자동으로 다시 실행됩니다."))))
			}
			destroyWindow.Call(hwnd)
			return 0
		case wmClose:
			return 0
		case wmDestroy:
			postQuit.Call(0)
			return 0
		}
		result, _, _ := defWindowProc.Call(hwnd, uintptr(m), wp, lp)
		return result
	})

	hInstance, _, _ := kernel32.NewProc("GetModuleHandleW").Call(0)
	className := p16("JTSNUpdateProgressWindow")
	wc := wndClassEx{Size: uint32(unsafe.Sizeof(wndClassEx{})), WndProc: wndProc, Instance: hInstance, Background: 6, ClassName: uintptr(unsafe.Pointer(className))}
	user32.NewProc("RegisterClassExW").Call(uintptr(unsafe.Pointer(&wc)))

	create := user32.NewProc("CreateWindowExW")
	updateHWND, _, _ = create.Call(0, uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(p16("JTSN 업데이트"))), 0x00CA0000,
		uintptr(0x80000000), uintptr(0x80000000), 640, 330, 0, 0, hInstance, 0)
	if updateHWND == 0 {
		return install(r, nil)
	}
	rgn, _, _ := gdi32.NewProc("CreateRoundRectRgn").Call(0, 0, 641, 331, 24, 24)
	user32.NewProc("SetWindowRgn").Call(updateHWND, rgn, 1)

	makeFont := func(height int, weight int) uintptr {
		font, _, _ := gdi32.NewProc("CreateFontW").Call(^uintptr(height-1), 0, 0, 0, uintptr(weight), 0, 0, 0, 1, 0, 0, 5, 0, uintptr(unsafe.Pointer(p16("Segoe UI"))))
		return font
	}
	bodyFont, titleFont, subFont, percentFont, logoFont := makeFont(17, 400), makeFont(26, 700), makeFont(15, 400), makeFont(18, 700), makeFont(24, 800)
	add := func(class, text string, style uintptr, x, y, w, h int, font uintptr) uintptr {
		child, _, _ := create.Call(0, uintptr(unsafe.Pointer(p16(class))), uintptr(unsafe.Pointer(p16(text))), style|0x40000000|0x10000000,
			uintptr(x), uintptr(y), uintptr(w), uintptr(h), updateHWND, 0, hInstance, 0)
		sendMessage.Call(child, wmSetFont, font, 1)
		return child
	}
	headerBand = add("STATIC", "", 0, 0, 0, 640, 108, bodyFont)
	accentBar = add("STATIC", "", 0, 0, 0, 8, 108, bodyFont)
	logoText = add("STATIC", "JT·SN", 0, 34, 31, 82, 34, logoFont)
	headerTitle = add("STATIC", "JTSN 업데이트", 0, 126, 24, 450, 36, titleFont)
	headerSub = add("STATIC", "새 버전을 안전하게 적용하고 있습니다", 0, 127, 66, 450, 24, subFont)
	add("STATIC", fmt.Sprintf("v%s  →  %s", launcherVersion, r.Tag), 0, 38, 132, 400, 26, bodyFont)
	updateTextHWND = add("STATIC", "업데이트를 준비하고 있습니다...", 0, 38, 174, 500, 26, subFont)
	updateBarHWND = add("msctls_progress32", "", 0, 38, 218, 496, 16, bodyFont)
	updatePctHWND = add("STATIC", "0%", 0x00000002, 544, 212, 48, 28, percentFont)
	sendMessage.Call(updateBarHWND, pbmSetRange32, 0, 100)
	sendMessage.Call(updateBarHWND, pbmSetBarColor, 0, 0x00EF6F2E)
	sendMessage.Call(updateBarHWND, pbmSetBkColor, 0, 0x00EEEAE6)
	add("STATIC", "창을 닫지 마세요. 완료되면 자동으로 다시 실행됩니다.", 0, 38, 252, 554, 24, subFont)

	postMessage := user32.NewProc("PostMessageW")
	go func() {
		err := install(r, func(percent int, status string) {
			setUpdateResult(status, nil)
			postMessage.Call(updateHWND, wmUpdateStep, uintptr(percent), 0)
		})
		setUpdateResult("", err)
		if err != nil {
			postMessage.Call(updateHWND, wmUpdateDone, 0, 0)
			return
		}
		postMessage.Call(updateHWND, wmUpdateDone, 1, 0)
	}()

	user32.NewProc("ShowWindow").Call(updateHWND, 5)
	user32.NewProc("UpdateWindow").Call(updateHWND)
	var m msg
	getMessage := user32.NewProc("GetMessageW")
	translate := user32.NewProc("TranslateMessage")
	dispatch := user32.NewProc("DispatchMessageW")
	for {
		result, _, _ := getMessage.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(result) <= 0 {
			break
		}
		translate.Call(uintptr(unsafe.Pointer(&m)))
		dispatch.Call(uintptr(unsafe.Pointer(&m)))
	}
	_, err := getUpdateResult()
	return err
}

func copyFileProgress(src, dst string, progress func(int)) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	tmp := dst + ".installing"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	buf := make([]byte, 1024*1024)
	var done int64
	for {
		n, readErr := in.Read(buf)
		if n > 0 {
			if _, err = out.Write(buf[:n]); err != nil {
				out.Close()
				_ = os.Remove(tmp)
				return err
			}
			done += int64(n)
			if info.Size() > 0 {
				progress(int(done * 100 / info.Size()))
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			out.Close()
			_ = os.Remove(tmp)
			return readErr
		}
	}
	if err = out.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := replaceInstalledFile(tmp, dst); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func stopRunningJTSN(dst string) {
	// Close the visible core gracefully, then terminate stale background
	// launchers that can keep the installed JTSN.exe locked on Windows.
	u := syscall.NewLazyDLL("user32.dll")
	hwnd, _, _ := u.NewProc("FindWindowW").Call(uintptr(unsafe.Pointer(p16("JTSNUtilityWindow"))), 0)
	if hwnd != 0 {
		u.NewProc("PostMessageW").Call(hwnd, wmAppUpdateExit, 0, 0)
	}
	quotedPath := strings.ReplaceAll(dst, "'", "''")
	script := fmt.Sprintf("Get-CimInstance Win32_Process | Where-Object { $_.ExecutablePath -eq '%s' -and $_.ProcessId -ne %d } | ForEach-Object { Stop-Process -Id $_.ProcessId -Force -ErrorAction SilentlyContinue }", quotedPath, os.Getpid())
	_ = runHidden("powershell.exe", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", script)
}

func renameWithRetry(oldPath, newPath string) error {
	var lastErr error
	for i := 0; i < 30; i++ {
		if err := os.Rename(oldPath, newPath); err == nil {
			return nil
		} else {
			lastErr = err
		}
		time.Sleep(200 * time.Millisecond)
	}
	return lastErr
}

func replaceInstalledFile(tmp, dst string) error {
	backup := dst + ".install-backup"
	_ = os.Remove(backup)

	oldExists := false
	if _, err := os.Stat(dst); err == nil {
		oldExists = true
		stopRunningJTSN(dst)
		if err := renameWithRetry(dst, backup); err != nil {
			return fmt.Errorf("기존 JTSN 실행 파일을 닫거나 백업하지 못했습니다: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("설치 대상 파일을 확인하지 못했습니다: %w", err)
	}

	if err := renameWithRetry(tmp, dst); err != nil {
		if oldExists {
			_ = renameWithRetry(backup, dst)
		}
		return fmt.Errorf("새 JTSN 실행 파일을 설치하지 못했습니다: %w", err)
	}
	if oldExists {
		_ = os.Remove(backup)
	}
	return nil
}

func runHidden(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd.Run()
}

func createDesktopShortcut() {
	// Resolve the target from the installing user's environment inside
	// PowerShell. This prevents a build-machine profile path from leaking into
	// the desktop shortcut when the installer is packaged elsewhere.
	script := "$d=[Environment]::GetFolderPath('Desktop');$wd=Join-Path $env:LOCALAPPDATA 'Programs\\JTSN';$t=Join-Path $wd 'JTSN.exe';$i=Join-Path $wd 'JTSN_v" + launcherVersion + ".ico';Remove-Item -LiteralPath (Join-Path $d '잡툴사니.lnk') -Force -ErrorAction SilentlyContinue;Remove-Item -LiteralPath (Join-Path $d 'JTSN.lnk') -Force -ErrorAction SilentlyContinue;$w=New-Object -ComObject WScript.Shell;$s=$w.CreateShortcut((Join-Path $d 'JTSN.lnk'));$s.TargetPath=$t;$s.WorkingDirectory=$wd;$s.IconLocation=$i+',0';$s.Description='JTSN · 잡툴사니';$s.Save()"
	_ = runHidden("powershell.exe", "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-Command", script)
}

func refreshBranding() {
	_ = os.MkdirAll(installDir(), 0755)
	iconPath := filepath.Join(installDir(), "JTSN_v"+launcherVersion+".ico")
	_ = os.WriteFile(iconPath, appIcon, 0644)
	createDesktopShortcut()
	registerUninstall(installedPath())
	_ = runHidden("ie4uinit.exe", "-show")
}

func removeDesktopShortcut() {
	script := "$d=[Environment]::GetFolderPath('Desktop');Remove-Item -LiteralPath (Join-Path $d 'JTSN.lnk') -Force -ErrorAction SilentlyContinue;Remove-Item -LiteralPath (Join-Path $d '잡툴사니.lnk') -Force -ErrorAction SilentlyContinue"
	_ = runHidden("powershell.exe", "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-Command", script)
}

func registerUninstall(target string) {
	key := `HKCU\Software\Microsoft\Windows\CurrentVersion\Uninstall\JTSN`
	_ = runHidden("reg.exe", "add", key, "/v", "DisplayName", "/t", "REG_SZ", "/d", "잡툴사니 (JTSN)", "/f")
	_ = runHidden("reg.exe", "add", key, "/v", "DisplayVersion", "/t", "REG_SZ", "/d", launcherVersion, "/f")
	_ = runHidden("reg.exe", "add", key, "/v", "Publisher", "/t", "REG_SZ", "/d", "JTSN", "/f")
	_ = runHidden("reg.exe", "add", key, "/v", "InstallLocation", "/t", "REG_SZ", "/d", filepath.Dir(target), "/f")
	iconPath := filepath.Join(filepath.Dir(target), "JTSN_v"+launcherVersion+".ico")
	_ = runHidden("reg.exe", "add", key, "/v", "DisplayIcon", "/t", "REG_SZ", "/d", iconPath, "/f")
	_ = runHidden("reg.exe", "add", key, "/v", "UninstallString", "/t", "REG_SZ", "/d", `"`+target+`" --uninstall`, "/f")
	_ = runHidden("reg.exe", "add", key, "/v", "NoModify", "/t", "REG_DWORD", "/d", "1", "/f")
	_ = runHidden("reg.exe", "add", key, "/v", "NoRepair", "/t", "REG_DWORD", "/d", "1", "/f")
}

func cleanupSource(source, pidText string) {
	pid, _ := strconv.Atoi(pidText)
	waitProcess(uint32(pid), 20*time.Second)
	time.Sleep(300 * time.Millisecond)
	_ = os.Remove(source + ".bak")
	cmd := exec.Command(installedPath())
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	_ = cmd.Start()
}

func startUninstall() {
	if message("잡툴사니를 제거할까요?\n\n프로그램 파일과 바로가기가 삭제됩니다.", 0x00000004|0x00000030) != 6 {
		return
	}
	self, err := os.Executable()
	if err != nil {
		return
	}
	helper := filepath.Join(os.TempDir(), "JTSN-Uninstall.exe")
	if copyFile(self, helper) != nil {
		return
	}
	cmd := exec.Command(helper, "--finish-uninstall", installDir(), strconv.Itoa(os.Getpid()))
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	_ = cmd.Start()
}

func finishUninstall(dir, pidText string) {
	pid, _ := strconv.Atoi(pidText)
	waitProcess(uint32(pid), 20*time.Second)
	time.Sleep(300 * time.Millisecond)
	removeDesktopShortcut()
	_ = runHidden("reg.exe", "delete", `HKCU\Software\Microsoft\Windows\CurrentVersion\Uninstall\JTSN`, "/f")
	_ = os.RemoveAll(dir)
	message("잡툴사니가 제거되었습니다.", 0x40)
}

func request(url string, limit int64) ([]byte, error) {
	return requestProgress(url, limit, nil)
}

func requestProgress(url string, limit int64, progress func(done, total int64)) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "JTSN-Updater/"+launcherVersion)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("서버 응답 코드 %d", resp.StatusCode)
	}
	if resp.ContentLength > limit {
		return nil, fmt.Errorf("다운로드 파일이 허용 크기를 초과했습니다")
	}
	var buf bytes.Buffer
	chunk := make([]byte, 256<<10)
	var done int64
	for {
		n, readErr := resp.Body.Read(chunk)
		if n > 0 {
			done += int64(n)
			if done > limit {
				return nil, fmt.Errorf("다운로드 파일이 허용 크기를 초과했습니다")
			}
			_, _ = buf.Write(chunk[:n])
			if progress != nil {
				progress(done, resp.ContentLength)
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return nil, readErr
		}
	}
	return buf.Bytes(), nil
}

func latest() (release, error) {
	var r release
	b, err := request(releaseAPI, 2<<20)
	if err != nil {
		return r, err
	}
	if err := json.Unmarshal(b, &r); err != nil {
		return r, err
	}
	for _, a := range r.Assets {
		name := strings.ToLower(a.Name)
		if strings.HasSuffix(name, ".exe") && strings.Contains(name, "jtsn") {
			r.EXE = a.URL
			r.HASH = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(a.Digest)), "sha256:")
		}
		if strings.HasSuffix(name, ".sha256") || strings.HasSuffix(name, ".sha256.txt") || strings.Contains(name, "sha256sums") {
			r.SUM = a.URL
		}
	}
	if r.EXE == "" || (r.HASH == "" && r.SUM == "") {
		return r, fmt.Errorf("릴리스 파일이 없습니다")
	}
	return r, nil
}

func newer(remote, current string) bool {
	parse := func(v string) []int {
		v = strings.TrimPrefix(strings.TrimSpace(strings.ToLower(v)), "v")
		parts := strings.Split(v, ".")
		out := make([]int, len(parts))
		for i, p := range parts {
			out[i], _ = strconv.Atoi(strings.SplitN(p, "-", 2)[0])
		}
		return out
	}
	a, b := parse(remote), parse(current)
	n := len(a)
	if len(b) > n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		av, bv := 0, 0
		if i < len(a) {
			av = a[i]
		}
		if i < len(b) {
			bv = b[i]
		}
		if av != bv {
			return av > bv
		}
	}
	return false
}

func install(r release, progress func(int, string)) error {
	report := func(percent int, status string) {
		if progress != nil {
			progress(percent, status)
		}
	}
	report(2, "업데이트 파일을 준비하고 있습니다...")
	exe, err := requestProgress(r.EXE, 300<<20, func(done, total int64) {
		percent := 8
		if total > 0 {
			percent = 5 + int(done*77/total)
			if percent > 82 {
				percent = 82
			}
		}
		status := fmt.Sprintf("업데이트 파일을 다운로드하고 있습니다... %.1f MB", float64(done)/(1<<20))
		if total > 0 {
			status = fmt.Sprintf("업데이트 파일을 다운로드하고 있습니다... %.1f / %.1f MB", float64(done)/(1<<20), float64(total)/(1<<20))
		}
		report(percent, status)
	})
	if err != nil {
		return err
	}
	report(85, "다운로드 파일의 무결성을 확인하고 있습니다...")
	want := r.HASH
	if want == "" {
		sum, err := request(r.SUM, 1<<20)
		if err != nil {
			return err
		}
		f := strings.Fields(string(sum))
		if len(f) == 0 {
			return fmt.Errorf("체크섬이 없습니다")
		}
		want = strings.ToLower(f[0])
	}
	gotRaw := sha256.Sum256(exe)
	got := hex.EncodeToString(gotRaw[:])
	if len(want) != 64 || want != got {
		return fmt.Errorf("SHA-256 검증에 실패했습니다")
	}
	report(92, "업데이트 설치를 준비하고 있습니다...")
	dir := filepath.Join(os.TempDir(), "JTSN-Update")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	p := filepath.Join(dir, "JTSN_"+strings.TrimPrefix(r.Tag, "v")+"_update.exe")
	if err := os.WriteFile(p, exe, 0755); err != nil {
		return err
	}
	report(97, "새 버전을 적용하고 있습니다...")
	if updateCorePID > 0 {
		// Ask the running core to terminate rather than hide to the tray. This is
		// only sent after the user accepted and the new installer passed SHA-256.
		u := syscall.NewLazyDLL("user32.dll")
		hwnd, _, _ := u.NewProc("FindWindowW").Call(uintptr(unsafe.Pointer(p16("JTSNUtilityWindow"))), 0)
		if hwnd != 0 {
			u.NewProc("PostMessageW").Call(hwnd, wmAppUpdateExit, 0, 0)
		}
		waitProcess(uint32(updateCorePID), 15*time.Second)
	}
	self, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.Command(p, "--apply-update", self, strconv.Itoa(os.Getpid()))
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	report(100, "업데이트를 시작했습니다. 잠시 후 다시 실행됩니다.")
	return nil
}

func applyUpdate(target, pidText string) {
	pid, err := strconv.Atoi(pidText)
	if err != nil {
		return
	}
	waitProcess(uint32(pid), 25*time.Second)
	time.Sleep(300 * time.Millisecond)
	self, err := os.Executable()
	if err != nil {
		return
	}
	backup := target + ".bak"
	_ = os.Remove(backup)
	if err := os.Rename(target, backup); err != nil {
		return
	}
	if err := copyFile(self, target); err != nil {
		_ = os.Rename(backup, target)
		return
	}
	cmd := exec.Command(target, "--post-update", backup)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := cmd.Start(); err != nil {
		_ = os.Remove(target)
		_ = os.Rename(backup, target)
		_ = exec.Command(target).Start()
	}
}

func waitProcess(pid uint32, timeout time.Duration) {
	k := syscall.NewLazyDLL("kernel32.dll")
	h, _, _ := k.NewProc("OpenProcess").Call(0x00100000, 0, uintptr(pid))
	if h == 0 {
		return
	}
	defer k.NewProc("CloseHandle").Call(h)
	k.NewProc("WaitForSingleObject").Call(h, uintptr(timeout/time.Millisecond))
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0755)
	if err != nil {
		return err
	}
	_, e := io.Copy(out, in)
	c := out.Close()
	if e != nil {
		return e
	}
	return c
}

func launchCore() {
	entries, err := embeddedCore.ReadDir("core")
	if err != nil {
		message("내장 JTSN 본체를 확인하지 못했습니다.\n\n"+err.Error(), 0x10)
		return
	}
	coreName := ""
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasPrefix(name, "JTSN_v") || !strings.HasSuffix(strings.ToLower(name), ".exe") {
			continue
		}
		if coreName != "" {
			message("설치 패키지 안에 JTSN 본체가 둘 이상 포함되어 있습니다.", 0x10)
			return
		}
		coreName = name
	}
	if coreName == "" {
		message("설치 패키지에서 JTSN 본체를 찾지 못했습니다.", 0x10)
		return
	}
	b, err := embeddedCore.ReadFile("core/" + coreName)
	if err != nil {
		message(err.Error(), 0x10)
		return
	}
	coreHash := sha256.Sum256(b)
	if hex.EncodeToString(coreHash[:]) != expectedCoreSHA {
		message("내장 JTSN 본체의 무결성 검증에 실패했습니다.", 0x10)
		return
	}
	dir := filepath.Join(installDir(), "core")
	if os.MkdirAll(dir, 0755) != nil {
		return
	}
	p := filepath.Join(dir, "JTSN_core_v"+launcherVersion+".exe")
	need := true
	if st, e := os.Stat(p); e == nil && st.Size() == int64(len(b)) {
		need = false
	}
	if need {
		if os.WriteFile(p, b, 0755) != nil {
			return
		}
	}
	cmd := exec.Command(p, os.Args[1:]...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	_ = cmd.Start()
}
