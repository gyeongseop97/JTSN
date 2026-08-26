//go:build windows

package main

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
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
	"unsafe"
)

const (
	WM_LBUTTONDOWN = 0x0201
	WM_LBUTTONUP   = 0x0202
	WM_MOUSEMOVE   = 0x0200
	WM_MOUSEWHEEL  = 0x020A
	MK_LBUTTON     = 0x0001
	CBN_SELCHANGE  = 1

	WM_APP_PDFLOADED = WM_APP + 30
	WM_APP_PDFTHUMBS = WM_APP + 31

	ID_PDF_MODE_MERGE   = 501
	ID_PDF_MODE_SPLIT   = 502
	ID_PDF_MODE_EXTRACT = 503
	ID_PDF_PREV         = 504
	ID_PDF_NEXT         = 505
	ID_PDF_SELECT_ALL   = 506
	ID_PDF_SELECT_NONE  = 507
	ID_PDF_AUTORANGE    = 508
	ID_PDF_SPLIT_COUNT  = 451

	PDF_MODE_MERGE = iota
	PDF_MODE_SPLIT
	PDF_MODE_EXTRACT

	BI_RGB         = 0
	DIB_RGB_COLORS = 0
	SRCCOPY        = 0x00CC0020
)

type BITMAPINFOHEADER struct {
	BiSize          uint32
	BiWidth         int32
	BiHeight        int32
	BiPlanes        uint16
	BiBitCount      uint16
	BiCompression   uint32
	BiSizeImage     uint32
	BiXPelsPerMeter int32
	BiYPelsPerMeter int32
	BiClrUsed       uint32
	BiClrImportant  uint32
}
type BITMAPINFO struct {
	BmiHeader BITMAPINFOHEADER
	BmiColors [1]uint32
}
type FS_SIZEF struct{ Width, Height float32 }
type pdfThumb struct {
	Page          int
	Width, Height int
	Stride        int
	Pixels        []byte
}
type pdfRange struct{ Start, End int }

type pdfiumFns struct {
	dll                                                                       *syscall.DLL
	initLib, destroyLib, loadMem, closeDoc, getPageCount, loadPage, closePage *syscall.Proc
	getPageSize, bmpCreate, bmpDestroy, bmpFill, bmpBuffer, bmpStride, render *syscall.Proc
}

var (
	procStretchDIBits  = gdi32.NewProc("StretchDIBits")
	procSetCapture     = user32.NewProc("SetCapture")
	procReleaseCapture = user32.NewProc("ReleaseCapture")

	pdfMode          = PDF_MODE_MERGE
	pdfPageCount     int
	pdfPageBatch     int
	pdfMergeScroll   int
	pdfDragIndex     = -1
	pdfDragHover     = -1
	pdfOutputDir     string
	pdfSelectedPages = map[int]bool{}
	pdfThumbMu       sync.Mutex
	pdfThumbs        = map[int]*pdfThumb{}
	pdfLoadToken     uint64

	pdfModeMergeBtn, pdfModeSplitBtn, pdfModeExtractBtn       syscall.Handle
	pdfFileInfo, pdfOutputInfo                                syscall.Handle
	pdfPrevBtn, pdfNextBtn, pdfSelectAllBtn, pdfSelectNoneBtn syscall.Handle
	pdfSplitCountCombo                                        syscall.Handle
	pdfSplitStartEdits                                        [10]syscall.Handle
	pdfSplitEndEdits                                          [10]syscall.Handle
	pdfSplitLabels                                            [10]syscall.Handle
	pdfSplitSepLabels                                         [10]syscall.Handle
)

func renderPDF() {
	toolHeader("PDF 도구", "")

	// Compact segmented selector with clear spacing.
	pdfModeMergeBtn = panelButton("PDF 병합", 44, 112, 116, 38, ID_PDF_MODE_MERGE, pdfModeButtonKind(PDF_MODE_MERGE))
	pdfModeSplitBtn = panelButton("PDF 분할", 168, 112, 116, 38, ID_PDF_MODE_SPLIT, pdfModeButtonKind(PDF_MODE_SPLIT))
	pdfModeExtractBtn = panelButton("페이지 추출", 292, 112, 132, 38, ID_PDF_MODE_EXTRACT, pdfModeButtonKind(PDF_MODE_EXTRACT))
	panelButton("+ PDF 추가", 786, 112, 104, 38, ID_BTN_ADD, BTN_PRIMARY)
	panelButton("목록 비우기", 900, 112, 92, 38, ID_BTN_CLEAR, BTN_SECONDARY)

	switch pdfMode {
	case PDF_MODE_MERGE:
		pdfRenderMergeControls()
	case PDF_MODE_SPLIT:
		pdfRenderSplitControls()
	case PDF_MODE_EXTRACT:
		pdfRenderExtractControls()
	}
	makeStatus(pdfInitialStatus())
	pdfUpdateFileInfo()
	pdfUpdateOutputInfo()
}

func pdfModeButtonKind(mode int) int {
	if pdfMode == mode {
		return BTN_PRIMARY
	}
	return BTN_SECONDARY
}

func pdfRenderMergeControls() {
	pdfFileInfo = panelSmall("PDF 파일을 추가한 뒤 목록에서 드래그해 병합 순서를 정하세요.", 44, 170, 930, 24, true)
	panelButton("저장 폴더", 44, 610, 106, 36, ID_BTN_OUTPUT, BTN_SECONDARY)
	pdfOutputInfo = panelSmall("", 164, 617, 590, 24, true)
	runButton = panelButton("PDF 병합", 790, 606, 184, 42, ID_BTN_RUN, BTN_PRIMARY)
}

