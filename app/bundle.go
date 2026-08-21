//go:build windows

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode"
	"unsafe"
)

const (
	ID_BUNDLE_ADD_FILES     = 7601
	ID_BUNDLE_ADD_FOLDER    = 7602
	ID_BUNDLE_SELECT_ALL    = 7603
	ID_BUNDLE_SELECT_NONE   = 7604
	ID_BUNDLE_REMOVE        = 7605
	ID_BUNDLE_PICK_DEST     = 7606
	ID_BUNDLE_SUGGEST       = 7607
	ID_BUNDLE_RUN           = 7608
	ID_BUNDLE_OPEN          = 7609
	ID_BUNDLE_COPY_PATH     = 7610
	ID_BUNDLE_REGISTER_MENU = 7611
)

type bundleEntry struct {
	Path  string
	IsDir bool
}
type bundleResult struct {
	Source, Destination string
	Error               string
}

var (
	bundleList         syscall.Handle
	bundleEntries      []bundleEntry
	bundleNameEdit     syscall.Handle
	bundleLocationEdit syscall.Handle
	bundleResultPath   string
	bundleResultMu     sync.Mutex
	bundleResults      []bundleResult
	bundleStartupPaths []string
)

func renderBundle() {
	toolHeader("선택파일 새 폴더로 묶기", "파일과 폴더를 선택한 뒤 새 폴더를 만들어 한 번에 이동합니다.")
	panelSmall("파일 또는 폴더를 아래 목록으로 끌어다 놓으세요.", 44, 112, 620, 22, true)
	panelButton("+ 파일 추가", 44, 140, 112, 36, ID_BUNDLE_ADD_FILES, BTN_PRIMARY)
	panelButton("+ 폴더 추가", 166, 140, 112, 36, ID_BUNDLE_ADD_FOLDER, BTN_SECONDARY)
	panelButton("전체 선택", 300, 140, 100, 36, ID_BUNDLE_SELECT_ALL, BTN_SECONDARY)
	panelButton("전체 해제", 410, 140, 100, 36, ID_BUNDLE_SELECT_NONE, BTN_SECONDARY)
	panelButton("목록 삭제", 520, 140, 100, 36, ID_BUNDLE_REMOVE, BTN_DANGER)
	bundleList = createWindow(0, "SysListView32", "", WS_CHILD|WS_VISIBLE|WS_TABSTOP|LVS_REPORT|LVS_SHOWSELALWAYS, 44, 188, 932, 300, mainHWND, 0)
	sendFont(bundleList, fontSmall)
	procSetWindowTheme.Call(uintptr(bundleList), uintptr(unsafe.Pointer(p16("Explorer"))), 0)
	procSendMessageW.Call(uintptr(bundleList), LVM_SETEXTENDEDLISTVIEWSTYLE, LVS_EX_FULLROWSELECT|LVS_EX_DOUBLEBUFFER|LVS_EX_CHECKBOXES, LVS_EX_FULLROWSELECT|LVS_EX_DOUBLEBUFFER|LVS_EX_CHECKBOXES)
	listViewAddColumn(bundleList, 0, "파일/폴더명", 300)
	listViewAddColumn(bundleList, 1, "현재 경로", 500)
	listViewAddColumn(bundleList, 2, "종류", 100)
	dynamicControls = append(dynamicControls, bundleList)
	panelSmall("새 폴더명", 44, 510, 100, 22, true)
	bundleNameEdit = panelEdit("", 44, 534, 350, 36, false, false, ID_EDIT_A)
	panelButton("이름 추천", 404, 534, 104, 36, ID_BUNDLE_SUGGEST, BTN_SECONDARY)
	panelSmall("생성 위치", 44, 578, 100, 22, true)
	bundleLocationEdit = panelEdit("", 44, 602, 700, 36, false, false, ID_EDIT_B)
	panelButton("위치 선택", 754, 602, 106, 36, ID_BUNDLE_PICK_DEST, BTN_SECONDARY)
	panelButton("새 폴더로 묶기", 870, 534, 106, 104, ID_BUNDLE_RUN, BTN_PRIMARY)
	panelButton("생성 폴더 열기", 44, 650, 132, 34, ID_BUNDLE_OPEN, BTN_SECONDARY)
	panelButton("경로 복사", 186, 650, 100, 34, ID_BUNDLE_COPY_PATH, BTN_SECONDARY)
	panelButton("탐색기 우클릭 메뉴 등록", 296, 650, 190, 34, ID_BUNDLE_REGISTER_MENU, BTN_SECONDARY)
	makeStatus("이동할 항목을 추가해 주세요. 체크된 항목만 이동합니다.")
	refreshBundleList()
	if len(bundleStartupPaths) > 0 {
		paths := append([]string(nil), bundleStartupPaths...)
		bundleStartupPaths = nil
		bundleAddPaths(paths)
	}
}

