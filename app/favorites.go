//go:build windows

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"
)

const (
	LVS_NOCOLUMNHEADER  = 0x4000
	LVNI_SELECTED_FAV   = 0x0002
	LVM_GETNEXTITEM_FAV = 0x100C
)

var (
	favoritesHWND  syscall.Handle
	favoritesList  syscall.Handle
	favoritesOrder []int
)

func allLauncherTools() []int {
	return []int{ID_NAV_PDF, ID_NAV_PRINT, ID_NAV_RENAME, ID_NAV_FOLDERS, ID_NAV_DUP, ID_NAV_IMAGE, ID_NAV_COLOR, ID_NAV_TEXT, ID_NAV_CLIP, ID_NAV_BUNDLE, ID_NAV_OCR}
}

func launcherFavoritesPath() string {
	cache, err := os.UserCacheDir()
	if err != nil || cache == "" {
		cache = os.TempDir()
	}
	return filepath.Join(cache, "JTSN", "launcher_favorites.json")
}

func loadLauncherFavorites() []int {
	defaults := allLauncherTools()
	b, err := os.ReadFile(launcherFavoritesPath())
	if err != nil {
		return defaults
	}
	var ids []int
	if json.Unmarshal(b, &ids) != nil {
		return defaults
	}
	valid := map[int]bool{}
	for _, id := range defaults {
		valid[id] = true
	}
	out, seen := []int{}, map[int]bool{}
	for _, id := range ids {
		if valid[id] && !seen[id] {
			out = append(out, id)
			seen[id] = true
		}
	}
	return out
}

func saveLauncherFavorites(ids []int) {
	_ = os.MkdirAll(filepath.Dir(launcherFavoritesPath()), 0755)
	b, _ := json.Marshal(ids)
	_ = os.WriteFile(launcherFavoritesPath(), b, 0644)
}

func registerFavoritesEditorClass(hInst, cursor syscall.Handle) {
	name := p16("JTSNFavoritesEditor")
	wc := WNDCLASSEX{CbSize: uint32(unsafe.Sizeof(WNDCLASSEX{})), Style: 0x00020000,
		LpfnWndProc: syscall.NewCallback(favoritesWndProc), HInstance: hInst, HCursor: cursor,
		HbrBackground: brushPanel, LpszClassName: name, HIcon: appIconBig, HIconSm: appIconSmall}
	if r, _, _ := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); r == 0 {
		panic("failed to register favorites editor")
	}
}

func openFavoritesEditor() {
	if favoritesHWND != 0 {
		procShowWindow.Call(uintptr(favoritesHWND), SW_SHOW)
		procSetForegroundWindow.Call(uintptr(favoritesHWND))
		return
	}
	favorites := loadLauncherFavorites()
	selected := map[int]bool{}
	for _, id := range favorites {
		selected[id] = true
	}
	favoritesOrder = append([]int{}, favorites...)
	for _, id := range allLauncherTools() {
		if !selected[id] {
			favoritesOrder = append(favoritesOrder, id)
		}
	}
	var owner RECT
	procGetWindowRect.Call(uintptr(mainHWND), uintptr(unsafe.Pointer(&owner)))
	w, h := int32(500), int32(570)
	favoritesHWND = createWindow(0, "JTSNFavoritesEditor", "즐겨찾기 편집", WS_POPUP|WS_VISIBLE|WS_CLIPCHILDREN,
		int(owner.Left+(owner.Right-owner.Left-w)/2), int(owner.Top+(owner.Bottom-owner.Top-h)/2), int(w), int(h), mainHWND, 0)
	enableNativeWindowShadow(favoritesHWND)
	procSetForegroundWindow.Call(uintptr(favoritesHWND))
}

func rebuildFavoritesEditorList(checked map[int]bool) {
	listViewClear(favoritesList)
	for i, id := range favoritesOrder {
		listViewAddRow(favoritesList, i, []string{toolName(id)})
		if checked[id] {
			it := LVITEMW{StateMask: LVIS_STATEIMAGEMASK, State: 2 << 12, IItem: int32(i)}
			procSendMessageW.Call(uintptr(favoritesList), LVM_SETITEMSTATE, uintptr(i), uintptr(unsafe.Pointer(&it)))
		}
	}
}

func favoritesChecked() map[int]bool {
	out := map[int]bool{}
	for i, id := range favoritesOrder {
		state, _, _ := procSendMessageW.Call(uintptr(favoritesList), LVM_GETITEMSTATE, uintptr(i), LVIS_STATEIMAGEMASK)
		out[id] = ((state >> 12) & 0xf) == 2
	}
	return out
}