func pdfRenderSplitControls() {
	pdfFileInfo = panelSmall("분할할 PDF 1개를 추가해 주세요.", 44, 170, 930, 24, true)

	panelSmall("분할 개수", 52, 210, 78, 22, true)
	pdfSplitCountCombo = panelCombo(132, 204, 96, 36, ID_PDF_SPLIT_COUNT)
	for i := 2; i <= 10; i++ {
		comboAdd(pdfSplitCountCombo, strconv.Itoa(i)+"개")
	}
	comboSelect(pdfSplitCountCombo, 0)
	panelButton("자동으로 나누기", 244, 202, 150, 38, ID_PDF_AUTORANGE, BTN_SECONDARY)
	panelSmall("페이지 범위는 아래에서 직접 수정할 수 있습니다.", 414, 210, 420, 22, true)

	for i := 0; i < 10; i++ {
		col, row := i/5, i%5
		x := 60 + col*454
		y := 322 + row*48
		pdfSplitLabels[i] = panelSmall(fmt.Sprintf("분할 %d", i+1), x, y+7, 62, 22, false)
		pdfSplitStartEdits[i] = panelEdit("", x+72, y, 78, 32, false, false, 600+i*2)
		pdfSplitSepLabels[i] = panelSmall("~", x+158, y+7, 18, 22, true)
		pdfSplitEndEdits[i] = panelEdit("", x+184, y, 78, 32, false, false, 601+i*2)
	}
	pdfApplySplitControlVisibility()

	panelButton("저장 폴더", 44, 610, 106, 36, ID_BTN_OUTPUT, BTN_SECONDARY)
	pdfOutputInfo = panelSmall("", 164, 617, 590, 24, true)
	runButton = panelButton("PDF 분할", 790, 606, 184, 42, ID_BTN_RUN, BTN_PRIMARY)
}

func pdfRenderExtractControls() {
	pdfFileInfo = panelSmall("추출할 PDF 1개를 추가하고 원하는 페이지를 클릭하세요.", 44, 170, 650, 24, true)
	pdfSelectAllBtn = panelButton("전체 선택", 770, 160, 98, 34, ID_PDF_SELECT_ALL, BTN_SECONDARY)
	pdfSelectNoneBtn = panelButton("선택 해제", 878, 160, 98, 34, ID_PDF_SELECT_NONE, BTN_SECONDARY)
	pdfPrevBtn = panelButton("◀ 이전", 44, 610, 84, 34, ID_PDF_PREV, BTN_SECONDARY)
	pdfNextBtn = panelButton("다음 ▶", 138, 610, 84, 34, ID_PDF_NEXT, BTN_SECONDARY)
	panelButton("저장 폴더", 244, 610, 102, 34, ID_BTN_OUTPUT, BTN_SECONDARY)
	pdfOutputInfo = panelSmall("", 360, 617, 390, 24, true)
	runButton = panelButton("선택 페이지 추출", 790, 606, 184, 42, ID_BTN_RUN, BTN_PRIMARY)
}

func pdfInitialStatus() string {
	switch pdfMode {
	case PDF_MODE_MERGE:
		return "여러 PDF를 추가한 뒤 파일 행을 드래그해 순서를 정하고 병합하세요."
	case PDF_MODE_SPLIT:
		return "PDF를 추가하면 전체 페이지 수를 확인하고 분할 구간을 자동으로 제안합니다."
	default:
		return "PDF를 추가하면 페이지 미리보기가 표시됩니다. 원하는 페이지만 클릭해 선택하세요."
	}
}

func pdfHandleCommand(id, notify int) {
	if id == ID_PDF_SPLIT_COUNT && notify == CBN_SELCHANGE {
		pdfApplySplitControlVisibility()
		pdfAutoRanges()
		return
	}
	switch id {
	case ID_PDF_MODE_MERGE:
		pdfSwitchMode(PDF_MODE_MERGE)
	case ID_PDF_MODE_SPLIT:
		pdfSwitchMode(PDF_MODE_SPLIT)
	case ID_PDF_MODE_EXTRACT:
		pdfSwitchMode(PDF_MODE_EXTRACT)
	case ID_BTN_ADD:
		files := openFiles("PDF 파일 선택", "PDF 파일\x00*.pdf\x00\x00")
		if len(files) == 0 {
			return
		}
		pdfAcceptFiles(files)
	case ID_BTN_CLEAR:
		pdfClearFiles()
	case ID_BTN_OUTPUT:
		if d := pickFolder(); d != "" {
			pdfOutputDir = d
			pdfUpdateOutputInfo()
		}
	case ID_PDF_PREV:
		if pdfPageBatch > 0 {
			pdfPageBatch--
			pdfEnsureThumbBatch()
			pdfInvalidate()
		}
	case ID_PDF_NEXT:
		if (pdfPageBatch+1)*10 < pdfPageCount {
			pdfPageBatch++
			pdfEnsureThumbBatch()
			pdfInvalidate()
		}
	case ID_PDF_SELECT_ALL:
		for i := 1; i <= pdfPageCount; i++ {
			pdfSelectedPages[i] = true
		}
		pdfInvalidate()
		pdfUpdateFileInfo()
	case ID_PDF_SELECT_NONE:
		pdfSelectedPages = map[int]bool{}
		pdfInvalidate()
		pdfUpdateFileInfo()
	case ID_PDF_AUTORANGE:
		pdfAutoRanges()
	case ID_BTN_RUN:
		pdfRunCurrentAction()
	}
}

// keep handleAction() linkable although PDF commands are routed with notification info.
func handlePDF(id int) { pdfHandleCommand(id, 0) }

func pdfSwitchMode(mode int) {
	if pdfMode == mode {
		return
	}
	pdfMode = mode
	pdfMergeScroll, pdfDragIndex, pdfDragHover = 0, -1, -1
	pdfPageBatch = 0
	pdfSelectedPages = map[int]bool{}
	pdfClearThumbs()
	pdfPageCount = 0
	if mode != PDF_MODE_MERGE && len(currentFiles) > 1 {
		currentFiles = currentFiles[:1]
	}
	pdfRebuildUI()
	if mode != PDF_MODE_MERGE && len(currentFiles) == 1 {
		pdfLoadSingleAsync(currentFiles[0])
	}
}