func bundleAddPaths(paths []string) {
	seen := map[string]bool{}
	for _, e := range bundleEntries {
		seen[strings.ToLower(filepath.Clean(e.Path))] = true
	}
	for _, p := range paths {
		p = filepath.Clean(p)
		st, err := os.Stat(p)
		if err == nil && !seen[strings.ToLower(p)] {
			bundleEntries = append(bundleEntries, bundleEntry{p, st.IsDir()})
			seen[strings.ToLower(p)] = true
		}
	}
	refreshBundleList()
	bundleAutoLocation()
}

func refreshBundleList() {
	if bundleList == 0 {
		return
	}
	listViewClear(bundleList)
	for i, e := range bundleEntries {
		kind := "파일"
		if e.IsDir {
			kind = "폴더"
		}
		listViewAddRow(bundleList, i, []string{filepath.Base(e.Path), filepath.Dir(e.Path), kind})
		bundleSetChecked(i, true)
	}
	setStatus(fmt.Sprintf("%d개 항목 · 체크된 항목만 이동합니다.", len(bundleEntries)))
}
func bundleSetChecked(i int, checked bool) {
	state := uint32(1 << 12)
	if checked {
		state = 2 << 12
	}
	it := LVITEMW{StateMask: LVIS_STATEIMAGEMASK, State: state, IItem: int32(i)}
	procSendMessageW.Call(uintptr(bundleList), LVM_SETITEMSTATE, uintptr(i), uintptr(unsafe.Pointer(&it)))
}
func bundleChecked(i int) bool {
	s, _, _ := procSendMessageW.Call(uintptr(bundleList), LVM_GETITEMSTATE, uintptr(i), LVIS_STATEIMAGEMASK)
	return ((s >> 12) & 0xf) == 2
}
func bundleSelectedEntries() []bundleEntry {
	out := []bundleEntry{}
	for i, e := range bundleEntries {
		if bundleChecked(i) {
			out = append(out, e)
		}
	}
	return out
}
func bundleAutoLocation() {
	entries := bundleSelectedEntries()
	if len(entries) == 0 {
		return
	}
	dir := filepath.Dir(entries[0].Path)
	for _, e := range entries[1:] {
		if !strings.EqualFold(dir, filepath.Dir(e.Path)) {
			setText(bundleLocationEdit, "")
			setStatus("선택 항목의 위치가 서로 다릅니다. 생성 위치를 직접 선택해 주세요.")
			return
		}
	}
	setText(bundleLocationEdit, dir)
}

func bundleMenu(items []string) int {
	m, _, _ := procCreatePopupMenu.Call()
	if m == 0 {
		return -1
	}
	defer procDestroyMenu.Call(m)
	for i, s := range items {
		procAppendMenuW.Call(m, MF_STRING, uintptr(7700+i), uintptr(unsafe.Pointer(p16(s))))
	}
	var pt POINT
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	r, _, _ := procTrackPopupMenu.Call(m, TPM_RETURNCMD|TPM_NONOTIFY, uintptr(pt.X), uintptr(pt.Y), 0, uintptr(mainHWND), 0)
	if r < 7700 || int(r-7700) >= len(items) {
		return -1
	}
	return int(r - 7700)
}

func commonBundleName(entries []bundleEntry) string {
	if len(entries) == 0 {
		return ""
	}
	prefix := strings.TrimSuffix(filepath.Base(entries[0].Path), filepath.Ext(entries[0].Path))
	for _, e := range entries[1:] {
		s := strings.TrimSuffix(filepath.Base(e.Path), filepath.Ext(e.Path))
		rr, ss := []rune(prefix), []rune(s)
		n := min(len(rr), len(ss))
		i := 0
		for i < n && unicode.ToLower(rr[i]) == unicode.ToLower(ss[i]) {
			i++
		}
		prefix = string(rr[:i])
		if prefix == "" {
			break
		}
	}
	return strings.Trim(prefix, " _-.()[]")
}

