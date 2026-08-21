//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

var (
	launcherFavoriteEditing bool
	inlineFavoriteCards     = map[syscall.Handle]int{}
	inlineDragCard          syscall.Handle
	inlineDragStart         POINT
	inlineFavoriteOriginal  []int
)

func beginInlineFavoriteEdit() {
	inlineFavoriteOriginal = append([]int(nil), loadLauncherFavorites()...)
	launcherFavoriteEditing = true
}
func finishInlineFavoriteEdit() { inlineFavoriteOriginal = nil; launcherFavoriteEditing = false }
func cancelInlineFavoriteEdit() {
	if inlineFavoriteOriginal != nil {
		saveLauncherFavorites(inlineFavoriteOriginal)
	}
	inlineFavoriteOriginal = nil
	launcherFavoriteEditing = false
}

func resetInlineFavoriteCards()                              { inlineFavoriteCards = map[syscall.Handle]int{} }
func registerInlineFavoriteCard(hwnd syscall.Handle, id int) { inlineFavoriteCards[hwnd] = id }

func removeInlineFavorite(id int) {
	ids := loadLauncherFavorites()
	next := make([]int, 0, len(ids))
	for _, v := range ids {
		if v != id {
			next = append(next, v)
		}
	}
	saveLauncherFavorites(next)
	procPostMessageW.Call(uintptr(mainHWND), WM_APP_FAV_REBUILD, 0, 0)
}

func showInlineFavoriteAddMenu() {
	current := map[int]bool{}
	for _, id := range loadLauncherFavorites() {
		current[id] = true
	}
	menu, _, _ := procCreatePopupMenu.Call()
	if menu == 0 {
		return
	}
	defer procDestroyMenu.Call(menu)
	count := 0
	for _, id := range allLauncherTools() {
		if !current[id] {
			procAppendMenuW.Call(menu, MF_STRING, uintptr(6800+id), uintptr(unsafe.Pointer(p16(toolName(id)))))
			count++
		}
	}
	if count == 0 {
		procAppendMenuW.Call(menu, 0x0001, 6899, uintptr(unsafe.Pointer(p16("모든 기능이 추가되어 있습니다"))))
	}
	var pt POINT
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	cmd, _, _ := procTrackPopupMenu.Call(menu, TPM_RETURNCMD|TPM_NONOTIFY, uintptr(pt.X), uintptr(pt.Y), 0, uintptr(mainHWND), 0)
	if cmd >= 6800+ID_NAV_PRINT && cmd <= 6800+ID_NAV_OCR {
		ids := loadLauncherFavorites()
		ids = append(ids, int(cmd)-6800)
		saveLauncherFavorites(ids)
		rebuildLauncher(mainHWND)
	}
}

func handleInlineFavoriteCardMouse(hwnd syscall.Handle, msg uint32, lParam uintptr) bool {
	if !launcherFavoriteEditing || launcherCategory != ID_SIDE_FAVORITES {
		return false
	}
	if _, ok := inlineFavoriteCards[hwnd]; !ok {
		return false
	}
	switch msg {
	case WM_LBUTTONDOWN:
		inlineDragCard = hwnd
		procGetCursorPos.Call(uintptr(unsafe.Pointer(&inlineDragStart)))
		procSetCapture.Call(uintptr(hwnd))
		return true
	case WM_MOUSEMOVE:
		if inlineDragCard == hwnd {
			return true
		}
	case WM_LBUTTONUP:
		if inlineDragCard != hwnd {
			return true
		}
		procReleaseCapture.Call()
		inlineDragCard = 0
		var local RECT
		procGetClientRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&local)))
		x := int32(int16(lParam & 0xffff))
		y := int32(int16((lParam >> 16) & 0xffff))
		if x >= local.Right-44 && y <= 44 {
			removeInlineFavorite(inlineFavoriteCards[hwnd])
			return true
		}
		var pt POINT
		procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
		fromID := inlineFavoriteCards[hwnd]
		targetID := fromID
		best := int64(1 << 62)
		for card, id := range inlineFavoriteCards {
			var rc RECT
			procGetWindowRect.Call(uintptr(card), uintptr(unsafe.Pointer(&rc)))
			cx, cy := int64((rc.Left+rc.Right)/2), int64((rc.Top+rc.Bottom)/2)
			dx, dy := int64(pt.X)-cx, int64(pt.Y)-cy
			d := dx*dx + dy*dy
			if d < best {
				best, targetID = d, id
			}
		}
		if targetID != fromID {
			ids := loadLauncherFavorites()
			from, to := -1, -1
			for i, id := range ids {
				if id == fromID {
					from = i
				}
				if id == targetID {
					to = i
				}
			}
			if from >= 0 && to >= 0 {
				moved := ids[from]
				ids = append(ids[:from], ids[from+1:]...)
				if from < to {
					to--
				}
				ids = append(ids, nilInt...)
				copy(ids[to+1:], ids[to:])
				ids[to] = moved
				saveLauncherFavorites(ids)
				procPostMessageW.Call(uintptr(mainHWND), WM_APP_FAV_REBUILD, 0, 0)
			}
		}
		return true
	}
	return false
}

