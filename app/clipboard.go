//go:build windows

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"
)

const (
	WM_CLIPBOARDUPDATE        = 0x031D
	WM_HOTKEY                 = 0x0312
	CF_DIB                    = 8
	GMEM_MOVEABLE_CLIP        = 0x0002
	LVM_SETIMAGELIST_CLIP     = 0x1003
	LVM_SETITEMW_CLIP         = 0x104C
	LVM_SETCOLUMNW_CLIP       = 0x1060
	LVSIL_SMALL_CLIP          = 1
	LVIF_IMAGE_CLIP           = 0x0002
	LVS_EX_SUBITEMIMAGES_CLIP = 0x00000002

	ID_CLIP_SEARCH       = 7100
	ID_CLIP_FILTER_ALL   = 7101
	ID_CLIP_FILTER_TEXT  = 7102
	ID_CLIP_FILTER_IMAGE = 7103
	ID_CLIP_FILTER_URL   = 7104
	ID_CLIP_FILTER_EMAIL = 7105
	ID_CLIP_FILTER_PATH  = 7106
	ID_CLIP_FILTER_STAR  = 7107
	ID_CLIP_TOGGLE       = 7110
	ID_CLIP_DELETE       = 7111
	ID_CLIP_SETTINGS     = 7112
	ID_CLIP_COPY         = 7114
	ID_CLIP_DETAIL       = 7116
	ID_TIMER_CLIPBOARD   = 7191
	ID_TIMER_CLIP_CONFIG = 7192
	EN_CHANGE_CLIP       = 0x0300
)

type clipboardConfig struct {
	MaxItems         int      `json:"max_items"`
	RetentionDays    int      `json:"retention_days"`
	Persist          bool     `json:"persist"`
	CaptureImages    bool     `json:"capture_images"`
	RemoveDuplicate  bool     `json:"remove_duplicate"`
	Paused           bool     `json:"paused"`
	ExcludedApps     []string `json:"excluded_apps,omitempty"`
	ExcludedPatterns []string `json:"excluded_patterns,omitempty"`
	ExcludePasswords bool     `json:"exclude_passwords"`
}

type nmCustomDrawClip struct {
	Hdr        NMHDR
	DrawStage  uint32
	Hdc        syscall.Handle
	Rc         RECT
	ItemSpec   uintptr
	ItemState  uint32
	ItemLParam uintptr
}

type nmListViewCustomDrawClip struct {
	Nmcd      nmCustomDrawClip
	ClrText   uintptr
	ClrTextBk uintptr
	ISubItem  int32
}

type guiThreadInfoClip struct {
	CbSize                                                                     uint32
	Flags                                                                      uint32
	HwndActive, HwndFocus, HwndCapture, HwndMenuOwner, HwndMoveSize, HwndCaret syscall.Handle
	RcCaret                                                                    RECT
}

type clipboardRecord struct {
	ID       string    `json:"id"`
	Kind     string    `json:"kind"`
	Text     string    `json:"text,omitempty"`
	Image    string    `json:"image,omitempty"`
	Width    int       `json:"width,omitempty"`
	Height   int       `json:"height,omitempty"`
	Created  time.Time `json:"created"`
	Favorite bool      `json:"favorite"`
	Hash     string    `json:"hash"`
	Source   string    `json:"source,omitempty"`
}

var (
	clipMu            sync.Mutex
	clipConfig        clipboardConfig
	clipRecords       []clipboardRecord
	clipLoaded        bool
	clipSearch        syscall.Handle
	clipList          syscall.Handle
	clipFiltered      []clipboardRecord
	clipFilter        = "all"
	clipLastMod       time.Time
	clipConfigLastMod time.Time
	clipCaptureBusy   bool
	clipImageList     uintptr
	clipToggleButton  syscall.Handle
	clipNotifyResult  uintptr

	clipUser32                        = syscall.NewLazyDLL("user32.dll")
	clipKernel32                      = syscall.NewLazyDLL("kernel32.dll")
	procAddClipboardFormatListener    = clipUser32.NewProc("AddClipboardFormatListener")
	procRemoveClipboardFormatListener = clipUser32.NewProc("RemoveClipboardFormatListener")
	procGetClipboardDataClip          = clipUser32.NewProc("GetClipboardData")
	procIsClipboardFormatAvailable    = clipUser32.NewProc("IsClipboardFormatAvailable")
	procGlobalLockClip                = clipKernel32.NewProc("GlobalLock")
	procGlobalUnlockClip              = clipKernel32.NewProc("GlobalUnlock")
	procGlobalSizeClip                = clipKernel32.NewProc("GlobalSize")
	procGlobalAllocClip               = clipKernel32.NewProc("GlobalAlloc")
	procCreateDIBitmapClip            = gdi32.NewProc("CreateDIBitmap")
	procImageListCreateClip           = comctl32.NewProc("ImageList_Create")
	procImageListAddClip              = comctl32.NewProc("ImageList_Add")
	procImageListDestroyClip          = comctl32.NewProc("ImageList_Destroy")
	procGetForegroundWindowClip       = clipUser32.NewProc("GetForegroundWindow")
	procGetWindowThreadProcessIDClip  = clipUser32.NewProc("GetWindowThreadProcessId")
	procGetGUIThreadInfoClip          = clipUser32.NewProc("GetGUIThreadInfo")
	procGetWindowLongPtrClip          = clipUser32.NewProc("GetWindowLongPtrW")
	procOpenProcessClip               = clipKernel32.NewProc("OpenProcess")
	procQueryFullProcessImageNameClip = clipKernel32.NewProc("QueryFullProcessImageNameW")
	procCloseHandleClip               = clipKernel32.NewProc("CloseHandle")
)