func handleBundleCommand(id int) {
	switch id {
	case ID_BUNDLE_ADD_FILES:
		bundleAddPaths(openFiles("묶을 파일 선택", "모든 파일\x00*.*\x00\x00"))
	case ID_BUNDLE_ADD_FOLDER:
		if p := pickFolder(); p != "" {
			bundleAddPaths([]string{p})
		}
	case ID_BUNDLE_SELECT_ALL:
		for i := range bundleEntries {
			bundleSetChecked(i, true)
		}
		bundleAutoLocation()
	case ID_BUNDLE_SELECT_NONE:
		for i := range bundleEntries {
			bundleSetChecked(i, false)
		}
		setText(bundleLocationEdit, "")
	case ID_BUNDLE_REMOVE:
		next := []bundleEntry{}
		for i, e := range bundleEntries {
			if !bundleChecked(i) {
				next = append(next, e)
			}
		}
		bundleEntries = next
		refreshBundleList()
		bundleAutoLocation()
	case ID_BUNDLE_PICK_DEST:
		if p := pickFolder(); p != "" {
			setText(bundleLocationEdit, p)
		}
	case ID_BUNDLE_SUGGEST:
		switch bundleMenu([]string{"현재 날짜", "선택 항목의 공통 이름", "직접 입력"}) {
		case 0:
			setText(bundleNameEdit, time.Now().Format("2006-01-02"))
		case 1:
			if s := commonBundleName(bundleSelectedEntries()); s != "" {
				setText(bundleNameEdit, s)
			} else {
				info("공통으로 사용할 파일명을 찾지 못했습니다.")
			}
		}
	case ID_BUNDLE_RUN:
		runBundleMove()
	case ID_BUNDLE_OPEN:
		if bundleResultPath != "" {
			_ = exec.Command("explorer.exe", bundleResultPath).Start()
		} else {
			info("먼저 새 폴더로 묶기를 실행해 주세요.")
		}
	case ID_BUNDLE_COPY_PATH:
		if bundleResultPath != "" {
			_ = copyClipboard(bundleResultPath)
			setStatus("생성된 폴더 경로를 복사했습니다.")
		} else {
			info("먼저 새 폴더로 묶기를 실행해 주세요.")
		}
	case ID_BUNDLE_REGISTER_MENU:
		registerBundleExplorerMenu()
	}
}

func registerBundleExplorerMenu() {
	exe, err := os.Executable()
	if err != nil {
		errorBox("실행 파일 경로를 확인할 수 없습니다.")
		return
	}
	command := fmt.Sprintf("\"%s\" --bundle-shell \"%%1\"", exe)
	keys := []string{`HKCU\Software\Classes\*\shell\JTSNBundle`, `HKCU\Software\Classes\Directory\shell\JTSNBundle`}
	for _, key := range keys {
		commands := [][]string{{"add", key, "/v", "MUIVerb", "/d", "JTSN 새 폴더에 넣기", "/f"}, {"add", key, "/v", "MultiSelectModel", "/d", "Player", "/f"}, {"add", key + `\command`, "/ve", "/d", command, "/f"}}
		for _, args := range commands {
			cmd := exec.Command("reg.exe", args...)
			cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
			if out, e := cmd.CombinedOutput(); e != nil {
				errorBox("탐색기 메뉴 등록 실패: " + e.Error() + "\n" + string(out))
				return
			}
		}
	}
	info("탐색기 우클릭 메뉴에 ‘JTSN 새 폴더에 넣기’를 등록했습니다.\n\nWindows 11에서는 ‘더 많은 옵션 표시’ 안에 나타날 수 있습니다.")
}

// runBundleShellImmediate is intentionally independent of the main window.
// Explorer context-menu launches therefore finish without creating another
// JTSN window or taskbar item.
func runBundleShellImmediate(paths []string) {
	paths = collectBundleShellInvocations(paths)
	if len(paths) == 0 {
		return
	}
	entries := make([]bundleEntry, 0, len(paths))
	seen := map[string]bool{}
	for _, path := range paths {
		path = filepath.Clean(strings.TrimSpace(path))
		st, err := os.Stat(path)
		key := strings.ToLower(path)
		if err != nil || seen[key] {
			continue
		}
		seen[key] = true
		entries = append(entries, bundleEntry{Path: path, IsDir: st.IsDir()})
	}
	if len(entries) == 0 {
		errorBox("이동할 파일이나 폴더를 찾을 수 없습니다.")
		return
	}

	base := filepath.Dir(entries[0].Path)
	dest := uniqueBundlePath(filepath.Join(base, "새 폴더"))
	if err := os.MkdirAll(dest, 0755); err != nil {
		errorBox("새 폴더를 만들지 못했습니다.\n\n" + err.Error())
		return
	}

	failures := make([]string, 0)
	for _, entry := range entries {
		target := filepath.Join(dest, filepath.Base(entry.Path))
		if _, err := os.Stat(target); err == nil {
			target = uniqueBundleItemPath(target, entry.IsDir)
		}
		if err := moveBundlePath(entry.Path, target); err != nil {
			failures = append(failures, filepath.Base(entry.Path)+": "+err.Error())
		}
	}
	if len(failures) > 0 {
		message := strings.Join(failures, "\n")
		if len([]rune(message)) > 1200 {
			message = string([]rune(message)[:1200]) + "\n…"
		}
		errorBox(fmt.Sprintf("%d개 항목을 이동하지 못했습니다.\n\n%s", len(failures), message))
	}
}