var nilInt = []int{0}

func drawInlineFavoriteAction(dis *DRAWITEMSTRUCT, kind int) {
	hovered := hoveredButtons[dis.HwndItem]
	if kind == BTN_FAV_ADD {
		fill, border, textColor := rgb(242, 253, 246), rgb(34, 197, 94), rgb(22, 101, 52)
		if hovered {
			fill = rgb(220, 252, 231)
		}
		drawSoftCard(dis.HDC, dis.RcItem, 12, border, fill)
		procSetBkMode.Call(uintptr(dis.HDC), TRANSPARENT)
		procSetTextColor.Call(uintptr(dis.HDC), textColor)
		old, _, _ := procSelectObject.Call(uintptr(dis.HDC), uintptr(fontButton))
		txt := "추가   ⊕"
		rc := dis.RcItem
		procDrawTextW.Call(uintptr(dis.HDC), uintptr(unsafe.Pointer(p16(txt))), uintptr(len(syscall.StringToUTF16(txt))-1), uintptr(unsafe.Pointer(&rc)), DT_CENTER|DT_VCENTER|DT_SINGLELINE)
		procSelectObject.Call(uintptr(dis.HDC), old)
		return
	}
	procFillRect.Call(uintptr(dis.HDC), uintptr(unsafe.Pointer(&dis.RcItem)), uintptr(brushPanel))
	cx, cy := (dis.RcItem.Left+dis.RcItem.Right)/2, (dis.RcItem.Top+dis.RcItem.Bottom)/2
	red := byte(220)
	if hovered {
		red = 185
	}
	b := solidBrush(red, 38, 38)
	oldB, _, _ := procSelectObject.Call(uintptr(dis.HDC), uintptr(b))
	pen, _, _ := procCreatePen.Call(PS_SOLID, 1, rgb(red, 38, 38))
	oldP, _, _ := procSelectObject.Call(uintptr(dis.HDC), pen)
	procEllipse.Call(uintptr(dis.HDC), uintptr(cx-10), uintptr(cy-10), uintptr(cx+10), uintptr(cy+10))
	procSelectObject.Call(uintptr(dis.HDC), oldB)
	procSelectObject.Call(uintptr(dis.HDC), oldP)
	procDeleteObject.Call(uintptr(b))
	procDeleteObject.Call(pen)
	whitePen, _, _ := procCreatePen.Call(PS_SOLID, 2, rgb(255, 255, 255))
	op, _, _ := procSelectObject.Call(uintptr(dis.HDC), whitePen)
	procMoveToEx.Call(uintptr(dis.HDC), uintptr(cx-5), uintptr(cy), 0)
	procLineTo.Call(uintptr(dis.HDC), uintptr(cx+6), uintptr(cy))
	procSelectObject.Call(uintptr(dis.HDC), op)
	procDeleteObject.Call(whitePen)
}
