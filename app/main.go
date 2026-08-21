//go:build windows

package main

import (
	"archive/zip"
	"bufio"
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
	WS_OVERLAPPEDWINDOW = 0x00CF0000
	WS_VISIBLE          = 0x10000000
	WS_CHILD            = 0x40000000
	WS_TABSTOP          = 0x00010000
	WS_BORDER           = 0x00800000
	WS_VSCROLL          = 0x00200000
	WS_CLIPCHILDREN     = 0x02000000

	LVS_REPORT           = 0x0001
	LVS_SHOWSELALWAYS    = 0x0008
	LVS_NOSORTHEADER     = 0x8000
	LVS_EX_GRIDLINES     = 0x00000001
	LVS_EX_CHECKBOXES    = 0x00000004
	LVS_EX_FULLROWSELECT = 0x00000020
	LVS_EX_DOUBLEBUFFER  = 0x00010000

	LVM_FIRST                    = 0x1000
	LVM_GETITEMCOUNT             = LVM_FIRST + 4
	LVM_DELETEALLITEMS           = LVM_FIRST + 9
	LVM_SETITEMSTATE             = LVM_FIRST + 43
	LVM_GETITEMSTATE             = LVM_FIRST + 44
	LVM_SETEXTENDEDLISTVIEWSTYLE = LVM_FIRST + 54
	LVM_INSERTITEMW              = LVM_FIRST + 77
	LVM_INSERTCOLUMNW            = LVM_FIRST + 97
	LVM_SETITEMTEXTW             = LVM_FIRST + 116
	LVIF_TEXT                    = 0x0001
	LVCF_FMT                     = 0x0001
	LVCF_WIDTH                   = 0x0002
	LVCF_TEXT                    = 0x0004
	LVCF_SUBITEM                 = 0x0008
	LVCFMT_LEFT                  = 0x0000
	LVIS_STATEIMAGEMASK          = 0xF000

	BS_PUSHBUTTON = 0x00000000
	BS_OWNERDRAW  = 0x0000000B

	ES_LEFT         = 0x0000
	ES_MULTILINE    = 0x0004
	ES_AUTOVSCROLL  = 0x0040
	ES_READONLY     = 0x0800
	ES_AUTOHSCROLL  = 0x0080
	ES_WANTRETURN   = 0x1000
	EM_SETMARGINS   = 0x00D3
	EM_SETSEL       = 0x00B1
	EM_SETCUEBANNER = 0x1501
	EC_LEFTMARGIN   = 0x0001
	EC_RIGHTMARGIN  = 0x0002

	CBS_DROPDOWNLIST = 0x0003
	CBN_SETFOCUS     = 3
	CBN_KILLFOCUS    = 4
	EN_SETFOCUS      = 0x0100
	EN_KILLFOCUS     = 0x0200
	EN_CHANGE        = 0x0300

	WM_CREATE           = 0x0001
	WM_DESTROY          = 0x0002
	WM_SIZE             = 0x0005
	WM_PAINT            = 0x000F
	WM_CLOSE            = 0x0010
	WM_DRAWITEM         = 0x002B
	WM_SETFONT          = 0x0030
	WM_COMMAND          = 0x0111
	WM_SETCURSOR        = 0x0020
	WM_CAPTURECHANGED   = 0x0215
	WM_TIMER            = 0x0113
	WM_HOTKEY           = 0x0312
	WM_MOUSELEAVE       = 0x02A3
	WM_CTLCOLORSTATIC   = 0x0138
	WM_CTLCOLOREDIT     = 0x0133
	WM_APP              = 0x8000
	WM_APP_STATUS       = WM_APP + 1
	WM_APP_DUPDONE      = WM_APP + 2
	WM_APP_COLOR        = WM_APP + 3
	WM_APP_PRINTERS     = WM_APP + 4
	WM_APP_PROGRESS     = WM_APP + 5
	WM_APP_TASKDONE     = WM_APP + 6
	WM_APP_ERROR        = WM_APP + 7
	WM_APP_DUPDELETED   = WM_APP + 8
	WM_APP_SEARCH       = WM_APP + 9
	WM_DROPFILES        = 0x0233
	WM_SETICON          = 0x0080
	ICON_SMALL          = 0
	ICON_BIG            = 1
	IMAGE_BITMAP        = 0
	IMAGE_ICON          = 1
	LR_LOADFROMFILE     = 0x0010
	LR_CREATEDIBSECTION = 0x00002000
	DI_NORMAL           = 0x0003

	SW_HIDE    = 0
	SW_SHOW    = 5
	SW_RESTORE = 9

	MOD_ALT     = 0x0001
	MOD_CONTROL = 0x0002
	MOD_SHIFT   = 0x0004
	HOTKEY_ID   = 0x4A54

	VK_LBUTTON          = 0x01
	ID_TIMER_EYEDROPPER = 9101

	MB_OK              = 0x00000000
	MB_ICONINFORMATION = 0x00000040
	MB_ICONERROR       = 0x00000010
	MB_YESNO           = 0x00000004
	MB_ICONQUESTION    = 0x00000020
	IDYES              = 6

	OFN_EXPLORER         = 0x00080000
	OFN_ALLOWMULTISELECT = 0x00000200
	OFN_FILEMUSTEXIST    = 0x00001000
	OFN_PATHMUSTEXIST    = 0x00000800
	OFN_OVERWRITEPROMPT  = 0x00000002

	CB_ADDSTRING    = 0x0143
	CB_GETCURSEL    = 0x0147
	CB_GETLBTEXT    = 0x0148
	CB_SETCURSEL    = 0x014E
	CB_RESETCONTENT = 0x014B

	PBM_SETPOS     = 0x0402
	PBM_SETRANGE32 = 0x0406

	DEFAULT_GUI_FONT = 17
	TRANSPARENT      = 1
	DT_LEFT          = 0x0000
	DT_CENTER        = 0x0001
	DT_VCENTER       = 0x0004
	DT_SINGLELINE    = 0x0020
	DT_END_ELLIPSIS  = 0x8000
	PS_SOLID         = 0
	ODS_SELECTED     = 0x0001
	ODS_DISABLED     = 0x0004
	TME_LEAVE        = 0x00000002

	PRINTER_ENUM_LOCAL       = 0x00000002
	PRINTER_ENUM_CONNECTIONS = 0x00000004

	BIF_RETURNONLYFSDIRS = 0x00000001
	BIF_EDITBOX          = 0x00000010
	BIF_NEWDIALOGSTYLE   = 0x00000040
	BIF_USENEWUI         = BIF_EDITBOX | BIF_NEWDIALOGSTYLE

	CF_UNICODETEXT = 13
	GMEM_MOVEABLE  = 0x0002

	ID_NAV_PRINT   = 101
	ID_NAV_PDF     = 102
	ID_NAV_RENAME  = 103
	ID_NAV_FOLDERS = 104
	ID_NAV_DUP     = 105
	ID_NAV_IMAGE   = 106
	ID_NAV_COLOR   = 107
	ID_NAV_TEXT    = 108

	// Launcher-only navigation IDs. These never open a tool directly; they
	// switch the dashboard category or perform a launcher action.
	ID_SIDE_FAVORITES = 601
	ID_SIDE_PDF       = 602
	ID_SIDE_FILES     = 603
	ID_SIDE_IMAGES    = 604
	ID_SIDE_TEXT      = 605
	ID_SIDE_UTIL      = 606
	ID_SIDE_SETTINGS  = 620
	ID_SIDE_INFO      = 621
	ID_LAUNCH_EDIT    = 630
	ID_RECENT_CLEAR   = 631
	ID_LAUNCH_SEARCH  = 632

	ID_BTN_ADD        = 201
	ID_BTN_CLEAR      = 202
	ID_BTN_RUN        = 203
	ID_BTN_FOLDER     = 204
	ID_BTN_EXPORT     = 205
	ID_BTN_RECYCLE    = 206
	ID_BTN_CAPTURE    = 207
	ID_BTN_SAVE       = 208
	ID_BTN_OUTPUT     = 209
	ID_BTN_COPY       = 210
	ID_BTN_AUTOSELECT = 211
	ID_BTN_UNSELECT   = 212

	ID_EDIT_MAIN  = 301
	ID_EDIT_A     = 302
	ID_EDIT_B     = 303
	ID_EDIT_C     = 304
	ID_EDIT_D     = 305
	ID_COMBO_MAIN = 401
	ID_COMBO_B    = 402
	ID_COMBO_C    = 403
	ID_COMBO_D    = 404
	ID_COMBO_E    = 405
	ID_COMBO_F    = 406

	BTN_NAV           = 1
	BTN_PRIMARY       = 2
	BTN_SECONDARY     = 3
	BTN_DANGER        = 4
	BTN_COMBO         = 5
	BTN_COLOR_SWATCH  = 6
	BTN_COLOR_PREVIEW = 7
	BTN_EYEDROPPER    = 8
	BTN_SIDEBAR       = 9
	BTN_LAUNCH_CARD   = 10
	BTN_RECENT        = 11
	BTN_LAUNCH_GHOST  = 12

	MF_STRING     = 0x0000
	MF_CHECKED    = 0x0008
	TPM_NONOTIFY  = 0x0080
	TPM_RETURNCMD = 0x0100
)

type POINT struct{ X, Y int32 }
type RECT struct{ Left, Top, Right, Bottom int32 }
type PAINTSTRUCT struct {
	Hdc         syscall.Handle
	FErase      int32
	RcPaint     RECT
	FRestore    int32
	FIncUpdate  int32
	RgbReserved [32]byte
}
type DRAWITEMSTRUCT struct {
	CtlType    uint32
	CtlID      uint32
	ItemID     uint32
	ItemAction uint32
	ItemState  uint32
	HwndItem   syscall.Handle
	HDC        syscall.Handle
	RcItem     RECT
	ItemData   uintptr
}
type TRACKMOUSEEVENT struct {
	CbSize      uint32
	DwFlags     uint32
	HwndTrack   syscall.Handle
	DwHoverTime uint32
}
type inputFrame struct {
	Hwnd syscall.Handle
	Rect RECT
}

type customCombo struct {
	ID       int
	Items    []string
	Selected int
}
type WNDCLASSEX struct {
	CbSize        uint32
	Style         uint32
	LpfnWndProc   uintptr
	CbClsExtra    int32
	CbWndExtra    int32
	HInstance     syscall.Handle
	HIcon         syscall.Handle
	HCursor       syscall.Handle
	HbrBackground syscall.Handle
	LpszMenuName  *uint16
	LpszClassName *uint16
	HIconSm       syscall.Handle
}
type MSG struct {
	Hwnd     syscall.Handle
	Message  uint32
	WParam   uintptr
	LParam   uintptr
	Time     uint32
	Pt       POINT
	LPrivate uint32
}
type OPENFILENAME struct {
	LStructSize       uint32
	HwndOwner         syscall.Handle
	HInstance         syscall.Handle
	LpstrFilter       *uint16
	LpstrCustomFilter *uint16
	NMaxCustFilter    uint32
	NFilterIndex      uint32
	LpstrFile         *uint16
	NMaxFile          uint32
	LpstrFileTitle    *uint16
	NMaxFileTitle     uint32
	LpstrInitialDir   *uint16
	LpstrTitle        *uint16
	Flags             uint32
	NFileOffset       uint16
	NFileExtension    uint16
	LpstrDefExt       *uint16
	LCustData         uintptr
	LpfnHook          uintptr
	LpTemplateName    *uint16
	PvReserved        uintptr
	DwReserved        uint32
	FlagsEx           uint32
}
type BROWSEINFO struct {
	HwndOwner      syscall.Handle
	PidlRoot       uintptr
	PszDisplayName *uint16
	LpszTitle      *uint16
	UlFlags        uint32
	Lpfn           uintptr
	LParam         uintptr
	IImage         int32
}
type INITCOMMONCONTROLSEX struct {
	DwSize uint32
	DwICC  uint32
}
type PRINTER_INFO_4 struct {
	PPrinterName *uint16
	PServerName  *uint16
	Attributes   uint32
}
type duplicateGroup struct {
	Hash  string
	Size  int64
	Files []string
}
type duplicateRow struct {
	Group     int
	FileIndex int
	File      string
	Size      int64
	Hash      string
}
type LVCOLUMNW struct {
	Mask       uint32
	Fmt        int32
	Cx         int32
	PszText    *uint16
	CchTextMax int32
	ISubItem   int32
	IImage     int32
	IOrder     int32
	CxMin      int32
	CxDefault  int32
	CxIdeal    int32
}
type LVITEMW struct {
	Mask       uint32
	IItem      int32
	ISubItem   int32
	State      uint32
	StateMask  uint32
	PszText    *uint16
	CchTextMax int32
	IImage     int32
	LParam     uintptr
	IIndent    int32
	IGroupID   int32
	CColumns   uint32
	PuColumns  *uint32
	PiColFmt   *int32
	IGroup     int32
}

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	comdlg32 = syscall.NewLazyDLL("comdlg32.dll")
	shell32  = syscall.NewLazyDLL("shell32.dll")
	ole32    = syscall.NewLazyDLL("ole32.dll")
	winspool = syscall.NewLazyDLL("winspool.drv")
	comctl32 = syscall.NewLazyDLL("comctl32.dll")
	uxtheme  = syscall.NewLazyDLL("uxtheme.dll")

	procRegisterClassExW     = user32.NewProc("RegisterClassExW")
	procCreateWindowExW      = user32.NewProc("CreateWindowExW")
	procDefWindowProcW       = user32.NewProc("DefWindowProcW")
	procShowWindow           = user32.NewProc("ShowWindow")
	procUpdateWindow         = user32.NewProc("UpdateWindow")
	procGetMessageW          = user32.NewProc("GetMessageW")
	procTranslateMessage     = user32.NewProc("TranslateMessage")
	procDispatchMessageW     = user32.NewProc("DispatchMessageW")
	procPostQuitMessage      = user32.NewProc("PostQuitMessage")
	procDestroyWindow        = user32.NewProc("DestroyWindow")
	procSendMessageW         = user32.NewProc("SendMessageW")
	procSetWindowTextW       = user32.NewProc("SetWindowTextW")
	procGetWindowTextW       = user32.NewProc("GetWindowTextW")
	procGetWindowTextLengthW = user32.NewProc("GetWindowTextLengthW")
	procMessageBoxW          = user32.NewProc("MessageBoxW")
	procGetCursorPos         = user32.NewProc("GetCursorPos")
	procGetAsyncKeyState     = user32.NewProc("GetAsyncKeyState")
	procSetTimer             = user32.NewProc("SetTimer")
	procKillTimer            = user32.NewProc("KillTimer")
	procGetDC                = user32.NewProc("GetDC")
	procReleaseDC            = user32.NewProc("ReleaseDC")
	procPostMessageW         = user32.NewProc("PostMessageW")
	procEnableWindow         = user32.NewProc("EnableWindow")
	procInvalidateRect       = user32.NewProc("InvalidateRect")
	procBeginPaint           = user32.NewProc("BeginPaint")
	procEndPaint             = user32.NewProc("EndPaint")
	procFillRect             = user32.NewProc("FillRect")
	procDrawTextW            = user32.NewProc("DrawTextW")
	procOpenClipboard        = user32.NewProc("OpenClipboard")
	procEmptyClipboard       = user32.NewProc("EmptyClipboard")
	procSetClipboardData     = user32.NewProc("SetClipboardData")
	procCloseClipboard       = user32.NewProc("CloseClipboard")
	procTrackMouseEvent      = user32.NewProc("TrackMouseEvent")
	procSetCursor            = user32.NewProc("SetCursor")
	procSetWindowLongPtrW    = user32.NewProc("SetWindowLongPtrW")
	procCallWindowProcW      = user32.NewProc("CallWindowProcW")
	procGetClientRect        = user32.NewProc("GetClientRect")
	procGetWindowRect        = user32.NewProc("GetWindowRect")
	procCreatePopupMenu      = user32.NewProc("CreatePopupMenu")
	procAppendMenuW          = user32.NewProc("AppendMenuW")
	procTrackPopupMenu       = user32.NewProc("TrackPopupMenu")
	procDestroyMenu          = user32.NewProc("DestroyMenu")
	procLoadImageW           = user32.NewProc("LoadImageW")
	procDrawIconEx           = user32.NewProc("DrawIconEx")
	procFindWindowW          = user32.NewProc("FindWindowW")
	procSetForegroundWindow  = user32.NewProc("SetForegroundWindow")
	procRegisterHotKey       = user32.NewProc("RegisterHotKey")
	procUnregisterHotKey     = user32.NewProc("UnregisterHotKey")

	procCreateFontW            = gdi32.NewProc("CreateFontW")
	procGetPixel               = gdi32.NewProc("GetPixel")
	procCreateCompatibleDC     = gdi32.NewProc("CreateCompatibleDC")
	procCreateCompatibleBitmap = gdi32.NewProc("CreateCompatibleBitmap")
	procDeleteDC               = gdi32.NewProc("DeleteDC")
	procBitBlt                 = gdi32.NewProc("BitBlt")
	procStretchBlt             = gdi32.NewProc("StretchBlt")
	procSetStretchBltMode      = gdi32.NewProc("SetStretchBltMode")
	procCreateSolidBrush       = gdi32.NewProc("CreateSolidBrush")
	procDeleteObject           = gdi32.NewProc("DeleteObject")
	procSetBkMode              = gdi32.NewProc("SetBkMode")
	procSetTextColor           = gdi32.NewProc("SetTextColor")
	procSelectObject           = gdi32.NewProc("SelectObject")
	procCreatePen              = gdi32.NewProc("CreatePen")
	procRoundRect              = gdi32.NewProc("RoundRect")
	procRectangle              = gdi32.NewProc("Rectangle")
	procEllipse                = gdi32.NewProc("Ellipse")
	procMoveToEx               = gdi32.NewProc("MoveToEx")
	procLineTo                 = gdi32.NewProc("LineTo")
	procGetStockObject         = gdi32.NewProc("GetStockObject")

	procGetModuleHandleW = kernel32.NewProc("GetModuleHandleW")
	procCreateMutexW     = kernel32.NewProc("CreateMutexW")
	procCloseHandle      = kernel32.NewProc("CloseHandle")
	procGlobalAlloc      = kernel32.NewProc("GlobalAlloc")
	procGlobalLock       = kernel32.NewProc("GlobalLock")
	procGlobalUnlock     = kernel32.NewProc("GlobalUnlock")
	procGlobalFree       = kernel32.NewProc("GlobalFree")
	procRtlMoveMemory    = kernel32.NewProc("RtlMoveMemory")

	procGetOpenFileNameW = comdlg32.NewProc("GetOpenFileNameW")
	procGetSaveFileNameW = comdlg32.NewProc("GetSaveFileNameW")

	procSHBrowseForFolderW   = shell32.NewProc("SHBrowseForFolderW")
	procSHGetPathFromIDListW = shell32.NewProc("SHGetPathFromIDListW")
	procDragAcceptFiles      = shell32.NewProc("DragAcceptFiles")
	procDragQueryFileW       = shell32.NewProc("DragQueryFileW")
	procDragFinish           = shell32.NewProc("DragFinish")
	procCoTaskMemFree        = ole32.NewProc("CoTaskMemFree")
	procOleInitialize        = ole32.NewProc("OleInitialize")
	procOleUninitialize      = ole32.NewProc("OleUninitialize")

	procEnumPrintersW        = winspool.NewProc("EnumPrintersW")
	procGetDefaultPrinterW   = winspool.NewProc("GetDefaultPrinterW")
	procInitCommonControlsEx = comctl32.NewProc("InitCommonControlsEx")
	procSetWindowTheme       = uxtheme.NewProc("SetWindowTheme")
)

//go:embed jtsn.ico
var jtsnIconData []byte

//go:embed jtsn_brand.bmp
var jtsnBrandBitmapData []byte

var appIconBig, appIconSmall, appIconBrand syscall.Handle
var appBrandBitmap syscall.Handle
var mainHWND syscall.Handle
var dynamicControls []syscall.Handle
var navButtons []syscall.Handle
var launcherControls []syscall.Handle
var launcherCategory = ID_SIDE_FAVORITES
var launcherRecent []int
var launcherRecentLoaded bool
var launcherSearchHandle syscall.Handle
var launcherSearchQuery string
var launcherRebuilding bool
var startupFiles []string
var startupFolder string
var launcherMutex syscall.Handle
var launcherHotkeyRegistered bool
var sidebarControls = map[syscall.Handle]bool{}
var panelControls = map[syscall.Handle]bool{}
var headerControls = map[syscall.Handle]bool{}
var mutedControls = map[syscall.Handle]bool{}
var buttonKinds = map[syscall.Handle]int{}
var buttonIDs = map[syscall.Handle]int{}
var buttonOldProcs = map[syscall.Handle]uintptr{}
var hoveredButtons = map[syscall.Handle]bool{}
var customCombos = map[syscall.Handle]*customCombo{}
var buttonWndProcPtr uintptr
var handCursor uintptr
var inputFrames []inputFrame
var focusedControl syscall.Handle
var currentTool = ID_NAV_PRINT
var currentFiles []string
var currentFolder string
var currentOutput string
var duplicateGroups []duplicateGroup
var duplicateRows []duplicateRow
var duplicateList syscall.Handle
var statusHandle, progressHandle, progressLabelHandle, editMain, comboMain, editA, editB, editC, editD, runButton syscall.Handle
var comboB, comboC, comboD, comboE, comboF syscall.Handle
var fontNormal, fontSmall, fontTitle, fontApp, fontButton syscall.Handle
var fontLauncherTitle, fontLauncherSection, fontLauncherCard, fontLauncherSide syscall.Handle
var brushBg, brushPanel, brushSidebar syscall.Handle
var busy bool
var currentHex string
var colorSwatchHandle, colorPreviewHandle, colorEyeHandle syscall.Handle
var colorHexEdit, colorRGBEdit syscall.Handle
var eyedropperDragging bool
var eyedropperPreviewReady bool
var eyedropperLiveR, eyedropperLiveG, eyedropperLiveB byte
var eyedropperFinalR, eyedropperFinalG, eyedropperFinalB byte = 255, 255, 255
var eyedropperLastSample time.Time

// 13x13 persistent GDI snapshot. Capturing the screen in one BitBlt is dramatically
// faster than calling GetPixel 169 times on every mouse move.
var eyedropperMemDC, eyedropperBitmap, eyedropperOldBitmap syscall.Handle
var eyedropperScreenDC syscall.Handle
var eyedropperLastX, eyedropperLastY int32 = -1 << 30, -1 << 30
var launchMode string
var mailMu sync.Mutex
var statusMailbox string
var progressMailbox int
var printerMailbox []string
var duplicateMailbox []duplicateGroup
var duplicateDeleteMailbox []string
var duplicateDeleteErr string
var colorMailbox string
var errorMailbox string

func p16(s string) *uint16     { p, _ := syscall.UTF16PtrFromString(s); return p }
func rgb(r, g, b byte) uintptr { return uintptr(uint32(r) | uint32(g)<<8 | uint32(b)<<16) }

