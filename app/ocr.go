//go:build windows

package main

import (
	"bytes"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

const (
	ID_OCR_SCREEN  = 7801
	ID_OCR_IMAGE   = 7802
	ID_OCR_COPY    = 7803
	ID_OCR_CLEAN   = 7804
	ID_OCR_CLIP    = 7805
	ID_OCR_SAVE    = 7806
	ID_OCR_NOTEPAD = 7807
	ID_OCR_CLOSE   = 7808
	STM_SETIMAGE   = 0x0172
	SS_BITMAP      = 0x000E
)

var (
	ocrRawText      string
	ocrLanguage     string
	ocrImagePath    string
	ocrResultMu     sync.Mutex
	ocrMailboxText  string
	ocrMailboxLang  string
	ocrMailboxImage string
	ocrMailboxError string
	ocrPreview      syscall.Handle
	ocrPreviewHWND  syscall.Handle
	tesseractOnce   sync.Once
	tesseractDir    string
	tesseractErr    error
)

//go:embed assets/tesseract/* assets/tesseract/tessdata/*
var tesseractFS embed.FS

func ensureTesseractRuntime() (string, error) {
	tesseractOnce.Do(func() {
		cache, err := os.UserCacheDir()
		if err != nil || cache == "" {
			cache = os.TempDir()
		}
		tesseractDir = filepath.Join(cache, "JTSN", "tesseract-5.5.0")
		tesseractErr = fs.WalkDir(tesseractFS, "assets/tesseract", func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			rel, relErr := filepath.Rel("assets/tesseract", path)
			if relErr != nil || rel == "." {
				return relErr
			}
			dst := filepath.Join(tesseractDir, filepath.FromSlash(rel))
			if d.IsDir() {
				return os.MkdirAll(dst, 0755)
			}
			data, readErr := tesseractFS.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			if st, statErr := os.Stat(dst); statErr == nil && st.Size() == int64(len(data)) {
				return nil
			}
			if mkErr := os.MkdirAll(filepath.Dir(dst), 0755); mkErr != nil {
				return mkErr
			}
			return os.WriteFile(dst, data, 0644)
		})
	})
	return tesseractDir, tesseractErr
}

func renderOCR() {
	toolHeader("화면 글자 추출 OCR", "화면이나 이미지 속 복사할 수 없는 글자를 로컬 PC에서 추출합니다.")
	panelButton("화면 캡처 OCR", 44, 124, 150, 40, ID_OCR_SCREEN, BTN_PRIMARY)
	panelButton("이미지 불러오기", 204, 124, 150, 40, ID_OCR_IMAGE, BTN_SECONDARY)
	panelSmall("단축키: Ctrl + Shift + O", 374, 133, 220, 24, true)
	panelSmall("인식 방식", 660, 124, 90, 22, true)
	comboB = panelCombo(756, 116, 220, 120, ID_COMBO_B)
	comboAdd(comboB, "일반 문서")
	comboAdd(comboB, "숫자 위주")
	comboSelect(comboB, 0)

	panelLabel("캡처 / 이미지", 44, 188, 180, 28, false)
	panelSmall("화면 영역을 선택하거나 PNG·JPG·BMP·WEBP 이미지를 끌어다 놓으세요.", 44, 220, 500, 46, true)
	ocrPreviewHWND = createWindow(0, "STATIC", "", WS_CHILD|WS_VISIBLE|SS_BITMAP, 44, 278, 400, 200, mainHWND, 0)
	dynamicControls = append(dynamicControls, ocrPreviewHWND)
	editD = panelSmall("선택된 이미지가 없습니다.", 44, 486, 500, 28, true)
	panelSmall("클립보드 저장", 44, 526, 110, 22, true)
	comboC = panelCombo(158, 518, 286, 120, ID_COMBO_C)
	comboAdd(comboC, "복사할 때만 저장")
	comboAdd(comboC, "OCR 완료 즉시 저장")
	comboSelect(comboC, 0)

	panelLabel("추출 텍스트", 560, 188, 180, 28, false)
	editMain = panelEdit("", 560, 220, 416, 326, true, false, ID_EDIT_MAIN)
	panelSmall("표시 방식", 560, 558, 90, 22, true)
	comboMain = panelCombo(650, 550, 210, 120, ID_COMBO_MAIN)
	comboAdd(comboMain, "원본 레이아웃 유지")
	comboAdd(comboMain, "텍스트 정리")
	comboSelect(comboMain, 0)

	panelButton("복사", 44, 584, 90, 38, ID_OCR_COPY, BTN_PRIMARY)
	panelButton("정리 복사", 144, 584, 104, 38, ID_OCR_CLEAN, BTN_SECONDARY)
	panelButton("고급 클립보드 저장", 258, 584, 166, 38, ID_OCR_CLIP, BTN_SECONDARY)
	panelButton("TXT 저장", 434, 584, 100, 38, ID_OCR_SAVE, BTN_SECONDARY)
	panelButton("메모장", 544, 584, 90, 38, ID_OCR_NOTEPAD, BTN_SECONDARY)
	panelButton("닫기", 886, 584, 90, 38, ID_OCR_CLOSE, BTN_SECONDARY)
	makeStatus("화면 캡처 OCR 또는 이미지 불러오기를 선택하세요.")
}