// Explorer starts a classic context-menu command once per selected item on
// some Windows versions. Each short-lived process writes its item to a queue;
// the first process collects the burst and performs one move operation.
func collectBundleShellInvocations(paths []string) []string {
	cache, err := os.UserCacheDir()
	if err != nil || cache == "" {
		cache = os.TempDir()
	}
	queueDir := filepath.Join(cache, "JTSN", "bundle-shell-queue")
	_ = os.MkdirAll(queueDir, 0755)
	payload, _ := json.Marshal(paths)
	requestPath := filepath.Join(queueDir, fmt.Sprintf("%d-%d.json", os.Getpid(), time.Now().UnixNano()))
	_ = os.WriteFile(requestPath, payload, 0600)

	mutex, _, mutexErr := procCreateMutexW.Call(0, 1, uintptr(unsafe.Pointer(p16("Local\\JTSN_Bundle_Shell_Batch_v1"))))
	if mutex != 0 {
		defer procCloseHandleMain.Call(mutex)
	}
	if mutexErr == syscall.Errno(183) {
		return nil
	}

	// Wait until Explorer's burst of per-item launches has gone quiet.
	lastCount := -1
	stableRounds := 0
	deadline := time.Now().Add(2500 * time.Millisecond)
	for time.Now().Before(deadline) && stableRounds < 3 {
		time.Sleep(180 * time.Millisecond)
		files, _ := filepath.Glob(filepath.Join(queueDir, "*.json"))
		if len(files) == lastCount {
			stableRounds++
		} else {
			lastCount = len(files)
			stableRounds = 0
		}
	}

	all := make([]string, 0)
	files, _ := filepath.Glob(filepath.Join(queueDir, "*.json"))
	for _, file := range files {
		info, statErr := os.Stat(file)
		if statErr != nil {
			continue
		}
		if time.Since(info.ModTime()) > 15*time.Second {
			_ = os.Remove(file)
			continue
		}
		data, readErr := os.ReadFile(file)
		if readErr == nil {
			var queued []string
			if json.Unmarshal(data, &queued) == nil {
				all = append(all, queued...)
			}
		}
		_ = os.Remove(file)
	}
	return all
}

func runBundleMove() {
	if busy {
		return
	}
	entries := bundleSelectedEntries()
	if len(entries) == 0 {
		info("이동할 항목을 체크해 주세요.")
		return
	}
	name := strings.TrimSpace(getText(bundleNameEdit))
	if name == "" || strings.ContainsAny(name, `<>:"/\\|?*`) {
		info("사용할 수 있는 새 폴더 이름을 입력해 주세요.")
		return
	}
	base := strings.TrimSpace(getText(bundleLocationEdit))
	if base == "" {
		bundleAutoLocation()
		base = strings.TrimSpace(getText(bundleLocationEdit))
	}
	if base == "" {
		info("새 폴더를 생성할 위치를 선택해 주세요.")
		return
	}
	dest := filepath.Join(base, name)
	if st, err := os.Stat(dest); err == nil && st.IsDir() {
		switch bundleMenu([]string{"기존 폴더에 넣기", "(1), (2)를 붙여 새 폴더 생성", "취소"}) {
		case 0:
		case 1:
			dest = uniqueBundlePath(dest)
		default:
			return
		}
	}
	policy := 0
	hasConflict := false
	for _, e := range entries {
		if _, err := os.Stat(filepath.Join(dest, filepath.Base(e.Path))); err == nil {
			hasConflict = true
			break
		}
	}
	if hasConflict {
		policy = bundleMenu([]string{"덮어쓰기 · 전체 적용", "건너뛰기 · 전체 적용", "자동 이름 변경 · 전체 적용", "취소"})
		if policy < 0 || policy == 3 {
			return
		}
	}
	if ask(fmt.Sprintf("체크한 %d개 항목을 다음 새 폴더로 이동할까요?\n\n%s", len(entries), dest)) != IDYES {
		return
	}
	startBusy("새 폴더를 만들고 선택 항목을 이동하고 있습니다...")
	go bundleMoveWorker(entries, dest, policy)
}