func pdfRebuildUI() {
	for _, h := range dynamicControls {
		if h != 0 {
			procDestroyWindow.Call(uintptr(h))
		}
		delete(customCombos, h)
		delete(panelControls, h)
		delete(headerControls, h)
		delete(mutedControls, h)
		delete(buttonKinds, h)
		delete(buttonIDs, h)
	}
	dynamicControls = nil
	inputFrames = nil
	focusedControl = 0
	statusHandle, progressHandle, progressLabelHandle = 0, 0, 0
	editMain, comboMain, editA, editB, editC, editD, runButton = 0, 0, 0, 0, 0, 0, 0
	comboB, comboC, comboD, comboE, comboF = 0, 0, 0, 0, 0
	pdfSplitStartEdits = [10]syscall.Handle{}
	pdfSplitEndEdits = [10]syscall.Handle{}
	pdfSplitLabels = [10]syscall.Handle{}
	pdfSplitSepLabels = [10]syscall.Handle{}
	renderPDF()
	pdfInvalidate()
}

func pdfAcceptFiles(files []string) {
	var valid []string
	for _, f := range files {
		if isExt(f, ".pdf") {
			valid = append(valid, f)
		}
	}
	if len(valid) == 0 {
		return
	}
	if pdfMode == PDF_MODE_MERGE {
		currentFiles = appendUnique(currentFiles, valid...)
		if pdfOutputDir == "" && len(currentFiles) > 0 {
			pdfOutputDir = filepath.Dir(currentFiles[0])
		}
		setStatus(fmt.Sprintf("PDF %d개 추가됨 · 파일 행을 드래그해 병합 순서를 정하세요.", len(currentFiles)))
		pdfUpdateFileInfo()
		pdfUpdateOutputInfo()
		pdfInvalidate()
		return
	}
	currentFiles = []string{valid[0]}
	pdfOutputDir = filepath.Dir(valid[0])
	pdfPageBatch = 0
	pdfPageCount = 0
	pdfSelectedPages = map[int]bool{}
	pdfClearThumbs()
	pdfUpdateFileInfo()
	pdfUpdateOutputInfo()
	pdfInvalidate()
	pdfLoadSingleAsync(valid[0])
}

func pdfHandleExternalDrop(paths []string) {
	var files []string
	for _, p := range paths {
		if st, err := os.Stat(p); err == nil && st.IsDir() {
			files = append(files, collectTopLevelFiles(p, map[string]bool{".pdf": true})...)
		} else if isExt(p, ".pdf") {
			files = append(files, p)
		}
	}
	pdfAcceptFiles(files)
}

func pdfClearFiles() {
	currentFiles = nil
	pdfOutputDir = ""
	pdfPageCount = 0
	pdfPageBatch = 0
	pdfMergeScroll = 0
	pdfSelectedPages = map[int]bool{}
	pdfClearThumbs()
	pdfLoadToken++
	pdfUpdateFileInfo()
	pdfUpdateOutputInfo()
	setStatus(pdfInitialStatus())
	pdfInvalidate()
	if pdfMode == PDF_MODE_SPLIT {
		pdfAutoRanges()
	}
}

func pdfUpdateFileInfo() {
	if pdfFileInfo == 0 {
		return
	}
	switch pdfMode {
	case PDF_MODE_MERGE:
		if len(currentFiles) == 0 {
			setText(pdfFileInfo, "PDF 파일을 이 영역에 끌어다 놓으세요. 목록 안에서 위·아래로 드래그하면 병합 순서가 바뀝니다.")
		} else {
			setText(pdfFileInfo, fmt.Sprintf("총 %d개 PDF · 첫 번째 파일부터 순서대로 병합됩니다.", len(currentFiles)))
		}
	case PDF_MODE_SPLIT:
		if len(currentFiles) == 0 {
			setText(pdfFileInfo, "분할할 PDF 1개를 추가해 주세요.")
		} else if pdfPageCount > 0 {
			setText(pdfFileInfo, fmt.Sprintf("%s · 총 %d페이지", filepath.Base(currentFiles[0]), pdfPageCount))
		} else {
			setText(pdfFileInfo, filepath.Base(currentFiles[0])+" · 페이지 수 확인 중...")
		}
	case PDF_MODE_EXTRACT:
		sel := len(pdfSelectedPages)
		if len(currentFiles) == 0 {
			setText(pdfFileInfo, "추출할 PDF 1개를 추가해 주세요. 페이지 미리보기를 클릭하면 선택/해제됩니다.")
		} else if pdfPageCount > 0 {
			setText(pdfFileInfo, fmt.Sprintf("%s · 총 %d페이지 · %d페이지 선택", filepath.Base(currentFiles[0]), pdfPageCount, sel))
		} else {
			setText(pdfFileInfo, filepath.Base(currentFiles[0])+" · 미리보기 준비 중...")
		}
	}
}

func pdfUpdateOutputInfo() {
	if pdfOutputInfo == 0 {
		return
	}
	if len(currentFiles) == 0 {
		setText(pdfOutputInfo, "파일을 추가하면 저장 파일명이 자동 설정됩니다.")
		return
	}
	dir := pdfOutputDir
	if dir == "" {
		dir = filepath.Dir(currentFiles[0])
	}
	base := strings.TrimSuffix(filepath.Base(currentFiles[0]), filepath.Ext(currentFiles[0]))
	switch pdfMode {
	case PDF_MODE_MERGE:
		setText(pdfOutputInfo, filepath.Join(dir, base+"_병합.pdf"))
	case PDF_MODE_SPLIT:
		setText(pdfOutputInfo, filepath.Join(dir, base+"_분할1.pdf")+" ...")
	case PDF_MODE_EXTRACT:
		setText(pdfOutputInfo, filepath.Join(dir, base+"_추출.pdf"))
	}
}

func pdfApplySplitControlVisibility() {
	if pdfSplitCountCombo == 0 {
		return
	}
	n := comboIndex(pdfSplitCountCombo) + 2
	for i := 0; i < 10; i++ {
		show := SW_HIDE
		if i < n {
			show = SW_SHOW
		}
		for _, h := range []syscall.Handle{pdfSplitLabels[i], pdfSplitStartEdits[i], pdfSplitSepLabels[i], pdfSplitEndEdits[i]} {
			if h != 0 {
				procShowWindow.Call(uintptr(h), uintptr(show))
			}
		}
	}
}