func main() {
	var printWorkerJob string
	for _, a := range os.Args[1:] {
		if strings.HasPrefix(a, "--tool=") {
			launchMode = strings.TrimPrefix(a, "--tool=")
		}
		if strings.HasPrefix(a, "--print-worker=") {
			printWorkerJob = strings.TrimPrefix(a, "--print-worker=")
		}
		if !strings.HasPrefix(a, "--") {
			if st, err := os.Stat(a); err == nil && st.IsDir() {
				startupFolder = a
			} else if err == nil {
				startupFiles = appendUnique(startupFiles, a)
			}
		}
	}
	if printWorkerJob != "" {
		runPrintWorker(printWorkerJob)
		return
	}

	// Win32 windows and their message loop must stay on the OS thread that
	// created them. Locking the GUI goroutine prevents intermittent
	// "Not responding" states while background goroutines are active.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if launchMode == "" && !ensureSingleLauncherInstance() {
		return
	}
	defer releaseSingleLauncherInstance()
	if launchMode != "" {
		currentTool = toolIDFromMode(launchMode)
	}
	icc := INITCOMMONCONTROLSEX{DwSize: uint32(unsafe.Sizeof(INITCOMMONCONTROLSEX{})), DwICC: 0x00000021}
	procInitCommonControlsEx.Call(uintptr(unsafe.Pointer(&icc)))
	procOleInitialize.Call(0)
	defer procOleUninitialize.Call()

	hInst, _, _ := procGetModuleHandleW.Call(0)
	className := p16("JTSNUtilityWindow")
	cursor, _, _ := user32.NewProc("LoadCursorW").Call(0, 32512)
	brushBg = solidBrush(246, 248, 252)
	brushPanel = solidBrush(255, 255, 255)
	brushSidebar = solidBrush(255, 255, 255)
	appIconBig, appIconSmall, appIconBrand = loadEmbeddedAppIcons()
	appBrandBitmap = loadEmbeddedBrandBitmap()
	wc := WNDCLASSEX{CbSize: uint32(unsafe.Sizeof(WNDCLASSEX{})), LpfnWndProc: syscall.NewCallback(wndProc), HInstance: syscall.Handle(hInst), HCursor: syscall.Handle(cursor), HbrBackground: brushBg, LpszClassName: className, HIcon: appIconBig, HIconSm: appIconSmall}
	r, _, err := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	if r == 0 {
		panic(err)
	}

	title := "JTSN · 잡툴사니"
	// v5.1 dashboard launcher: compact desktop dashboard with a fixed sidebar
	// and card grid. Tool windows still open as independent processes.
	w, h := 1160, 760
	windowStyle := uint32(0x00CA0000 | WS_VISIBLE | WS_CLIPCHILDREN) // caption + sysmenu + minimize, fixed size
	if launchMode != "" {
		title = "JTSN · " + toolName(currentTool)
		w, h = 1040, 800
		// The eyedropper is intentionally a compact always-handy utility window,
		// closer to Color Cop than to the full-size document tools.
		if currentTool == ID_NAV_COLOR {
			w, h = 414, 566
		}
		// Tool windows use a fixed workspace. The previous resizable window used
		// absolute child-control coordinates, so shrinking it could make PDF controls overlap.
		windowStyle = uint32(0x00CA0000 | WS_VISIBLE | WS_CLIPCHILDREN)
	}
	mainHWND = createWindow(0, "JTSNUtilityWindow", title, windowStyle, 120, 70, w, h, 0, 0)
	if appIconBig != 0 {
		procSendMessageW.Call(uintptr(mainHWND), WM_SETICON, ICON_BIG, uintptr(appIconBig))
	}
	if appIconSmall != 0 {
		procSendMessageW.Call(uintptr(mainHWND), WM_SETICON, ICON_SMALL, uintptr(appIconSmall))
	}
	procShowWindow.Call(uintptr(mainHWND), SW_SHOW)
	procUpdateWindow.Call(uintptr(mainHWND))
	if launchMode == "" {
		launcherHotkeyRegistered = registerLauncherHotkey(mainHWND)
	}

	var msg MSG
	for {
		ret, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(ret) == -1 || ret == 0 {
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}
}

func toolIDFromMode(m string) int {
	switch m {
	case "print":
		return ID_NAV_PRINT
	case "pdf":
		return ID_NAV_PDF
	case "rename":
		return ID_NAV_RENAME
	case "folders":
		return ID_NAV_FOLDERS
	case "duplicate":
		return ID_NAV_DUP
	case "image":
		return ID_NAV_IMAGE
	case "color":
		return ID_NAV_COLOR
	case "text":
		return ID_NAV_TEXT
	}
	return ID_NAV_PRINT
}
func modeFromToolID(id int) string {
	switch id {
	case ID_NAV_PRINT:
		return "print"
	case ID_NAV_PDF:
		return "pdf"
	case ID_NAV_RENAME:
		return "rename"
	case ID_NAV_FOLDERS:
		return "folders"
	case ID_NAV_DUP:
		return "duplicate"
	case ID_NAV_IMAGE:
		return "image"
	case ID_NAV_COLOR:
		return "color"
	case ID_NAV_TEXT:
		return "text"
	}
	return "print"
}
func toolName(id int) string {
	switch id {
	case ID_NAV_PRINT:
		return "파일 일괄 인쇄"
	case ID_NAV_PDF:
		return "PDF 도구"
	case ID_NAV_RENAME:
		return "파일명 일괄 변경"
	case ID_NAV_FOLDERS:
		return "폴더 일괄 생성"
	case ID_NAV_DUP:
		return "중복파일 찾기"
	case ID_NAV_IMAGE:
		return "이미지 변환"
	case ID_NAV_COLOR:
		return "화면 스포이드"
	case ID_NAV_TEXT:
		return "텍스트 정리"
	}
	return "도구"
}
func launchTool(id int) {
	exe, err := os.Executable()
	if err != nil {
		info("실행 파일 경로를 찾을 수 없습니다.")
		return
	}
	cmd := exec.Command(exe, "--tool="+modeFromToolID(id))
	if err := cmd.Start(); err != nil {
		info("도구 실행 실패: " + err.Error())
		return
	}
	if launchMode == "" {
		rememberLauncherRecent(id)
		rebuildLauncher(mainHWND)
	}
}

func wndProc(hwnd syscall.Handle, msg uint32, wParam, lParam uintptr) uintptr {
	if launchMode != "" && currentTool == ID_NAV_PDF {
		if handled, ret := pdfWindowMessage(hwnd, msg, wParam, lParam); handled {
			return ret
		}
	}
	switch msg {
	case WM_CREATE:
		// CreateWindowExW sends WM_CREATE before createWindow() returns.
		// Store the actual hwnd immediately so child controls created by
		// renderTool() have the correct parent window.
		mainHWND = hwnd
		initFonts()
		if launchMode == "" {
			buildLauncher(hwnd)
		} else {
			procDragAcceptFiles.Call(uintptr(hwnd), 1)
			renderTool(currentTool)
			if len(startupFiles) > 0 {
				currentFiles = appendUnique(currentFiles, startupFiles...)
				refreshFileList()
			}
			if startupFolder != "" {
				currentFolder = startupFolder
				if currentTool == ID_NAV_FOLDERS {
					setText(editD, "기준 폴더: "+currentFolder)
				} else if currentTool == ID_NAV_DUP {
					setText(editD, "검사 폴더: "+currentFolder)
				}
			}
		}
		return 0
	case WM_PAINT:
		paintWindow(hwnd)
		return 0
	case WM_DRAWITEM:
		dis := (*DRAWITEMSTRUCT)(unsafe.Pointer(lParam))
		if dis != nil && dis.HwndItem != 0 {
			if kind, ok := buttonKinds[dis.HwndItem]; ok {
				drawOwnerButton(dis, kind)
				return 1
			}
		}
	case WM_CTLCOLORSTATIC:
		hdc := syscall.Handle(wParam)
		ctl := syscall.Handle(lParam)
		procSetBkMode.Call(uintptr(hdc), TRANSPARENT)
		if sidebarControls[ctl] {
			if mutedControls[ctl] {
				procSetTextColor.Call(uintptr(hdc), rgb(148, 163, 184))
			} else {
				procSetTextColor.Call(uintptr(hdc), rgb(255, 255, 255))
			}
			return uintptr(brushSidebar)
		}
		if panelControls[ctl] {
			if mutedControls[ctl] {
				procSetTextColor.Call(uintptr(hdc), rgb(100, 116, 139))
			} else {
				procSetTextColor.Call(uintptr(hdc), rgb(15, 23, 42))
			}
			return uintptr(brushPanel)
		}
		if headerControls[ctl] {
			if mutedControls[ctl] {
				procSetTextColor.Call(uintptr(hdc), rgb(100, 116, 139))
			} else {
				procSetTextColor.Call(uintptr(hdc), rgb(15, 23, 42))
			}
			return uintptr(brushBg)
		}
	case WM_CTLCOLOREDIT:
		hdc := syscall.Handle(wParam)
		procSetTextColor.Call(uintptr(hdc), rgb(15, 23, 42))
		return uintptr(brushPanel)
	case WM_MOUSEMOVE:
		// During eyedropper capture Windows routes mouse movement to this top-level
		// window even when the pointer is outside the app. Sample immediately instead
		// of waiting for the timer so the magnifier tracks the cursor smoothly.
		if currentTool == ID_NAV_COLOR && eyedropperDragging {
			updateEyedropperFromCursor(false)
			return 0
		}
	case WM_LBUTTONUP:
		if currentTool == ID_NAV_COLOR && eyedropperDragging {
			finishEyedropperDrag(true)
			return 0
		}
	case WM_TIMER:
		if wParam == ID_TIMER_EYEDROPPER && currentTool == ID_NAV_COLOR && eyedropperDragging {
			updateEyedropperFromCursor(false)
			state, _, _ := procGetAsyncKeyState.Call(VK_LBUTTON)
			if (state & 0x8000) == 0 {
				finishEyedropperDrag(true)
			}
			return 0
		}
	case WM_HOTKEY:
		if launchMode == "" && wParam == HOTKEY_ID {
			procShowWindow.Call(uintptr(hwnd), SW_RESTORE)
			procSetForegroundWindow.Call(uintptr(hwnd))
			return 0
		}
	case WM_COMMAND:
		id := int(wParam & 0xffff)
		notify := int((wParam >> 16) & 0xffff)
		ctl := syscall.Handle(lParam)
		if ctl != 0 {
			if cc, ok := customCombos[ctl]; ok && notify == 0 {
				if showCustomCombo(ctl) {
					if currentTool == ID_NAV_PDF {
						pdfHandleCommand(cc.ID, CBN_SELCHANGE)
					}
				}
				return 0
			}
		}
		if ctl != 0 {
			if notify == EN_SETFOCUS || notify == CBN_SETFOCUS {
				focusedControl = ctl
				procInvalidateRect.Call(uintptr(hwnd), 0, 0)
			} else if (notify == EN_KILLFOCUS || notify == CBN_KILLFOCUS) && focusedControl == ctl {
				focusedControl = 0
				procInvalidateRect.Call(uintptr(hwnd), 0, 0)
			}
		}
		if launchMode == "" {
			if id == ID_LAUNCH_SEARCH && notify == EN_CHANGE && !launcherRebuilding {
				launcherSearchQuery = getText(ctl)
				procPostMessageW.Call(uintptr(hwnd), WM_APP_SEARCH, 0, 0)
				return 0
			}
			switch {
			case id >= ID_NAV_PRINT && id <= ID_NAV_TEXT:
				launchTool(id)
			case id >= ID_SIDE_FAVORITES && id <= ID_SIDE_UTIL:
				launcherCategory = id
				rebuildLauncher(hwnd)
			case id == ID_RECENT_CLEAR:
				launcherRecent = nil
				saveLauncherRecent()
				rebuildLauncher(hwnd)
			case id == ID_LAUNCH_EDIT:
				info("즐겨찾기 편집은 다음 단계에서 도구 고정/순서 변경 기능으로 확장할 수 있습니다.\n\n현재 버전은 실제 제공되는 8개 도구를 기본 즐겨찾기로 표시합니다.")
			case id == ID_SIDE_SETTINGS:
				toggleExplorerContextMenus()
			case id == ID_SIDE_INFO:
				info("잡툴사니 · JTSN v5.1\n\n회사에서 자주 쓰는 파일·PDF·이미지·텍스트 작업을 빠르게 처리하는 로컬 유틸리티입니다.")
			}
		} else if currentTool == ID_NAV_PDF {
			pdfHandleCommand(id, notify)
		} else {
			handleAction(id)
		}
		return 0
	case WM_APP_SEARCH:
		if launchMode == "" {
			rebuildLauncher(hwnd)
		}
		return 0
	case WM_DROPFILES:
		if launchMode != "" {
			handleDrop(syscall.Handle(wParam))
		}
		return 0
	case WM_APP_STATUS:
		mailMu.Lock()
		s := statusMailbox
		mailMu.Unlock()
		if statusHandle != 0 {
			setText(statusHandle, s)
		}
		return 0
	case WM_APP_PROGRESS:
		mailMu.Lock()
		p := progressMailbox
		mailMu.Unlock()
		setProgress(p)
		return 0
	case WM_APP_PRINTERS:
		mailMu.Lock()
		ps := append([]string(nil), printerMailbox...)
		mailMu.Unlock()
		if currentTool == ID_NAV_PRINT && comboMain != 0 {
			comboReset(comboMain)
			if len(ps) == 0 {
				ps = []string{"기본 프린터"}
			}
			for _, p := range ps {
				comboAdd(comboMain, p)
			}
			comboSelect(comboMain, 0)
			setStatus(fmt.Sprintf("프린터 %d대 확인됨 · 파일을 끌어다 놓거나 추가해 주세요.", len(ps)))
		}
		return 0
	case WM_APP_DUPDONE:
		mailMu.Lock()
		duplicateGroups = append([]duplicateGroup(nil), duplicateMailbox...)
		mailMu.Unlock()
		showDuplicateResults()
		finishBusy()
		return 0
	case WM_APP_DUPDELETED:
		mailMu.Lock()
		deleted := append([]string(nil), duplicateDeleteMailbox...)
		errText := duplicateDeleteErr
		mailMu.Unlock()
		if errText != "" {
			errorBox("휴지통 이동 중 오류가 발생했습니다.\n\n" + errText)
		} else {
			applyDuplicateDeletion(deleted)
			showDuplicateResults()
			setStatus(fmt.Sprintf("%d개 파일을 휴지통으로 이동했습니다. 남은 중복 그룹 %d개", len(deleted), len(duplicateGroups)))
		}
		finishBusy()
		return 0
	case WM_APP_COLOR:
		mailMu.Lock()
		c := colorMailbox
		mailMu.Unlock()
		applyFinalColorString(c)
		setStatus("색상을 추출했습니다.")
		return 0
	case WM_APP_ERROR:
		mailMu.Lock()
		s := errorMailbox
		mailMu.Unlock()
		errorBox(s)
		return 0
	case WM_APP_TASKDONE:
		finishBusy()
		return 0
	case WM_CLOSE:
		if busy && ask("작업이 진행 중입니다. 이 도구 창을 닫을까요?") != IDYES {
			return 0
		}
		procDestroyWindow.Call(uintptr(hwnd))
		return 0
	case WM_DESTROY:
		if launcherHotkeyRegistered {
			procUnregisterHotKey.Call(uintptr(hwnd), HOTKEY_ID)
			launcherHotkeyRegistered = false
		}
		if eyedropperDragging {
			finishEyedropperDrag(false)
		}
		destroyEyedropperBuffer()
		if appBrandBitmap != 0 {
			procDeleteObject.Call(uintptr(appBrandBitmap))
			appBrandBitmap = 0
		}
		procPostQuitMessage.Call(0)
		return 0
	}
	ret, _, _ := procDefWindowProcW.Call(uintptr(hwnd), uintptr(msg), wParam, lParam)
	return ret
}

func ensureSingleLauncherInstance() bool {
	h, _, callErr := procCreateMutexW.Call(0, 0, uintptr(unsafe.Pointer(p16(`Local\JTSNUtilityLauncher`))))
	launcherMutex = syscall.Handle(h)
	if callErr == syscall.Errno(183) {
		existing, _, _ := procFindWindowW.Call(uintptr(unsafe.Pointer(p16("JTSNUtilityWindow"))), 0)
		if existing != 0 {
			procShowWindow.Call(existing, SW_RESTORE)
			procSetForegroundWindow.Call(existing)
		}
		return false
	}
	return h != 0
}

func releaseSingleLauncherInstance() {
	if launcherMutex != 0 {
		procCloseHandle.Call(uintptr(launcherMutex))
		launcherMutex = 0
	}
}

func registerLauncherHotkey(hwnd syscall.Handle) bool {
	// v5.60의 기본 호출키를 먼저 복원합니다. 설정 UI에서 변경하는 기능은
	// 다음 복원 단계에서 이 등록 함수를 재사용합니다.
	r, _, _ := procRegisterHotKey.Call(uintptr(hwnd), HOTKEY_ID, MOD_CONTROL|MOD_SHIFT, uintptr('J'))
	return r != 0
}

func initFonts() {
	fontNormal = createFont(-17, 400, "Segoe UI Variable Text")
	fontSmall = createFont(-15, 400, "Segoe UI Variable Text")
	fontButton = createFont(-16, 600, "Segoe UI Variable Text")
	fontTitle = createFont(-29, 700, "Segoe UI Variable Display")
	fontApp = createFont(-23, 700, "Segoe UI Variable Display")
	fontLauncherTitle = createFont(-26, 700, "Segoe UI Variable Display")
	fontLauncherSection = createFont(-20, 700, "Segoe UI Variable Text")
	fontLauncherCard = createFont(-18, 650, "Segoe UI Variable Text")
	fontLauncherSide = createFont(-17, 550, "Segoe UI Variable Text")
}
func createFont(height int32, weight uintptr, face string) syscall.Handle {
	h, _, _ := procCreateFontW.Call(uintptr(height), 0, 0, 0, weight, 0, 0, 0, 1, 0, 0, 5, 0, uintptr(unsafe.Pointer(p16(face))))
	return syscall.Handle(h)
}
func solidBrush(r, g, b byte) syscall.Handle {
	h, _, _ := procCreateSolidBrush.Call(rgb(r, g, b))
	return syscall.Handle(h)
}

func paintWindow(hwnd syscall.Handle) {
	var ps PAINTSTRUCT
	hdc, _, _ := procBeginPaint.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&ps)))
	if hdc == 0 {
		return
	}
	defer procEndPaint.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&ps)))
	var client RECT
	procGetClientRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&client)))
	procFillRect.Call(hdc, uintptr(unsafe.Pointer(&client)), uintptr(brushBg))

	if launchMode == "" {
		// v5.1 dashboard shell: white navigation rail, soft-gray workspace,
		// and a quiet footer. Cards themselves are owner-drawn controls.
		side := RECT{0, 0, 246, client.Bottom}
		procFillRect.Call(hdc, uintptr(unsafe.Pointer(&side)), uintptr(brushPanel))
		sepPen, _, _ := procCreatePen.Call(PS_SOLID, 1, rgb(226, 232, 240))
		oldSep, _, _ := procSelectObject.Call(hdc, sepPen)
		procMoveToEx.Call(hdc, 246, 0, 0)
		procLineTo.Call(hdc, 246, uintptr(client.Bottom))
		procSelectObject.Call(hdc, oldSep)
		procDeleteObject.Call(sepPen)

		// Recent-use tray and footer give the dashboard the same visual
		// hierarchy as a modern desktop utility without embedding a browser.
		recentTop := int32(558)
		if client.Bottom < 720 {
			recentTop = client.Bottom - 164
		}
		drawSoftCard(syscall.Handle(hdc), RECT{282, recentTop, client.Right - 28, recentTop + 88}, 16, rgb(226, 232, 240), rgb(255, 255, 255))
		footerY := client.Bottom - 42
		footer := RECT{0, footerY, client.Right, client.Bottom}
		footerBrush := solidBrush(249, 250, 252)
		procFillRect.Call(hdc, uintptr(unsafe.Pointer(&footer)), uintptr(footerBrush))
		procDeleteObject.Call(uintptr(footerBrush))
		footerPen, _, _ := procCreatePen.Call(PS_SOLID, 1, rgb(226, 232, 240))
		oldFooter, _, _ := procSelectObject.Call(hdc, footerPen)
		procMoveToEx.Call(hdc, 0, uintptr(footerY), 0)
		procLineTo.Call(hdc, uintptr(client.Right), uintptr(footerY))
		procSelectObject.Call(hdc, oldFooter)
		procDeleteObject.Call(footerPen)

		drawLauncherBrand(syscall.Handle(hdc))
		return
	}

	// Main workspace card. The screen eyedropper uses a compact Color-Cop-like layout.
	if currentTool == ID_NAV_COLOR {
		drawSoftCard(syscall.Handle(hdc), RECT{14, 62, client.Right - 14, client.Bottom - 14}, 18, rgb(226, 232, 240), rgb(255, 255, 255))
	} else {
		drawSoftCard(syscall.Handle(hdc), RECT{24, 88, 1016, 748}, 20, rgb(226, 232, 240), rgb(255, 255, 255))
	}

	if currentTool == ID_NAV_PDF {
		paintPDFCanvas(syscall.Handle(hdc))
	}

	// Rounded input surfaces are painted after custom tool canvases so their borders stay visible.
	for _, f := range inputFrames {
		border := rgb(222, 228, 237)
		if f.Hwnd == focusedControl {
			border = rgb(74, 118, 245)
		}
		drawSoftCard(syscall.Handle(hdc), f.Rect, 10, border, rgb(255, 255, 255))
	}

	if currentTool == ID_NAV_PRINT || currentTool == ID_NAV_RENAME || currentTool == ID_NAV_IMAGE {
		sepPen, _, _ := procCreatePen.Call(PS_SOLID, 1, rgb(238, 242, 247))
		oldSep, _, _ := procSelectObject.Call(hdc, sepPen)
		gdi32.NewProc("MoveToEx").Call(hdc, 644, 118, 0)
		gdi32.NewProc("LineTo").Call(hdc, 644, 638)
		procSelectObject.Call(hdc, oldSep)
		procDeleteObject.Call(sepPen)
	}
}

func drawSoftCard(hdc syscall.Handle, rc RECT, radius int, borderColor, fillColor uintptr) {
	brush := solidBrush(byte(fillColor&0xff), byte((fillColor>>8)&0xff), byte((fillColor>>16)&0xff))
	pen, _, _ := procCreatePen.Call(PS_SOLID, 1, borderColor)
	oldB, _, _ := procSelectObject.Call(uintptr(hdc), uintptr(brush))
	oldP, _, _ := procSelectObject.Call(uintptr(hdc), pen)
	procRoundRect.Call(uintptr(hdc), uintptr(rc.Left), uintptr(rc.Top), uintptr(rc.Right), uintptr(rc.Bottom), uintptr(radius), uintptr(radius))
	procSelectObject.Call(uintptr(hdc), oldB)
	procSelectObject.Call(uintptr(hdc), oldP)
	procDeleteObject.Call(uintptr(brush))
	procDeleteObject.Call(pen)
}