func uniqueBundlePath(path string) string {
	if _, e := os.Stat(path); os.IsNotExist(e) {
		return path
	}
	for i := 1; ; i++ {
		p := fmt.Sprintf("%s(%d)", path, i)
		if _, e := os.Stat(p); os.IsNotExist(e) {
			return p
		}
	}
}

func uniqueBundleItemPath(path string, isDir bool) string {
	if isDir {
		return uniqueBundlePath(path)
	}
	ext := filepath.Ext(path)
	base := strings.TrimSuffix(path, ext)
	for i := 1; ; i++ {
		p := fmt.Sprintf("%s(%d)%s", base, i, ext)
		if _, e := os.Stat(p); os.IsNotExist(e) {
			return p
		}
	}
}
func bundleMoveWorker(entries []bundleEntry, dest string, policy int) {
	_ = os.MkdirAll(dest, 0755)
	results := make([]bundleResult, 0, len(entries))
	for _, e := range entries {
		target := filepath.Join(dest, filepath.Base(e.Path))
		if strings.EqualFold(filepath.Clean(e.Path), filepath.Clean(target)) {
			results = append(results, bundleResult{e.Path, target, "원본과 대상이 같습니다"})
			continue
		}
		if _, err := os.Stat(target); err == nil {
			if policy == 1 {
				results = append(results, bundleResult{e.Path, target, "건너뜀"})
				continue
			}
			if policy == 2 {
				target = uniqueBundleItemPath(target, e.IsDir)
			} else {
				_ = os.RemoveAll(target)
			}
		}
		cleanSrc, cleanDest := strings.ToLower(filepath.Clean(e.Path)), strings.ToLower(filepath.Clean(dest))
		if e.IsDir && strings.HasPrefix(cleanDest+string(os.PathSeparator), cleanSrc+string(os.PathSeparator)) {
			results = append(results, bundleResult{e.Path, target, "대상 폴더를 원본 폴더 내부에 만들 수 없습니다"})
			continue
		}
		err := moveBundlePath(e.Path, target)
		br := bundleResult{Source: e.Path, Destination: target}
		if err != nil {
			br.Error = err.Error()
		}
		results = append(results, br)
	}
	bundleResultMu.Lock()
	bundleResults = results
	bundleResultPath = dest
	bundleResultMu.Unlock()
	procPostMessageW.Call(uintptr(mainHWND), WM_APP_BUNDLE_DONE, 0, 0)
}
func moveBundlePath(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	st, err := os.Stat(src)
	if err != nil {
		return err
	}
	if st.IsDir() {
		if err = copyBundleDir(src, dst); err == nil {
			err = os.RemoveAll(src)
		}
		return err
	}
	if err = copyBundleFile(src, dst, st.Mode()); err == nil {
		err = os.Remove(src)
	}
	return err
}
func copyBundleFile(src, dst string, mode os.FileMode) error {
	in, e := os.Open(src)
	if e != nil {
		return e
	}
	defer in.Close()
	out, e := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if e != nil {
		return e
	}
	_, e = io.Copy(out, in)
	ce := out.Close()
	if e == nil {
		e = ce
	}
	return e
}
func copyBundleDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, e := filepath.Rel(src, path)
		if e != nil {
			return e
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		return copyBundleFile(path, target, info.Mode())
	})
}

func bundleFinishUI() {
	bundleResultMu.Lock()
	results := append([]bundleResult(nil), bundleResults...)
	dest := bundleResultPath
	bundleResultMu.Unlock()
	finishBusy()
	ok, failed, skipped := 0, []string{}, 0
	moved := map[string]bool{}
	for _, r := range results {
		if r.Error == "" {
			ok++
			moved[strings.ToLower(filepath.Clean(r.Source))] = true
		} else if r.Error == "건너뜀" {
			skipped++
		} else {
			failed = append(failed, filepath.Base(r.Source)+" : "+r.Error)
		}
	}
	next := []bundleEntry{}
	for _, e := range bundleEntries {
		if !moved[strings.ToLower(filepath.Clean(e.Path))] {
			next = append(next, e)
		}
	}
	bundleEntries = next
	refreshBundleList()
	setStatus(fmt.Sprintf("완료 · 성공 %d개 / 건너뜀 %d개 / 실패 %d개 · %s", ok, skipped, len(failed), dest))
	if len(failed) > 0 {
		msg := "일부 항목을 이동하지 못했습니다.\n\n" + strings.Join(failed, "\n")
		if len([]rune(msg)) > 1800 {
			msg = string([]rune(msg)[:1800]) + "\n…"
		}
		info(msg)
	}
}