func clipboardDir() string {
	base, err := os.UserCacheDir()
	if err != nil {
		base = os.TempDir()
	}
	return filepath.Join(base, "JTSN", "clipboard")
}
func clipboardRecordsPath() string { return filepath.Join(clipboardDir(), "records.json") }
func clipboardConfigPath() string  { return filepath.Join(clipboardDir(), "settings.json") }

func defaultClipboardConfig() clipboardConfig {
	return clipboardConfig{MaxItems: 300, RetentionDays: 30, Persist: true, CaptureImages: true, RemoveDuplicate: true, ExcludePasswords: true}
}

func loadClipboardData() {
	clipMu.Lock()
	defer clipMu.Unlock()
	if clipLoaded {
		return
	}
	clipConfig = defaultClipboardConfig()
	_ = os.MkdirAll(clipboardDir(), 0700)
	if b, err := os.ReadFile(clipboardConfigPath()); err == nil {
		_ = json.Unmarshal(b, &clipConfig)
	}
	if clipConfig.MaxItems < 20 {
		clipConfig.MaxItems = 300
	}
	if clipConfig.RetentionDays < 1 {
		clipConfig.RetentionDays = 30
	}
	if !clipConfig.Persist && launchMode == "" {
		_ = os.Remove(clipboardRecordsPath())
	} else if b, err := os.ReadFile(clipboardRecordsPath()); err == nil {
		_ = json.Unmarshal(b, &clipRecords)
	}
	pruneClipboardLocked()
	clipLoaded = true
}

func saveClipboardConfig() {
	clipMu.Lock()
	defer clipMu.Unlock()
	_ = os.MkdirAll(clipboardDir(), 0700)
	b, _ := json.MarshalIndent(clipConfig, "", "  ")
	_ = os.WriteFile(clipboardConfigPath(), b, 0600)
	if st, err := os.Stat(clipboardConfigPath()); err == nil {
		clipConfigLastMod = st.ModTime()
	}
}

func saveClipboardRecords() {
	clipMu.Lock()
	defer clipMu.Unlock()
	saveClipboardRecordsLocked()
}
func saveClipboardRecordsLocked() {
	_ = os.MkdirAll(clipboardDir(), 0700)
	b, _ := json.Marshal(clipRecords)
	tmp := clipboardRecordsPath() + ".tmp"
	if os.WriteFile(tmp, b, 0600) == nil {
		_ = os.Rename(tmp, clipboardRecordsPath())
		if st, err := os.Stat(clipboardRecordsPath()); err == nil {
			clipLastMod = st.ModTime()
		}
	}
}

func initClipboardMonitor(hwnd syscall.Handle) {
	loadClipboardData()
	procAddClipboardFormatListener.Call(uintptr(hwnd))
	procSetTimer.Call(uintptr(hwnd), ID_TIMER_CLIP_CONFIG, 700, 0)
}
func shutdownClipboardMonitor(hwnd syscall.Handle) {
	procKillTimer.Call(uintptr(hwnd), ID_TIMER_CLIP_CONFIG)
	procRemoveClipboardFormatListener.Call(uintptr(hwnd))
	clipMu.Lock()
	if !clipConfig.Persist {
		for _, r := range clipRecords {
			if r.Image != "" {
				_ = os.Remove(filepath.Join(clipboardDir(), r.Image))
			}
		}
		clipRecords = nil
		_ = os.Remove(clipboardRecordsPath())
	} else {
		saveClipboardRecordsLocked()
	}
	clipMu.Unlock()
}

func clipboardChanged(hwnd syscall.Handle) {
	loadClipboardData()
	reloadClipboardSharedState()
	clipMu.Lock()
	if clipConfig.Paused || clipCaptureBusy {
		clipMu.Unlock()
		return
	}
	clipCaptureBusy = true
	clipMu.Unlock()
	source, passwordField := clipboardForegroundProcess()
	text, dib, w, h := readClipboardSnapshot(hwnd)
	clipMu.Lock()
	clipCaptureBusy = false
	cfg := clipConfig
	excludedApp := false
	for _, app := range cfg.ExcludedApps {
		if strings.EqualFold(app, source) {
			excludedApp = true
			break
		}
	}
	clipMu.Unlock()
	if excludedApp || (passwordField && cfg.ExcludePasswords) || (text == "" && (len(dib) == 0 || !cfg.CaptureImages)) {
		return
	}
	go func() {
		if text != "" {
			addClipboardText(text, source)
		} else {
			addClipboardImage(dib, w, h, source)
		}
	}()
}

func reloadClipboardSharedState() {
	clipMu.Lock()
	defer clipMu.Unlock()
	cfg := defaultClipboardConfig()
	if b, e := os.ReadFile(clipboardConfigPath()); e == nil {
		_ = json.Unmarshal(b, &cfg)
	}
	if cfg.MaxItems < 20 {
		cfg.MaxItems = 300
	}
	if cfg.RetentionDays < 1 {
		cfg.RetentionDays = 30
	}
	clipConfig = cfg
	if b, e := os.ReadFile(clipboardRecordsPath()); e == nil {
		var rs []clipboardRecord
		if json.Unmarshal(b, &rs) == nil {
			clipRecords = rs
		}
	}
}