func handleOCRCommand(id int) {
	switch id {
	case ID_OCR_SCREEN:
		startScreenOCR()
	case ID_OCR_IMAGE:
		paths := openFiles("OCR 이미지 선택", "이미지 파일\x00*.png;*.jpg;*.jpeg;*.bmp;*.webp\x00\x00")
		if len(paths) > 0 {
			startImageOCR(paths[0])
		}
	case ID_OCR_COPY:
		copyOCRText(false, false)
	case ID_OCR_CLEAN:
		copyOCRText(true, false)
	case ID_OCR_CLIP:
		copyOCRText(false, true)
	case ID_OCR_SAVE:
		text := strings.TrimSpace(getText(editMain))
		if text == "" {
			info("저장할 OCR 결과가 없습니다.")
			return
		}
		if path := saveFile("OCR_추출결과.txt", "OCR 결과 저장", "텍스트 파일\x00*.txt\x00\x00"); path != "" {
			if err := os.WriteFile(path, []byte(text), 0644); err != nil {
				errorBox(err.Error())
			} else {
				setStatus("OCR 결과를 저장했습니다: " + path)
			}
		}
	case ID_OCR_NOTEPAD:
		text := strings.TrimSpace(getText(editMain))
		if text == "" {
			info("메모장으로 열 OCR 결과가 없습니다.")
			return
		}
		path := filepath.Join(os.TempDir(), fmt.Sprintf("JTSN_OCR_%d.txt", time.Now().UnixNano()))
		if os.WriteFile(path, []byte(text), 0644) == nil {
			_ = exec.Command("notepad.exe", path).Start()
		}
	case ID_OCR_CLOSE:
		procDestroyWindow.Call(uintptr(mainHWND))
	}
}

func handleOCRComboChange(id int) {
	if id != ID_COMBO_MAIN || editMain == 0 || ocrRawText == "" {
		return
	}
	if comboText(comboMain) == "텍스트 정리" {
		setText(editMain, cleanedOCRText(ocrRawText))
	} else {
		setText(editMain, ocrRawText)
	}
}

func cleanedOCRText(text string) string {
	lines := splitLines(text)
	parts := make([]string, 0, len(lines))
	for _, line := range lines {
		if t := strings.Join(strings.Fields(line), " "); t != "" {
			parts = append(parts, t)
		}
	}
	return strings.Join(parts, " ")
}

func copyOCRText(clean, explicitClipboardSave bool) {
	text := getText(editMain)
	if clean || comboText(comboMain) == "텍스트 정리" {
		text = cleanedOCRText(text)
	}
	if strings.TrimSpace(text) == "" {
		info("복사할 OCR 결과가 없습니다.")
		return
	}
	if err := copyClipboard(text); err != nil {
		errorBox(err.Error())
		return
	}
	if explicitClipboardSave {
		setStatus("OCR 결과를 Windows 및 고급 클립보드에 저장했습니다.")
	} else {
		setStatus("OCR 결과를 복사했습니다.")
	}
}

func startScreenOCR() {
	if busy {
		return
	}
	procShowWindow.Call(uintptr(mainHWND), SW_HIDE)
	startOCRWorker("screen", "")
}

func startImageOCR(path string) {
	if busy {
		return
	}
	if !isExt(path, ".png", ".jpg", ".jpeg", ".bmp", ".webp") {
		info("PNG, JPG, JPEG, BMP, WEBP 이미지만 사용할 수 있습니다.")
		return
	}
	startOCRWorker("image", path)
}