func pdfAutoRanges() {
	if pdfMode != PDF_MODE_SPLIT || pdfSplitCountCombo == 0 {
		return
	}
	n := comboIndex(pdfSplitCountCombo) + 2
	if pdfPageCount <= 0 {
		for i := 0; i < n; i++ {
			setText(pdfSplitStartEdits[i], "")
			setText(pdfSplitEndEdits[i], "")
		}
		return
	}
	if n > pdfPageCount {
		n = pdfPageCount
	}
	base := pdfPageCount / n
	rem := pdfPageCount % n
	start := 1
	for i := 0; i < 10; i++ {
		if i < n {
			size := base
			if i < rem {
				size++
			}
			end := start + size - 1
			setText(pdfSplitStartEdits[i], strconv.Itoa(start))
			setText(pdfSplitEndEdits[i], strconv.Itoa(end))
			start = end + 1
		} else {
			setText(pdfSplitStartEdits[i], "")
			setText(pdfSplitEndEdits[i], "")
		}
	}
}

func pdfReadRanges() ([]pdfRange, error) {
	n := comboIndex(pdfSplitCountCombo) + 2
	ranges := make([]pdfRange, 0, n)
	for i := 0; i < n; i++ {
		a, _ := strconv.Atoi(strings.TrimSpace(getText(pdfSplitStartEdits[i])))
		b, _ := strconv.Atoi(strings.TrimSpace(getText(pdfSplitEndEdits[i])))
		if a < 1 || b < a || (pdfPageCount > 0 && b > pdfPageCount) {
			return nil, fmt.Errorf("분할 %d의 페이지 범위를 확인해 주세요", i+1)
		}
		ranges = append(ranges, pdfRange{a, b})
	}
	return ranges, nil
}

func pdfRunCurrentAction() {
	if busy {
		return
	}
	if len(currentFiles) == 0 {
		info("PDF 파일을 먼저 추가해 주세요.")
		return
	}
	switch pdfMode {
	case PDF_MODE_MERGE:
		if len(currentFiles) < 2 {
			info("병합할 PDF를 2개 이상 추가해 주세요.")
			return
		}
		files := append([]string(nil), currentFiles...)
		dir := pdfEffectiveOutputDir()
		out := pdfUniquePath(filepath.Join(dir, pdfBase(files[0])+"_병합.pdf"))
		startBusy("PDF 병합 준비 중...")
		go func() {
			q, err := ensureQPDF(func(s string, p int) { postStatus(s); postProgress(p) })
			if err == nil {
				postStatus("PDF를 병합하고 있습니다...")
				err = pdfMerge(q, files, out)
			}
			if err != nil {
				postStatus("PDF 병합 오류: " + err.Error())
				postError("PDF 병합 오류\n" + err.Error())
			} else {
				postStatus("병합 완료 · " + out)
				postProgress(100)
			}
			postDone()
		}()
	case PDF_MODE_SPLIT:
		if pdfPageCount <= 0 {
			info("PDF 페이지 정보를 불러오는 중입니다. 잠시 후 다시 시도해 주세요.")
			return
		}
		ranges, err := pdfReadRanges()
		if err != nil {
			info(err.Error())
			return
		}
		input := currentFiles[0]
		dir := pdfEffectiveOutputDir()
		startBusy("PDF 분할 준비 중...")
		go func() {
			q, e := ensureQPDF(func(s string, p int) { postStatus(s); postProgress(p) })
			if e == nil {
				e = pdfSplitCustomRanges(q, input, ranges, dir)
			}
			if e != nil {
				postStatus("PDF 분할 오류: " + e.Error())
				postError("PDF 분할 오류\n" + e.Error())
			} else {
				postStatus(fmt.Sprintf("분할 완료 · %d개 파일 · %s", len(ranges), dir))
				postProgress(100)
			}
			postDone()
		}()
	case PDF_MODE_EXTRACT:
		if len(pdfSelectedPages) == 0 {
			info("추출할 페이지를 미리보기에서 선택해 주세요.")
			return
		}
		pages := make([]int, 0, len(pdfSelectedPages))
		for p := range pdfSelectedPages {
			pages = append(pages, p)
		}
		sort.Ints(pages)
		parts := make([]string, len(pages))
		for i, p := range pages {
			parts[i] = strconv.Itoa(p)
		}
		input := currentFiles[0]
		out := pdfUniquePath(filepath.Join(pdfEffectiveOutputDir(), pdfBase(input)+"_추출.pdf"))
		pageSpec := strings.Join(parts, ",")
		startBusy("선택 페이지 추출 준비 중...")
		go func() {
			q, e := ensureQPDF(func(s string, p int) { postStatus(s); postProgress(p) })
			if e == nil {
				postStatus("선택한 페이지를 추출하고 있습니다...")
				e = pdfExtract(q, input, pageSpec, out)
			}
			if e != nil {
				postStatus("페이지 추출 오류: " + e.Error())
				postError("페이지 추출 오류\n" + e.Error())
			} else {
				postStatus("추출 완료 · " + out)
				postProgress(100)
			}
			postDone()
		}()
	}
}

func pdfEffectiveOutputDir() string {
	if pdfOutputDir != "" {
		return pdfOutputDir
	}
	if len(currentFiles) > 0 {
		return filepath.Dir(currentFiles[0])
	}
	return ""
}
func pdfBase(path string) string { return strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)) }
func pdfUniquePath(path string) string {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return path
	}
	ext := filepath.Ext(path)
	base := strings.TrimSuffix(path, ext)
	for i := 2; i < 1000; i++ {
		p := fmt.Sprintf("%s (%d)%s", base, i, ext)
		if _, err := os.Stat(p); os.IsNotExist(err) {
			return p
		}
	}
	return path
}

func pdfSplitCustomRanges(q, input string, ranges []pdfRange, outDir string) error {
	base := pdfBase(input)
	for i, r := range ranges {
		out := pdfUniquePath(filepath.Join(outDir, fmt.Sprintf("%s_분할%d.pdf", base, i+1)))
		spec := fmt.Sprintf("%d-%d", r.Start, r.End)
		if err := pdfExtract(q, input, spec, out); err != nil {
			return fmt.Errorf("분할 %d (%s): %w", i+1, spec, err)
		}
	}
	return nil
}

