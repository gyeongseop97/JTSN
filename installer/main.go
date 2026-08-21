//go:build windows

package main

import (
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
	"syscall"
	"time"
	"unsafe"
)

const (
	launcherVersion = "5.55"
	releaseAPI      = "https://api.github.com/repos/gyeongseop97/JTSN/releases/latest"
	appFolderName   = "JTSN"
	installedName   = "JTSN.exe"
)

//go:embed core/JTSN_v5.50.exe
var embeddedCore embed.FS

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
	wmDestroy     = 0x0002
	wmCommand     = 0x0111
	wmClose       = 0x0010
	wmSetFont     = 0x0030
	wmAppProgress = 0x8001
	wmAppDone     = 0x8002
	pbmSetRange32 = 0x0406
	pbmSetPos     = 0x0402
	idInstall     = 1001
	idCancel      = 1002
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
)

func p16(s string) *uint16 { p, _ := syscall.UTF16PtrFromString(s); return p }

func message(text string, flags uintptr) uintptr {
	u := syscall.NewLazyDLL("user32.dll")
	r, _, _ := u.NewProc("MessageBoxW").Call(0, uintptr(unsafe.Pointer(p16(text))), uintptr(unsafe.Pointer(p16("JTSN · 잡툴사니 업데이트"))), flags)
	return r
}

func main() {
	runtime.LockOSThread()
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
		_ = os.Remove(os.Args[2])
		os.Args = os.Args[:1]
	}
	if !ensureInstalled() {
		return
	}
	if rel, err := latest(); err == nil && newer(rel.Tag, launcherVersion) {
		body := strings.TrimSpace(rel.Body)
		if len([]rune(body)) > 380 {
			body = string([]rune(body)[:380]) + "…"
		}
		prompt := fmt.Sprintf("새 버전 %s을 사용할 수 있습니다.\n현재 버전: v%s", rel.Tag, launcherVersion)
		if body != "" {
			prompt += "\n\n" + body
		}
		prompt += "\n\n지금 업데이트할까요?"
		if message(prompt, 0x00000004|0x00000020) == 6 {
			if err := install(rel); err == nil {
				return
			} else {
				message("업데이트에 실패했습니다. 기존 버전을 실행합니다.\n\n"+err.Error(), 0x00000000|0x00000010)
			}
		}
	}
	launchCore()
}

func installDir() string {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		base, _ = os.UserCacheDir()
	}
	return filepath.Join(base, "Programs", appFolderName)
}

func installedPath() string { return filepath.Join(installDir(), installedName) }

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
	registerUninstall(want)
	createDesktopShortcut()
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

	wndProc := syscall.NewCallback(func(hwnd uintptr, m uint32, wp, lp uintptr) uintptr {
		switch m {
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
				message("JTSN 설치에 실패했습니다.", 0x10)
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
	className := p16("JTSNInstallerWindow")
	wc := wndClassEx{Size: uint32(unsafe.Sizeof(wndClassEx{})), WndProc: wndProc, Instance: hInstance, Background: 6, ClassName: uintptr(unsafe.Pointer(className))}
	user32.NewProc("RegisterClassExW").Call(uintptr(unsafe.Pointer(&wc)))

	create := user32.NewProc("CreateWindowExW")
	installerHWND, _, _ = create.Call(0, uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(p16("JTSN 설치"))), 0x00CA0000,
		uintptr(0x80000000), uintptr(0x80000000), 570, 330, 0, 0, hInstance, 0)
	if installerHWND == 0 {
		if message("JTSN을 다음 위치에 설치합니다.\n\n"+filepath.Dir(want)+"\n\n설치하시겠습니까?", 0x24) != 6 {
			return false
		}
		if err := os.MkdirAll(filepath.Dir(want), 0755); err != nil {
			return false
		}
		return copyFile(self, want) == nil
	}

	font, _, _ := gdi32.NewProc("CreateFontW").Call(^uintptr(17-1), 0, 0, 0, 400, 0, 0, 0, 1, 0, 0, 5, 0, uintptr(unsafe.Pointer(p16("Segoe UI"))))
	add := func(class, text string, style uintptr, x, y, w, h int, id uintptr) uintptr {
		child, _, _ := create.Call(0, uintptr(unsafe.Pointer(p16(class))), uintptr(unsafe.Pointer(p16(text))), style|0x40000000|0x10000000,
			uintptr(x), uintptr(y), uintptr(w), uintptr(h), installerHWND, id, hInstance, 0)
		sendMessage.Call(child, wmSetFont, font, 1)
		return child
	}
	add("STATIC", "JTSN · 잡툴사니 설치", 0, 32, 25, 480, 32, 0)
	add("STATIC", "프로그램을 사용자 전용 폴더에 설치합니다.", 0, 32, 65, 480, 22, 0)
	add("STATIC", "설치 위치", 0, 32, 105, 90, 22, 0)
	add("EDIT", filepath.Dir(want), 0x00800800, 32, 130, 495, 30, 0)
	progressHWND = add("msctls_progress32", "", 0, 32, 183, 495, 18, 0)
	sendMessage.Call(progressHWND, pbmSetRange32, 0, 100)
	statusHWND = add("STATIC", "설치 준비가 완료되었습니다.", 0, 32, 210, 495, 24, 0)
	installBtnHWND = add("BUTTON", "설치", 0x00000001, 337, 250, 90, 34, idInstall)
	cancelBtnHWND = add("BUTTON", "취소", 0, 437, 250, 90, 34, idCancel)

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
	_ = os.Remove(dst)
	return os.Rename(tmp, dst)
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
	script := "$d=[Environment]::GetFolderPath('Desktop');$t=Join-Path $env:LOCALAPPDATA 'Programs\\JTSN\\JTSN.exe';$wd=Split-Path -Parent $t;Remove-Item -LiteralPath (Join-Path $d '잡툴사니.lnk') -Force -ErrorAction SilentlyContinue;$w=New-Object -ComObject WScript.Shell;$s=$w.CreateShortcut((Join-Path $d 'JTSN.lnk'));$s.TargetPath=$t;$s.WorkingDirectory=$wd;$s.IconLocation=$t+',0';$s.Description='JTSN · 잡툴사니';$s.Save()"
	_ = runHidden("powershell.exe", "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-Command", script)
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
	_ = runHidden("reg.exe", "add", key, "/v", "DisplayIcon", "/t", "REG_SZ", "/d", target, "/f")
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
	return io.ReadAll(io.LimitReader(resp.Body, limit))
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
		if strings.HasSuffix(name, ".sha256") || strings.HasSuffix(name, ".sha256.txt") {
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

func install(r release) error {
	exe, err := request(r.EXE, 300<<20)
	if err != nil {
		return err
	}
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
	dir := filepath.Join(os.TempDir(), "JTSN-Update")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	p := filepath.Join(dir, "JTSN_"+strings.TrimPrefix(r.Tag, "v")+"_update.exe")
	if err := os.WriteFile(p, exe, 0755); err != nil {
		return err
	}
	self, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.Command(p, "--apply-update", self, strconv.Itoa(os.Getpid()))
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd.Start()
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
	b, err := embeddedCore.ReadFile("core/JTSN_v5.50.exe")
	if err != nil {
		message(err.Error(), 0x10)
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