func startOCRWorker(mode, source string) {
	startBusy("텍스트 추출 중...")
	go func() {
		cache, _ := os.UserCacheDir()
		if cache == "" {
			cache = os.TempDir()
		}
		dir := filepath.Join(cache, "JTSN", "ocr")
		_ = os.MkdirAll(dir, 0755)
		script := filepath.Join(dir, "jtsn_ocr.ps1")
		_ = os.WriteFile(script, []byte(ocrPowerShell), 0600)
		stamp := fmt.Sprintf("%d-%d", os.Getpid(), time.Now().UnixNano())
		imagePath := source
		if mode == "screen" {
			imagePath = filepath.Join(dir, "capture-"+stamp+".bmp")
		}
		outPath := filepath.Join(dir, "result-"+stamp+".txt")
		previewPath := filepath.Join(dir, "preview-"+stamp+".bmp")
		cmd := exec.Command("powershell.exe", "-NoProfile", "-ExecutionPolicy", "Bypass", "-STA", "-File", script, "-Mode", mode, "-ImagePath", imagePath, "-OutputPath", outPath, "-PreviewPath", previewPath)
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
		output, err := cmd.CombinedOutput()
		text, lang, errText := "", "한국어 + 영어 (Tesseract)", ""
		if err != nil {
			if mode == "screen" && strings.Contains(strings.ToLower(string(output)), "cancelled") {
				errText = "취소"
			} else {
				errText = strings.TrimSpace(string(output))
				if errText == "" {
					errText = err.Error()
				}
			}
		} else {
			runtimeDir, runtimeErr := ensureTesseractRuntime()
			if runtimeErr != nil {
				errText = "내장 OCR 엔진 준비 실패: " + runtimeErr.Error()
			} else {
				recognitionPath := outPath + ".ocr.bmp"
				tess := exec.Command(filepath.Join(runtimeDir, "tesseract.exe"), recognitionPath, "stdout", "-l", "kor+eng", "--psm", "6", "--tessdata-dir", filepath.Join(runtimeDir, "tessdata"), "-c", "preserve_interword_spaces=1")
				tess.Dir = runtimeDir
				tess.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
				var stderr bytes.Buffer
				tess.Stderr = &stderr
				recognized, tessErr := tess.Output()
				_ = os.Remove(recognitionPath)
				if tessErr != nil {
					errText = strings.TrimSpace(stderr.String())
					if errText == "" {
						errText = tessErr.Error()
					}
				} else {
					text = strings.TrimSpace(string(recognized))
				}
			}
		}
		_ = os.Remove(outPath)
		ocrResultMu.Lock()
		ocrMailboxText, ocrMailboxLang, ocrMailboxImage, ocrMailboxError = text, lang, imagePath, errText
		if errText == "" {
			ocrMailboxImage = previewPath
		}
		ocrResultMu.Unlock()
		procPostMessageW.Call(uintptr(mainHWND), WM_APP_OCR_DONE, 0, 0)
	}()
}

func handleOCRDone() {
	ocrResultMu.Lock()
	text, lang, imagePath, errText := ocrMailboxText, ocrMailboxLang, ocrMailboxImage, ocrMailboxError
	ocrResultMu.Unlock()
	finishBusy()
	procShowWindow.Call(uintptr(mainHWND), SW_SHOW)
	procSetForegroundWindow.Call(uintptr(mainHWND))
	if errText == "취소" {
		setStatus("화면 OCR을 취소했습니다.")
		return
	}
	if errText != "" {
		errorBox("OCR 처리 중 오류가 발생했습니다.\n\n" + errText)
		return
	}
	ocrRawText, ocrLanguage, ocrImagePath = text, lang, imagePath
	setText(editMain, text)
	setText(editD, "원본 캡처/이미지 미리보기")
	if ocrPreview != 0 {
		procDeleteObject.Call(uintptr(ocrPreview))
		ocrPreview = 0
	}
	if h, _, _ := procLoadImageW.Call(0, uintptr(unsafe.Pointer(p16(imagePath))), IMAGE_BITMAP, 400, 200, LR_LOADFROMFILE|LR_CREATEDIBSECTION); h != 0 {
		ocrPreview = syscall.Handle(h)
		procSendMessageW.Call(uintptr(ocrPreviewHWND), STM_SETIMAGE, IMAGE_BITMAP, h)
	}
	if strings.TrimSpace(text) == "" {
		setStatus("텍스트를 인식하지 못했습니다. 더 넓거나 선명한 영역으로 다시 시도해 주세요.")
	} else {
		if comboText(comboC) == "OCR 완료 즉시 저장" {
			_ = copyClipboard(text)
			setStatus("OCR 완료 · 인식 언어: " + lang + " · 고급 클립보드 저장 완료")
		} else {
			setStatus("OCR 완료 · 인식 언어: " + lang)
		}
	}
}