func pdfLoadSingleAsync(path string) {
	pdfLoadToken++
	token := pdfLoadToken
	setStatus("페이지 수와 미리보기를 준비하고 있습니다...")
	go func() {
		count, thumbs, err := pdfLoadWithPDFium(path, 0, 10, func(s string) { postStatus(s) })
		if token != pdfLoadToken {
			return
		}
		if err != nil {
			// Preview failure must not break PDF operations; fall back to qpdf for page count.
			q, qerr := ensureQPDF(func(s string, p int) { postStatus(s); postProgress(p) })
			if qerr == nil {
				count, qerr = pdfPageCountQPDF(q, path)
			}
			if qerr != nil {
				postStatus("페이지 정보를 읽지 못했습니다: " + err.Error())
				return
			}
			postStatus("페이지 미리보기 엔진을 사용할 수 없어 페이지 번호로 표시합니다.")
		}
		pdfThumbMu.Lock()
		if token == pdfLoadToken {
			pdfPageCount = count
			for k, v := range thumbs {
				pdfThumbs[k] = v
			}
		}
		pdfThumbMu.Unlock()
		procPostMessageW.Call(uintptr(mainHWND), WM_APP_PDFLOADED, 0, 0)
	}()
}

func pdfEnsureThumbBatch() {
	if pdfMode != PDF_MODE_EXTRACT || len(currentFiles) == 0 || pdfPageCount <= 0 {
		return
	}
	start := pdfPageBatch*10 + 1
	end := start + 9
	if end > pdfPageCount {
		end = pdfPageCount
	}
	missing := false
	pdfThumbMu.Lock()
	for p := start; p <= end; p++ {
		if pdfThumbs[p] == nil {
			missing = true
			break
		}
	}
	pdfThumbMu.Unlock()
	if !missing {
		return
	}
	path := currentFiles[0]
	token := pdfLoadToken
	go func() {
		_, thumbs, err := pdfLoadWithPDFium(path, start-1, end-start+1, nil)
		if err != nil || token != pdfLoadToken {
			return
		}
		pdfThumbMu.Lock()
		for k, v := range thumbs {
			pdfThumbs[k] = v
		}
		pdfThumbMu.Unlock()
		procPostMessageW.Call(uintptr(mainHWND), WM_APP_PDFTHUMBS, 0, 0)
	}()
}

func pdfPageCountQPDF(q, path string) (int, error) {
	cmd := execCommand(q, "--show-npages", path)
	b, err := cmd.CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("%v: %s", err, strings.TrimSpace(string(b)))
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil {
		return 0, err
	}
	return n, nil
}

// indirection makes this file easier to audit alongside the existing runCmd implementation.
func execCommand(name string, args ...string) *exec.Cmd { return exec.Command(name, args...) }

func pdfWindowMessage(hwnd syscall.Handle, msg uint32, wParam, lParam uintptr) (bool, uintptr) {
	switch msg {
	case WM_APP_PDFLOADED:
		pdfUpdateFileInfo()
		pdfUpdateOutputInfo()
		if pdfMode == PDF_MODE_SPLIT {
			pdfAutoRanges()
		}
		if pdfMode == PDF_MODE_EXTRACT {
			pdfEnsureThumbBatch()
		}
		pdfInvalidate()
		return true, 0
	case WM_APP_PDFTHUMBS:
		pdfInvalidate()
		return true, 0
	case WM_LBUTTONDOWN:
		x, y := pdfXY(lParam)
		if pdfMode == PDF_MODE_MERGE {
			if idx := pdfMergeIndexAt(x, y); idx >= 0 {
				pdfDragIndex = idx
				pdfDragHover = idx
				procSetCapture.Call(uintptr(hwnd))
				pdfInvalidate()
				return true, 0
			}
		} else if pdfMode == PDF_MODE_EXTRACT {
			if page := pdfExtractPageAt(x, y); page > 0 {
				pdfSelectedPages[page] = !pdfSelectedPages[page]
				if !pdfSelectedPages[page] {
					delete(pdfSelectedPages, page)
				}
				pdfUpdateFileInfo()
				pdfInvalidate()
				return true, 0
			}
		}
	case WM_MOUSEMOVE:
		if pdfMode == PDF_MODE_MERGE && pdfDragIndex >= 0 && (wParam&MK_LBUTTON) != 0 {
			x, y := pdfXY(lParam)
			idx := pdfMergeIndexAt(x, y)
			if idx >= 0 && idx != pdfDragHover {
				pdfDragHover = idx
				pdfInvalidate()
			}
			return true, 0
		}
	case WM_LBUTTONUP:
		if pdfMode == PDF_MODE_MERGE && pdfDragIndex >= 0 {
			x, y := pdfXY(lParam)
			idx := pdfMergeIndexAt(x, y)
			if idx < 0 {
				idx = pdfDragHover
			}
			pdfMoveFile(pdfDragIndex, idx)
			pdfDragIndex, pdfDragHover = -1, -1
			procReleaseCapture.Call()
			pdfUpdateOutputInfo()
			pdfInvalidate()
			return true, 0
		}
	case WM_MOUSEWHEEL:
		if pdfMode == PDF_MODE_MERGE && len(currentFiles) > 8 {
			delta := int16((wParam >> 16) & 0xffff)
			if delta < 0 {
				pdfMergeScroll++
			} else {
				pdfMergeScroll--
			}
			maxScroll := len(currentFiles) - 8
			if pdfMergeScroll < 0 {
				pdfMergeScroll = 0
			}
			if pdfMergeScroll > maxScroll {
				pdfMergeScroll = maxScroll
			}
			pdfInvalidate()
			return true, 0
		}
	}
	return false, 0
}

