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
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

const (
	launcherVersion = "5.53"
	releaseAPI      = "https://api.github.com/repos/gyeongseop97/JTSN/releases/latest"
	appFolderName   = "JTSN"
	installedName   = "JTSN.exe"
)

//go:embed core/JTSN_v5.50.exe
var embeddedCore embed.FS

type asset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
}

type release struct {
	Tag    string  `json:"tag_name"`
	Body   string  `json:"body"`
	Assets []asset `json:"assets"`
	EXE    string  `json:"-"`
	SUM    string  `json:"-"`
}

var client = &http.Client{Timeout: 5 * time.Minute}

func p16(s string) *uint16 { p, _ := syscall.UTF16PtrFromString(s); return p }

func message(text string, flags uintptr) uintptr {
	u := syscall.NewLazyDLL("user32.dll")
	r, _, _ := u.NewProc("MessageBoxW").Call(0, uintptr(unsafe.Pointer(p16(text))), uintptr(unsafe.Pointer(p16("JTSN · 잡툴사니 업데이트"))), flags)
	return r
}

func main() {
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
	if err := os.MkdirAll(filepath.Dir(want), 0755); err != nil {
		message("프로그램 설치 폴더를 만들지 못했습니다.\n\n"+err.Error(), 0x10)
		return false
	}
	if err := copyFile(self, want); err != nil {
		message("잡툴사니 설치에 실패했습니다.\n\n"+err.Error(), 0x10)
		return false
	}
	registerUninstall(want)
	createDesktopShortcut(want)
	cmd := exec.Command(want, "--cleanup-source", self, strconv.Itoa(os.Getpid()))
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := cmd.Start(); err != nil {
		message("설치는 완료됐지만 자동 실행에 실패했습니다. 바탕화면의 잡툴사니 바로가기를 실행해 주세요.", 0x40)
	}
	return false
}

func runHidden(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd.Run()
}

func psQuote(s string) string { return strings.ReplaceAll(s, "'", "''") }

func createDesktopShortcut(target string) {
	icon := target + ",0"
	script := "$d=[Environment]::GetFolderPath('Desktop');$w=New-Object -ComObject WScript.Shell;$s=$w.CreateShortcut((Join-Path $d '잡툴사니.lnk'));$s.TargetPath='" + psQuote(target) + "';$s.WorkingDirectory='" + psQuote(filepath.Dir(target)) + "';$s.IconLocation='" + psQuote(icon) + "';$s.Description='잡툴사니 (JTSN)';$s.Save()"
	_ = runHidden("powershell.exe", "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-Command", script)
}

func removeDesktopShortcut() {
	script := "$p=Join-Path ([Environment]::GetFolderPath('Desktop')) '잡툴사니.lnk';Remove-Item -LiteralPath $p -Force -ErrorAction SilentlyContinue"
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
	_ = os.Remove(source)
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
		}
		if strings.HasSuffix(name, ".sha256") || strings.HasSuffix(name, ".sha256.txt") {
			r.SUM = a.URL
		}
	}
	if r.EXE == "" || r.SUM == "" {
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
	sum, err := request(r.SUM, 1<<20)
	if err != nil {
		return err
	}
	f := strings.Fields(string(sum))
	if len(f) == 0 {
		return fmt.Errorf("체크섬이 없습니다")
	}
	want := strings.ToLower(f[0])
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
	// The preserved v5.50 core uses fixed-width version strings. Replacing the
	// equal-length version token keeps its PE layout intact while making the
	// installed app and first-run patch-note state follow the launcher version.
	b = bytes.ReplaceAll(b, []byte("5.50"), []byte(launcherVersion))
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