func reloadClipboardConfigOnly() {
	clipMu.Lock()
	defer clipMu.Unlock()
	cfg := defaultClipboardConfig()
	if b, e := os.ReadFile(clipboardConfigPath()); e == nil {
		_ = json.Unmarshal(b, &cfg)
	}
	if cfg.MaxItems < 20 {
		cfg.MaxItems = 300
	}
	if cfg.RetentionDays < 1 {
		cfg.RetentionDays = 30
	}
	clipConfig = cfg
}

func clipboardForegroundProcess() (string, bool) {
	hwnd, _, _ := procGetForegroundWindowClip.Call()
	if hwnd == 0 {
		return "", false
	}
	var pid uint32
	tid, _, _ := procGetWindowThreadProcessIDClip.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	if pid == 0 {
		return "", false
	}
	password := false
	gi := guiThreadInfoClip{CbSize: uint32(unsafe.Sizeof(guiThreadInfoClip{}))}
	if ok, _, _ := procGetGUIThreadInfoClip.Call(tid, uintptr(unsafe.Pointer(&gi))); ok != 0 && gi.HwndFocus != 0 {
		style, _, _ := procGetWindowLongPtrClip.Call(uintptr(gi.HwndFocus), ^uintptr(15))
		password = (style & 0x20) != 0
	}
	h, _, _ := procOpenProcessClip.Call(0x1000, 0, uintptr(pid))
	if h == 0 {
		return "", password
	}
	defer procCloseHandleClip.Call(h)
	buf := make([]uint16, 1024)
	n := uint32(len(buf))
	ok, _, _ := procQueryFullProcessImageNameClip.Call(h, 0, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&n)))
	if ok == 0 {
		return "", password
	}
	return strings.ToLower(filepath.Base(syscall.UTF16ToString(buf[:n]))), password
}

func readClipboardSnapshot(hwnd syscall.Handle) (string, []byte, int, int) {
	if r, _, _ := procOpenClipboard.Call(uintptr(hwnd)); r == 0 {
		return "", nil, 0, 0
	}
	defer procCloseClipboard.Call()
	if ok, _, _ := procIsClipboardFormatAvailable.Call(CF_UNICODETEXT); ok != 0 {
		h, _, _ := procGetClipboardDataClip.Call(CF_UNICODETEXT)
		if h != 0 {
			p, _, _ := procGlobalLockClip.Call(h)
			if p != 0 {
				sz, _, _ := procGlobalSizeClip.Call(h)
				u := unsafe.Slice((*uint16)(unsafe.Pointer(p)), int(sz/2))
				n := 0
				for n < len(u) && u[n] != 0 {
					n++
				}
				text := string(utf16.Decode(u[:n]))
				procGlobalUnlockClip.Call(h)
				return text, nil, 0, 0
			}
		}
	}
	if ok, _, _ := procIsClipboardFormatAvailable.Call(CF_DIB); ok != 0 {
		h, _, _ := procGetClipboardDataClip.Call(CF_DIB)
		if h != 0 {
			p, _, _ := procGlobalLockClip.Call(h)
			sz, _, _ := procGlobalSizeClip.Call(h)
			if p != 0 && sz > 40 {
				b := append([]byte(nil), unsafe.Slice((*byte)(unsafe.Pointer(p)), int(sz))...)
				procGlobalUnlockClip.Call(h)
				w := int(*(*int32)(unsafe.Pointer(&b[4])))
				hgt := int(*(*int32)(unsafe.Pointer(&b[8])))
				if hgt < 0 {
					hgt = -hgt
				}
				return "", b, w, hgt
			}
		}
	}
	return "", nil, 0, 0
}

var urlRE = regexp.MustCompile(`(?i)^https?://\S+$`)
var emailRE = regexp.MustCompile(`(?i)^[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,}$`)
var pathRE = regexp.MustCompile(`(?i)^(?:[a-z]:\\|\\\\|/)[^\r\n]+$`)