func pdfXY(l uintptr) (int, int) { return int(int16(l & 0xffff)), int(int16((l >> 16) & 0xffff)) }
func pdfMergeIndexAt(x, y int) int {
	if x < 54 || x > 964 || y < 212 || y > 588 {
		return -1
	}
	row := (y - 212) / 45
	idx := pdfMergeScroll + row
	if idx >= 0 && idx < len(currentFiles) {
		return idx
	}
	return -1
}
func pdfMoveFile(from, to int) {
	if from < 0 || to < 0 || from >= len(currentFiles) || to >= len(currentFiles) || from == to {
		return
	}
	f := currentFiles[from]
	if from < to {
		copy(currentFiles[from:to], currentFiles[from+1:to+1])
	} else {
		copy(currentFiles[to+1:from+1], currentFiles[to:from])
	}
	currentFiles[to] = f
	setStatus(fmt.Sprintf("병합 순서 변경 · %d번째 → %d번째", from+1, to+1))
}
func pdfExtractPageAt(x, y int) int {
	if x < 54 || x > 974 || y < 214 || y > 586 {
		return 0
	}
	col := (x - 54) / 184
	row := (y - 214) / 180
	if col < 0 || col >= 5 || row < 0 || row >= 2 {
		return 0
	}
	p := pdfPageBatch*10 + row*5 + col + 1
	if p > pdfPageCount {
		return 0
	}
	return p
}
func pdfInvalidate() {
	if mainHWND != 0 {
		procInvalidateRect.Call(uintptr(mainHWND), 0, 1)
	}
}
func pdfClearThumbs() { pdfThumbMu.Lock(); pdfThumbs = map[int]*pdfThumb{}; pdfThumbMu.Unlock() }

func paintPDFCanvas(hdc syscall.Handle) {
	switch pdfMode {
	case PDF_MODE_MERGE:
		pdfPaintMerge(hdc)
	case PDF_MODE_SPLIT:
		pdfPaintSplit(hdc)
	case PDF_MODE_EXTRACT:
		pdfPaintExtract(hdc)
	}
}

func pdfPaintMerge(hdc syscall.Handle) {
	// Large neutral drop/list surface. It behaves like a modern file queue rather than a text box.
	pdfDrawPanel(hdc, 44, 204, 930, 388, rgb(248, 250, 252), rgb(226, 232, 240), 14)
	if len(currentFiles) == 0 {
		pdfDrawCentered(hdc, "PDF 파일을 이곳으로 끌어다 놓으세요", RECT{70, 350, 948, 386}, rgb(71, 85, 105), fontNormal)
		pdfDrawCentered(hdc, "여러 파일을 한 번에 추가할 수 있습니다", RECT{70, 388, 948, 418}, rgb(148, 163, 184), fontSmall)
		return
	}
	visible := 8
	end := pdfMergeScroll + visible
	if end > len(currentFiles) {
		end = len(currentFiles)
	}
	y := 212
	for i := pdfMergeScroll; i < end; i++ {
		selected := pdfDragHover == i && pdfDragIndex >= 0
		bg := rgb(255, 255, 255)
		border := rgb(226, 232, 240)
		if selected {
			bg = rgb(239, 246, 255)
			border = rgb(59, 130, 246)
		}
		pdfDrawPanel(hdc, 54, y, 910, 38, bg, border, 9)
		pdfDrawText(hdc, "⋮⋮", RECT{68, int32(y + 8), 96, int32(y + 31)}, rgb(148, 163, 184), fontButton, DT_LEFT|DT_SINGLELINE)
		pdfDrawText(hdc, fmt.Sprintf("%02d", i+1), RECT{104, int32(y + 8), 140, int32(y + 31)}, rgb(47, 97, 235), fontButton, DT_LEFT|DT_SINGLELINE)
		name := filepath.Base(currentFiles[i])
		dir := filepath.Dir(currentFiles[i])
		pdfDrawText(hdc, name, RECT{150, int32(y + 5), 612, int32(y + 26)}, rgb(15, 23, 42), fontNormal, DT_LEFT|DT_SINGLELINE|DT_END_ELLIPSIS)
		pdfDrawText(hdc, dir, RECT{628, int32(y + 8), 950, int32(y + 30)}, rgb(100, 116, 139), fontSmall, DT_LEFT|DT_SINGLELINE|DT_END_ELLIPSIS)
		y += 45
	}
	if len(currentFiles) > visible {
		pdfDrawText(hdc, fmt.Sprintf("마우스 휠로 목록 이동  ·  %d/%d", pdfMergeScroll+1, len(currentFiles)), RECT{760, 568, 950, 588}, rgb(100, 116, 139), fontSmall, DT_LEFT|DT_SINGLELINE)
	}
}

func pdfPaintSplit(hdc syscall.Handle) {
	// Split settings are grouped in one soft card so the page-range inputs read as a single task.
	pdfDrawPanel(hdc, 44, 270, 930, 318, rgb(248, 250, 252), rgb(226, 232, 240), 14)
	pdfDrawText(hdc, "분할 범위", RECT{60, 286, 180, 312}, rgb(51, 65, 85), fontButton, DT_LEFT|DT_SINGLELINE)
	if len(currentFiles) == 0 {
		pdfDrawCentered(hdc, "분할할 PDF를 추가하면 전체 페이지 수를 확인한 뒤 구간을 자동으로 나눠드립니다.", RECT{90, 402, 930, 444}, rgb(100, 116, 139), fontNormal)
		return
	}
	// subtle row surfaces behind each visible range
	n := 2
	if pdfSplitCountCombo != 0 {
		n = comboIndex(pdfSplitCountCombo) + 2
	}
	for i := 0; i < n && i < 10; i++ {
		col, row := i/5, i%5
		x := 52 + col*454
		y := 316 + row*48
		pdfDrawPanel(hdc, x, y, 430, 40, rgb(255, 255, 255), rgb(235, 240, 246), 9)
	}
	if pdfPageCount > 0 {
		pdfDrawText(hdc, fmt.Sprintf("전체 %d페이지", pdfPageCount), RECT{820, 286, 956, 310}, rgb(47, 97, 235), fontButton, DT_LEFT|DT_SINGLELINE)
	} else {
		pdfDrawText(hdc, "페이지 정보 확인 중…", RECT{790, 286, 956, 310}, rgb(148, 163, 184), fontSmall, DT_LEFT|DT_SINGLELINE)
	}
}