func selectedFavoriteEditorIndex() int {
	r, _, _ := procSendMessageW.Call(uintptr(favoritesList), LVM_GETNEXTITEM_FAV, ^uintptr(0), LVNI_SELECTED_FAV)
	if int32(r) < 0 {
		return -1
	}
	return int(r)
}

func favoritesWndProc(hwnd syscall.Handle, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case WM_CREATE:
		favoritesList = createWindow(0, "SysListView32", "", WS_CHILD|WS_VISIBLE|WS_TABSTOP|LVS_REPORT|LVS_SHOWSELALWAYS|LVS_NOCOLUMNHEADER, 28, 76, 330, 420, hwnd, ID_FAVORITES_LIST)
		sendFont(favoritesList, fontNormal)
		procSendMessageW.Call(uintptr(favoritesList), LVM_SETEXTENDEDLISTVIEWSTYLE, LVS_EX_FULLROWSELECT|LVS_EX_DOUBLEBUFFER|LVS_EX_CHECKBOXES, LVS_EX_FULLROWSELECT|LVS_EX_DOUBLEBUFFER|LVS_EX_CHECKBOXES)
		listViewAddColumn(favoritesList, 0, "도구", 300)
		checked := map[int]bool{}
		for _, id := range loadLauncherFavorites() {
			checked[id] = true
		}
		rebuildFavoritesEditorList(checked)
		createOwnerButton(hwnd, "위로", 374, 100, 96, 42, ID_FAVORITES_UP, BTN_SECONDARY)
		createOwnerButton(hwnd, "아래로", 374, 152, 96, 42, ID_FAVORITES_DOWN, BTN_SECONDARY)
		createOwnerButton(hwnd, "저장", 374, 410, 96, 42, ID_FAVORITES_SAVE, BTN_PRIMARY)
		createOwnerButton(hwnd, "취소", 374, 462, 96, 34, ID_FAVORITES_CANCEL, BTN_LAUNCH_GHOST)
		return 0
	case WM_PAINT:
		var ps PAINTSTRUCT
		hdc, _, _ := procBeginPaint.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&ps)))
		if hdc != 0 {
			var rc RECT
			procGetClientRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&rc)))
			procFillRect.Call(hdc, uintptr(unsafe.Pointer(&rc)), uintptr(brushPanel))
			drawSettingsText(syscall.Handle(hdc), "즐겨찾기 편집", RECT{28, 18, 300, 52}, fontLauncherSection, rgb(17, 24, 39))
			drawSettingsText(syscall.Handle(hdc), "체크로 추가·제거하고 선택한 항목의 순서를 바꾸세요.", RECT{28, 48, 460, 72}, fontSmall, rgb(100, 116, 139))
			procEndPaint.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&ps)))
		}
		return 0
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
		switch id {
		case ID_FAVORITES_UP, ID_FAVORITES_DOWN:
			i := selectedFavoriteEditorIndex()
			j := i - 1
			if id == ID_FAVORITES_DOWN {
				j = i + 1
			}
			if i >= 0 && j >= 0 && j < len(favoritesOrder) {
				c := favoritesChecked()
				favoritesOrder[i], favoritesOrder[j] = favoritesOrder[j], favoritesOrder[i]
				rebuildFavoritesEditorList(c)
				it := LVITEMW{StateMask: 0x0002, State: 0x0002, IItem: int32(j)}
				procSendMessageW.Call(uintptr(favoritesList), LVM_SETITEMSTATE, uintptr(j), uintptr(unsafe.Pointer(&it)))
			}
		case ID_FAVORITES_SAVE:
			c := favoritesChecked()
			ids := []int{}
			for _, toolID := range favoritesOrder {
				if c[toolID] {
					ids = append(ids, toolID)
				}
			}
			saveLauncherFavorites(ids)
			procDestroyWindow.Call(uintptr(hwnd))
			rebuildLauncher(mainHWND)
		case ID_FAVORITES_CANCEL:
			procDestroyWindow.Call(uintptr(hwnd))
		}
		return 0
	case WM_CLOSE:
		procDestroyWindow.Call(uintptr(hwnd))
		return 0
	case WM_DESTROY:
		favoritesHWND = 0
		favoritesList = 0
		return 0
	}
	r, _, _ := procDefWindowProcW.Call(uintptr(hwnd), uintptr(msg), wParam, lParam)
	return r
}