func classifyClipboardText(s string) string {
	t := strings.TrimSpace(s)
	if urlRE.MatchString(t) {
		return "url"
	}
	if emailRE.MatchString(t) {
		return "email"
	}
	if pathRE.MatchString(t) {
		return "path"
	}
	return "text"
}
func clipHash(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func excludedClipboardText(s string) bool {
	for _, p := range clipConfig.ExcludedPatterns {
		if p != "" && strings.Contains(strings.ToLower(s), strings.ToLower(p)) {
			return true
		}
	}
	return false
}
func addClipboardText(text, source string) {
	text = strings.ReplaceAll(text, "\x00", "")
	if strings.TrimSpace(text) == "" {
		return
	}
	h := clipHash([]byte(text))
	clipMu.Lock()
	defer clipMu.Unlock()
	if excludedClipboardText(text) {
		return
	}
	addClipboardRecordLocked(clipboardRecord{ID: fmt.Sprintf("%d", time.Now().UnixNano()), Kind: classifyClipboardText(text), Text: text, Created: time.Now(), Hash: h, Source: source})
}
func addClipboardImage(dib []byte, w, h int, source string) {
	if len(dib) == 0 {
		return
	}
	hash := clipHash(dib)
	clipMu.Lock()
	defer clipMu.Unlock()
	name := hash[:20] + ".dib"
	_ = os.MkdirAll(clipboardDir(), 0700)
	if os.WriteFile(filepath.Join(clipboardDir(), name), dib, 0600) != nil {
		return
	}
	addClipboardRecordLocked(clipboardRecord{ID: fmt.Sprintf("%d", time.Now().UnixNano()), Kind: "image", Image: name, Width: w, Height: h, Created: time.Now(), Hash: hash, Source: source})
}
func addClipboardRecordLocked(rec clipboardRecord) {
	if clipConfig.RemoveDuplicate {
		for i, r := range clipRecords {
			if r.Hash == rec.Hash {
				rec.Favorite = r.Favorite
				clipRecords = append(clipRecords[:i], clipRecords[i+1:]...)
				break
			}
		}
	}
	clipRecords = append([]clipboardRecord{rec}, clipRecords...)
	pruneClipboardLocked()
	saveClipboardRecordsLocked()
}
func pruneClipboardLocked() {
	cut := time.Now().AddDate(0, 0, -clipConfig.RetentionDays)
	keep := make([]clipboardRecord, 0, len(clipRecords))
	for _, r := range clipRecords {
		if !r.Favorite && (r.Created.Before(cut) || len(keep) >= clipConfig.MaxItems) {
			if r.Image != "" {
				_ = os.Remove(filepath.Join(clipboardDir(), r.Image))
			}
			continue
		}
		keep = append(keep, r)
	}
	clipRecords = keep
}

func renderClipboard() {
	loadClipboardData()
	toolHeader("고급 클립보드", "복사 기록을 검색하고 즐겨찾기로 보관한 뒤 다시 사용할 수 있습니다. 모든 데이터는 이 PC에만 저장됩니다.")
	panelLabel("검색", 44, 112, 70, 26, false)
	clipSearch = panelEdit("", 104, 106, 520, 36, false, false, ID_CLIP_SEARCH)
	x := 44
	for _, b := range []struct {
		t     string
		id, w int
	}{{"전체", ID_CLIP_FILTER_ALL, 66}, {"텍스트", ID_CLIP_FILTER_TEXT, 74}, {"이미지", ID_CLIP_FILTER_IMAGE, 74}, {"URL", ID_CLIP_FILTER_URL, 62}, {"이메일", ID_CLIP_FILTER_EMAIL, 74}, {"경로", ID_CLIP_FILTER_PATH, 66}, {"★", ID_CLIP_FILTER_STAR, 52}} {
		panelButton(b.t, x, 154, b.w, 34, b.id, BTN_SECONDARY)
		x += b.w + 8
	}
	clipList = createClipboardList(44, 202, 932, 382)
	clipToggleButton = panelButton("기록 ON", 44, 600, 120, 38, ID_CLIP_TOGGLE, BTN_PRIMARY)
	panelButton("전체 삭제", 174, 600, 104, 38, ID_CLIP_DELETE, BTN_DANGER)
	panelButton("설정", 288, 600, 84, 38, ID_CLIP_SETTINGS, BTN_SECONDARY)
	panelButton("전체 보기", 684, 600, 100, 38, ID_CLIP_DETAIL, BTN_SECONDARY)
	panelButton("복사", 794, 600, 126, 38, ID_CLIP_COPY, BTN_PRIMARY)
	makeStatus("왼쪽 별을 누르면 즐겨찾기, 오른쪽 삭제를 누르면 해당 기록만 삭제됩니다.")
	refreshClipboardList()
	procSetTimer.Call(uintptr(mainHWND), ID_TIMER_CLIPBOARD, 700, 0)
}

func shutdownClipboardTool() {
	procKillTimer.Call(uintptr(mainHWND), ID_TIMER_CLIPBOARD)
	if clipImageList != 0 {
		procImageListDestroyClip.Call(clipImageList)
		clipImageList = 0
	}
}

func createClipboardList(x, y, w, h int) syscall.Handle {
	style := uint32(WS_CHILD | WS_VISIBLE | WS_TABSTOP | LVS_REPORT | LVS_SHOWSELALWAYS)
	hnd := createWindow(0, "SysListView32", "", style, x+1, y+1, w-2, h-2, mainHWND, 0)
	inputFrames = append(inputFrames, inputFrame{Hwnd: hnd, Rect: RECT{int32(x), int32(y), int32(x + w), int32(y + h)}})
	sendFont(hnd, fontSmall)
	procSetWindowTheme.Call(uintptr(hnd), uintptr(unsafe.Pointer(p16("Explorer"))), 0)
	procSendMessageW.Call(uintptr(hnd), LVM_SETEXTENDEDLISTVIEWSTYLE, uintptr(LVS_EX_FULLROWSELECT|LVS_EX_DOUBLEBUFFER|LVS_EX_SUBITEMIMAGES_CLIP), uintptr(LVS_EX_FULLROWSELECT|LVS_EX_DOUBLEBUFFER|LVS_EX_SUBITEMIMAGES_CLIP))
	// Windows reserves the thumbnail slot in the first column. This width keeps
	// the favorite marker visible instead of letting that reserved slot cover it.
	listViewAddColumn(hnd, 0, "★", 76)
	centerColumn := LVCOLUMNW{Mask: LVCF_FMT | LVCF_SUBITEM, Fmt: 2, ISubItem: 0}
	procSendMessageW.Call(uintptr(hnd), LVM_SETCOLUMNW_CLIP, 0, uintptr(unsafe.Pointer(&centerColumn)))
	listViewAddColumn(hnd, 1, "내용 미리보기", 458)
	listViewAddColumn(hnd, 2, "유형", 90)
	listViewAddColumn(hnd, 3, "복사한 시간", 170)
	listViewAddColumn(hnd, 4, "삭제", 74)
	dynamicControls = append(dynamicControls, hnd)
	return hnd
}
func clipboardPreview(r clipboardRecord) string {
	if r.Kind == "image" {
		return fmt.Sprintf("이미지 썸네일 · %d × %d", r.Width, r.Height)
	}
	s := strings.Join(strings.Fields(r.Text), " ")
	rr := []rune(s)
	if len(rr) > 110 {
		s = string(rr[:110]) + "…"
	}
	return s
}
func clipTypeName(k string) string {
	return map[string]string{"text": "텍스트", "image": "이미지", "url": "URL", "email": "이메일", "path": "경로"}[k]
}
func refreshClipboardList() {
	if clipList == 0 {
		return
	}
	q := strings.ToLower(strings.TrimSpace(getText(clipSearch)))
	clipMu.Lock()
	src := append([]clipboardRecord(nil), clipRecords...)
	clipMu.Unlock()
	filtered := make([]clipboardRecord, 0, len(src))
	for _, r := range src {
		if clipFilter == "favorite" && !r.Favorite {
			continue
		}
		if clipFilter != "all" && clipFilter != "favorite" && r.Kind != clipFilter {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(r.Text), q) {
			continue
		}
		filtered = append(filtered, r)
	}
	clipFiltered = filtered
	newImages, _, _ := procImageListCreateClip.Call(48, 48, 0x20|0x01, uintptr(max(1, len(filtered))), 8)
	oldImages := clipImageList
	clipImageList = newImages
	procSendMessageW.Call(uintptr(clipList), LVM_SETIMAGELIST_CLIP, LVSIL_SMALL_CLIP, newImages)
	procSendMessageW.Call(uintptr(clipList), WM_SETREDRAW, 0, 0)
	listViewClear(clipList)
	for i, r := range filtered {
		star := ""
		if r.Favorite {
			star = "★"
		}
		if star == "" {
			star = "☆"
		}
		listViewAddRow(clipList, i, []string{star, clipboardPreview(r), clipTypeName(r.Kind), r.Created.Local().Format("2006-01-02 15:04:05"), "삭제"})
		// A ListView with an attached image list defaults newly inserted rows to
		// image index 0 in the first column. Explicitly clear it so column zero
		// always contains only the favorite marker.
		favoriteCell := LVITEMW{Mask: LVIF_IMAGE_CLIP, IItem: int32(i), ISubItem: 0, IImage: -1}
		procSendMessageW.Call(uintptr(clipList), LVM_SETITEMW_CLIP, 0, uintptr(unsafe.Pointer(&favoriteCell)))
		imageIndex := -1
		if r.Kind == "image" {
			if hb := clipboardThumbnail(r); hb != 0 {
				idx, _, _ := procImageListAddClip.Call(newImages, hb, 0)
				procDeleteObject.Call(hb)
				imageIndex = int(int32(idx))
			}
		}
		if imageIndex >= 0 {
			// Keep the first column exclusively for ☆/★. Windows otherwise places
			// every list image in column zero and visually covers the favorite mark.
			preview := clipboardPreview(r)
			it := LVITEMW{Mask: LVIF_IMAGE_CLIP | LVIF_TEXT, IItem: int32(i), ISubItem: 1, IImage: int32(imageIndex), PszText: p16(preview)}
			procSendMessageW.Call(uintptr(clipList), LVM_SETITEMW_CLIP, 0, uintptr(unsafe.Pointer(&it)))
		}
	}
	procSendMessageW.Call(uintptr(clipList), WM_SETREDRAW, 1, 0)
	procInvalidateRect.Call(uintptr(clipList), 0, 0)
	if oldImages != 0 {
		procImageListDestroyClip.Call(oldImages)
	}
	setStatus(fmt.Sprintf("%d개 기록 표시 · 기록 %s", len(filtered), map[bool]string{true: "OFF", false: "ON"}[clipConfig.Paused]))
	updateClipboardToggleButton()
}

func updateClipboardToggleButton() {
	if clipToggleButton == 0 {
		return
	}
	text, kind := "기록 ON", BTN_PRIMARY
	if clipConfig.Paused {
		text, kind = "기록 OFF", BTN_DANGER
	}
	changed := getText(clipToggleButton) != text || buttonKinds[clipToggleButton] != kind
	if getText(clipToggleButton) != text {
		setText(clipToggleButton, text)
	}
	buttonKinds[clipToggleButton] = kind
	if changed {
		procInvalidateRect.Call(uintptr(clipToggleButton), 0, 0)
	}
}

func clipboardThumbnail(r clipboardRecord) uintptr {
	b, err := os.ReadFile(filepath.Join(clipboardDir(), r.Image))
	if err != nil || len(b) < 40 || r.Width <= 0 || r.Height <= 0 {
		return 0
	}
	headerSize := int(*(*uint32)(unsafe.Pointer(&b[0])))
	if headerSize < 40 || headerSize > len(b) {
		return 0
	}
	bitCount := int(*(*uint16)(unsafe.Pointer(&b[14])))
	compression := *(*uint32)(unsafe.Pointer(&b[16]))
	clrUsed := int(*(*uint32)(unsafe.Pointer(&b[32])))
	if clrUsed == 0 && bitCount > 0 && bitCount <= 8 {
		clrUsed = 1 << bitCount
	}
	offset := headerSize + clrUsed*4
	if compression == 3 && headerSize == 40 {
		offset += 12
	}
	if offset >= len(b) {
		return 0
	}
	hdc, _, _ := procGetDC.Call(0)
	if hdc == 0 {
		return 0
	}
	defer procReleaseDC.Call(0, hdc)
	src, _, _ := procCreateDIBitmapClip.Call(hdc, uintptr(unsafe.Pointer(&b[0])), 4, uintptr(unsafe.Pointer(&b[offset])), uintptr(unsafe.Pointer(&b[0])), 0)
	if src == 0 {
		return 0
	}
	defer procDeleteObject.Call(src)
	srcDC, _, _ := procCreateCompatibleDC.Call(hdc)
	dstDC, _, _ := procCreateCompatibleDC.Call(hdc)
	if srcDC == 0 || dstDC == 0 {
		if srcDC != 0 {
			procDeleteDC.Call(srcDC)
		}
		if dstDC != 0 {
			procDeleteDC.Call(dstDC)
		}
		return 0
	}
	defer procDeleteDC.Call(srcDC)
	defer procDeleteDC.Call(dstDC)
	dst, _, _ := procCreateCompatibleBitmap.Call(hdc, 48, 48)
	if dst == 0 {
		return 0
	}
	oldSrc, _, _ := procSelectObject.Call(srcDC, src)
	oldDst, _, _ := procSelectObject.Call(dstDC, dst)
	// Preserve aspect ratio and center the thumbnail on a white 48px tile.
	rc := RECT{0, 0, 48, 48}
	procFillRect.Call(dstDC, uintptr(unsafe.Pointer(&rc)), uintptr(brushPanel))
	tw, th := 48, 48
	if r.Width > r.Height {
		th = max(1, 48*r.Height/r.Width)
	} else {
		tw = max(1, 48*r.Width/r.Height)
	}
	procStretchBlt.Call(dstDC, uintptr((48-tw)/2), uintptr((48-th)/2), uintptr(tw), uintptr(th), srcDC, 0, 0, uintptr(r.Width), uintptr(r.Height), SRCCOPY)
	procSelectObject.Call(srcDC, oldSrc)
	procSelectObject.Call(dstDC, oldDst)
	return dst
}

func selectedClipboardIndex() int {
	const LVM_GETNEXTITEM = 0x100C
	const LVNI_SELECTED = 0x0002
	r, _, _ := procSendMessageW.Call(uintptr(clipList), LVM_GETNEXTITEM, ^uintptr(0), LVNI_SELECTED)
	if int32(r) < 0 || int(r) >= len(clipFiltered) {
		return -1
	}
	return int(r)
}
func selectedClipboardRecord() (clipboardRecord, bool) {
	i := selectedClipboardIndex()
	if i < 0 {
		return clipboardRecord{}, false
	}
	return clipFiltered[i], true
}

func handleClipboardCommand(id, notify int) bool {
	if id == ID_CLIP_SEARCH && notify == EN_CHANGE_CLIP {
		refreshClipboardList()
		return true
	}
	filters := map[int]string{ID_CLIP_FILTER_ALL: "all", ID_CLIP_FILTER_TEXT: "text", ID_CLIP_FILTER_IMAGE: "image", ID_CLIP_FILTER_URL: "url", ID_CLIP_FILTER_EMAIL: "email", ID_CLIP_FILTER_PATH: "path", ID_CLIP_FILTER_STAR: "favorite"}
	if f, ok := filters[id]; ok {
		clipFilter = f
		refreshClipboardList()
		return true
	}
	switch id {
	case ID_CLIP_TOGGLE:
		clipMu.Lock()
		clipConfig.Paused = !clipConfig.Paused
		clipMu.Unlock()
		saveClipboardConfig()
		refreshClipboardList()
		return true
	case ID_CLIP_DELETE:
		menu, _, _ := procCreatePopupMenu.Call()
		if menu != 0 {
			defer procDestroyMenu.Call(menu)
			procAppendMenuW.Call(menu, MF_STRING, 7220, uintptr(unsafe.Pointer(p16("전체 삭제"))))
			procAppendMenuW.Call(menu, MF_STRING, 7221, uintptr(unsafe.Pointer(p16("즐겨찾기 제외 삭제"))))
			procAppendMenuW.Call(menu, MF_STRING, 7222, uintptr(unsafe.Pointer(p16("취소"))))
			var pt POINT
			procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
			cmd, _, _ := procTrackPopupMenu.Call(menu, TPM_RETURNCMD|TPM_NONOTIFY, uintptr(pt.X), uintptr(pt.Y), 0, uintptr(mainHWND), 0)
			if cmd == 7220 && ask("즐겨찾기를 포함한 모든 클립보드 기록을 완전히 삭제할까요?") == IDYES {
				deleteClipboardRecords(true)
				refreshClipboardList()
			}
			if cmd == 7221 && ask("즐겨찾기를 제외한 일반 기록을 모두 삭제할까요?") == IDYES {
				deleteClipboardRecords(false)
				refreshClipboardList()
			}
		}
		return true
	case ID_CLIP_COPY:
		if r, ok := selectedClipboardRecord(); ok {
			restoreClipboardRecord(r)
			setStatus("클립보드에 다시 복사했습니다.")
		}
		return true
	case ID_CLIP_SETTINGS:
		showClipboardSettingsMenu()
		return true
	case ID_CLIP_DETAIL:
		if r, ok := selectedClipboardRecord(); ok {
			if r.Kind == "image" {
				info(fmt.Sprintf("이미지\n\n크기: %d × %d\n복사 시간: %s", r.Width, r.Height, r.Created.Local().Format("2006-01-02 15:04:05")))
			} else {
				info(r.Text)
			}
		}
		return true
	}
	return false
}

func clipboardHandleNotify(lParam uintptr) bool {
	clipNotifyResult = 0
	if clipList == 0 || lParam == 0 {
		return false
	}
	hdr := (*NMHDR)(unsafe.Pointer(lParam))
	if hdr.HwndFrom != clipList {
		return false
	}
	if int32(hdr.Code) == -12 { // NM_CUSTOMDRAW
		cd := (*nmListViewCustomDrawClip)(unsafe.Pointer(lParam))
		switch cd.Nmcd.DrawStage {
		case 0x00000001: // CDDS_PREPAINT
			clipNotifyResult = 0x00000020 // CDRF_NOTIFYITEMDRAW
		case 0x00010001: // CDDS_ITEMPREPAINT
			clipNotifyResult = 0x00000020 // CDRF_NOTIFYSUBITEMDRAW
		case 0x00030001: // item + subitem
			if cd.ISubItem == 0 && int(cd.Nmcd.ItemSpec) < len(clipFiltered) {
				fill := brushPanel
				if cd.Nmcd.ItemState&1 != 0 {
					fill = solidBrush(232, 240, 254)
					defer procDeleteObject.Call(uintptr(fill))
				}
				procFillRect.Call(uintptr(cd.Nmcd.Hdc), uintptr(unsafe.Pointer(&cd.Nmcd.Rc)), uintptr(fill))
				star := "☆"
				if clipFiltered[cd.Nmcd.ItemSpec].Favorite {
					star = "★"
				}
				procSetBkMode.Call(uintptr(cd.Nmcd.Hdc), TRANSPARENT)
				procSetTextColor.Call(uintptr(cd.Nmcd.Hdc), rgb(15, 23, 42))
				old, _, _ := procSelectObject.Call(uintptr(cd.Nmcd.Hdc), uintptr(fontButton))
				rc := cd.Nmcd.Rc
				procDrawTextW.Call(uintptr(cd.Nmcd.Hdc), uintptr(unsafe.Pointer(p16(star))), 1, uintptr(unsafe.Pointer(&rc)), DT_CENTER|DT_VCENTER|DT_SINGLELINE)
				procSelectObject.Call(uintptr(cd.Nmcd.Hdc), old)
				clipNotifyResult = 0x00000004 // CDRF_SKIPDEFAULT
			}
		}
		return true
	}
	nm := (*NMLISTVIEW)(unsafe.Pointer(lParam))
	if nm.Hdr.HwndFrom != clipList {
		return false
	}
	if int32(nm.Hdr.Code) == -2 {
		if nm.IItem >= 0 && int(nm.IItem) < len(clipFiltered) {
			r := clipFiltered[nm.IItem]
			if nm.ISubItem == 0 {
				toggleClipboardFavorite(r.ID)
				refreshClipboardList()
				return true
			}
			if nm.ISubItem == 4 {
				deleteClipboardRecord(r.ID)
				refreshClipboardList()
				return true
			}
		}
	}
	if int32(nm.Hdr.Code) == -3 {
		if r, ok := selectedClipboardRecord(); ok {
			restoreClipboardRecord(r)
			setStatus("클립보드에 다시 복사했습니다.")
		}
		return true
	}
	return false
}

func deleteClipboardRecord(id string) {
	clipMu.Lock()
	defer clipMu.Unlock()
	for i, r := range clipRecords {
		if r.ID == id {
			if r.Image != "" {
				_ = os.Remove(filepath.Join(clipboardDir(), r.Image))
			}
			clipRecords = append(clipRecords[:i], clipRecords[i+1:]...)
			break
		}
	}
	saveClipboardRecordsLocked()
}
func clipboardTimerTick() {
	changed := false
	if cfgStat, cfgErr := os.Stat(clipboardConfigPath()); cfgErr == nil && cfgStat.ModTime().After(clipConfigLastMod) {
		clipConfigLastMod = cfgStat.ModTime()
		reloadClipboardConfigOnly()
		changed = true
	}
	st, err := os.Stat(clipboardRecordsPath())
	if err != nil {
		if changed {
			refreshClipboardList()
		}
		return
	}
	if st.ModTime().After(clipLastMod) {
		clipLastMod = st.ModTime()
		clipMu.Lock()
		if b, e := os.ReadFile(clipboardRecordsPath()); e == nil {
			_ = json.Unmarshal(b, &clipRecords)
		}
		clipMu.Unlock()
		changed = true
	}
	if changed {
		refreshClipboardList()
	}
}
func toggleClipboardFavorite(id string) {
	clipMu.Lock()
	defer clipMu.Unlock()
	for i := range clipRecords {
		if clipRecords[i].ID == id {
			clipRecords[i].Favorite = !clipRecords[i].Favorite
			break
		}
	}
	saveClipboardRecordsLocked()
}
func deleteClipboardRecords(includeFavorites bool) {
	clipMu.Lock()
	defer clipMu.Unlock()
	keep := clipRecords[:0]
	for _, r := range clipRecords {
		if r.Favorite && !includeFavorites {
			keep = append(keep, r)
		} else if r.Image != "" {
			_ = os.Remove(filepath.Join(clipboardDir(), r.Image))
		}
	}
	clipRecords = keep
	saveClipboardRecordsLocked()
}

func restoreClipboardRecord(r clipboardRecord) bool {
	if r.Kind != "image" {
		return copyClipboard(r.Text) == nil
	}
	b, err := os.ReadFile(filepath.Join(clipboardDir(), r.Image))
	if err != nil {
		return false
	}
	if ok, _, _ := procOpenClipboard.Call(uintptr(mainHWND)); ok == 0 {
		return false
	}
	defer procCloseClipboard.Call()
	procEmptyClipboard.Call()
	h, _, _ := procGlobalAllocClip.Call(GMEM_MOVEABLE_CLIP, uintptr(len(b)))
	if h == 0 {
		return false
	}
	p, _, _ := procGlobalLockClip.Call(h)
	if p == 0 {
		return false
	}
	copy(unsafe.Slice((*byte)(unsafe.Pointer(p)), len(b)), b)
	procGlobalUnlockClip.Call(h)
	ok, _, _ := procSetClipboardData.Call(CF_DIB, h)
	return ok != 0
}

func showClipboardSettingsMenu() {
	menu, _, _ := procCreatePopupMenu.Call()
	if menu == 0 {
		return
	}
	defer procDestroyMenu.Call(menu)
	clipMu.Lock()
	c := clipConfig
	clipMu.Unlock()
	check := func(v bool) uintptr {
		if v {
			return MF_CHECKED
		}
		return 0
	}
	procAppendMenuW.Call(menu, MF_STRING|check(c.CaptureImages), 7201, uintptr(unsafe.Pointer(p16("이미지 기록 사용"))))
	procAppendMenuW.Call(menu, MF_STRING|check(c.Persist), 7202, uintptr(unsafe.Pointer(p16("앱 종료 후 기록 유지"))))
	procAppendMenuW.Call(menu, MF_STRING|check(c.RemoveDuplicate), 7203, uintptr(unsafe.Pointer(p16("자동 중복 제거"))))
	procAppendMenuW.Call(menu, MF_STRING|check(c.ExcludePasswords), 7213, uintptr(unsafe.Pointer(p16("비밀번호 입력창 기록 제외"))))
	procAppendMenuW.Call(menu, MF_STRING, 7204, uintptr(unsafe.Pointer(p16("최대 100개"))))
	procAppendMenuW.Call(menu, MF_STRING, 7205, uintptr(unsafe.Pointer(p16("최대 300개 (기본)"))))
	procAppendMenuW.Call(menu, MF_STRING, 7206, uintptr(unsafe.Pointer(p16("최대 500개"))))
	procAppendMenuW.Call(menu, MF_STRING, 7207, uintptr(unsafe.Pointer(p16("보관 7일"))))
	procAppendMenuW.Call(menu, MF_STRING, 7208, uintptr(unsafe.Pointer(p16("보관 30일 (기본)"))))
	procAppendMenuW.Call(menu, MF_STRING, 7209, uintptr(unsafe.Pointer(p16("보관 90일"))))
	procAppendMenuW.Call(menu, MF_STRING, 7210, uintptr(unsafe.Pointer(p16("즐겨찾기 포함 전체 삭제"))))
	procAppendMenuW.Call(menu, MF_STRING, 7214, uintptr(unsafe.Pointer(p16("프로그램·문자열 제외 규칙 초기화"))))
	q := strings.TrimSpace(getText(clipSearch))
	if q != "" {
		procAppendMenuW.Call(menu, MF_STRING, 7211, uintptr(unsafe.Pointer(p16("현재 검색어를 기록 제외 패턴에 추가"))))
	}
	selected, hasSelected := selectedClipboardRecord()
	if hasSelected && selected.Source != "" {
		procAppendMenuW.Call(menu, MF_STRING, 7212, uintptr(unsafe.Pointer(p16("선택 항목의 원본 프로그램 제외: "+selected.Source))))
	}
	var pt POINT
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	cmd, _, _ := procTrackPopupMenu.Call(menu, TPM_RETURNCMD|TPM_NONOTIFY, uintptr(pt.X), uintptr(pt.Y), 0, uintptr(mainHWND), 0)
	clipMu.Lock()
	switch cmd {
	case 7201:
		clipConfig.CaptureImages = !clipConfig.CaptureImages
	case 7202:
		clipConfig.Persist = !clipConfig.Persist
	case 7203:
		clipConfig.RemoveDuplicate = !clipConfig.RemoveDuplicate
	case 7204:
		clipConfig.MaxItems = 100
	case 7205:
		clipConfig.MaxItems = 300
	case 7206:
		clipConfig.MaxItems = 500
	case 7207:
		clipConfig.RetentionDays = 7
	case 7208:
		clipConfig.RetentionDays = 30
	case 7209:
		clipConfig.RetentionDays = 90
	case 7211:
		if q != "" {
			clipConfig.ExcludedPatterns = append(clipConfig.ExcludedPatterns, q)
		}
	case 7212:
		if hasSelected && selected.Source != "" {
			clipConfig.ExcludedApps = append(clipConfig.ExcludedApps, selected.Source)
		}
	case 7213:
		clipConfig.ExcludePasswords = !clipConfig.ExcludePasswords
	case 7214:
		clipConfig.ExcludedApps = nil
		clipConfig.ExcludedPatterns = nil
	}
	pruneClipboardLocked()
	clipMu.Unlock()
	saveClipboardConfig()
	saveClipboardRecords()
	if cmd == 7210 && ask("즐겨찾기를 포함한 모든 클립보드 기록을 완전히 삭제할까요?") == IDYES {
		deleteClipboardRecords(true)
	}
	refreshClipboardList()
}

// Keep imports used in older Windows SDK builds where constants are typed differently.
var _ = sort.Slice
var _ = strconv.Itoa