func pdfPaintExtract(hdc syscall.Handle) {
	pdfDrawPanel(hdc, 44, 204, 930, 388, rgb(248, 250, 252), rgb(226, 232, 240), 14)
	if len(currentFiles) == 0 {
		pdfDrawCentered(hdc, "PDF를 추가하면 페이지 미리보기가 여기에 표시됩니다", RECT{75, 360, 948, 396}, rgb(71, 85, 105), fontNormal)
		pdfDrawCentered(hdc, "원하는 페이지를 클릭해 선택하세요", RECT{75, 398, 948, 428}, rgb(148, 163, 184), fontSmall)
		return
	}
	if pdfPageCount <= 0 {
		pdfDrawCentered(hdc, "페이지 미리보기를 준비하고 있습니다…", RECT{75, 368, 948, 408}, rgb(100, 116, 139), fontNormal)
		return
	}
	start := pdfPageBatch*10 + 1
	for k := 0; k < 10; k++ {
		p := start + k
		if p > pdfPageCount {
			break
		}
		col, row := k%5, k/5
		x := 54 + col*184
		y := 214 + row*180
		selected := pdfSelectedPages[p]
		bg := rgb(255, 255, 255)
		border := rgb(226, 232, 240)
		if selected {
			bg = rgb(239, 246, 255)
			border = rgb(47, 97, 235)
		}
		pdfDrawPanel(hdc, x, y, 168, 156, bg, border, 10)
		pdfThumbMu.Lock()
		th := pdfThumbs[p]
		pdfThumbMu.Unlock()
		if th != nil && len(th.Pixels) > 0 {
			dx := x + (168-th.Width)/2
			dy := y + 8
			pdfDrawThumb(hdc, th, dx, dy)
		} else {
			pdfDrawCentered(hdc, "미리보기", RECT{int32(x + 18), int32(y + 50), int32(x + 150), int32(y + 78)}, rgb(148, 163, 184), fontSmall)
		}
		pdfDrawCentered(hdc, fmt.Sprintf("%d 페이지", p), RECT{int32(x + 10), int32(y + 132), int32(x + 158), int32(y + 154)}, rgb(15, 23, 42), fontSmall)
		if selected {
			pdfDrawPanel(hdc, x+134, y+8, 24, 24, rgb(47, 97, 235), rgb(47, 97, 235), 12)
			pdfDrawCentered(hdc, "✓", RECT{int32(x + 134), int32(y + 8), int32(x + 158), int32(y + 32)}, rgb(255, 255, 255), fontButton)
		}
	}
	last := start + 9
	if last > pdfPageCount {
		last = pdfPageCount
	}
	pdfDrawText(hdc, fmt.Sprintf("%d–%d / %d페이지", start, last, pdfPageCount), RECT{580, 606, 750, 630}, rgb(100, 116, 139), fontSmall, DT_LEFT|DT_SINGLELINE)
}

func pdfDrawPanel(hdc syscall.Handle, x, y, w, h int, bg, border uintptr, radius int) {
	br := solidBrush(byte(bg&0xff), byte((bg>>8)&0xff), byte((bg>>16)&0xff))
	pen, _, _ := procCreatePen.Call(PS_SOLID, 1, border)
	ob, _, _ := procSelectObject.Call(uintptr(hdc), uintptr(br))
	op, _, _ := procSelectObject.Call(uintptr(hdc), pen)
	procRoundRect.Call(uintptr(hdc), uintptr(x), uintptr(y), uintptr(x+w), uintptr(y+h), uintptr(radius), uintptr(radius))
	procSelectObject.Call(uintptr(hdc), ob)
	procSelectObject.Call(uintptr(hdc), op)
	procDeleteObject.Call(uintptr(br))
	procDeleteObject.Call(pen)
}
func pdfDrawText(hdc syscall.Handle, text string, rc RECT, color uintptr, font syscall.Handle, flags uintptr) {
	old, _, _ := procSelectObject.Call(uintptr(hdc), uintptr(font))
	procSetBkMode.Call(uintptr(hdc), TRANSPARENT)
	procSetTextColor.Call(uintptr(hdc), color)
	u := syscall.StringToUTF16(text)
	procDrawTextW.Call(uintptr(hdc), uintptr(unsafe.Pointer(&u[0])), uintptr(len(u)-1), uintptr(unsafe.Pointer(&rc)), flags)
	procSelectObject.Call(uintptr(hdc), old)
}
func pdfDrawCentered(hdc syscall.Handle, text string, rc RECT, color uintptr, font syscall.Handle) {
	pdfDrawText(hdc, text, rc, color, font, DT_CENTER|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS)
}
func pdfDrawThumb(hdc syscall.Handle, t *pdfThumb, x, y int) {
	if len(t.Pixels) == 0 {
		return
	}
	bmi := BITMAPINFO{BmiHeader: BITMAPINFOHEADER{BiSize: uint32(unsafe.Sizeof(BITMAPINFOHEADER{})), BiWidth: int32(t.Width), BiHeight: -int32(t.Height), BiPlanes: 1, BiBitCount: 32, BiCompression: BI_RGB}}
	procStretchDIBits.Call(uintptr(hdc), uintptr(x), uintptr(y), uintptr(t.Width), uintptr(t.Height), 0, 0, uintptr(t.Width), uintptr(t.Height), uintptr(unsafe.Pointer(&t.Pixels[0])), uintptr(unsafe.Pointer(&bmi)), DIB_RGB_COLORS, SRCCOPY)
}

const pdfiumURL = "https://github.com/bblanchon/pdfium-binaries/releases/download/chromium/7999/pdfium-win-x64.tgz"