func drawLauncherBrand(hdc syscall.Handle) {
	// Keep the complete user-supplied mark inside a generous sidebar area.
	// The bitmap already contains safe margins; we copy it 1:1 so neither
	// the JT/SN symbol nor the bottom wordmark is cropped or rescaled.
	if appBrandBitmap != 0 {
		memDC, _, _ := procCreateCompatibleDC.Call(uintptr(hdc))
		if memDC != 0 {
			old, _, _ := procSelectObject.Call(memDC, uintptr(appBrandBitmap))
			procBitBlt.Call(uintptr(hdc), 67, 18, 112, 112, memDC, 0, 0, 0x00CC0020)
			procSelectObject.Call(memDC, old)
			procDeleteDC.Call(memDC)
			return
		}
	}
	if appIconBrand != 0 {
		procDrawIconEx.Call(uintptr(hdc), 79, 30, uintptr(appIconBrand), 88, 88, 0, 0, DI_NORMAL)
	}
}

func loadEmbeddedAppIcons() (syscall.Handle, syscall.Handle, syscall.Handle) {
	if len(jtsnIconData) == 0 {
		return 0, 0, 0
	}
	cache, err := os.UserCacheDir()
	if err != nil || cache == "" {
		cache = os.TempDir()
	}
	dir := filepath.Join(cache, "JTSN", "brand")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return 0, 0, 0
	}
	iconPath := filepath.Join(dir, "jtsn.ico")
	// Overwrite to keep the icon in sync when a new build is deployed.
	if err := os.WriteFile(iconPath, jtsnIconData, 0644); err != nil {
		return 0, 0, 0
	}
	load := func(cx, cy int) syscall.Handle {
		r, _, _ := procLoadImageW.Call(0, uintptr(unsafe.Pointer(p16(iconPath))), IMAGE_ICON, uintptr(cx), uintptr(cy), LR_LOADFROMFILE)
		return syscall.Handle(r)
	}
	// Request larger source rasters than the final title-bar size so Windows only
	// downsamples. This avoids the previous small-raster -> upscale blur.
	return load(64, 64), load(32, 32), load(128, 128)
}

func loadEmbeddedBrandBitmap() syscall.Handle {
	if len(jtsnBrandBitmapData) == 0 {
		return 0
	}
	cache, err := os.UserCacheDir()
	if err != nil || cache == "" {
		cache = os.TempDir()
	}
	dir := filepath.Join(cache, "JTSN", "brand")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return 0
	}
	bmpPath := filepath.Join(dir, "jtsn_brand_v50.bmp")
	if err := os.WriteFile(bmpPath, jtsnBrandBitmapData, 0644); err != nil {
		return 0
	}
	r, _, _ := procLoadImageW.Call(0, uintptr(unsafe.Pointer(p16(bmpPath))), IMAGE_BITMAP, 0, 0, LR_LOADFROMFILE|LR_CREATEDIBSECTION)
	return syscall.Handle(r)
}

func launcherLabel(parent syscall.Handle, text string, x, y, w, h int, font syscall.Handle, muted bool, sidebar bool) syscall.Handle {
	hnd := createLabel(parent, text, x, y, w, h, font, muted, false)
	launcherControls = append(launcherControls, hnd)
	if sidebar {
		panelControls[hnd] = true
	} else {
		headerControls[hnd] = true
	}
	if muted {
		mutedControls[hnd] = true
	}
	return hnd
}

func launcherButton(parent syscall.Handle, text string, x, y, w, h, id, kind int) syscall.Handle {
	b := createOwnerButton(parent, text, x, y, w, h, id, kind)
	launcherControls = append(launcherControls, b)
	return b
}

func clearLauncherControls() {
	for _, h := range launcherControls {
		delete(sidebarControls, h)
		delete(panelControls, h)
		delete(headerControls, h)
		delete(mutedControls, h)
		delete(buttonKinds, h)
		delete(buttonIDs, h)
		delete(hoveredButtons, h)
		delete(buttonOldProcs, h)
		procDestroyWindow.Call(uintptr(h))
	}
	launcherControls = nil
	launcherSearchHandle = 0
}

func rebuildLauncher(hwnd syscall.Handle) {
	launcherRebuilding = true
	clearLauncherControls()
	buildLauncher(hwnd)
	launcherRebuilding = false
	procInvalidateRect.Call(uintptr(hwnd), 0, 1)
}

func launcherCategoryTitle() string {
	switch launcherCategory {
	case ID_SIDE_PDF:
		return "PDF 도구"
	case ID_SIDE_FILES:
		return "파일 / 폴더"
	case ID_SIDE_IMAGES:
		return "이미지 도구"
	case ID_SIDE_TEXT:
		return "텍스트 도구"
	case ID_SIDE_UTIL:
		return "유틸리티"
	default:
		return "즐겨찾기"
	}
}

func launcherToolsForCategory() []int {
	if q := strings.TrimSpace(strings.ToLower(launcherSearchQuery)); q != "" {
		var matches []int
		for _, id := range []int{ID_NAV_PDF, ID_NAV_PRINT, ID_NAV_RENAME, ID_NAV_FOLDERS, ID_NAV_DUP, ID_NAV_IMAGE, ID_NAV_COLOR, ID_NAV_TEXT} {
			if strings.Contains(strings.ToLower(toolSearchText(id)), q) {
				matches = append(matches, id)
			}
		}
		return matches
	}
	switch launcherCategory {
	case ID_SIDE_PDF:
		return []int{ID_NAV_PDF}
	case ID_SIDE_FILES:
		return []int{ID_NAV_PRINT, ID_NAV_RENAME, ID_NAV_FOLDERS, ID_NAV_DUP}
	case ID_SIDE_IMAGES:
		return []int{ID_NAV_IMAGE, ID_NAV_COLOR}
	case ID_SIDE_TEXT:
		return []int{ID_NAV_TEXT}
	case ID_SIDE_UTIL:
		return []int{ID_NAV_PRINT, ID_NAV_DUP, ID_NAV_COLOR, ID_NAV_TEXT}
	default:
		return []int{ID_NAV_PDF, ID_NAV_PRINT, ID_NAV_RENAME, ID_NAV_FOLDERS, ID_NAV_DUP, ID_NAV_IMAGE, ID_NAV_COLOR, ID_NAV_TEXT}
	}
}

func toolSearchText(id int) string {
	aliases := map[int]string{
		ID_NAV_PRINT:   "인쇄 프린트 출력 문서 파일",
		ID_NAV_PDF:     "pdf 병합 분할 추출 변환 문서",
		ID_NAV_RENAME:  "파일명 이름 변경 일괄 이름바꾸기",
		ID_NAV_FOLDERS: "폴더 생성 새폴더 일괄",
		ID_NAV_DUP:     "중복 파일 찾기 검사 삭제",
		ID_NAV_IMAGE:   "이미지 사진 변환 png jpg jpeg bmp webp",
		ID_NAV_COLOR:   "색상 컬러 스포이드 rgb hex 코드",
		ID_NAV_TEXT:    "텍스트 글자 정리 공백 줄바꿈",
	}
	return toolName(id) + " " + aliases[id]
}

type explorerMenuSpec struct {
	key   string
	label string
	mode  string
}

func explorerMenuSpecs() []explorerMenuSpec {
	return []explorerMenuSpec{
		{`HKCU\Software\Classes\*\shell\JTSN.Print`, "JTSN · 파일 일괄 인쇄", "print"},
		{`HKCU\Software\Classes\*\shell\JTSN.PDF`, "JTSN · PDF 도구", "pdf"},
		{`HKCU\Software\Classes\*\shell\JTSN.Rename`, "JTSN · 파일명 일괄 변경", "rename"},
		{`HKCU\Software\Classes\*\shell\JTSN.Image`, "JTSN · 이미지 변환", "image"},
		{`HKCU\Software\Classes\Directory\shell\JTSN.Duplicate`, "JTSN · 중복파일 찾기", "duplicate"},
		{`HKCU\Software\Classes\Directory\Background\shell\JTSN.Folders`, "JTSN · 폴더 일괄 생성", "folders"},
	}
}

func explorerContextMenusInstalled() bool {
	cmd := exec.Command("reg.exe", "query", explorerMenuSpecs()[0].key)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd.Run() == nil
}

func toggleExplorerContextMenus() {
	installed := explorerContextMenusInstalled()
	action := "등록"
	if installed {
		action = "해제"
	}
	if ask("Windows 탐색기 우클릭 메뉴를 "+action+"할까요?\n\n파일: 인쇄, PDF, 파일명 변경, 이미지 변환\n폴더: 중복파일 찾기, 폴더 일괄 생성") != IDYES {
		return
	}
	var err error
	if installed {
		err = uninstallExplorerContextMenus()
	} else {
		err = installExplorerContextMenus()
	}
	if err != nil {
		errorBox("탐색기 우클릭 메뉴 처리 중 오류가 발생했습니다.\n\n" + err.Error())
		return
	}
	if installed {
		info("탐색기 우클릭 메뉴를 해제했습니다.")
	} else {
		info("탐색기 우클릭 메뉴를 등록했습니다.\n\n파일이나 폴더를 우클릭하면 JTSN 도구를 바로 열 수 있습니다.")
	}
}