const ocrPowerShell = `param([string]$Mode,[string]$ImagePath,[string]$OutputPath,[string]$PreviewPath)
$ErrorActionPreference='Stop'
$OutputEncoding=New-Object Text.UTF8Encoding($false)
[Console]::OutputEncoding=$OutputEncoding
Add-Type -AssemblyName System.Drawing
Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Runtime.WindowsRuntime
Add-Type @"
using System; using System.Runtime.InteropServices;
public static class JTSNDpi { [DllImport("user32.dll")] public static extern bool SetProcessDpiAwarenessContext(IntPtr v); }
"@
[JTSNDpi]::SetProcessDpiAwarenessContext([IntPtr](-4)) | Out-Null
if($Mode -eq 'screen') {
  $bounds=[System.Windows.Forms.SystemInformation]::VirtualScreen
  $form=New-Object System.Windows.Forms.Form
  $form.FormBorderStyle='None'; $form.StartPosition='Manual'; $form.Bounds=$bounds
  $form.BackColor=[Drawing.Color]::Black; $form.Opacity=0.28; $form.TopMost=$true
  $form.Cursor=[Windows.Forms.Cursors]::Cross; $form.KeyPreview=$true; $form.ShowInTaskbar=$false
  $script:down=$false; $script:chosen=$false; $script:start=New-Object Drawing.Point; $script:rect=New-Object Drawing.Rectangle
  $form.Add_KeyDown({if($_.KeyCode -eq 'Escape'){$form.Close()}})
  $form.Add_MouseDown({$script:down=$true;$script:start=$_.Location})
  $form.Add_MouseMove({if($script:down){$x=[Math]::Min($script:start.X,$_.X);$y=[Math]::Min($script:start.Y,$_.Y);$w=[Math]::Abs($_.X-$script:start.X);$h=[Math]::Abs($_.Y-$script:start.Y);$script:rect=New-Object Drawing.Rectangle($x,$y,$w,$h);$form.Invalidate()}})
  $form.Add_Paint({if($script:down -and $script:rect.Width -gt 0){$p=New-Object Drawing.Pen([Drawing.Color]::DeepSkyBlue,4);$_.Graphics.DrawRectangle($p,$script:rect);$p.Dispose()}})
  $form.Add_MouseUp({$script:down=$false;if($script:rect.Width -ge 8 -and $script:rect.Height -ge 8){$script:chosen=$true;$form.Close()}})
  [void]$form.ShowDialog(); $form.Dispose()
  if(-not $script:chosen){Write-Output 'cancelled';exit 2}
  Start-Sleep -Milliseconds 120
  $bmp=New-Object Drawing.Bitmap($script:rect.Width,$script:rect.Height)
  $g=[Drawing.Graphics]::FromImage($bmp)
  $g.CopyFromScreen($bounds.X+$script:rect.X,$bounds.Y+$script:rect.Y,0,0,$bmp.Size)
  $g.Dispose();$bmp.Save($ImagePath,[Drawing.Imaging.ImageFormat]::Bmp);$bmp.Dispose()
}
$source=[Drawing.Image]::FromFile($ImagePath)
$preview=New-Object Drawing.Bitmap(400,200)
$pg=[Drawing.Graphics]::FromImage($preview);$pg.Clear([Drawing.Color]::White)
$scale=[Math]::Min(400.0/$source.Width,200.0/$source.Height)
$dw=[int]($source.Width*$scale);$dh=[int]($source.Height*$scale)
$pg.DrawImage($source,[int]((400-$dw)/2),[int]((200-$dh)/2),$dw,$dh)
$pg.Dispose();$preview.Save($PreviewPath,[Drawing.Imaging.ImageFormat]::Bmp);$preview.Dispose()
# Small screen text is enlarged before recognition. Windows OCR accuracy drops
# sharply when glyph height is below roughly 20 pixels.
$ocrScale=[Math]::Min(4.0,[Math]::Max(1.0,1800.0/$source.Width))
$ow=[int]($source.Width*$ocrScale);$oh=[int]($source.Height*$ocrScale)
if($ow -gt 7600){$ratio=7600.0/$ow;$ow=7600;$oh=[int]($oh*$ratio)}
if($oh -gt 7600){$ratio=7600.0/$oh;$oh=7600;$ow=[int]($ow*$ratio)}
$ocrBitmap=New-Object Drawing.Bitmap($ow,$oh)
$og=[Drawing.Graphics]::FromImage($ocrBitmap)
$og.Clear([Drawing.Color]::White)
$og.InterpolationMode=[Drawing.Drawing2D.InterpolationMode]::HighQualityBicubic
$og.SmoothingMode=[Drawing.Drawing2D.SmoothingMode]::HighQuality
$og.PixelOffsetMode=[Drawing.Drawing2D.PixelOffsetMode]::HighQuality
$og.DrawImage($source,0,0,$ow,$oh)
$og.Dispose();$source.Dispose()
$ocrPath=$OutputPath+'.ocr.bmp'
$ocrBitmap.Save($ocrPath,[Drawing.Imaging.ImageFormat]::Bmp);$ocrBitmap.Dispose()
`