func ensurePDFium(status func(string)) (string, error) {
	if exe, err := os.Executable(); err == nil {
		p := filepath.Join(filepath.Dir(exe), "tools", "pdfium.dll")
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		cache = os.TempDir()
	}
	root := filepath.Join(cache, "JTSN", "pdfium-153-7999")
	p := findFile(root, "pdfium.dll")
	if p != "" {
		return p, nil
	}
	if status != nil {
		status("페이지 미리보기 엔진을 준비하는 중입니다 · 최초 1회")
	}
	if err := downloadAndExtractTGZ(pdfiumURL, root); err != nil {
		return "", err
	}
	p = findFile(root, "pdfium.dll")
	if p == "" {
		return "", fmt.Errorf("pdfium.dll을 찾지 못했습니다")
	}
	return p, nil
}
func downloadAndExtractTGZ(url, root string) error {
	if err := os.MkdirAll(root, 0755); err != nil {
		return err
	}
	tmp := filepath.Join(root, "download.tmp.tgz")
	client := &http.Client{Timeout: 4 * time.Minute}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "JTSN/4.3")
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
	if _, err = io.Copy(f, resp.Body); err != nil {
		f.Close()
		return err
	}
	f.Close()
	f, err = os.Open(tmp)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	clean := filepath.Clean(root) + string(os.PathSeparator)
	for {
		h, e := tr.Next()
		if e == io.EOF {
			break
		}
		if e != nil {
			return e
		}
		dst := filepath.Join(root, h.Name)
		if !strings.HasPrefix(filepath.Clean(dst)+string(os.PathSeparator), clean) {
			continue
		}
		if h.FileInfo().IsDir() {
			os.MkdirAll(dst, 0755)
			continue
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
			return err
		}
		out, err := os.Create(dst)
		if err != nil {
			return err
		}
		_, err = io.Copy(out, tr)
		out.Close()
		if err != nil {
			return err
		}
	}
	os.Remove(tmp)
	return nil
}

func pdfLoadWithPDFium(path string, startZero, count int, status func(string)) (int, map[int]*pdfThumb, error) {
	dllPath, err := ensurePDFium(status)
	if err != nil {
		return 0, nil, err
	}
	api, err := loadPDFiumAPI(dllPath)
	if err != nil {
		return 0, nil, err
	}
	defer api.close()
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, nil, err
	}
	if len(data) == 0 {
		return 0, nil, fmt.Errorf("빈 PDF 파일입니다")
	}
	doc, _, _ := api.loadMem.Call(uintptr(unsafe.Pointer(&data[0])), uintptr(len(data)), 0)
	if doc == 0 {
		return 0, nil, fmt.Errorf("PDF 문서를 열지 못했습니다")
	}
	defer api.closeDoc.Call(doc)
	n, _, _ := api.getPageCount.Call(doc)
	total := int(n)
	if startZero < 0 {
		startZero = 0
	}
	if count <= 0 {
		count = 10
	}
	end := startZero + count
	if end > total {
		end = total
	}
	thumbs := map[int]*pdfThumb{}
	for i := startZero; i < end; i++ {
		th, e := api.renderPage(doc, i)
		if e == nil {
			thumbs[i+1] = th
		}
	}
	runtime.KeepAlive(data)
	return total, thumbs, nil
}
func loadPDFiumAPI(path string) (*pdfiumFns, error) {
	d, err := syscall.LoadDLL(path)
	if err != nil {
		return nil, err
	}
	get := func(n string) (*syscall.Proc, error) { p, e := d.FindProc(n); return p, e }
	a := &pdfiumFns{dll: d}
	var e error
	if a.initLib, e = get("FPDF_InitLibrary"); e != nil {
		d.Release()
		return nil, e
	}
	a.destroyLib, _ = get("FPDF_DestroyLibrary")
	a.loadMem, e = get("FPDF_LoadMemDocument64")
	if e != nil {
		d.Release()
		return nil, e
	}
	a.closeDoc, _ = get("FPDF_CloseDocument")
	a.getPageCount, _ = get("FPDF_GetPageCount")
	a.loadPage, _ = get("FPDF_LoadPage")
	a.closePage, _ = get("FPDF_ClosePage")
	a.getPageSize, _ = get("FPDF_GetPageSizeByIndexF")
	a.bmpCreate, _ = get("FPDFBitmap_Create")
	a.bmpDestroy, _ = get("FPDFBitmap_Destroy")
	a.bmpFill, _ = get("FPDFBitmap_FillRect")
	a.bmpBuffer, _ = get("FPDFBitmap_GetBuffer")
	a.bmpStride, _ = get("FPDFBitmap_GetStride")
	a.render, _ = get("FPDF_RenderPageBitmap")
	a.initLib.Call()
	return a, nil
}
func (a *pdfiumFns) close() {
	if a.destroyLib != nil {
		a.destroyLib.Call()
	}
	if a.dll != nil {
		a.dll.Release()
	}
}
//go:nocheckptr
func (a *pdfiumFns) renderPage(doc uintptr, index int) (*pdfThumb, error) {
	var sz FS_SIZEF
	ok, _, _ := a.getPageSize.Call(doc, uintptr(index), uintptr(unsafe.Pointer(&sz)))
	if ok == 0 || sz.Width <= 0 || sz.Height <= 0 {
		return nil, fmt.Errorf("페이지 크기 확인 실패")
	}
	maxW, maxH := 128.0, 116.0
	scale := maxW / float64(sz.Width)
	if float64(sz.Height)*scale > maxH {
		scale = maxH / float64(sz.Height)
	}
	w := int(float64(sz.Width) * scale)
	h := int(float64(sz.Height) * scale)
	if w < 30 {
		w = 30
	}
	if h < 30 {
		h = 30
	}
	page, _, _ := a.loadPage.Call(doc, uintptr(index))
	if page == 0 {
		return nil, fmt.Errorf("페이지 열기 실패")
	}
	defer a.closePage.Call(page)
	bmp, _, _ := a.bmpCreate.Call(uintptr(w), uintptr(h), 0)
	if bmp == 0 {
		return nil, fmt.Errorf("미리보기 버퍼 생성 실패")
	}
	defer a.bmpDestroy.Call(bmp)
	a.bmpFill.Call(bmp, 0, 0, uintptr(w), uintptr(h), 0xFFFFFFFF)
	a.render.Call(bmp, page, 0, 0, uintptr(w), uintptr(h), 0, 1)
	buf, _, _ := a.bmpBuffer.Call(bmp)
	strideV, _, _ := a.bmpStride.Call(bmp)
	stride := int(strideV)
	if buf == 0 || stride <= 0 {
		return nil, fmt.Errorf("미리보기 버퍼 읽기 실패")
	}
	pixels := make([]byte, stride*h)
	src := unsafe.Slice((*byte)(unsafe.Pointer(buf)), stride*h)
	copy(pixels, src)
	return &pdfThumb{Page: index + 1, Width: w, Height: h, Stride: stride, Pixels: pixels}, nil
}