func installExplorerContextMenus() error {
	exePath, err := os.Executable()
	if err != nil {
		return err
	}
	for _, spec := range explorerMenuSpecs() {
		if err := runReg("add", spec.key, "/v", "MUIVerb", "/t", "REG_SZ", "/d", spec.label, "/f"); err != nil {
			return err
		}
		if err := runReg("add", spec.key, "/v", "Icon", "/t", "REG_SZ", "/d", exePath, "/f"); err != nil {
			return err
		}
		if strings.Contains(spec.key, `\*\shell\`) {
			if err := runReg("add", spec.key, "/v", "MultiSelectModel", "/t", "REG_SZ", "/d", "Player", "/f"); err != nil {
				return err
			}
		}
		placeholder := `%1`
		if strings.Contains(spec.key, `Directory\Background`) {
			placeholder = `%V`
		}
		command := fmt.Sprintf(`"%s" --tool=%s "%s"`, exePath, spec.mode, placeholder)
		if err := runReg("add", spec.key+`\command`, "/ve", "/t", "REG_SZ", "/d", command, "/f"); err != nil {
			return err
		}
	}
	return nil
}

func uninstallExplorerContextMenus() error {
	var firstErr error
	for _, spec := range explorerMenuSpecs() {
		if err := runReg("delete", spec.key, "/f"); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func runReg(args ...string) error {
	cmd := exec.Command("reg.exe", args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

func launcherRecentPath() string {
	cache, err := os.UserCacheDir()
	if err != nil || cache == "" {
		cache = os.TempDir()
	}
	return filepath.Join(cache, "JTSN", "launcher_recent.json")
}

func loadLauncherRecent() {
	if launcherRecentLoaded {
		return
	}
	launcherRecentLoaded = true
	b, err := os.ReadFile(launcherRecentPath())
	if err != nil {
		return
	}
	var ids []int
	if json.Unmarshal(b, &ids) != nil {
		return
	}
	for _, id := range ids {
		if id >= ID_NAV_PRINT && id <= ID_NAV_TEXT {
			launcherRecent = append(launcherRecent, id)
		}
		if len(launcherRecent) >= 4 {
			break
		}
	}
}

func saveLauncherRecent() {
	p := launcherRecentPath()
	_ = os.MkdirAll(filepath.Dir(p), 0755)
	b, _ := json.Marshal(launcherRecent)
	_ = os.WriteFile(p, b, 0644)
}

func rememberLauncherRecent(id int) {
	loadLauncherRecent()
	next := []int{id}
	for _, v := range launcherRecent {
		if v != id {
			next = append(next, v)
		}
		if len(next) >= 4 {
			break
		}
	}
	launcherRecent = next
	saveLauncherRecent()
}

func buildLauncher(hwnd syscall.Handle) {
	loadLauncherRecent()

	// Sidebar navigation. The full JTSN logo is painted by WM_PAINT above it.
	sideItems := []struct {
		id    int
		title string
	}{
		{ID_SIDE_FAVORITES, "즐겨찾기"},
		{ID_SIDE_PDF, "PDF 도구"},
		{ID_SIDE_FILES, "파일 / 폴더"},
		{ID_SIDE_IMAGES, "이미지 도구"},
		{ID_SIDE_TEXT, "텍스트 도구"},
		{ID_SIDE_UTIL, "유틸리티"},
	}
	sy := 142
	for i, item := range sideItems {
		launcherButton(hwnd, item.title, 16, sy+i*54, 214, 44, item.id, BTN_SIDEBAR)
	}

	// Bottom rail actions. They are intentionally quiet and secondary.
	launcherButton(hwnd, "우클릭 메뉴", 18, 618, 98, 42, ID_SIDE_SETTINGS, BTN_LAUNCH_GHOST)
	launcherButton(hwnd, "정보", 128, 618, 98, 42, ID_SIDE_INFO, BTN_LAUNCH_GHOST)

	// Main workspace heading.
	section := "☆  " + launcherCategoryTitle()
	if strings.TrimSpace(launcherSearchQuery) != "" {
		section = "⌕  검색 결과"
	}
	launcherLabel(hwnd, section, 282, 28, 300, 34, fontLauncherTitle, false, false)
	launcherSearchHandle = createWindow(0, "EDIT", launcherSearchQuery, WS_CHILD|WS_VISIBLE|WS_TABSTOP|WS_BORDER|ES_LEFT|ES_AUTOHSCROLL, 700, 25, 314, 36, hwnd, ID_LAUNCH_SEARCH)
	launcherControls = append(launcherControls, launcherSearchHandle)
	panelControls[launcherSearchHandle] = true
	sendFont(launcherSearchHandle, fontNormal)
	procSendMessageW.Call(uintptr(launcherSearchHandle), EM_SETMARGINS, EC_LEFTMARGIN|EC_RIGHTMARGIN, uintptr(10|(10<<16)))
	procSendMessageW.Call(uintptr(launcherSearchHandle), EM_SETCUEBANNER, 1, uintptr(unsafe.Pointer(p16("기능 검색..."))))
	if launcherSearchQuery != "" {
		procSendMessageW.Call(uintptr(launcherSearchHandle), EM_SETSEL, ^uintptr(0), ^uintptr(0))
		user32.NewProc("SetFocus").Call(uintptr(launcherSearchHandle))
	}
	if launcherCategory == ID_SIDE_FAVORITES && strings.TrimSpace(launcherSearchQuery) == "" {
		launcherButton(hwnd, "편집", 1026, 24, 86, 38, ID_LAUNCH_EDIT, BTN_LAUNCH_GHOST)
	}

	// Actual JTSN tools only: no decorative placeholder cards are created.
	tools := launcherToolsForCategory()
	cardW, cardH := 252, 126
	gapX, gapY := 16, 14
	startX, startY := 282, 82
	for i, id := range tools {
		col, row := i%3, i/3
		x := startX + col*(cardW+gapX)
		y := startY + row*(cardH+gapY)
		launcherButton(hwnd, toolName(id), x, y, cardW, cardH, id, BTN_LAUNCH_CARD)
	}
	if len(tools) == 0 {
		message := "표시할 도구가 없습니다."
		if strings.TrimSpace(launcherSearchQuery) != "" {
			message = "검색 결과가 없습니다. 다른 이름이나 키워드로 찾아보세요."
		}
		launcherLabel(hwnd, message, 282, 110, 560, 28, fontNormal, true, false)
	}

	// Recent-use section remains in a fixed tray so the dashboard does not jump
	// around when switching categories.
	launcherLabel(hwnd, "◷  최근 사용", 282, 520, 250, 28, fontLauncherSection, false, false)
	if len(launcherRecent) > 0 {
		launcherButton(hwnd, "모두 지우기", 1008, 516, 104, 34, ID_RECENT_CLEAR, BTN_LAUNCH_GHOST)
		for i, id := range launcherRecent {
			launcherButton(hwnd, toolName(id), 296+i*198, 571, 184, 60, id, BTN_RECENT)
		}
	} else {
		launcherLabel(hwnd, "아직 최근 사용 기록이 없습니다.", 304, 588, 360, 24, fontNormal, true, false)
	}

	// Footer.
	launcherLabel(hwnd, "JTSN v5.1", 24, 681, 130, 22, fontSmall, true, false)
	launcherLabel(hwnd, "사용 팁: 자주 쓰는 도구는 즐겨찾기 화면에서 바로 실행할 수 있습니다.", 620, 681, 492, 22, fontSmall, true, false)
}

func renderTool(id int) {
	currentTool = id
	currentFiles = nil
	currentFolder = ""
	currentOutput = ""
	duplicateGroups = nil
	duplicateRows = nil
	duplicateList = 0
	currentHex = ""
	colorSwatchHandle = 0
	colorPreviewHandle = 0
	colorEyeHandle = 0
	colorHexEdit = 0
	colorRGBEdit = 0
	eyedropperDragging = false
	eyedropperPreviewReady = false
	eyedropperFinalR, eyedropperFinalG, eyedropperFinalB = 255, 255, 255
	busy = false
	dynamicControls = nil
	inputFrames = nil
	focusedControl = 0
	statusHandle = 0
	progressHandle = 0
	progressLabelHandle = 0
	editMain = 0
	comboMain = 0
	editA = 0
	editB = 0
	editC = 0
	editD = 0
	comboB = 0
	comboC = 0
	comboD = 0
	comboE = 0
	comboF = 0
	runButton = 0
	switch id {
	case ID_NAV_PRINT:
		renderPrint()
	case ID_NAV_PDF:
		renderPDF()
	case ID_NAV_RENAME:
		renderRename()
	case ID_NAV_FOLDERS:
		renderFolders()
	case ID_NAV_DUP:
		renderDuplicate()
	case ID_NAV_IMAGE:
		renderImage()
	case ID_NAV_COLOR:
		renderColor()
	case ID_NAV_TEXT:
		renderText()
	}
}
func toolHeader(title, desc string) {
	t := createLabel(mainHWND, title, 34, 24, 720, 40, fontTitle, false, false)
	headerControls[t] = true
	dynamicControls = append(dynamicControls, t)
	procInvalidateRect.Call(uintptr(t), 0, 1)
	chip := createLabel(mainHWND, "JTSN", 914, 34, 88, 22, fontSmall, true, false)
	headerControls[chip] = true
	mutedControls[chip] = true
	dynamicControls = append(dynamicControls, chip)
	procInvalidateRect.Call(uintptr(chip), 0, 1)
}

func panelLabel(text string, x, y, w, h int, muted bool) syscall.Handle {
	hnd := createLabel(mainHWND, text, x, y, w, h, fontNormal, muted, false)
	panelControls[hnd] = true
	if muted {
		mutedControls[hnd] = true
	}
	dynamicControls = append(dynamicControls, hnd)
	procInvalidateRect.Call(uintptr(hnd), 0, 1)
	return hnd
}
func panelSmall(text string, x, y, w, h int, muted bool) syscall.Handle {
	hnd := createLabel(mainHWND, text, x, y, w, h, fontSmall, muted, false)
	panelControls[hnd] = true
	if muted {
		mutedControls[hnd] = true
	}
	dynamicControls = append(dynamicControls, hnd)
	procInvalidateRect.Call(uintptr(hnd), 0, 1)
	return hnd
}
func panelButton(text string, x, y, w, h, id, kind int) syscall.Handle {
	hnd := createOwnerButton(mainHWND, text, x, y, w, h, id, kind)
	dynamicControls = append(dynamicControls, hnd)
	return hnd
}
func panelEdit(text string, x, y, w, h int, multiline, readonly bool, id int) syscall.Handle {
	style := uint32(WS_CHILD | WS_VISIBLE | WS_TABSTOP | ES_LEFT)
	if multiline {
		style |= ES_MULTILINE | ES_AUTOVSCROLL | WS_VSCROLL | ES_WANTRETURN
	}
	if readonly {
		style |= ES_READONLY
	}
	if !multiline {
		style |= ES_AUTOHSCROLL
	}
	// EDIT itself is borderless; parent paints a rounded input frame and padding.
	ix, iy, iw, ih := x+8, y+5, w-16, h-10
	if multiline {
		ix, iy, iw, ih = x+8, y+7, w-16, h-14
	}
	hnd := createWindow(0, "EDIT", text, style, ix, iy, iw, ih, mainHWND, uintptr(id))
	sendFont(hnd, fontNormal)
	procSendMessageW.Call(uintptr(hnd), EM_SETMARGINS, EC_LEFTMARGIN|EC_RIGHTMARGIN, uintptr(6|(6<<16)))
	panelControls[hnd] = true
	dynamicControls = append(dynamicControls, hnd)
	inputFrames = append(inputFrames, inputFrame{Hwnd: hnd, Rect: RECT{int32(x), int32(y), int32(x + w), int32(y + h)}})
	return hnd
}

func panelCombo(x, y, w, h, id int) syscall.Handle {
	// Custom combo surface: a modern owner-drawn button + popup menu.
	// This avoids the classic Win32 COMBOBOX chrome that clashed with the rest of the UI.
	hnd := createOwnerButton(mainHWND, "선택", x, y, w, 36, id, BTN_COMBO)
	customCombos[hnd] = &customCombo{ID: id, Selected: -1}
	panelControls[hnd] = true
	dynamicControls = append(dynamicControls, hnd)
	return hnd
}

func makeStatus(initial string) {
	statusHandle = panelSmall(initial, 44, 682, 900, 24, true)
	progressLabelHandle = panelSmall("0%", 44, 714, 42, 20, true)
	progressHandle = createWindow(0, "msctls_progress32", "", WS_CHILD|WS_VISIBLE, 94, 719, 882, 8, mainHWND, 0)
	dynamicControls = append(dynamicControls, progressHandle)
	procSendMessageW.Call(uintptr(progressHandle), PBM_SETRANGE32, 0, 100)
	procSendMessageW.Call(uintptr(progressHandle), PBM_SETPOS, 0, 0)
	procShowWindow.Call(uintptr(progressLabelHandle), SW_HIDE)
	procShowWindow.Call(uintptr(progressHandle), SW_HIDE)
}

func renderPrint() {
	toolHeader("파일 일괄 인쇄", "PDF와 Word 파일을 끌어다 놓고 한 번에 출력합니다. 인쇄 작업은 별도 프로세스에서 처리됩니다.")
	panelLabel("인쇄할 파일", 44, 132, 180, 28, false)
	panelSmall("파일 또는 폴더를 아래 영역으로 끌어다 놓으세요.", 44, 162, 560, 22, true)
	editMain = panelEdit("", 44, 194, 580, 360, true, true, ID_EDIT_MAIN)
	panelButton("+ 파일 추가", 44, 574, 112, 38, ID_BTN_ADD, BTN_PRIMARY)
	panelButton("목록 비우기", 166, 574, 112, 38, ID_BTN_CLEAR, BTN_SECONDARY)

	panelLabel("인쇄 옵션", 660, 132, 160, 30, false)
	panelSmall("프린터", 660, 176, 120, 22, true)
	comboMain = panelCombo(660, 200, 316, 210, ID_COMBO_MAIN)
	comboAdd(comboMain, "프린터 불러오는 중...")
	comboSelect(comboMain, 0)

	panelSmall("인쇄 매수", 660, 248, 110, 22, true)
	panelSmall("페이지 범위", 824, 248, 130, 22, true)
	editA = panelEdit("1", 660, 272, 138, 34, false, false, ID_EDIT_A)
	editB = panelEdit("", 824, 272, 152, 34, false, false, ID_EDIT_B)
	panelSmall("비우면 전체", 824, 309, 130, 20, true)

	panelSmall("양면 설정", 660, 342, 120, 22, true)
	panelSmall("색상", 824, 342, 110, 22, true)
	comboB = panelCombo(660, 366, 138, 150, ID_COMBO_B)
	for _, v := range []string{"단면", "양면 - 긴쪽", "양면 - 짧은쪽"} {
		comboAdd(comboB, v)
	}
	comboSelect(comboB, 0)
	comboC = panelCombo(824, 366, 152, 120, ID_COMBO_C)
	comboAdd(comboC, "컬러")
	comboAdd(comboC, "흑백")
	comboSelect(comboC, 0)

	panelSmall("배율", 660, 416, 110, 22, true)
	panelSmall("용지", 824, 416, 110, 22, true)
	comboD = panelCombo(660, 440, 138, 150, ID_COMBO_D)
	for _, v := range []string{"용지에 맞춤", "실제 크기", "축소만"} {
		comboAdd(comboD, v)
	}
	comboSelect(comboD, 0)
	comboE = panelCombo(824, 440, 152, 150, ID_COMBO_E)
	for _, v := range []string{"프린터 기본", "A4", "A3", "Letter", "Legal"} {
		comboAdd(comboE, v)
	}
	comboSelect(comboE, 0)

	panelSmall("출력 정렬", 660, 492, 120, 22, true)
	comboF = panelCombo(660, 516, 316, 120, ID_COMBO_F)
	comboAdd(comboF, "한 부씩 정렬")
	comboAdd(comboF, "정렬 안 함")
	comboSelect(comboF, 0)

	runButton = panelButton("인쇄 시작", 660, 574, 316, 42, ID_BTN_RUN, BTN_PRIMARY)
	makeStatus("프린터 목록을 불러오는 중입니다...")
	go loadPrintersAsync()
}
func renderPDFLegacy() {
	toolHeader("PDF 도구", "PDF 파일을 끌어다 놓아 병합 · 페이지 추출 · 페이지별 분할을 실행합니다.")
	panelSmall("PDF 파일을 이 영역에 끌어다 놓으세요", 48, 132, 540, 26, true)
	editMain = panelEdit("", 48, 162, 555, 350, true, true, ID_EDIT_MAIN)
	panelButton("+ PDF 추가", 48, 527, 112, 38, ID_BTN_ADD, BTN_PRIMARY)
	panelButton("목록 비우기", 170, 527, 112, 38, ID_BTN_CLEAR, BTN_SECONDARY)
	panelLabel("작업 설정", 635, 132, 160, 30, false)
	panelSmall("작업", 635, 180, 70, 22, true)
	comboMain = panelCombo(705, 172, 195, 160, ID_COMBO_MAIN)
	for _, v := range []string{"PDF 병합", "페이지 추출", "페이지별 분할"} {
		comboAdd(comboMain, v)
	}
	comboSelect(comboMain, 0)
	panelSmall("페이지 범위", 635, 235, 100, 22, true)
	editA = panelEdit("1,3-5", 735, 228, 165, 32, false, false, ID_EDIT_A)
	panelSmall("추출 작업에서 사용합니다.", 635, 266, 260, 22, true)
	panelButton("출력 위치 선택", 635, 316, 150, 38, ID_BTN_OUTPUT, BTN_SECONDARY)
	editD = panelSmall("출력 위치: 선택되지 않음", 635, 364, 265, 55, true)
	runButton = panelButton("PDF 작업 실행", 635, 527, 265, 42, ID_BTN_RUN, BTN_PRIMARY)
	makeStatus("파일을 추가하거나 끌어다 놓으세요.")
}
func renderRename() {
	toolHeader("파일명 일괄 변경", "파일을 끌어다 놓고 접두어·접미어·문자 치환·연속번호를 한 번에 적용합니다.")
	panelLabel("변경할 파일", 44, 132, 180, 28, false)
	panelSmall("파일을 아래 영역으로 끌어다 놓으세요.", 44, 162, 520, 22, true)
	editMain = panelEdit("", 44, 194, 580, 360, true, true, ID_EDIT_MAIN)
	panelButton("+ 파일 추가", 44, 574, 112, 38, ID_BTN_ADD, BTN_PRIMARY)
	panelButton("목록 비우기", 166, 574, 112, 38, ID_BTN_CLEAR, BTN_SECONDARY)

	panelLabel("변경 규칙", 660, 132, 160, 30, false)
	panelSmall("접두어", 660, 184, 90, 22, true)
	editA = panelEdit("", 660, 208, 316, 34, false, false, ID_EDIT_A)
	panelSmall("접미어", 660, 260, 90, 22, true)
	editB = panelEdit("", 660, 284, 316, 34, false, false, ID_EDIT_B)
	panelSmall("찾을 문자", 660, 336, 100, 22, true)
	editC = panelEdit("", 660, 360, 316, 34, false, false, ID_EDIT_C)
	panelSmall("바꿀 문자", 660, 412, 100, 22, true)
	editD = panelEdit("", 660, 436, 316, 34, false, false, ID_EDIT_D)
	panelSmall("연속번호는 001, 002, 003… 순서로 앞에 붙습니다.", 660, 492, 300, 46, true)
	runButton = panelButton("연속번호 + 변경", 660, 574, 316, 42, ID_BTN_RUN, BTN_PRIMARY)
	makeStatus("원본 확장자는 유지됩니다.")
}
func renderFolders() {
	toolHeader("폴더 일괄 생성", "기준 폴더를 선택한 뒤 만들 폴더 이름을 한 줄씩 입력하면 한 번에 생성합니다.")
	panelLabel("기준 위치", 44, 136, 120, 28, false)
	panelButton("기준 폴더 선택", 44, 172, 145, 38, ID_BTN_FOLDER, BTN_PRIMARY)
	editD = panelSmall("기준 폴더가 선택되지 않았습니다.", 206, 181, 760, 26, true)
	panelLabel("만들 폴더 목록", 44, 230, 180, 28, false)
	panelSmall("한 줄에 하나씩 입력하세요. TXT 파일을 끌어다 놓아 목록을 불러올 수도 있습니다.", 44, 260, 760, 22, true)
	editMain = panelEdit("기획팀\r\n재경팀\r\n총무팀", 44, 292, 932, 272, true, false, ID_EDIT_MAIN)
	runButton = panelButton("폴더 생성", 790, 586, 186, 42, ID_BTN_RUN, BTN_PRIMARY)
	makeStatus("폴더를 창에 끌어다 놓으면 기준 폴더로 바로 설정됩니다.")
}
func renderDuplicate() {
	toolHeader("중복파일 찾기", "")
	panelButton("검사 폴더", 44, 124, 118, 38, ID_BTN_FOLDER, BTN_SECONDARY)
	runButton = panelButton("중복 검사", 172, 124, 118, 38, ID_BTN_RUN, BTN_PRIMARY)
	panelButton("CSV 저장", 300, 124, 102, 38, ID_BTN_EXPORT, BTN_SECONDARY)
	panelButton("선택 휴지통 이동", 804, 124, 172, 38, ID_BTN_RECYCLE, BTN_DANGER)
	editD = panelSmall("검사 폴더: 선택되지 않음", 44, 176, 660, 24, true)
	panelSmall("삭제할 파일만 체크 · 그룹별 최소 1개 유지", 710, 176, 266, 24, true)

	duplicateList = createDuplicateList(44, 214, 932, 430)
	makeStatus("폴더를 선택한 뒤 중복 검사를 실행하세요.")
}

func renderImage() {
	toolHeader("이미지 변환", "PNG · JPG · GIF 이미지를 원하는 형식과 크기로 한 번에 변환합니다. 작업은 백그라운드에서 실행됩니다.")
	panelLabel("변환할 이미지", 44, 132, 180, 28, false)
	panelSmall("이미지를 아래 영역으로 끌어다 놓으세요.", 44, 162, 520, 22, true)
	editMain = panelEdit("", 44, 194, 580, 360, true, true, ID_EDIT_MAIN)
	panelButton("+ 이미지 추가", 44, 574, 125, 38, ID_BTN_ADD, BTN_PRIMARY)
	panelButton("출력 폴더", 179, 574, 112, 38, ID_BTN_FOLDER, BTN_SECONDARY)
	panelButton("목록 비우기", 301, 574, 112, 38, ID_BTN_CLEAR, BTN_SECONDARY)

	panelLabel("변환 설정", 660, 132, 160, 30, false)
	panelSmall("출력 형식", 660, 190, 100, 22, true)
	comboMain = panelCombo(660, 214, 316, 130, ID_COMBO_MAIN)
	comboAdd(comboMain, "PNG")
	comboAdd(comboMain, "JPG")
	comboSelect(comboMain, 0)
	panelSmall("이미지 크기", 660, 270, 110, 22, true)
	editA = panelEdit("100", 660, 294, 138, 34, false, false, ID_EDIT_A)
	panelSmall("%", 808, 301, 30, 22, true)
	panelSmall("출력 폴더를 지정하지 않으면 첫 번째 이미지와 같은 폴더에 저장합니다.", 660, 360, 300, 70, true)
	runButton = panelButton("변환 실행", 660, 574, 316, 42, ID_BTN_RUN, BTN_PRIMARY)
	makeStatus("이미지를 추가하거나 끌어다 놓으세요.")
}
func renderColor() {
	// Compact Color-Cop-like layout: final swatch at the top, live magnifier below.
	t := createLabel(mainHWND, "화면 스포이드", 20, 16, 250, 34, fontTitle, false, false)
	headerControls[t] = true
	dynamicControls = append(dynamicControls, t)
	chip := createLabel(mainHWND, "JTSN", 328, 24, 54, 22, fontSmall, true, false)
	headerControls[chip] = true
	mutedControls[chip] = true
	dynamicControls = append(dynamicControls, chip)

	panelSmall("선택 색상", 28, 78, 92, 20, true)
	colorSwatchHandle = createOwnerPanel(28, 102, 110, 88, BTN_COLOR_SWATCH)

	panelSmall("HEX", 158, 78, 56, 20, true)
	colorHexEdit = panelEdit("#FFFFFF", 158, 100, 214, 32, false, true, ID_EDIT_MAIN)
	editMain = colorHexEdit
	panelSmall("RGB", 158, 142, 56, 20, true)
	colorRGBEdit = panelEdit("RGB(255, 255, 255)", 158, 164, 214, 32, false, true, ID_EDIT_A)
	editA = colorRGBEdit
	panelButton("HEX 복사", 158, 208, 104, 34, ID_BTN_COPY, BTN_SECONDARY)

	panelSmall("이 아이콘을 누른 채 원하는 곳까지 끌어가세요", 28, 258, 275, 20, true)
	colorEyeHandle = createOwnerButton(mainHWND, "", 306, 244, 66, 66, ID_BTN_CAPTURE, BTN_EYEDROPPER)
	dynamicControls = append(dynamicControls, colorEyeHandle)

	panelSmall("실시간 확대", 28, 318, 100, 20, true)
	colorPreviewHandle = createOwnerPanel(28, 342, 344, 132, BTN_COLOR_PREVIEW)

	statusHandle = panelSmall("스포이드를 누른 채 드래그하고, 원하는 색에서 놓으세요.", 28, 490, 344, 34, true)
	progressHandle = 0
	progressLabelHandle = 0
	currentHex = "#FFFFFF"
}

func renderText() {
	toolHeader("텍스트 정리", "PDF나 웹에서 복사한 문장의 불필요한 줄바꿈과 연속 공백을 한 번에 정리합니다.")
	panelLabel("텍스트", 44, 136, 120, 28, false)
	panelSmall("직접 붙여넣거나 TXT 파일을 창에 끌어다 놓으세요.", 44, 166, 620, 22, true)
	editMain = panelEdit("여기에 텍스트를 붙여 넣으세요.", 44, 198, 932, 378, true, false, ID_EDIT_MAIN)
	runButton = panelButton("줄바꿈 + 공백 정리", 790, 596, 186, 42, ID_BTN_RUN, BTN_PRIMARY)
	makeStatus("원본 파일은 변경하지 않습니다.")
}

func handleAction(id int) {
	switch currentTool {
	case ID_NAV_PRINT:
		handlePrint(id)
	case ID_NAV_PDF:
		handlePDF(id)
	case ID_NAV_RENAME:
		handleRename(id)
	case ID_NAV_FOLDERS:
		handleFolders(id)
	case ID_NAV_DUP:
		handleDuplicate(id)
	case ID_NAV_IMAGE:
		handleImage(id)
	case ID_NAV_COLOR:
		handleColor(id)
	case ID_NAV_TEXT:
		handleText(id)
	}
}
func handlePrint(id int) {
	switch id {
	case ID_BTN_ADD:
		currentFiles = appendUnique(currentFiles, openFiles("인쇄할 파일 선택", "PDF / Word\x00*.pdf;*.doc;*.docx\x00모든 파일\x00*.*\x00\x00")...)
		refreshFileList()
	case ID_BTN_CLEAR:
		currentFiles = nil
		refreshFileList()
	case ID_BTN_RUN:
		if busy {
			return
		}
		if len(currentFiles) == 0 {
			info("파일을 먼저 추가해 주세요.")
			return
		}
		printer := comboText(comboMain)
		if strings.Contains(printer, "불러오는 중") {
			info("프린터 목록을 불러오는 중입니다.")
			return
		}
		copies, _ := strconv.Atoi(strings.TrimSpace(getText(editA)))
		if copies < 1 {
			copies = 1
		}
		if copies > 99 {
			copies = 99
		}
		opt := printOptions{Printer: printer, Copies: copies, Pages: strings.TrimSpace(getText(editB)), Duplex: comboIndex(comboB), Color: comboIndex(comboC), Scale: comboIndex(comboD), Paper: comboIndex(comboE), Collate: comboIndex(comboF) == 0}
		files := append([]string(nil), currentFiles...)
		startBusy("인쇄 작업 프로세스를 시작하고 있습니다...")
		go runPrintWorkerProcess(files, opt)
	}
}
func handlePDFLegacy(id int) {
	switch id {
	case ID_BTN_ADD:
		currentFiles = appendUnique(currentFiles, openFiles("PDF 파일 선택", "PDF 파일\x00*.pdf\x00\x00")...)
		refreshFileList()
	case ID_BTN_CLEAR:
		currentFiles = nil
		currentOutput = ""
		refreshFileList()
		updateOutputLabel()
	case ID_BTN_OUTPUT:
		op := comboIndex(comboMain)
		if op == 2 {
			currentOutput = pickFolder()
		} else {
			currentOutput = saveFile("output.pdf", "PDF 저장 위치", "PDF 파일\x00*.pdf\x00\x00")
		}
		updateOutputLabel()
	case ID_BTN_RUN:
		if busy {
			return
		}
		if len(currentFiles) == 0 {
			info("PDF 파일을 먼저 추가해 주세요.")
			return
		}
		op := comboIndex(comboMain)
		if currentOutput == "" {
			if op == 2 {
				currentOutput = pickFolder()
			} else {
				currentOutput = saveFile("output.pdf", "PDF 저장 위치", "PDF 파일\x00*.pdf\x00\x00")
			}
			updateOutputLabel()
			if currentOutput == "" {
				return
			}
		}
		files := append([]string(nil), currentFiles...)
		out := currentOutput
		pages := strings.TrimSpace(getText(editA))
		startBusy("PDF 엔진을 확인하고 있습니다...")
		go func() {
			q, err := ensureQPDF(func(s string, p int) { postStatus(s); postProgress(p) })
			if err != nil {
				postStatus("PDF 엔진 준비 실패: " + err.Error())
				postDone()
				return
			}
			postProgress(35)
			switch op {
			case 0:
				postStatus("PDF 병합 중...")
				err = pdfMerge(q, files, out)
			case 1:
				if pages == "" {
					pages = "1-z"
				}
				postStatus("페이지 추출 중...")
				err = pdfExtract(q, files[0], pages, out)
			case 2:
				postStatus("페이지별 분할 중...")
				err = pdfSplit(q, files[0], out)
			}
			if err != nil {
				postStatus("PDF 작업 오류: " + err.Error())
			} else {
				postStatus("PDF 작업이 완료되었습니다.")
			}
			postProgress(100)
			postDone()
		}()
	}
}
func handleRename(id int) {
	switch id {
	case ID_BTN_ADD:
		currentFiles = appendUnique(currentFiles, openFiles("이름을 변경할 파일 선택", "모든 파일\x00*.*\x00\x00")...)
		refreshFileList()
	case ID_BTN_CLEAR:
		currentFiles = nil
		refreshFileList()
	case ID_BTN_RUN:
		if len(currentFiles) == 0 {
			info("파일을 먼저 추가해 주세요.")
			return
		}
		if ask("선택한 파일의 이름을 실제로 변경할까요?") != IDYES {
			return
		}
		changed, errs := batchRename(currentFiles, getText(editA), getText(editB), getText(editC), getText(editD), true)
		currentFiles = changed
		refreshFileList()
		if len(errs) > 0 {
			setStatus(fmt.Sprintf("변경 완료 · %d건 오류", len(errs)))
		} else {
			setStatus(fmt.Sprintf("%d개 파일 이름을 변경했습니다.", len(changed)))
		}
	}
}
func handleFolders(id int) {
	switch id {
	case ID_BTN_FOLDER:
		currentFolder = pickFolder()
		if currentFolder != "" {
			setText(editD, "기준 폴더: "+currentFolder)
			setStatus("기준 폴더가 선택되었습니다.")
		}
	case ID_BTN_RUN:
		if currentFolder == "" {
			info("기준 폴더를 먼저 선택해 주세요.")
			return
		}
		n, errs := createFolders(currentFolder, splitLines(getText(editMain)))
		if len(errs) > 0 {
			setStatus(fmt.Sprintf("%d개 생성 · %d개 오류", n, len(errs)))
		} else {
			setStatus(fmt.Sprintf("%d개 폴더를 생성했습니다.", n))
		}
	}
}
func handleDuplicate(id int) {
	switch id {
	case ID_BTN_FOLDER:
		currentFolder = pickFolder()
		if currentFolder != "" {
			setText(editD, "검사 폴더: "+currentFolder)
			setStatus("검사 폴더가 선택되었습니다. [중복 검사]를 눌러주세요.")
		}
	case ID_BTN_RUN:
		if busy {
			return
		}
		if currentFolder == "" {
			info("검사할 폴더를 먼저 선택해 주세요.")
			return
		}
		folder := currentFolder
		listViewClear(duplicateList)
		duplicateRows = nil
		startBusy("파일 목록을 확인하고 있습니다...")
		go func() {
			gs := scanDuplicates(folder, func(s string, p int) { postStatus(s); postProgress(p) })
			mailMu.Lock()
			duplicateMailbox = gs
			mailMu.Unlock()
			procPostMessageW.Call(uintptr(mainHWND), WM_APP_DUPDONE, 0, 0)
		}()
	case ID_BTN_AUTOSELECT:
		if len(duplicateRows) == 0 {
			info("먼저 중복 검사를 실행해 주세요.")
			return
		}
		for i, r := range duplicateRows {
			listViewSetChecked(duplicateList, i, r.FileIndex > 0)
		}
		setStatus("각 그룹에서 첫 번째 파일 1개를 남기고 나머지를 삭제 대상으로 선택했습니다.")
	case ID_BTN_UNSELECT:
		for i := range duplicateRows {
			listViewSetChecked(duplicateList, i, false)
		}
		setStatus("모든 선택을 해제했습니다.")
	case ID_BTN_EXPORT:
		if len(duplicateGroups) == 0 {
			info("먼저 중복 검사를 실행해 주세요.")
			return
		}
		out := filepath.Join(currentFolder, "duplicate_files.csv")
		if err := exportDuplicateCSV(out, duplicateGroups); err != nil {
			errorBox(err.Error())
		} else {
			info("저장 완료:\n" + out)
		}
	case ID_BTN_RECYCLE:
		if busy {
			return
		}
		if len(duplicateRows) == 0 {
			info("삭제할 중복 결과가 없습니다.")
			return
		}
		files, bytes, err := selectedDuplicateFiles()
		if err != nil {
			errorBox(err.Error())
			return
		}
		if len(files) == 0 {
			info("휴지통으로 이동할 파일을 체크해 주세요.")
			return
		}
		if ask(fmt.Sprintf("선택한 %d개 파일(%s)을 Windows 휴지통으로 이동할까요?\n\n원본 폴더에서 즉시 사라지지만 휴지통에서 복원할 수 있습니다.", len(files), humanBytes(bytes))) != IDYES {
			return
		}
		startBusy("선택한 파일을 휴지통으로 이동하고 있습니다...")
		go func(targets []string) {
			n, err := recycleFiles(targets)
			mailMu.Lock()
			if err != nil {
				duplicateDeleteErr = err.Error()
				duplicateDeleteMailbox = nil
			} else {
				duplicateDeleteErr = ""
				duplicateDeleteMailbox = append([]string(nil), targets[:min(n, len(targets))]...)
			}
			mailMu.Unlock()
			procPostMessageW.Call(uintptr(mainHWND), WM_APP_DUPDELETED, uintptr(n), 0)
		}(append([]string(nil), files...))
	}
}

func handleImage(id int) {
	switch id {
	case ID_BTN_ADD:
		currentFiles = appendUnique(currentFiles, openFiles("이미지 선택", "이미지\x00*.png;*.jpg;*.jpeg;*.gif\x00모든 파일\x00*.*\x00\x00")...)
		refreshFileList()
	case ID_BTN_CLEAR:
		currentFiles = nil
		refreshFileList()
	case ID_BTN_FOLDER:
		currentFolder = pickFolder()
		if currentFolder != "" {
			setStatus("출력 폴더: " + currentFolder)
		}
	case ID_BTN_RUN:
		if busy {
			return
		}
		if len(currentFiles) == 0 {
			info("이미지를 먼저 추가해 주세요.")
			return
		}
		pct, _ := strconv.Atoi(strings.TrimSpace(getText(editA)))
		if pct < 1 {
			pct = 100
		}
		if pct > 800 {
			pct = 800
		}
		format := strings.ToLower(comboText(comboMain))
		files := append([]string(nil), currentFiles...)
		out := currentFolder
		if out == "" {
			out = filepath.Dir(files[0])
		}
		startBusy("이미지 변환을 시작합니다...")
		go func() {
			n, errs := convertImages(files, out, format, pct, func(done, total int) {
				postProgress(done * 100 / max(1, total))
				postStatus(fmt.Sprintf("이미지 변환 중 · %d/%d", done, total))
			})
			if len(errs) > 0 {
				postStatus(fmt.Sprintf("완료 · %d개 성공 / %d개 오류", n, len(errs)))
			} else {
				postStatus(fmt.Sprintf("완료 · %d개 이미지 변환", n))
			}
			postProgress(100)
			postDone()
		}()
	}
}
func handleColor(id int) {
	switch id {
	case ID_BTN_CAPTURE:
		setStatus("스포이드 아이콘을 누른 채 화면 위로 드래그해 주세요.")
	case ID_BTN_COPY:
		if currentHex == "" {
			info("먼저 색상을 추출해 주세요.")
			return
		}
		if err := copyClipboard(currentHex); err != nil {
			errorBox(err.Error())
		} else {
			setStatus("HEX 값 " + currentHex + " 을 클립보드에 복사했습니다.")
		}
	}
}
func handleText(id int) {
	if id != ID_BTN_RUN {
		return
	}
	lines := splitLines(getText(editMain))
	var parts []string
	for _, ln := range lines {
		t := strings.Join(strings.Fields(ln), " ")
		if t != "" {
			parts = append(parts, t)
		}
	}
	setText(editMain, strings.Join(parts, " "))
	setStatus("줄바꿈과 연속 공백을 정리했습니다.")
}

func handleDrop(hDrop syscall.Handle) {
	paths := droppedPaths(hDrop)
	if len(paths) == 0 {
		return
	}
	switch currentTool {
	case ID_NAV_PRINT:
		var add []string
		for _, p := range paths {
			if info, err := os.Stat(p); err == nil && info.IsDir() {
				add = append(add, collectTopLevelFiles(p, map[string]bool{".pdf": true, ".doc": true, ".docx": true})...)
			} else if isExt(p, ".pdf", ".doc", ".docx") {
				add = append(add, p)
			}
		}
		currentFiles = appendUnique(currentFiles, add...)
		refreshFileList()
		setStatus(fmt.Sprintf("드래그앤드롭 완료 · 총 %d개 파일", len(currentFiles)))
	case ID_NAV_PDF:
		pdfHandleExternalDrop(paths)
	case ID_NAV_RENAME:
		var add []string
		for _, p := range paths {
			if st, err := os.Stat(p); err == nil && !st.IsDir() {
				add = append(add, p)
			}
		}
		currentFiles = appendUnique(currentFiles, add...)
		refreshFileList()
	case ID_NAV_FOLDERS:
		for _, p := range paths {
			if st, err := os.Stat(p); err == nil && st.IsDir() {
				currentFolder = p
				setText(editD, "기준 폴더: "+p)
				setStatus("기준 폴더를 드래그앤드롭으로 설정했습니다.")
				return
			}
			if isExt(p, ".txt") {
				if b, err := os.ReadFile(p); err == nil && len(b) <= 5*1024*1024 {
					setText(editMain, string(b))
					setStatus("TXT에서 폴더명 목록을 불러왔습니다.")
					return
				}
			}
		}
	case ID_NAV_DUP:
		for _, p := range paths {
			if st, err := os.Stat(p); err == nil && st.IsDir() {
				currentFolder = p
				setText(editD, "검사 폴더: "+p)
				setStatus("검사 폴더를 드래그앤드롭으로 설정했습니다. [중복 검사]를 눌러주세요.")
				return
			}
		}
	case ID_NAV_IMAGE:
		var add []string
		for _, p := range paths {
			if isExt(p, ".png", ".jpg", ".jpeg", ".gif") {
				add = append(add, p)
			}
		}
		currentFiles = appendUnique(currentFiles, add...)
		refreshFileList()
	case ID_NAV_COLOR:
		for _, p := range paths {
			if isExt(p, ".png", ".jpg", ".jpeg", ".gif") {
				setStatus("이미지 중앙 색상을 읽고 있습니다...")
				go func(path string) {
					hex, rgbv, err := sampleImageCenter(path)
					if err != nil {
						postStatus("이미지 색상 추출 실패: " + err.Error())
						return
					}
					mailMu.Lock()
					colorMailbox = hex + "    " + rgbv
					mailMu.Unlock()
					procPostMessageW.Call(uintptr(mainHWND), WM_APP_COLOR, 0, 0)
				}(p)
				return
			}
		}
	case ID_NAV_TEXT:
		for _, p := range paths {
			if isExt(p, ".txt", ".csv", ".log") {
				if b, err := os.ReadFile(p); err == nil && len(b) <= 10*1024*1024 {
					setText(editMain, string(b))
					setStatus("텍스트 파일을 불러왔습니다.")
					return
				}
			}
		}
	}
}
func droppedPaths(hDrop syscall.Handle) []string {
	defer procDragFinish.Call(uintptr(hDrop))
	count, _, _ := procDragQueryFileW.Call(uintptr(hDrop), 0xFFFFFFFF, 0, 0)
	var out []string
	for i := uintptr(0); i < count; i++ {
		n, _, _ := procDragQueryFileW.Call(uintptr(hDrop), i, 0, 0)
		if n == 0 {
			continue
		}
		buf := make([]uint16, n+1)
		procDragQueryFileW.Call(uintptr(hDrop), i, uintptr(unsafe.Pointer(&buf[0])), n+1)
		p := syscall.UTF16ToString(buf)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
func isExt(p string, exts ...string) bool {
	e := strings.ToLower(filepath.Ext(p))
	for _, x := range exts {
		if e == x {
			return true
		}
	}
	return false
}
func collectTopLevelFiles(dir string, allow map[string]bool) []string {
	es, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range es {
		if e.IsDir() {
			continue
		}
		p := filepath.Join(dir, e.Name())
		if allow[strings.ToLower(filepath.Ext(p))] {
			out = append(out, p)
		}
	}
	sort.Strings(out)
	return out
}
func sampleImageCenter(path string) (string, string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", "", err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return "", "", err
	}
	b := img.Bounds()
	c := color.NRGBAModel.Convert(img.At(b.Min.X+b.Dx()/2, b.Min.Y+b.Dy()/2)).(color.NRGBA)
	return fmt.Sprintf("#%02X%02X%02X", c.R, c.G, c.B), fmt.Sprintf("RGB(%d, %d, %d)", c.R, c.G, c.B), nil
}

func startBusy(status string) {
	busy = true
	setStatus(status)
	if progressLabelHandle != 0 {
		procShowWindow.Call(uintptr(progressLabelHandle), SW_SHOW)
	}
	if progressHandle != 0 {
		procShowWindow.Call(uintptr(progressHandle), SW_SHOW)
	}
	setProgress(0)
	if runButton != 0 {
		procEnableWindow.Call(uintptr(runButton), 0)
	}
}
func finishBusy() {
	busy = false
	if runButton != 0 {
		procEnableWindow.Call(uintptr(runButton), 1)
	}
	if progressLabelHandle != 0 {
		procShowWindow.Call(uintptr(progressLabelHandle), SW_HIDE)
	}
	if progressHandle != 0 {
		procShowWindow.Call(uintptr(progressHandle), SW_HIDE)
	}
}
func postDone() { procPostMessageW.Call(uintptr(mainHWND), WM_APP_TASKDONE, 0, 0) }
func postStatus(s string) {
	mailMu.Lock()
	statusMailbox = s
	mailMu.Unlock()
	procPostMessageW.Call(uintptr(mainHWND), WM_APP_STATUS, 0, 0)
}
func postError(s string) {
	mailMu.Lock()
	errorMailbox = s
	mailMu.Unlock()
	procPostMessageW.Call(uintptr(mainHWND), WM_APP_ERROR, 0, 0)
}

func postProgress(p int) {
	if p < 0 {
		p = 0
	}
	if p > 100 {
		p = 100
	}
	mailMu.Lock()
	progressMailbox = p
	mailMu.Unlock()
	procPostMessageW.Call(uintptr(mainHWND), WM_APP_PROGRESS, 0, 0)
}
func setStatus(s string) {
	if statusHandle != 0 {
		setText(statusHandle, s)
	}
}
func setProgress(p int) {
	if p < 0 {
		p = 0
	}
	if p > 100 {
		p = 100
	}
	if progressLabelHandle != 0 {
		setText(progressLabelHandle, fmt.Sprintf("%d%%", p))
	}
	if progressHandle != 0 {
		procSendMessageW.Call(uintptr(progressHandle), PBM_SETPOS, uintptr(p), 0)
	}
}
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func showDuplicateResults() {
	listViewClear(duplicateList)
	duplicateRows = nil
	if len(duplicateGroups) == 0 {
		setStatus("검사 완료 · 중복파일이 없습니다.")
		setProgress(100)
		return
	}
	dupCount := 0
	var waste int64
	row := 0
	for gi, g := range duplicateGroups {
		for fi, f := range g.Files {
			r := duplicateRow{Group: gi, FileIndex: fi, File: f, Size: g.Size, Hash: g.Hash}
			duplicateRows = append(duplicateRows, r)
			kind := "중복"
			if fi == 0 {
				kind = "유지 권장"
			}
			listViewAddRow(duplicateList, row, []string{strconv.Itoa(gi + 1), kind, filepath.Base(f), filepath.Dir(f), humanBytes(g.Size)})
			listViewSetChecked(duplicateList, row, false)
			row++
		}
		dupCount += len(g.Files) - 1
		waste += int64(len(g.Files)-1) * g.Size
	}
	setStatus(fmt.Sprintf("완료 · %d개 중복 그룹 / 정리 가능 %d개 / 최대 %s 절약 · 삭제할 파일만 체크하세요.", len(duplicateGroups), dupCount, humanBytes(waste)))
	setProgress(100)
}

func createDuplicateList(x, y, w, h int) syscall.Handle {
	style := uint32(WS_CHILD | WS_VISIBLE | WS_TABSTOP | LVS_REPORT | LVS_SHOWSELALWAYS | LVS_NOSORTHEADER)
	hnd := createWindow(0, "SysListView32", "", style, x+1, y+1, w-2, h-2, mainHWND, 0)
	inputFrames = append(inputFrames, inputFrame{Hwnd: hnd, Rect: RECT{int32(x), int32(y), int32(x + w), int32(y + h)}})
	sendFont(hnd, fontSmall)
	procSetWindowTheme.Call(uintptr(hnd), uintptr(unsafe.Pointer(p16("Explorer"))), 0)
	procSendMessageW.Call(uintptr(hnd), LVM_SETEXTENDEDLISTVIEWSTYLE, uintptr(LVS_EX_CHECKBOXES|LVS_EX_FULLROWSELECT|LVS_EX_DOUBLEBUFFER), uintptr(LVS_EX_CHECKBOXES|LVS_EX_FULLROWSELECT|LVS_EX_DOUBLEBUFFER))
	listViewAddColumn(hnd, 0, "그룹", 62)
	listViewAddColumn(hnd, 1, "상태", 88)
	listViewAddColumn(hnd, 2, "파일명", 260)
	listViewAddColumn(hnd, 3, "위치", 420)
	listViewAddColumn(hnd, 4, "크기", 90)
	dynamicControls = append(dynamicControls, hnd)
	return hnd
}

func listViewAddColumn(hwnd syscall.Handle, index int, text string, width int) {
	p := p16(text)
	c := LVCOLUMNW{Mask: LVCF_FMT | LVCF_WIDTH | LVCF_TEXT | LVCF_SUBITEM, Fmt: LVCFMT_LEFT, Cx: int32(width), PszText: p, ISubItem: int32(index)}
	procSendMessageW.Call(uintptr(hwnd), LVM_INSERTCOLUMNW, uintptr(index), uintptr(unsafe.Pointer(&c)))
}

func listViewAddRow(hwnd syscall.Handle, row int, values []string) {
	if hwnd == 0 || len(values) == 0 {
		return
	}
	p := p16(values[0])
	it := LVITEMW{Mask: LVIF_TEXT, IItem: int32(row), ISubItem: 0, PszText: p}
	procSendMessageW.Call(uintptr(hwnd), LVM_INSERTITEMW, 0, uintptr(unsafe.Pointer(&it)))
	for col := 1; col < len(values); col++ {
		t := p16(values[col])
		si := LVITEMW{IItem: int32(row), ISubItem: int32(col), PszText: t}
		procSendMessageW.Call(uintptr(hwnd), LVM_SETITEMTEXTW, uintptr(row), uintptr(unsafe.Pointer(&si)))
	}
}

func listViewClear(hwnd syscall.Handle) {
	if hwnd != 0 {
		procSendMessageW.Call(uintptr(hwnd), LVM_DELETEALLITEMS, 0, 0)
	}
}

func listViewSetChecked(hwnd syscall.Handle, row int, checked bool) {
	stateIndex := uint32(1)
	if checked {
		stateIndex = 2
	}
	it := LVITEMW{StateMask: LVIS_STATEIMAGEMASK, State: stateIndex << 12}
	procSendMessageW.Call(uintptr(hwnd), LVM_SETITEMSTATE, uintptr(row), uintptr(unsafe.Pointer(&it)))
}

func listViewChecked(hwnd syscall.Handle, row int) bool {
	r, _, _ := procSendMessageW.Call(uintptr(hwnd), LVM_GETITEMSTATE, uintptr(row), LVIS_STATEIMAGEMASK)
	return ((uint32(r) >> 12) & 0xF) == 2
}

func selectedDuplicateFiles() ([]string, int64, error) {
	selectedPerGroup := map[int]int{}
	var files []string
	var bytes int64
	for i, r := range duplicateRows {
		if listViewChecked(duplicateList, i) {
			selectedPerGroup[r.Group]++
			files = append(files, r.File)
			bytes += r.Size
		}
	}
	for gi, n := range selectedPerGroup {
		if gi >= 0 && gi < len(duplicateGroups) && n >= len(duplicateGroups[gi].Files) {
			return nil, 0, fmt.Errorf("그룹 %d의 파일을 전부 삭제 대상으로 선택했습니다.\n\n각 중복 그룹에는 최소 1개의 파일을 남겨야 합니다.", gi+1)
		}
	}
	return files, bytes, nil
}

func applyDuplicateDeletion(deleted []string) {
	set := map[string]bool{}
	for _, f := range deleted {
		set[strings.ToLower(filepath.Clean(f))] = true
	}
	var next []duplicateGroup
	for _, g := range duplicateGroups {
		ng := g
		ng.Files = nil
		for _, f := range g.Files {
			if !set[strings.ToLower(filepath.Clean(f))] {
				ng.Files = append(ng.Files, f)
			}
		}
		if len(ng.Files) > 1 {
			next = append(next, ng)
		}
	}
	duplicateGroups = next
}

func updateOutputLabel() {
	if editD == 0 {
		return
	}
	s := "출력 위치: 선택되지 않음"
	if currentOutput != "" {
		s = "출력 위치: " + currentOutput
	}
	setText(editD, s)
}

func createWindow(exStyle uint32, class, text string, style uint32, x, y, w, h int, parent syscall.Handle, id uintptr) syscall.Handle {
	hInst, _, _ := procGetModuleHandleW.Call(0)
	hwnd, _, _ := procCreateWindowExW.Call(uintptr(exStyle), uintptr(unsafe.Pointer(p16(class))), uintptr(unsafe.Pointer(p16(text))), uintptr(style), uintptr(x), uintptr(y), uintptr(w), uintptr(h), uintptr(parent), id, hInst, 0)
	return syscall.Handle(hwnd)
}
func createLabel(parent syscall.Handle, text string, x, y, w, h int, font syscall.Handle, muted, sidebar bool) syscall.Handle {
	hnd := createWindow(0, "STATIC", text, WS_CHILD|WS_VISIBLE, x, y, w, h, parent, 0)
	sendFont(hnd, font)
	if sidebar {
		sidebarControls[hnd] = true
	}
	if muted {
		mutedControls[hnd] = true
	}
	return hnd
}
func createOwnerButton(parent syscall.Handle, text string, x, y, w, h, id, kind int) syscall.Handle {
	hnd := createWindow(0, "BUTTON", text, WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_OWNERDRAW, x, y, w, h, parent, uintptr(id))
	sendFont(hnd, fontButton)
	buttonKinds[hnd] = kind
	buttonIDs[hnd] = id
	if buttonWndProcPtr == 0 {
		buttonWndProcPtr = syscall.NewCallback(buttonWndProc)
		handCursor, _, _ = user32.NewProc("LoadCursorW").Call(0, 32649)
	}
	old, _, _ := procSetWindowLongPtrW.Call(uintptr(hnd), ^uintptr(3), buttonWndProcPtr)
	buttonOldProcs[hnd] = old
	return hnd
}

func createOwnerPanel(x, y, w, h, kind int) syscall.Handle {
	hnd := createWindow(0, "BUTTON", "", WS_CHILD|WS_VISIBLE|BS_OWNERDRAW, x, y, w, h, mainHWND, 0)
	buttonKinds[hnd] = kind
	dynamicControls = append(dynamicControls, hnd)
	return hnd
}

func buttonWndProc(hwnd syscall.Handle, msg uint32, wParam, lParam uintptr) uintptr {
	id := buttonIDs[hwnd]
	if id == ID_BTN_CAPTURE {
		switch msg {
		case WM_LBUTTONDOWN:
			startEyedropperDrag()
			procInvalidateRect.Call(uintptr(hwnd), 0, 0)
			return 0
		case WM_MOUSEMOVE:
			// Mouse capture is kept as a convenience, but live sampling is timer-driven.
			// This keeps working even when Windows routes move messages to another window.
			if eyedropperDragging {
				return 0
			}
		case WM_LBUTTONUP:
			if eyedropperDragging {
				finishEyedropperDrag(true)
				return 0
			}
		case WM_SETCURSOR:
			cross, _, _ := user32.NewProc("LoadCursorW").Call(0, 32515)
			if cross != 0 {
				procSetCursor.Call(cross)
				return 1
			}
		}
	}
	switch msg {
	case WM_MOUSEMOVE:
		if !hoveredButtons[hwnd] {
			hoveredButtons[hwnd] = true
			tme := TRACKMOUSEEVENT{CbSize: uint32(unsafe.Sizeof(TRACKMOUSEEVENT{})), DwFlags: TME_LEAVE, HwndTrack: hwnd}
			procTrackMouseEvent.Call(uintptr(unsafe.Pointer(&tme)))
			procInvalidateRect.Call(uintptr(hwnd), 0, 0)
		}
	case WM_MOUSELEAVE:
		hoveredButtons[hwnd] = false
		procInvalidateRect.Call(uintptr(hwnd), 0, 0)
	case WM_SETCURSOR:
		if handCursor != 0 {
			procSetCursor.Call(handCursor)
			return 1
		}
	}
	if old := buttonOldProcs[hwnd]; old != 0 {
		r, _, _ := procCallWindowProcW.Call(old, uintptr(hwnd), uintptr(msg), wParam, lParam)
		return r
	}
	r, _, _ := procDefWindowProcW.Call(uintptr(hwnd), uintptr(msg), wParam, lParam)
	return r
}
func sendFont(hwnd, font syscall.Handle) {
	if hwnd != 0 && font != 0 {
		procSendMessageW.Call(uintptr(hwnd), WM_SETFONT, uintptr(font), 1)
	}
}
func setText(hwnd syscall.Handle, s string) {
	if hwnd != 0 {
		procSetWindowTextW.Call(uintptr(hwnd), uintptr(unsafe.Pointer(p16(s))))
	}
}
func getText(hwnd syscall.Handle) string {
	n, _, _ := procGetWindowTextLengthW.Call(uintptr(hwnd))
	if n == 0 {
		return ""
	}
	buf := make([]uint16, n+1)
	procGetWindowTextW.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&buf[0])), n+1)
	return syscall.UTF16ToString(buf)
}
func info(s string) {
	procMessageBoxW.Call(uintptr(mainHWND), uintptr(unsafe.Pointer(p16(s))), uintptr(unsafe.Pointer(p16("JTSN · 잡툴사니"))), MB_OK|MB_ICONINFORMATION)
}
func errorBox(s string) {
	procMessageBoxW.Call(uintptr(mainHWND), uintptr(unsafe.Pointer(p16(s))), uintptr(unsafe.Pointer(p16("JTSN · 잡툴사니"))), MB_OK|MB_ICONERROR)
}
func ask(s string) int {
	r, _, _ := procMessageBoxW.Call(uintptr(mainHWND), uintptr(unsafe.Pointer(p16(s))), uintptr(unsafe.Pointer(p16("확인"))), MB_YESNO|MB_ICONQUESTION)
	return int(r)
}
func comboAdd(hwnd syscall.Handle, s string) {
	if c, ok := customCombos[hwnd]; ok {
		c.Items = append(c.Items, s)
		if c.Selected < 0 {
			c.Selected = 0
			setText(hwnd, s)
		}
		return
	}
	procSendMessageW.Call(uintptr(hwnd), CB_ADDSTRING, 0, uintptr(unsafe.Pointer(p16(s))))
}
func comboReset(hwnd syscall.Handle) {
	if c, ok := customCombos[hwnd]; ok {
		c.Items = nil
		c.Selected = -1
		setText(hwnd, "선택")
		return
	}
	procSendMessageW.Call(uintptr(hwnd), CB_RESETCONTENT, 0, 0)
}
func comboSelect(hwnd syscall.Handle, idx int) {
	if c, ok := customCombos[hwnd]; ok {
		if idx >= 0 && idx < len(c.Items) {
			c.Selected = idx
			setText(hwnd, c.Items[idx])
			procInvalidateRect.Call(uintptr(hwnd), 0, 1)
		}
		return
	}
	procSendMessageW.Call(uintptr(hwnd), CB_SETCURSEL, uintptr(idx), 0)
}
func comboIndex(hwnd syscall.Handle) int {
	if c, ok := customCombos[hwnd]; ok {
		return c.Selected
	}
	idx, _, _ := procSendMessageW.Call(uintptr(hwnd), CB_GETCURSEL, 0, 0)
	return int(idx)
}
func comboText(hwnd syscall.Handle) string {
	if c, ok := customCombos[hwnd]; ok {
		if c.Selected >= 0 && c.Selected < len(c.Items) {
			return c.Items[c.Selected]
		}
		return ""
	}
	idx := comboIndex(hwnd)
	if idx < 0 {
		return ""
	}
	buf := make([]uint16, 1024)
	procSendMessageW.Call(uintptr(hwnd), CB_GETLBTEXT, uintptr(idx), uintptr(unsafe.Pointer(&buf[0])))
	return syscall.UTF16ToString(buf)
}

func showCustomCombo(hwnd syscall.Handle) bool {
	c, ok := customCombos[hwnd]
	if !ok || len(c.Items) == 0 {
		return false
	}
	menu, _, _ := procCreatePopupMenu.Call()
	if menu == 0 {
		return false
	}
	defer procDestroyMenu.Call(menu)
	const baseID = 50000
	for i, item := range c.Items {
		flags := uintptr(MF_STRING)
		if i == c.Selected {
			flags |= MF_CHECKED
		}
		procAppendMenuW.Call(menu, flags, uintptr(baseID+i), uintptr(unsafe.Pointer(p16(item))))
	}
	var rc RECT
	procGetWindowRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&rc)))
	r, _, _ := procTrackPopupMenu.Call(menu, TPM_RETURNCMD|TPM_NONOTIFY, uintptr(rc.Left), uintptr(rc.Bottom+3), 0, uintptr(mainHWND), 0)
	if r < baseID || int(r-baseID) >= len(c.Items) {
		return false
	}
	idx := int(r - baseID)
	if idx == c.Selected {
		return false
	}
	c.Selected = idx
	setText(hwnd, c.Items[idx])
	procInvalidateRect.Call(uintptr(hwnd), 0, 1)
	return true
}

func drawOwnerButton(dis *DRAWITEMSTRUCT, kind int) {
	if dis == nil {
		return
	}
	if kind == BTN_COLOR_SWATCH {
		drawColorSwatch(dis)
		return
	}
	if kind == BTN_COLOR_PREVIEW {
		drawColorPreview(dis)
		return
	}
	if kind == BTN_EYEDROPPER {
		drawEyedropperButton(dis)
		return
	}
	if launchMode == "" {
		switch kind {
		case BTN_SIDEBAR:
			drawLauncherSidebarButton(dis)
			return
		case BTN_LAUNCH_CARD:
			drawLauncherCardButton(dis)
			return
		case BTN_RECENT:
			drawLauncherRecentButton(dis)
			return
		case BTN_LAUNCH_GHOST:
			drawLauncherGhostButton(dis)
			return
		}
	}
	pressed := dis.ItemState&ODS_SELECTED != 0
	disabled := dis.ItemState&ODS_DISABLED != 0
	hovered := hoveredButtons[dis.HwndItem]
	launcherCard := kind == BTN_NAV && launchMode == ""

	fr, fg, fb := byte(255), byte(255), byte(255)
	br, bg, bb := byte(222), byte(228), byte(237)
	tr, tg, tb := byte(45), byte(55), byte(72)
	if launcherCard {
		if hovered {
			fr, fg, fb = 246, 249, 255
			br, bg, bb = 177, 198, 255
		}
	} else if kind == BTN_COMBO {
		fr, fg, fb = 255, 255, 255
		br, bg, bb = 218, 225, 235
		tr, tg, tb = 31, 41, 55
		if hovered {
			fr, fg, fb = 250, 252, 255
			br, bg, bb = 126, 157, 242
		}
	} else if kind == BTN_PRIMARY {
		fr, fg, fb = 47, 97, 235
		br, bg, bb = 47, 97, 235
		tr, tg, tb = 255, 255, 255
		if hovered {
			fr, fg, fb = 38, 84, 220
		}
	} else if kind == BTN_DANGER {
		fr, fg, fb = 255, 247, 247
		br, bg, bb = 254, 202, 202
		tr, tg, tb = 190, 24, 24
		if hovered {
			fr, fg, fb = 254, 238, 238
		}
	} else {
		if hovered {
			fr, fg, fb = 247, 249, 252
			br, bg, bb = 184, 198, 219
		}
	}
	if pressed {
		fr = byte(int(fr) * 94 / 100)
		fg = byte(int(fg) * 94 / 100)
		fb = byte(int(fb) * 94 / 100)
	}
	if disabled {
		fr, fg, fb = 235, 239, 244
		tr, tg, tb = 148, 163, 184
	}

	brush := solidBrush(fr, fg, fb)
	pen, _, _ := procCreatePen.Call(PS_SOLID, 1, rgb(br, bg, bb))
	oldB, _, _ := procSelectObject.Call(uintptr(dis.HDC), uintptr(brush))
	oldP, _, _ := procSelectObject.Call(uintptr(dis.HDC), pen)
	radius := uintptr(12)
	if launcherCard {
		radius = 16
	}
	procRoundRect.Call(uintptr(dis.HDC), uintptr(dis.RcItem.Left), uintptr(dis.RcItem.Top), uintptr(dis.RcItem.Right), uintptr(dis.RcItem.Bottom), radius, radius)
	procSelectObject.Call(uintptr(dis.HDC), oldB)
	procSelectObject.Call(uintptr(dis.HDC), oldP)
	procDeleteObject.Call(uintptr(brush))
	procDeleteObject.Call(pen)

	procSetBkMode.Call(uintptr(dis.HDC), TRANSPARENT)
	txt := getText(dis.HwndItem)
	oldF, _, _ := procSelectObject.Call(uintptr(dis.HDC), uintptr(fontButton))
	procSetTextColor.Call(uintptr(dis.HDC), rgb(tr, tg, tb))
	if kind == BTN_COMBO {
		oldF2, _, _ := procSelectObject.Call(uintptr(dis.HDC), uintptr(fontNormal))
		rc := dis.RcItem
		rc.Left += 12
		rc.Right -= 38
		procDrawTextW.Call(uintptr(dis.HDC), uintptr(unsafe.Pointer(p16(txt))), uintptr(len(syscall.StringToUTF16(txt))-1), uintptr(unsafe.Pointer(&rc)), DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS)
		procSetTextColor.Call(uintptr(dis.HDC), rgb(100, 116, 139))
		arrow := "⌄"
		ar := dis.RcItem
		ar.Left = ar.Right - 34
		ar.Right -= 8
		procDrawTextW.Call(uintptr(dis.HDC), uintptr(unsafe.Pointer(p16(arrow))), uintptr(len(syscall.StringToUTF16(arrow))-1), uintptr(unsafe.Pointer(&ar)), DT_CENTER|DT_VCENTER|DT_SINGLELINE)
		procSelectObject.Call(uintptr(dis.HDC), oldF2)
	} else if launcherCard {
		rc := dis.RcItem
		rc.Left += 22
		rc.Right -= 48
		procDrawTextW.Call(uintptr(dis.HDC), uintptr(unsafe.Pointer(p16(txt))), uintptr(len(syscall.StringToUTF16(txt))-1), uintptr(unsafe.Pointer(&rc)), DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS)
		// Subtle chevron reinforces that each row opens its own tool window.
		procSetTextColor.Call(uintptr(dis.HDC), rgb(132, 145, 164))
		chev := "›"
		cr := dis.RcItem
		cr.Left = cr.Right - 38
		cr.Right -= 14
		procDrawTextW.Call(uintptr(dis.HDC), uintptr(unsafe.Pointer(p16(chev))), uintptr(len(syscall.StringToUTF16(chev))-1), uintptr(unsafe.Pointer(&cr)), DT_CENTER|DT_VCENTER|DT_SINGLELINE)
	} else {
		rc := dis.RcItem
		procDrawTextW.Call(uintptr(dis.HDC), uintptr(unsafe.Pointer(p16(txt))), uintptr(len(syscall.StringToUTF16(txt))-1), uintptr(unsafe.Pointer(&rc)), DT_CENTER|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS)
	}
	procSelectObject.Call(uintptr(dis.HDC), oldF)
}

func launcherToolVisual(id int) (string, byte, byte, byte) {
	switch id {
	case ID_NAV_PDF:
		return "PDF", 225, 45, 45
	case ID_NAV_PRINT:
		return "PRN", 71, 85, 105
	case ID_NAV_RENAME:
		return "Aa", 53, 116, 235
	case ID_NAV_FOLDERS:
		return "DIR", 245, 158, 11
	case ID_NAV_DUP:
		return "DUP", 94, 107, 126
	case ID_NAV_IMAGE:
		return "IMG", 20, 160, 110
	case ID_NAV_COLOR:
		return "HEX", 126, 87, 194
	case ID_NAV_TEXT:
		return "TXT", 15, 118, 190
	default:
		return "J", 47, 97, 235
	}
}

func launcherSidebarGlyph(id int) string {
	switch id {
	case ID_SIDE_FAVORITES:
		return "★"
	case ID_SIDE_PDF:
		return "P"
	case ID_SIDE_FILES:
		return "▣"
	case ID_SIDE_IMAGES:
		return "▧"
	case ID_SIDE_TEXT:
		return "T"
	case ID_SIDE_UTIL:
		return "⌘"
	default:
		return "·"
	}
}

func drawLauncherSidebarButton(dis *DRAWITEMSTRUCT) {
	id := buttonIDs[dis.HwndItem]
	active := id == launcherCategory
	hovered := hoveredButtons[dis.HwndItem]
	pressed := dis.ItemState&ODS_SELECTED != 0

	fr, fg, fb := byte(255), byte(255), byte(255)
	tr, tg, tb := byte(31), byte(41), byte(55)
	if hovered {
		fr, fg, fb = 247, 250, 255
	}
	if active {
		fr, fg, fb = 38, 105, 235
		tr, tg, tb = 255, 255, 255
	}
	if pressed {
		fr = byte(int(fr) * 94 / 100)
		fg = byte(int(fg) * 94 / 100)
		fb = byte(int(fb) * 94 / 100)
	}
	brush := solidBrush(fr, fg, fb)
	pen, _, _ := procCreatePen.Call(PS_SOLID, 1, rgb(fr, fg, fb))
	oldB, _, _ := procSelectObject.Call(uintptr(dis.HDC), uintptr(brush))
	oldP, _, _ := procSelectObject.Call(uintptr(dis.HDC), pen)
	procRoundRect.Call(uintptr(dis.HDC), uintptr(dis.RcItem.Left), uintptr(dis.RcItem.Top), uintptr(dis.RcItem.Right), uintptr(dis.RcItem.Bottom), 14, 14)
	procSelectObject.Call(uintptr(dis.HDC), oldB)
	procSelectObject.Call(uintptr(dis.HDC), oldP)
	procDeleteObject.Call(uintptr(brush))
	procDeleteObject.Call(pen)

	procSetBkMode.Call(uintptr(dis.HDC), TRANSPARENT)
	procSetTextColor.Call(uintptr(dis.HDC), rgb(tr, tg, tb))
	oldF, _, _ := procSelectObject.Call(uintptr(dis.HDC), uintptr(fontLauncherSide))
	glyph := launcherSidebarGlyph(id)
	gr := dis.RcItem
	gr.Left += 15
	gr.Right = gr.Left + 30
	procDrawTextW.Call(uintptr(dis.HDC), uintptr(unsafe.Pointer(p16(glyph))), uintptr(len(syscall.StringToUTF16(glyph))-1), uintptr(unsafe.Pointer(&gr)), DT_CENTER|DT_VCENTER|DT_SINGLELINE)
	text := getText(dis.HwndItem)
	trc := dis.RcItem
	trc.Left += 54
	trc.Right -= 10
	procDrawTextW.Call(uintptr(dis.HDC), uintptr(unsafe.Pointer(p16(text))), uintptr(len(syscall.StringToUTF16(text))-1), uintptr(unsafe.Pointer(&trc)), DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS)
	procSelectObject.Call(uintptr(dis.HDC), oldF)
}

func drawLauncherCardButton(dis *DRAWITEMSTRUCT) {
	id := buttonIDs[dis.HwndItem]
	hovered := hoveredButtons[dis.HwndItem]
	pressed := dis.ItemState&ODS_SELECTED != 0
	fill := rgb(255, 255, 255)
	border := rgb(225, 231, 239)
	if hovered {
		fill = rgb(248, 251, 255)
		border = rgb(169, 194, 252)
	}
	if pressed {
		fill = rgb(241, 246, 255)
	}
	drawSoftCard(dis.HDC, dis.RcItem, 18, border, fill)

	code, r, g, b := launcherToolVisual(id)
	w := dis.RcItem.Right - dis.RcItem.Left
	cx := dis.RcItem.Left + w/2
	icon := RECT{cx - 29, dis.RcItem.Top + 18, cx + 29, dis.RcItem.Top + 76}
	ib := solidBrush(r, g, b)
	ip, _, _ := procCreatePen.Call(PS_SOLID, 1, rgb(r, g, b))
	oldB, _, _ := procSelectObject.Call(uintptr(dis.HDC), uintptr(ib))
	oldP, _, _ := procSelectObject.Call(uintptr(dis.HDC), ip)
	procRoundRect.Call(uintptr(dis.HDC), uintptr(icon.Left), uintptr(icon.Top), uintptr(icon.Right), uintptr(icon.Bottom), 15, 15)
	procSelectObject.Call(uintptr(dis.HDC), oldB)
	procSelectObject.Call(uintptr(dis.HDC), oldP)
	procDeleteObject.Call(uintptr(ib))
	procDeleteObject.Call(ip)

	procSetBkMode.Call(uintptr(dis.HDC), TRANSPARENT)
	procSetTextColor.Call(uintptr(dis.HDC), rgb(255, 255, 255))
	oldIconF, _, _ := procSelectObject.Call(uintptr(dis.HDC), uintptr(fontButton))
	procDrawTextW.Call(uintptr(dis.HDC), uintptr(unsafe.Pointer(p16(code))), uintptr(len(syscall.StringToUTF16(code))-1), uintptr(unsafe.Pointer(&icon)), DT_CENTER|DT_VCENTER|DT_SINGLELINE)
	procSelectObject.Call(uintptr(dis.HDC), oldIconF)

	title := getText(dis.HwndItem)
	procSetTextColor.Call(uintptr(dis.HDC), rgb(17, 24, 39))
	oldF, _, _ := procSelectObject.Call(uintptr(dis.HDC), uintptr(fontLauncherCard))
	tr := dis.RcItem
	tr.Top += 87
	tr.Bottom -= 12
	tr.Left += 10
	tr.Right -= 10
	procDrawTextW.Call(uintptr(dis.HDC), uintptr(unsafe.Pointer(p16(title))), uintptr(len(syscall.StringToUTF16(title))-1), uintptr(unsafe.Pointer(&tr)), DT_CENTER|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS)
	procSelectObject.Call(uintptr(dis.HDC), oldF)
}

func drawLauncherRecentButton(dis *DRAWITEMSTRUCT) {
	id := buttonIDs[dis.HwndItem]
	hovered := hoveredButtons[dis.HwndItem]
	fill := rgb(255, 255, 255)
	if hovered {
		fill = rgb(247, 250, 255)
	}
	// No hard border: the common recent tray already provides the container.
	brush := solidBrush(byte(fill&0xff), byte((fill>>8)&0xff), byte((fill>>16)&0xff))
	pen, _, _ := procCreatePen.Call(PS_SOLID, 1, byteToColorRef(255, 255, 255))
	oldB, _, _ := procSelectObject.Call(uintptr(dis.HDC), uintptr(brush))
	oldP, _, _ := procSelectObject.Call(uintptr(dis.HDC), pen)
	procRoundRect.Call(uintptr(dis.HDC), uintptr(dis.RcItem.Left), uintptr(dis.RcItem.Top), uintptr(dis.RcItem.Right), uintptr(dis.RcItem.Bottom), 12, 12)
	procSelectObject.Call(uintptr(dis.HDC), oldB)
	procSelectObject.Call(uintptr(dis.HDC), oldP)
	procDeleteObject.Call(uintptr(brush))
	procDeleteObject.Call(pen)

	code, r, g, b := launcherToolVisual(id)
	icon := RECT{dis.RcItem.Left + 10, dis.RcItem.Top + 11, dis.RcItem.Left + 48, dis.RcItem.Top + 49}
	ib := solidBrush(r, g, b)
	ip, _, _ := procCreatePen.Call(PS_SOLID, 1, rgb(r, g, b))
	ob, _, _ := procSelectObject.Call(uintptr(dis.HDC), uintptr(ib))
	op, _, _ := procSelectObject.Call(uintptr(dis.HDC), ip)
	procRoundRect.Call(uintptr(dis.HDC), uintptr(icon.Left), uintptr(icon.Top), uintptr(icon.Right), uintptr(icon.Bottom), 10, 10)
	procSelectObject.Call(uintptr(dis.HDC), ob)
	procSelectObject.Call(uintptr(dis.HDC), op)
	procDeleteObject.Call(uintptr(ib))
	procDeleteObject.Call(ip)
	procSetBkMode.Call(uintptr(dis.HDC), TRANSPARENT)
	procSetTextColor.Call(uintptr(dis.HDC), rgb(255, 255, 255))
	oldF, _, _ := procSelectObject.Call(uintptr(dis.HDC), uintptr(fontSmall))
	procDrawTextW.Call(uintptr(dis.HDC), uintptr(unsafe.Pointer(p16(code))), uintptr(len(syscall.StringToUTF16(code))-1), uintptr(unsafe.Pointer(&icon)), DT_CENTER|DT_VCENTER|DT_SINGLELINE)
	procSelectObject.Call(uintptr(dis.HDC), oldF)

	title := getText(dis.HwndItem)
	procSetTextColor.Call(uintptr(dis.HDC), rgb(31, 41, 55))
	oldT, _, _ := procSelectObject.Call(uintptr(dis.HDC), uintptr(fontSmall))
	tr := dis.RcItem
	tr.Left += 58
	tr.Right -= 6
	procDrawTextW.Call(uintptr(dis.HDC), uintptr(unsafe.Pointer(p16(title))), uintptr(len(syscall.StringToUTF16(title))-1), uintptr(unsafe.Pointer(&tr)), DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS)
	procSelectObject.Call(uintptr(dis.HDC), oldT)
}

func byteToColorRef(r, g, b byte) uintptr { return rgb(r, g, b) }

func drawLauncherGhostButton(dis *DRAWITEMSTRUCT) {
	hovered := hoveredButtons[dis.HwndItem]
	pressed := dis.ItemState&ODS_SELECTED != 0
	fr, fg, fb := byte(255), byte(255), byte(255)
	br, bg, bb := byte(222), byte(228), byte(237)
	if hovered {
		fr, fg, fb = 247, 250, 255
		br, bg, bb = 188, 204, 230
	}
	if pressed {
		fr, fg, fb = 239, 245, 255
	}
	brush := solidBrush(fr, fg, fb)
	pen, _, _ := procCreatePen.Call(PS_SOLID, 1, rgb(br, bg, bb))
	ob, _, _ := procSelectObject.Call(uintptr(dis.HDC), uintptr(brush))
	op, _, _ := procSelectObject.Call(uintptr(dis.HDC), pen)
	procRoundRect.Call(uintptr(dis.HDC), uintptr(dis.RcItem.Left), uintptr(dis.RcItem.Top), uintptr(dis.RcItem.Right), uintptr(dis.RcItem.Bottom), 12, 12)
	procSelectObject.Call(uintptr(dis.HDC), ob)
	procSelectObject.Call(uintptr(dis.HDC), op)
	procDeleteObject.Call(uintptr(brush))
	procDeleteObject.Call(pen)
	procSetBkMode.Call(uintptr(dis.HDC), TRANSPARENT)
	procSetTextColor.Call(uintptr(dis.HDC), rgb(55, 65, 81))
	oldF, _, _ := procSelectObject.Call(uintptr(dis.HDC), uintptr(fontSmall))
	txt := getText(dis.HwndItem)
	r := dis.RcItem
	procDrawTextW.Call(uintptr(dis.HDC), uintptr(unsafe.Pointer(p16(txt))), uintptr(len(syscall.StringToUTF16(txt))-1), uintptr(unsafe.Pointer(&r)), DT_CENTER|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS)
	procSelectObject.Call(uintptr(dis.HDC), oldF)
}

func openFiles(title, filterSpec string) []string {
	filterSpec = strings.ReplaceAll(filterSpec, "\\x00", "\x00")
	filter16 := utf16.Encode([]rune(filterSpec))
	buf := make([]uint16, 64*1024)
	ofn := OPENFILENAME{LStructSize: uint32(unsafe.Sizeof(OPENFILENAME{})), HwndOwner: mainHWND, LpstrFilter: &filter16[0], LpstrFile: &buf[0], NMaxFile: uint32(len(buf)), LpstrTitle: p16(title), Flags: OFN_EXPLORER | OFN_ALLOWMULTISELECT | OFN_FILEMUSTEXIST | OFN_PATHMUSTEXIST}
	r, _, _ := procGetOpenFileNameW.Call(uintptr(unsafe.Pointer(&ofn)))
	if r == 0 {
		return nil
	}
	var parts []string
	start := 0
	for i, v := range buf {
		if v == 0 {
			if i == start {
				break
			}
			parts = append(parts, syscall.UTF16ToString(buf[start:i]))
			start = i + 1
		}
	}
	if len(parts) == 1 {
		return parts
	}
	if len(parts) > 1 {
		dir := parts[0]
		out := make([]string, 0, len(parts)-1)
		for _, n := range parts[1:] {
			out = append(out, filepath.Join(dir, n))
		}
		return out
	}
	return nil
}
func saveFile(defaultName, title, filterSpec string) string {
	filterSpec = strings.ReplaceAll(filterSpec, "\\x00", "\x00")
	filter16 := utf16.Encode([]rune(filterSpec))
	buf := make([]uint16, 32768)
	copy(buf, syscall.StringToUTF16(defaultName))
	ofn := OPENFILENAME{LStructSize: uint32(unsafe.Sizeof(OPENFILENAME{})), HwndOwner: mainHWND, LpstrFilter: &filter16[0], LpstrFile: &buf[0], NMaxFile: uint32(len(buf)), LpstrTitle: p16(title), Flags: OFN_EXPLORER | OFN_PATHMUSTEXIST | OFN_OVERWRITEPROMPT, LpstrDefExt: p16("pdf")}
	r, _, _ := procGetSaveFileNameW.Call(uintptr(unsafe.Pointer(&ofn)))
	if r == 0 {
		return ""
	}
	return syscall.UTF16ToString(buf)
}
func pickFolder() string {
	display := make([]uint16, 260)
	bi := BROWSEINFO{HwndOwner: mainHWND, PszDisplayName: &display[0], LpszTitle: p16("폴더를 선택하세요"), UlFlags: BIF_RETURNONLYFSDIRS | BIF_USENEWUI}
	pidl, _, _ := procSHBrowseForFolderW.Call(uintptr(unsafe.Pointer(&bi)))
	if pidl == 0 {
		return ""
	}
	defer procCoTaskMemFree.Call(pidl)
	buf := make([]uint16, 32768)
	r, _, _ := procSHGetPathFromIDListW.Call(pidl, uintptr(unsafe.Pointer(&buf[0])))
	if r == 0 {
		return ""
	}
	return syscall.UTF16ToString(buf)
}

func appendUnique(base []string, items ...string) []string {
	seen := map[string]bool{}
	for _, x := range base {
		seen[strings.ToLower(x)] = true
	}
	for _, x := range items {
		if x != "" && !seen[strings.ToLower(x)] {
			base = append(base, x)
			seen[strings.ToLower(x)] = true
		}
	}
	return base
}
func refreshFileList() {
	if editMain == 0 {
		return
	}
	var b strings.Builder
	for i, f := range currentFiles {
		fmt.Fprintf(&b, "%02d   %s\r\n", i+1, f)
	}
	setText(editMain, b.String())
	if statusHandle != 0 {
		setStatus(fmt.Sprintf("총 %d개 파일", len(currentFiles)))
	}
}
func splitLines(s string) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	sc := bufio.NewScanner(strings.NewReader(s))
	var out []string
	for sc.Scan() {
		t := strings.TrimSpace(sc.Text())
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}

func loadPrintersAsync() {
	ps := nativePrinters()
	mailMu.Lock()
	printerMailbox = ps
	mailMu.Unlock()
	procPostMessageW.Call(uintptr(mainHWND), WM_APP_PRINTERS, 0, 0)
}
func nativePrinters() []string {
	defaultP := getDefaultPrinter()
	var needed, returned uint32
	procEnumPrintersW.Call(PRINTER_ENUM_LOCAL|PRINTER_ENUM_CONNECTIONS, 0, 4, 0, 0, uintptr(unsafe.Pointer(&needed)), uintptr(unsafe.Pointer(&returned)))
	if needed == 0 {
		return []string{"기본 프린터"}
	}
	buf := make([]byte, needed)
	r, _, _ := procEnumPrintersW.Call(PRINTER_ENUM_LOCAL|PRINTER_ENUM_CONNECTIONS, 0, 4, uintptr(unsafe.Pointer(&buf[0])), uintptr(needed), uintptr(unsafe.Pointer(&needed)), uintptr(unsafe.Pointer(&returned)))
	if r == 0 {
		return []string{"기본 프린터"}
	}
	sz := unsafe.Sizeof(PRINTER_INFO_4{})
	var names []string
	for i := uint32(0); i < returned; i++ {
		pi := (*PRINTER_INFO_4)(unsafe.Pointer(uintptr(unsafe.Pointer(&buf[0])) + uintptr(i)*sz))
		if pi.PPrinterName != nil {
			n := utf16PtrToString(pi.PPrinterName)
			if n != "" {
				names = append(names, n)
			}
		}
	}
	sort.SliceStable(names, func(i, j int) bool {
		if strings.EqualFold(names[i], defaultP) {
			return true
		}
		if strings.EqualFold(names[j], defaultP) {
			return false
		}
		return strings.ToLower(names[i]) < strings.ToLower(names[j])
	})
	if len(names) > 0 && defaultP != "" && strings.EqualFold(names[0], defaultP) {
		names[0] = "[기본] " + names[0]
	}
	return names
}
func getDefaultPrinter() string {
	var n uint32
	procGetDefaultPrinterW.Call(0, uintptr(unsafe.Pointer(&n)))
	if n == 0 {
		return ""
	}
	buf := make([]uint16, n)
	r, _, _ := procGetDefaultPrinterW.Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&n)))
	if r == 0 {
		return ""
	}
	return syscall.UTF16ToString(buf)
}
func utf16PtrToString(p *uint16) string {
	if p == nil {
		return ""
	}
	var a []uint16
	for i := uintptr(0); ; i += 2 {
		v := *(*uint16)(unsafe.Pointer(uintptr(unsafe.Pointer(p)) + i))
		if v == 0 {
			break
		}
		a = append(a, v)
	}
	return syscall.UTF16ToString(a)
}

type printWorkerJob struct {
	Files   []string     `json:"files"`
	Options printOptions `json:"options"`
}

func workerLine(kind string, p int, text string) {
	text = strings.ReplaceAll(text, "\r", " ")
	text = strings.ReplaceAll(text, "\n", " ")
	text = strings.ReplaceAll(text, "\t", " ")
	fmt.Printf("%s\t%d\t%s\n", kind, p, text)
}

func runPrintWorker(jobPath string) {
	data, err := os.ReadFile(jobPath)
	if err != nil {
		workerLine("ERROR", 0, "인쇄 작업 파일을 읽을 수 없습니다: "+err.Error())
		return
	}
	var job printWorkerJob
	if err := json.Unmarshal(data, &job); err != nil {
		workerLine("ERROR", 0, "인쇄 작업 정보를 읽을 수 없습니다: "+err.Error())
		return
	}
	err = batchPrintV3(job.Files, job.Options, func(s string, p int) {
		workerLine("PROGRESS", p, s)
	})
	if err != nil {
		workerLine("ERROR", 100, err.Error())
		return
	}
	workerLine("DONE", 100, fmt.Sprintf("완료 · %d개 파일을 인쇄했습니다.", len(job.Files)))
}

func runPrintWorkerProcess(files []string, opt printOptions) {
	exe, err := os.Executable()
	if err != nil {
		postStatus("인쇄 오류: 실행 파일 경로를 찾을 수 없습니다.")
		postDone()
		return
	}
	dir, err := os.MkdirTemp("", "JTSNPrintJob-")
	if err != nil {
		postStatus("인쇄 오류: 임시 작업 폴더를 만들 수 없습니다.")
		postDone()
		return
	}
	defer os.RemoveAll(dir)
	jobPath := filepath.Join(dir, "job.json")
	data, _ := json.Marshal(printWorkerJob{Files: files, Options: opt})
	if err := os.WriteFile(jobPath, data, 0600); err != nil {
		postStatus("인쇄 오류: 작업 정보를 저장할 수 없습니다.")
		postDone()
		return
	}
	cmd := exec.Command(exe, "--print-worker="+jobPath)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		postStatus("인쇄 오류: 작업 프로세스 출력을 연결할 수 없습니다.")
		postDone()
		return
	}
	if err := cmd.Start(); err != nil {
		postStatus("인쇄 오류: 작업 프로세스를 시작할 수 없습니다: " + err.Error())
		postDone()
		return
	}
	finalSeen := false
	scanner := bufio.NewScanner(stdout)
	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) < 3 {
			continue
		}
		p, _ := strconv.Atoi(parts[1])
		switch parts[0] {
		case "PROGRESS":
			postStatus(parts[2])
			postProgress(p)
		case "ERROR":
			full := "인쇄 오류: " + parts[2]
			postStatus(full)
			postError(full)
			postProgress(p)
			finalSeen = true
		case "DONE":
			postStatus(parts[2])
			postProgress(100)
			finalSeen = true
		}
	}
	waitErr := cmd.Wait()
	if waitErr != nil && !finalSeen {
		postStatus("인쇄 작업 프로세스가 비정상 종료되었습니다: " + waitErr.Error())
	} else if !finalSeen {
		postStatus("인쇄 작업이 종료되었습니다.")
	}
	postDone()
}

type printOptions struct {
	Printer                     string
	Copies                      int
	Pages                       string
	Duplex, Color, Scale, Paper int
	Collate                     bool
}

const (
	sumatraURL     = "https://www.sumatrapdfreader.org/dl/rel/3.6.1/SumatraPDF-3.6.1-64.zip"
	sumatraExeName = "SumatraPDF-3.6.1-64.exe"
)

func looksLikePortableSumatra(path string) bool {
	if path == "" || strings.Contains(strings.ToLower(filepath.Base(path)), "install") {
		return false
	}
	info, err := os.Stat(path)
	if err != nil || info.IsDir() || info.Size() < 5*1024*1024 {
		return false
	}
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	var hdr [2]byte
	if _, err := io.ReadFull(f, hdr[:]); err != nil {
		return false
	}
	return hdr[0] == 'M' && hdr[1] == 'Z'
}

func ensureSumatra(progress func(string, int)) (string, error) {
	// IMPORTANT: never resolve SumatraPDF from PATH or the registry.
	// A machine can contain the installer/updater executable and invoking it with
	// viewer print arguments produces the "ParseFlags" error seen in v3.2.
	cache, err := os.UserCacheDir()
	if err != nil {
		cache = os.TempDir()
	}
	// New cache directory on purpose: never reuse the engine selected/cached by v3.2/v3.3.
	root := filepath.Join(cache, "JTSN", "engines", "sumatra-3.6.1-portable-x64-v36")
	exePath := filepath.Join(root, sumatraExeName)
	if looksLikePortableSumatra(exePath) {
		progress("인쇄 엔진 확인 완료", 20)
		return exePath, nil
	}

	_ = os.RemoveAll(root)
	if err := os.MkdirAll(root, 0755); err != nil {
		return "", fmt.Errorf("인쇄 엔진 폴더 생성 실패: %v", err)
	}

	progress("공식 Portable 인쇄 엔진 준비 중 · 최초 1회", 3)
	if err := downloadAndExtractZip(sumatraURL, root, func(done, total int64) {
		pct := 5
		if total > 0 {
			pct = 5 + int(done*12/total)
		}
		progress("SumatraPDF Portable 다운로드 중...", pct)
	}); err != nil {
		return "", fmt.Errorf("인쇄 엔진 다운로드 실패: %v", err)
	}

	if !looksLikePortableSumatra(exePath) {
		if found := findFile(root, sumatraExeName); found != "" {
			exePath = found
		}
	}
	if !looksLikePortableSumatra(exePath) {
		_ = os.RemoveAll(root)
		return "", fmt.Errorf("공식 Portable 인쇄 엔진을 찾지 못했습니다. 프로그램을 다시 실행해 주세요")
	}
	progress("인쇄 엔진 준비 완료", 20)
	return exePath, nil
}
func downloadAndExtractZip(url, root string, progress func(int64, int64)) error {
	if err := os.MkdirAll(root, 0755); err != nil {
		return err
	}
	tmp := filepath.Join(root, "download.tmp.zip")
	client := &http.Client{Timeout: 4 * time.Minute}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "JTSN/5.1")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %s", resp.Status)
	}
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	buf := make([]byte, 256*1024)
	var done int64
	total := resp.ContentLength
	for {
		n, e := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := f.Write(buf[:n]); werr != nil {
				f.Close()
				return werr
			}
			done += int64(n)
			if progress != nil {
				progress(done, total)
			}
		}
		if e == io.EOF {
			break
		}
		if e != nil {
			f.Close()
			return e
		}
	}
	if err := f.Close(); err != nil {
		return err
	}
	zr, err := zip.OpenReader(tmp)
	if err != nil {
		return err
	}
	defer zr.Close()
	cleanRoot := filepath.Clean(root) + string(os.PathSeparator)
	for _, zf := range zr.File {
		dst := filepath.Join(root, zf.Name)
		if !strings.HasPrefix(filepath.Clean(dst)+string(os.PathSeparator), cleanRoot) {
			continue
		}
		if zf.FileInfo().IsDir() {
			_ = os.MkdirAll(dst, 0755)
			continue
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
			return err
		}
		rc, err := zf.Open()
		if err != nil {
			return err
		}
		out, err := os.Create(dst)
		if err != nil {
			rc.Close()
			return err
		}
		_, err = io.Copy(out, rc)
		out.Close()
		rc.Close()
		if err != nil {
			return err
		}
	}
	_ = os.Remove(tmp)
	return nil
}
func buildPrintSettings(o printOptions, includeCopies bool) string {
	// Keep the command line intentionally minimal. 3.6.1 accepts all of the
	// options below, but printer drivers are more reliable when we don't force
	// values that are already the UI defaults.
	var t []string
	if strings.TrimSpace(o.Pages) != "" {
		t = append(t, strings.TrimSpace(o.Pages))
	}
	if includeCopies && o.Copies > 1 {
		t = append(t, fmt.Sprintf("%dx", o.Copies))
	}
	// UI index 0 means simplex. Do not force it unless the user selected duplex.
	switch o.Duplex {
	case 1:
		t = append(t, "duplexlong")
	case 2:
		t = append(t, "duplexshort")
	}
	// UI index 0 means color. Leave the driver default alone; only force B/W.
	if o.Color == 1 {
		t = append(t, "monochrome")
	}
	// Fit is useful but is the only default option we still ask Sumatra to apply.
	// If a driver rejects it, sumatraPrint() retries once without print-settings.
	switch o.Scale {
	case 1:
		t = append(t, "noscale")
	case 2:
		t = append(t, "shrink")
	default:
		t = append(t, "fit")
	}
	switch o.Paper {
	case 1:
		t = append(t, "paper=A4")
	case 2:
		t = append(t, "paper=A3")
	case 3:
		t = append(t, "paper=letter")
	case 4:
		t = append(t, "paper=legal")
	}
	return strings.Join(t, ",")
}

func canFallbackWordDirect(o printOptions, useDefaultPrinter bool) bool {
	// Safe fallback for the common office case: default printer, 1 copy,
	// all pages, simplex/color/default paper. Word itself can print this more
	// reliably than routing through a PDF engine. Scale is irrelevant for Word.
	return useDefaultPrinter && o.Copies <= 1 && strings.TrimSpace(o.Pages) == "" && o.Duplex == 0 && o.Color == 0 && o.Paper == 0
}

func wordDirectPrint(input string, useDefaultPrinter bool, printer string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	setPrinter := ""
	if !useDefaultPrinter && strings.TrimSpace(printer) != "" {
		setPrinter = fmt.Sprintf("$w.ActivePrinter='%s';", psQuote(printer))
	}
	script := fmt.Sprintf(`$ErrorActionPreference='Stop';$w=New-Object -ComObject Word.Application;$w.Visible=$false;$w.DisplayAlerts=0;try{%s$d=$w.Documents.Open('%s',$false,$true);$d.PrintOut($false);$d.Close($false)}finally{$w.Quit()}`, setPrinter, psQuote(input))
	cmd := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-STA", "-Command", script)
	if runtime.GOOS == "windows" {
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	}
	out, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return fmt.Errorf("Word 직접 인쇄 응답 시간이 초과되었습니다")
	}
	if err != nil {
		return fmt.Errorf("Word 직접 인쇄 실패: %v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func batchPrintV3(files []string, o printOptions, progress func(string, int)) error {
	sumatra, err := ensureSumatra(progress)
	if err != nil {
		return err
	}
	useDefaultPrinter := strings.HasPrefix(o.Printer, "[기본] ") || strings.TrimSpace(o.Printer) == "" || strings.TrimSpace(o.Printer) == "기본 프린터"
	printer := strings.TrimPrefix(o.Printer, "[기본] ")
	tempDir, err := os.MkdirTemp("", "JTSNPrint-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)

	// Convert Word documents once. Reusing the generated PDF avoids reopening
	// Word for every copy when collated output is requested.
	targets := make([]string, len(files))
	for i, f := range files {
		target := f
		ext := strings.ToLower(filepath.Ext(f))
		progress(fmt.Sprintf("문서 준비 중 · %d/%d · %s", i+1, len(files), filepath.Base(f)), 22+i*20/max(1, len(files)))
		if ext == ".doc" || ext == ".docx" {
			target = filepath.Join(tempDir, fmt.Sprintf("%03d.pdf", i+1))
			if err := wordToPDF(f, target); err != nil {
				return fmt.Errorf("Word 변환 실패 (%s): %v", filepath.Base(f), err)
			}
		}
		targets[i] = target
	}

	copies := o.Copies
	if copies < 1 {
		copies = 1
	}
	if copies > 99 {
		copies = 99
	}

	if o.Collate && copies > 1 {
		// Collated set: A,B,C,A,B,C. We implement this ourselves because
		// collate/nocollate tokens are not documented for SumatraPDF 3.6.1.
		settings := buildPrintSettings(o, false)
		total := len(targets) * copies
		done := 0
		for c := 0; c < copies; c++ {
			for i, target := range targets {
				done++
				progress(fmt.Sprintf("인쇄 중 · %d/%d · %s · %d부째", done, total, filepath.Base(files[i]), c+1), 45+done*50/max(1, total))
				fallback, err := sumatraPrint(sumatra, target, printer, useDefaultPrinter, settings)
				if err != nil {
					ext := strings.ToLower(filepath.Ext(files[i]))
					if (ext == ".doc" || ext == ".docx") && canFallbackWordDirect(o, useDefaultPrinter) {
						progress("PDF 인쇄엔진 호환 문제 감지 · Microsoft Word 직접 인쇄로 전환", 90)
						if werr := wordDirectPrint(files[i], useDefaultPrinter, printer); werr == nil {
							continue
						} else {
							return fmt.Errorf("%s: PDF 인쇄 실패(%v) / %v", filepath.Base(files[i]), err, werr)
						}
					}
					return fmt.Errorf("%s: %v", filepath.Base(files[i]), err)
				}
				if fallback {
					progress("호환 모드로 인쇄했습니다 · 일부 기본 옵션은 프린터 설정을 사용합니다", 95)
				}
			}
		}
		return nil
	}

	// Uncollated: A,A,B,B. SumatraPDF 3.6.1 officially supports the Nx token.
	settings := buildPrintSettings(o, true)
	for i, target := range targets {
		progress(fmt.Sprintf("인쇄 중 · %d/%d · %s", i+1, len(targets), filepath.Base(files[i])), 45+(i+1)*50/max(1, len(targets)))
		fallback, err := sumatraPrint(sumatra, target, printer, useDefaultPrinter, settings)
		if err != nil {
			ext := strings.ToLower(filepath.Ext(files[i]))
			if (ext == ".doc" || ext == ".docx") && canFallbackWordDirect(o, useDefaultPrinter) {
				progress("PDF 인쇄엔진 호환 문제 감지 · Microsoft Word 직접 인쇄로 전환", 90)
				if werr := wordDirectPrint(files[i], useDefaultPrinter, printer); werr == nil {
					continue
				} else {
					return fmt.Errorf("%s: PDF 인쇄 실패(%v) / %v", filepath.Base(files[i]), err, werr)
				}
			}
			return fmt.Errorf("%s: %v", filepath.Base(files[i]), err)
		}
		if fallback {
			progress("호환 모드로 인쇄했습니다 · 일부 기본 옵션은 프린터 설정을 사용합니다", 95)
		}
	}
	return nil
}

func wordToPDF(input, output string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	script := fmt.Sprintf(`$ErrorActionPreference='Stop';$w=New-Object -ComObject Word.Application;$w.Visible=$false;$w.DisplayAlerts=0;try{$d=$w.Documents.Open('%s',$false,$true);$d.ExportAsFixedFormat('%s',17);$d.Close($false)}finally{$w.Quit()}`, psQuote(input), psQuote(output))
	cmd := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-STA", "-Command", script)
	out, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return fmt.Errorf("Word 응답 시간이 초과되었습니다")
	}
	if err != nil {
		return fmt.Errorf("%v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
func runSumatraPrint(exe, file, printer string, useDefault bool, settings string) (string, int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	args := make([]string, 0, 8)
	if useDefault {
		args = append(args, "-print-to-default")
	} else {
		args = append(args, "-print-to", printer)
	}
	if strings.TrimSpace(settings) != "" {
		args = append(args, "-print-settings", settings)
	}
	// Keep -silent after the print command, matching the official examples.
	args = append(args, "-silent", file)

	// Do not use CombinedOutput/StdoutPipe here. Some GUI applications and
	// printer drivers behave badly when their standard handles are pipes.
	// A regular temporary file gives us diagnostics without coupling the child
	// process to the JTSN worker process.
	lf, lerr := os.CreateTemp("", "JTSN-Sumatra-*.log")
	if lerr != nil {
		return "", -1, lerr
	}
	logPath := lf.Name()
	defer os.Remove(logPath)

	cmd := exec.CommandContext(ctx, exe, args...)
	cmd.Stdout = lf
	cmd.Stderr = lf
	if runtime.GOOS == "windows" {
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	}
	err := cmd.Run()
	_ = lf.Close()
	data, _ := os.ReadFile(logPath)
	msg := strings.TrimSpace(string(data))

	if ctx.Err() == context.DeadlineExceeded {
		return msg, -1, fmt.Errorf("프린터 응답 시간이 초과되었습니다")
	}
	if err == nil {
		return msg, 0, nil
	}
	code := -1
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	}
	return msg, code, err
}

func usefulSumatraLog(msg string) string {
	if strings.TrimSpace(msg) == "" {
		return ""
	}
	lines := strings.Split(strings.ReplaceAll(msg, "\r\n", "\n"), "\n")
	var keep []string
	for _, line := range lines {
		l := strings.TrimSpace(line)
		if l == "" || strings.HasPrefix(l, "ParseFlags: argName:") {
			continue
		}
		keep = append(keep, l)
	}
	if len(keep) > 8 {
		keep = keep[len(keep)-8:]
	}
	return strings.Join(keep, " | ")
}

func sumatraPrint(exe, file, printer string, useDefault bool, settings string) (bool, error) {
	msg, code, err := runSumatraPrint(exe, file, printer, useDefault, settings)
	if err == nil {
		return false, nil
	}

	// For the normal UI defaults, a few drivers reject Sumatra's explicit
	// print-settings even though they can print the document. Retry once with
	// only the printer and file. This is safe because we only do it when the
	// requested settings are equivalent to the default UI choices.
	defaultLike := settings == "" || settings == "fit"
	if defaultLike && strings.TrimSpace(settings) != "" {
		msg2, code2, err2 := runSumatraPrint(exe, file, printer, useDefault, "")
		if err2 == nil {
			return true, nil
		}
		if u := usefulSumatraLog(msg2); u != "" {
			msg = u
		}
		code = code2
		err = err2
	}

	// ParseFlags lines are normal debug logging in SumatraPDF 3.6.1, not an
	// option-parsing error. Report the actual process/driver failure instead.
	detail := usefulSumatraLog(msg)
	if code == 2 {
		return false, fmt.Errorf("인쇄할 문서를 열 수 없습니다")
	}
	if code == 3 {
		return false, fmt.Errorf("문서에서 인쇄가 허용되지 않습니다")
	}
	if code == 4 {
		return false, fmt.Errorf("선택한 프린터를 찾을 수 없습니다")
	}
	if code == 5 {
		return false, fmt.Errorf("프린터 드라이버 또는 장치에서 인쇄를 거부했습니다")
	}
	if code == 6 {
		return false, fmt.Errorf("PC 보안 정책에서 인쇄가 제한되어 있습니다")
	}
	if detail != "" {
		return false, fmt.Errorf("인쇄 엔진이 종료 코드 %d로 실패했습니다. %s", code, detail)
	}
	return false, fmt.Errorf("인쇄 엔진이 종료 코드 %d로 실패했습니다. 기본 프린터 상태와 Windows 인쇄 대기열을 확인해 주세요", code)
}

const qpdfURL = "https://github.com/qpdf/qpdf/releases/download/v12.4.0/qpdf-12.4.0-msvc64.zip"

func ensureQPDF(progress func(string, int)) (string, error) {
	if p, err := exec.LookPath("qpdf.exe"); err == nil {
		return p, nil
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		cache = os.TempDir()
	}
	root := filepath.Join(cache, "JTSN", "qpdf-12.4.0")
	if p := findFile(root, "qpdf.exe"); p != "" {
		return p, nil
	}
	progress("PDF 엔진을 준비하는 중입니다 · 최초 1회", 3)
	if err := downloadAndExtractZip(qpdfURL, root, func(done, total int64) {
		v := 5
		if total > 0 {
			v = 5 + int(done*25/total)
		}
		progress("PDF 엔진 다운로드 중...", v)
	}); err != nil {
		return "", fmt.Errorf("PDF 엔진 다운로드 실패: %v", err)
	}
	p := findFile(root, "qpdf.exe")
	if p == "" {
		return "", fmt.Errorf("qpdf.exe를 찾지 못했습니다")
	}
	progress("PDF 엔진 준비 완료", 30)
	return p, nil
}
func psQuote(s string) string { return strings.ReplaceAll(s, "'", "''") }
func findFile(root, name string) string {
	var found string
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.EqualFold(d.Name(), name) {
			found = path
			return filepath.SkipAll
		}
		return nil
	})
	return found
}
func pdfMerge(q string, files []string, out string) error {
	if len(files) < 2 {
		return fmt.Errorf("병합하려면 PDF를 2개 이상 선택해 주세요")
	}
	args := []string{"--empty", "--pages"}
	for _, f := range files {
		args = append(args, f, "1-z")
	}
	args = append(args, "--", out)
	return runCmd(q, args...)
}
func pdfExtract(q, input, pages, out string) error {
	return runCmd(q, input, "--pages", ".", pages, "--", out)
}
func pdfSplit(q, input, outDir string) error {
	base := strings.TrimSuffix(filepath.Base(input), filepath.Ext(input))
	pattern := filepath.Join(outDir, base+"-%d.pdf")
	return runCmd(q, "--split-pages", input, pattern)
}
func runCmd(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = filepath.Dir(name)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func batchRename(files []string, prefix, suffix, find, repl string, number bool) ([]string, []error) {
	out := make([]string, 0, len(files))
	var errs []error
	for i, f := range files {
		dir := filepath.Dir(f)
		ext := filepath.Ext(f)
		base := strings.TrimSuffix(filepath.Base(f), ext)
		if find != "" {
			base = strings.ReplaceAll(base, find, repl)
		}
		if number {
			base = fmt.Sprintf("%03d_%s", i+1, base)
		}
		dst := filepath.Join(dir, prefix+base+suffix+ext)
		if strings.EqualFold(f, dst) {
			out = append(out, f)
			continue
		}
		if _, err := os.Stat(dst); err == nil {
			errs = append(errs, fmt.Errorf("이미 존재: %s", dst))
			out = append(out, f)
			continue
		}
		if err := os.Rename(f, dst); err != nil {
			errs = append(errs, err)
			out = append(out, f)
		} else {
			out = append(out, dst)
		}
	}
	return out, errs
}
func createFolders(root string, names []string) (int, []error) {
	n := 0
	var errs []error
	for _, name := range names {
		clean := filepath.Clean(name)
		if clean == "." || filepath.IsAbs(clean) || strings.HasPrefix(clean, "..") {
			errs = append(errs, fmt.Errorf("잘못된 경로: %s", name))
			continue
		}
		if err := os.MkdirAll(filepath.Join(root, clean), 0755); err != nil {
			errs = append(errs, err)
		} else {
			n++
		}
	}
	return n, errs
}

func scanDuplicates(root string, progress func(string, int)) []duplicateGroup {
	sizes := map[int64][]string{}
	filesSeen := 0
	filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err == nil {
			sizes[info.Size()] = append(sizes[info.Size()], path)
			filesSeen++
			if filesSeen%250 == 0 {
				progress(fmt.Sprintf("파일 목록 확인 중 · %d개", filesSeen), 5)
			}
		}
		return nil
	})
	var candidates []string
	for _, fs := range sizes {
		if len(fs) > 1 {
			candidates = append(candidates, fs...)
		}
	}
	hashMap := map[string][]string{}
	sizeByHash := map[string]int64{}
	for i, f := range candidates {
		info, err := os.Stat(f)
		if err != nil {
			continue
		}
		h, err := fileHash(f)
		if err != nil {
			continue
		}
		key := fmt.Sprintf("%d:%s", info.Size(), h)
		hashMap[key] = append(hashMap[key], f)
		sizeByHash[key] = info.Size()
		if i%5 == 0 {
			progress(fmt.Sprintf("내용 비교 중 · %d/%d", i+1, len(candidates)), 10+(i+1)*85/max(1, len(candidates)))
		}
	}
	var groups []duplicateGroup
	for key, fs := range hashMap {
		if len(fs) > 1 {
			sort.Strings(fs)
			groups = append(groups, duplicateGroup{Hash: strings.SplitN(key, ":", 2)[1], Size: sizeByHash[key], Files: fs})
		}
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].Size > groups[j].Size })
	return groups
}
func fileHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	buf := make([]byte, 1024*1024)
	if _, err := io.CopyBuffer(h, f, buf); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
func shortHash(s string) string {
	if len(s) > 12 {
		return s[:12] + "…"
	}
	return s
}
func humanBytes(n int64) string {
	units := []string{"B", "KB", "MB", "GB", "TB"}
	v := float64(n)
	i := 0
	for v >= 1024 && i < len(units)-1 {
		v /= 1024
		i++
	}
	if i == 0 {
		return fmt.Sprintf("%d B", n)
	}
	return fmt.Sprintf("%.1f %s", v, units[i])
}
func duplicateExtraFiles(gs []duplicateGroup) []string {
	var out []string
	for _, g := range gs {
		if len(g.Files) > 1 {
			out = append(out, g.Files[1:]...)
		}
	}
	return out
}
func exportDuplicateCSV(path string, gs []duplicateGroup) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	defer w.Flush()
	_ = w.Write([]string{"Group", "Status", "SizeBytes", "SHA256", "Path"})
	for i, g := range gs {
		for j, p := range g.Files {
			st := "Duplicate"
			if j == 0 {
				st = "Keep"
			}
			_ = w.Write([]string{strconv.Itoa(i + 1), st, strconv.FormatInt(g.Size, 10), g.Hash, p})
		}
	}
	return w.Error()
}
func recycleFiles(files []string) (int, error) {
	if len(files) == 0 {
		return 0, nil
	}
	var q []string
	for _, f := range files {
		q = append(q, "'"+strings.ReplaceAll(f, "'", "''")+"'")
	}
	script := `Add-Type -AssemblyName Microsoft.VisualBasic;$files=@(` + strings.Join(q, ",") + `);$n=0;foreach($f in $files){if(Test-Path -LiteralPath $f){[Microsoft.VisualBasic.FileIO.FileSystem]::DeleteFile($f,'OnlyErrorDialogs','SendToRecycleBin');$n++}};[Console]::Write($n)`
	out, err := exec.Command("powershell.exe", "-NoProfile", "-STA", "-Command", script).CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("%v: %s", err, string(out))
	}
	n, _ := strconv.Atoi(strings.TrimSpace(string(out)))
	return n, nil
}

func convertImages(files []string, outDir, format string, pct int, progress func(int, int)) (int, []error) {
	n := 0
	var errs []error
	for i, f := range files {
		src, err := os.Open(f)
		if err != nil {
			errs = append(errs, err)
			progress(i+1, len(files))
			continue
		}
		img, _, err := image.Decode(src)
		src.Close()
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %v", f, err))
			progress(i+1, len(files))
			continue
		}
		if pct != 100 {
			img = fastResize(img, pct)
		}
		if format == "jpg" {
			img = flattenWhite(img)
		}
		base := strings.TrimSuffix(filepath.Base(f), filepath.Ext(f))
		ext := ".png"
		if format == "jpg" {
			ext = ".jpg"
		}
		dst := uniqueOutputPath(outDir, base, ext, f)
		o, err := os.Create(dst)
		if err != nil {
			errs = append(errs, err)
			progress(i+1, len(files))
			continue
		}
		if format == "jpg" {
			err = jpeg.Encode(o, img, &jpeg.Options{Quality: 92})
		} else {
			err = png.Encode(o, img)
		}
		o.Close()
		if err != nil {
			errs = append(errs, err)
		} else {
			n++
		}
		progress(i+1, len(files))
	}
	return n, errs
}
func fastResize(src image.Image, pct int) image.Image {
	b := src.Bounds()
	sw, sh := b.Dx(), b.Dy()
	dw, dh := sw*pct/100, sh*pct/100
	if dw < 1 {
		dw = 1
	}
	if dh < 1 {
		dh = 1
	}
	s := image.NewNRGBA(image.Rect(0, 0, sw, sh))
	draw.Draw(s, s.Bounds(), src, b.Min, draw.Src)
	d := image.NewNRGBA(image.Rect(0, 0, dw, dh))
	for y := 0; y < dh; y++ {
		sy := y * sh / dh
		srow := s.Pix[sy*s.Stride:]
		drow := d.Pix[y*d.Stride:]
		for x := 0; x < dw; x++ {
			sx := x * sw / dw
			si := sx * 4
			di := x * 4
			copy(drow[di:di+4], srow[si:si+4])
		}
	}
	return d
}
func flattenWhite(src image.Image) image.Image {
	b := src.Bounds()
	d := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(d, d.Bounds(), &image.Uniform{C: color.White}, image.Point{}, draw.Src)
	draw.Draw(d, d.Bounds(), src, b.Min, draw.Over)
	return d
}
func uniqueOutputPath(dir, base, ext, src string) string {
	dst := filepath.Join(dir, base+ext)
	if !strings.EqualFold(dst, src) {
		if _, err := os.Stat(dst); os.IsNotExist(err) {
			return dst
		}
	}
	for i := 1; i < 10000; i++ {
		p := filepath.Join(dir, fmt.Sprintf("%s_converted_%02d%s", base, i, ext))
		if _, err := os.Stat(p); os.IsNotExist(err) {
			return p
		}
	}
	return filepath.Join(dir, base+"_converted"+ext)
}

func applyFinalColorString(s string) {
	parts := strings.Fields(s)
	if len(parts) == 0 {
		return
	}
	hex := strings.TrimSpace(parts[0])
	if len(hex) == 7 && strings.HasPrefix(hex, "#") {
		if v, err := strconv.ParseUint(hex[1:], 16, 32); err == nil {
			eyedropperFinalR = byte((v >> 16) & 0xff)
			eyedropperFinalG = byte((v >> 8) & 0xff)
			eyedropperFinalB = byte(v & 0xff)
			currentHex = strings.ToUpper(hex)
		}
	}
	rgbText := fmt.Sprintf("RGB(%d, %d, %d)", eyedropperFinalR, eyedropperFinalG, eyedropperFinalB)
	if colorHexEdit != 0 {
		setText(colorHexEdit, currentHex)
	}
	if colorRGBEdit != 0 {
		setText(colorRGBEdit, rgbText)
	}
	if colorSwatchHandle != 0 {
		procInvalidateRect.Call(uintptr(colorSwatchHandle), 0, 1)
	}
}

func startEyedropperDrag() {
	if eyedropperDragging {
		return
	}
	eyedropperDragging = true
	eyedropperLastSample = time.Time{}
	eyedropperLastX, eyedropperLastY = -1<<30, -1<<30
	// Capture to the top-level window. Mouse movement now updates immediately; the
	// short timer is only a fallback for stationary-pointer/release polling.
	procSetCapture.Call(uintptr(mainHWND))
	// Keep one desktop DC for the short drag session instead of acquiring/releasing
	// it on every mouse packet. This removes another source of micro-stutter.
	if eyedropperScreenDC == 0 {
		dc, _, _ := procGetDC.Call(0)
		eyedropperScreenDC = syscall.Handle(dc)
	}
	if eyedropperScreenDC != 0 {
		ensureEyedropperBuffer(eyedropperScreenDC)
	}
	procSetTimer.Call(uintptr(mainHWND), ID_TIMER_EYEDROPPER, 12, 0)
	setStatus("드래그 중 · 원하는 색상에서 마우스를 놓으세요.")
	updateEyedropperFromCursor(false)
	if colorEyeHandle != 0 {
		procInvalidateRect.Call(uintptr(colorEyeHandle), 0, 0)
	}
}

func finishEyedropperDrag(commit bool) {
	if !eyedropperDragging {
		return
	}
	if commit {
		updateEyedropperFromCursor(true)
	}
	eyedropperDragging = false
	procKillTimer.Call(uintptr(mainHWND), ID_TIMER_EYEDROPPER)
	if eyedropperScreenDC != 0 {
		procReleaseDC.Call(0, uintptr(eyedropperScreenDC))
		eyedropperScreenDC = 0
	}
	procReleaseCapture.Call()
	if colorEyeHandle != 0 {
		procInvalidateRect.Call(uintptr(colorEyeHandle), 0, 0)
	}
}

func ensureEyedropperBuffer(screenDC syscall.Handle) bool {
	if eyedropperMemDC != 0 && eyedropperBitmap != 0 {
		return true
	}
	mdc, _, _ := procCreateCompatibleDC.Call(uintptr(screenDC))
	if mdc == 0 {
		return false
	}
	bmp, _, _ := procCreateCompatibleBitmap.Call(uintptr(screenDC), 13, 13)
	if bmp == 0 {
		procDeleteDC.Call(mdc)
		return false
	}
	old, _, _ := procSelectObject.Call(mdc, bmp)
	eyedropperMemDC = syscall.Handle(mdc)
	eyedropperBitmap = syscall.Handle(bmp)
	eyedropperOldBitmap = syscall.Handle(old)
	return true
}

func destroyEyedropperBuffer() {
	if eyedropperScreenDC != 0 {
		procReleaseDC.Call(0, uintptr(eyedropperScreenDC))
		eyedropperScreenDC = 0
	}
	if eyedropperMemDC != 0 {
		if eyedropperOldBitmap != 0 {
			procSelectObject.Call(uintptr(eyedropperMemDC), uintptr(eyedropperOldBitmap))
		}
		if eyedropperBitmap != 0 {
			procDeleteObject.Call(uintptr(eyedropperBitmap))
		}
		procDeleteDC.Call(uintptr(eyedropperMemDC))
	}
	eyedropperMemDC, eyedropperBitmap, eyedropperOldBitmap = 0, 0, 0
}

func updateEyedropperFromCursor(commit bool) {
	var pt POINT
	r, _, _ := procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	if r == 0 {
		return
	}
	// Timer ticks can arrive between mouse moves. Avoid repainting the same pixel.
	if !commit && pt.X == eyedropperLastX && pt.Y == eyedropperLastY {
		return
	}
	eyedropperLastX, eyedropperLastY = pt.X, pt.Y

	screenDC := eyedropperScreenDC
	ownedDC := false
	if screenDC == 0 {
		dc, _, _ := procGetDC.Call(0)
		screenDC = syscall.Handle(dc)
		ownedDC = screenDC != 0
	}
	if screenDC == 0 {
		return
	}
	if ownedDC {
		defer procReleaseDC.Call(0, uintptr(screenDC))
	}
	if !ensureEyedropperBuffer(screenDC) {
		return
	}

	// One native copy for the whole 13x13 area instead of 169 GetPixel calls.
	procBitBlt.Call(uintptr(eyedropperMemDC), 0, 0, 13, 13, uintptr(screenDC),
		uintptr(int64(pt.X)-6), uintptr(int64(pt.Y)-6), SRCCOPY)
	c, _, _ := procGetPixel.Call(uintptr(eyedropperMemDC), 6, 6)
	if c == 0xFFFFFFFF {
		c = rgb(255, 255, 255)
	}
	eyedropperLiveR = byte(c & 0xff)
	eyedropperLiveG = byte((c >> 8) & 0xff)
	eyedropperLiveB = byte((c >> 16) & 0xff)
	eyedropperPreviewReady = true
	if colorPreviewHandle != 0 {
		procInvalidateRect.Call(uintptr(colorPreviewHandle), 0, 0)
		// Paint immediately while dragging so queued mouse messages cannot make the
		// preview visually lag behind the pointer.
		if eyedropperDragging {
			procUpdateWindow.Call(uintptr(colorPreviewHandle))
		}
	}
	if commit {
		eyedropperFinalR, eyedropperFinalG, eyedropperFinalB = eyedropperLiveR, eyedropperLiveG, eyedropperLiveB
		currentHex = fmt.Sprintf("#%02X%02X%02X", eyedropperFinalR, eyedropperFinalG, eyedropperFinalB)
		if colorHexEdit != 0 {
			setText(colorHexEdit, currentHex)
		}
		if colorRGBEdit != 0 {
			setText(colorRGBEdit, fmt.Sprintf("RGB(%d, %d, %d)", eyedropperFinalR, eyedropperFinalG, eyedropperFinalB))
		}
		if colorSwatchHandle != 0 {
			procInvalidateRect.Call(uintptr(colorSwatchHandle), 0, 1)
		}
		setStatus("색상 선택 완료 · " + currentHex)
	}
}

func drawColorSwatch(dis *DRAWITEMSTRUCT) {
	brush := solidBrush(eyedropperFinalR, eyedropperFinalG, eyedropperFinalB)
	pen, _, _ := procCreatePen.Call(PS_SOLID, 1, rgb(214, 222, 233))
	oldB, _, _ := procSelectObject.Call(uintptr(dis.HDC), uintptr(brush))
	oldP, _, _ := procSelectObject.Call(uintptr(dis.HDC), pen)
	procRoundRect.Call(uintptr(dis.HDC), uintptr(dis.RcItem.Left), uintptr(dis.RcItem.Top), uintptr(dis.RcItem.Right), uintptr(dis.RcItem.Bottom), 16, 16)
	procSelectObject.Call(uintptr(dis.HDC), oldB)
	procSelectObject.Call(uintptr(dis.HDC), oldP)
	procDeleteObject.Call(uintptr(brush))
	procDeleteObject.Call(pen)
}

func drawColorPreview(dis *DRAWITEMSTRUCT) {
	bg := solidBrush(248, 250, 253)
	pen, _, _ := procCreatePen.Call(PS_SOLID, 1, rgb(218, 225, 235))
	oldB, _, _ := procSelectObject.Call(uintptr(dis.HDC), uintptr(bg))
	oldP, _, _ := procSelectObject.Call(uintptr(dis.HDC), pen)
	procRoundRect.Call(uintptr(dis.HDC), uintptr(dis.RcItem.Left), uintptr(dis.RcItem.Top), uintptr(dis.RcItem.Right), uintptr(dis.RcItem.Bottom), 14, 14)
	procSelectObject.Call(uintptr(dis.HDC), oldB)
	procSelectObject.Call(uintptr(dis.HDC), oldP)
	procDeleteObject.Call(uintptr(bg))
	procDeleteObject.Call(pen)

	if !eyedropperPreviewReady {
		procSetBkMode.Call(uintptr(dis.HDC), TRANSPARENT)
		procSetTextColor.Call(uintptr(dis.HDC), rgb(100, 116, 139))
		oldF, _, _ := procSelectObject.Call(uintptr(dis.HDC), uintptr(fontNormal))
		txt := "스포이드를 끌면 포인터 주변이 확대됩니다."
		rc := dis.RcItem
		procDrawTextW.Call(uintptr(dis.HDC), uintptr(unsafe.Pointer(p16(txt))), uintptr(len(syscall.StringToUTF16(txt))-1), uintptr(unsafe.Pointer(&rc)), DT_CENTER|DT_VCENTER|DT_SINGLELINE)
		procSelectObject.Call(uintptr(dis.HDC), oldF)
		return
	}

	const n = 13
	pad := int32(6)
	left := dis.RcItem.Left + pad
	top := dis.RcItem.Top + pad
	w := dis.RcItem.Right - dis.RcItem.Left - 2*pad
	h := dis.RcItem.Bottom - dis.RcItem.Top - 2*pad
	cellW := w / n
	cellH := h / n
	if eyedropperMemDC != 0 {
		// Scale the persistent 13x13 snapshot in one GDI operation. COLORONCOLOR
		// keeps the hard pixel edges expected from a color-picker magnifier.
		procSetStretchBltMode.Call(uintptr(dis.HDC), 3)
		procStretchBlt.Call(uintptr(dis.HDC), uintptr(left), uintptr(top), uintptr(w), uintptr(h),
			uintptr(eyedropperMemDC), 0, 0, 13, 13, SRCCOPY)
	}

	half := int32(n / 2)
	cr := RECT{
		Left:   left + half*cellW,
		Top:    top + half*cellH,
		Right:  left + (half+1)*cellW + 1,
		Bottom: top + (half+1)*cellH + 1,
	}
	nullBrush, _, _ := procGetStockObject.Call(5)
	p1, _, _ := procCreatePen.Call(PS_SOLID, 3, rgb(255, 255, 255))
	oldNB, _, _ := procSelectObject.Call(uintptr(dis.HDC), nullBrush)
	oldP1, _, _ := procSelectObject.Call(uintptr(dis.HDC), p1)
	procRectangle.Call(uintptr(dis.HDC), uintptr(cr.Left), uintptr(cr.Top), uintptr(cr.Right), uintptr(cr.Bottom))
	procSelectObject.Call(uintptr(dis.HDC), oldP1)
	procDeleteObject.Call(p1)
	p2, _, _ := procCreatePen.Call(PS_SOLID, 1, rgb(15, 23, 42))
	oldP2, _, _ := procSelectObject.Call(uintptr(dis.HDC), p2)
	procRectangle.Call(uintptr(dis.HDC), uintptr(cr.Left+2), uintptr(cr.Top+2), uintptr(cr.Right-2), uintptr(cr.Bottom-2))
	procSelectObject.Call(uintptr(dis.HDC), oldP2)
	procSelectObject.Call(uintptr(dis.HDC), oldNB)
	procDeleteObject.Call(p2)
}

func drawEyedropperButton(dis *DRAWITEMSTRUCT) {
	hovered := hoveredButtons[dis.HwndItem]
	fr, fg, fb := byte(47), byte(97), byte(235)
	if hovered || eyedropperDragging {
		fr, fg, fb = 38, 84, 220
	}
	brush := solidBrush(fr, fg, fb)
	pen, _, _ := procCreatePen.Call(PS_SOLID, 1, rgb(fr, fg, fb))
	oldB, _, _ := procSelectObject.Call(uintptr(dis.HDC), uintptr(brush))
	oldP, _, _ := procSelectObject.Call(uintptr(dis.HDC), pen)
	procRoundRect.Call(uintptr(dis.HDC), uintptr(dis.RcItem.Left), uintptr(dis.RcItem.Top), uintptr(dis.RcItem.Right), uintptr(dis.RcItem.Bottom), 18, 18)
	procSelectObject.Call(uintptr(dis.HDC), oldB)
	procSelectObject.Call(uintptr(dis.HDC), oldP)
	procDeleteObject.Call(uintptr(brush))
	procDeleteObject.Call(pen)

	cx := (dis.RcItem.Left + dis.RcItem.Right) / 2
	cy := (dis.RcItem.Top + dis.RcItem.Bottom) / 2
	whitePen, _, _ := procCreatePen.Call(PS_SOLID, 4, rgb(255, 255, 255))
	oldWP, _, _ := procSelectObject.Call(uintptr(dis.HDC), whitePen)
	procMoveToEx.Call(uintptr(dis.HDC), uintptr(cx-18), uintptr(cy+18), 0)
	procLineTo.Call(uintptr(dis.HDC), uintptr(cx+12), uintptr(cy-12))
	procMoveToEx.Call(uintptr(dis.HDC), uintptr(cx-10), uintptr(cy+22), 0)
	procLineTo.Call(uintptr(dis.HDC), uintptr(cx+18), uintptr(cy-6))
	procSelectObject.Call(uintptr(dis.HDC), oldWP)
	procDeleteObject.Call(whitePen)
	whiteBrush := solidBrush(255, 255, 255)
	oldWB, _, _ := procSelectObject.Call(uintptr(dis.HDC), uintptr(whiteBrush))
	procEllipse.Call(uintptr(dis.HDC), uintptr(cx+8), uintptr(cy-22), uintptr(cx+28), uintptr(cy-2))
	procSelectObject.Call(uintptr(dis.HDC), oldWB)
	procDeleteObject.Call(uintptr(whiteBrush))
}

func captureCursorColor() (string, string, error) {
	var pt POINT
	r, _, e := procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	if r == 0 {
		return "", "", e
	}
	dc, _, _ := procGetDC.Call(0)
	if dc == 0 {
		return "", "", fmt.Errorf("화면 정보를 가져오지 못했습니다")
	}
	defer procReleaseDC.Call(0, dc)
	c, _, _ := procGetPixel.Call(dc, uintptr(pt.X), uintptr(pt.Y))
	if c == 0xFFFFFFFF {
		return "", "", fmt.Errorf("색상 정보를 읽지 못했습니다")
	}
	rr := byte(c & 0xff)
	gg := byte((c >> 8) & 0xff)
	bb := byte((c >> 16) & 0xff)
	return fmt.Sprintf("#%02X%02X%02X", rr, gg, bb), fmt.Sprintf("RGB(%d, %d, %d)", rr, gg, bb), nil
}
func copyClipboard(s string) error {
	r, _, _ := procOpenClipboard.Call(uintptr(mainHWND))
	if r == 0 {
		return fmt.Errorf("클립보드를 열 수 없습니다")
	}
	defer procCloseClipboard.Call()
	procEmptyClipboard.Call()
	u := syscall.StringToUTF16(s)
	size := uintptr(len(u) * 2)
	h, _, _ := procGlobalAlloc.Call(GMEM_MOVEABLE, size)
	if h == 0 {
		return fmt.Errorf("메모리 할당 실패")
	}
	p, _, _ := procGlobalLock.Call(h)
	if p == 0 {
		procGlobalFree.Call(h)
		return fmt.Errorf("메모리 잠금 실패")
	}
	procRtlMoveMemory.Call(p, uintptr(unsafe.Pointer(&u[0])), size)
	procGlobalUnlock.Call(h)
	if r, _, _ := procSetClipboardData.Call(CF_UNICODETEXT, h); r == 0 {
		procGlobalFree.Call(h)
		return fmt.Errorf("클립보드 복사 실패")
	}
	return nil
}
