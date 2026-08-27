from pathlib import Path
import re

APP = Path("app/main.go")
INSTALLER = Path("installer/main.go")

s = APP.read_text(encoding="utf-8")
s = re.sub(r'const appVersion = "[0-9.]+"', 'const appVersion = "5.67"', s, count=1)

if "ID_SETTINGS_UPDATE" not in s:
    marker = "\tID_SETTINGS_HOTKEY_EDIT = 646\n"
    if marker not in s:
        raise SystemExit("settings id marker not found")
    s = s.replace(marker, marker + "\tID_SETTINGS_UPDATE    = 647\n", 1)

if "ID_SETTINGS_UPDATE, BTN_SECONDARY" not in s:
    old = '\t\t\tcreateOwnerButton(hwnd, "고급 클립보드 설정", 154, 390, 374, 52, ID_SETTINGS_CLIP, BTN_SECONDARY),\n'
    new = (
        '\t\t\tcreateOwnerButton(hwnd, "고급 클립보드 설정", 154, 390, 238, 52, ID_SETTINGS_CLIP, BTN_SECONDARY),\n'
        '\t\t\tcreateOwnerButton(hwnd, "업데이트 확인", 402, 390, 126, 52, ID_SETTINGS_UPDATE, BTN_SECONDARY),\n'
    )
    if old not in s:
        raise SystemExit("settings button marker not found")
    s = s.replace(old, "".join(new), 1)

if "case ID_SETTINGS_UPDATE:" not in s:
    old = "\t\tcase ID_SETTINGS_CLIP:\n\t\t\tlaunchTool(ID_NAV_CLIP)\n\t\tcase ID_SETTINGS_APPLY:"
    new = (
        "\t\tcase ID_SETTINGS_CLIP:\n"
        "\t\t\tlaunchTool(ID_NAV_CLIP)\n"
        "\t\tcase ID_SETTINGS_UPDATE:\n"
        "\t\t\tcheckForUpdateManually()\n"
        "\t\tcase ID_SETTINGS_APPLY:"
    )
    if old not in s:
        raise SystemExit("settings command marker not found")
    s = s.replace(old, new, 1)

if "func checkForUpdateManually()" not in s:
    marker = "func checkForUpdateInBackground() {"
    if marker not in s:
        raise SystemExit("background update function marker not found")
    func_text = '''func checkForUpdateManually() {
\tbase := os.Getenv("LOCALAPPDATA")
\tif base == "" {
\t\terrorBox("설치 경로를 확인하지 못해 업데이트를 조회할 수 없습니다.")
\t\treturn
\t}
\tlauncher := filepath.Join(base, "Programs", "JTSN", "JTSN.exe")
\tif st, err := os.Stat(launcher); err != nil || st.IsDir() {
\t\terrorBox("설치된 JTSN 업데이트 프로그램을 찾지 못했습니다.\\n\\nJTSN 설치파일로 다시 설치한 뒤 이용해 주세요.")
\t\treturn
\t}
\tcmd := exec.Command(launcher, "--manual-update-check", strconv.Itoa(os.Getpid()))
\tcmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
\tif err := cmd.Start(); err != nil {
\t\terrorBox("업데이트 확인을 시작하지 못했습니다.\\n\\n" + err.Error())
\t}
}

'''
    s = s.replace(marker, func_text + marker, 1)

if "const latestPatchNotes = `v5.67" not in s:
    marker = "const latestPatchNotes = `v5.63"
    if marker not in s:
        raise SystemExit("patch notes marker not found")
    prefix = '''const latestPatchNotes = `v5.67

• 설정 화면에 '업데이트 확인' 버튼 추가
• 현재 버전과 GitHub 최신 릴리스를 사용자가 직접 비교할 수 있도록 개선
• 새 버전이 있으면 즉시 업데이트로 연결하고, 최신 상태·조회 오류도 명확히 안내
• 자동 업데이트 기능은 기존 방식 그대로 유지
• 누락됐던 v5.64~v5.66 프로그램 내부 패치노트 보완

v5.66

• 설치창 UI 전면 개선: 여백·타이포·실제 JT/SN 아이콘·상태 영역·버튼 디자인 정리
• Windows Common Controls v6 및 DPI 대응 매니페스트 적용
• 설치 패키지 내부 본체 파일명을 버전별로 하드코딩하던 구조 제거
• 설치파일에 포함된 JTSN 본체를 자동 탐색하여 실행하도록 변경
• 빌드 시 내장 본체가 정확히 1개인지와 SHA-256 일치 여부 자동 검증
• 실행 우선/백그라운드 업데이트 정책 유지

v5.65

• 실행 우선 정책 적용: 프로그램을 먼저 연 뒤 업데이트 확인
• GitHub 연결 실패·업데이트 서버 지연과 무관하게 잡툴사니 실행 보장
• 실행 후 약 15초 뒤 백그라운드에서 최신 버전 확인
• 새 버전 발견 시 사용자에게 업데이트 여부 안내 후 패치 진행
• v5.64 투명 JT/SN 로고 유지

v5.64

• 기존 JT/SN 로고 디자인은 그대로 유지하고 흰 배경만 투명 처리
• 프로그램 창·작업표시줄·실행파일 아이콘을 투명 로고로 교체
• 설치 프로그램 아이콘 및 브랜드 이미지도 동일 로고로 통일
• 아이콘 안전 여백을 적용하여 작은 크기에서도 잘림 방지

v5.63'''
    s = s.replace(marker, prefix, 1)

APP.write_text(s, encoding="utf-8")

t = INSTALLER.read_text(encoding="utf-8")
t = re.sub(r'launcherVersion = "[0-9.]+"', 'launcherVersion = "5.67"', t, count=1)

if '--manual-update-check' not in t:
    marker = '''\tif len(os.Args) >= 3 && os.Args[1] == "--background-update-check" {
\t\tpid, _ := strconv.Atoi(os.Args[2])
\t\tbackgroundUpdateCheck(pid)
\t\treturn
\t}
'''
    addition = marker + '''\tif len(os.Args) >= 3 && os.Args[1] == "--manual-update-check" {
\t\tpid, _ := strconv.Atoi(os.Args[2])
\t\tmanualUpdateCheck(pid)
\t\treturn
\t}
'''
    if marker not in t:
        raise SystemExit("installer argument marker not found")
    t = t.replace(marker, addition, 1)

if "func manualUpdateCheck(corePID int)" not in t:
    marker = "func backgroundUpdateCheck(corePID int) {"
    if marker not in t:
        raise SystemExit("installer update function marker not found")
    func_text = '''func manualUpdateCheck(corePID int) {
\trel, err := latest()
\tif err != nil {
\t\tmessage("최신 버전을 확인하지 못했습니다.\\n\\n인터넷 연결 또는 GitHub 접속 상태를 확인해 주세요.\\n\\n"+err.Error(), 0x10)
\t\treturn
\t}
\tif !newer(rel.Tag, launcherVersion) {
\t\tmessage(fmt.Sprintf("현재 v%s은 최신 버전입니다.\\n\\n설치된 JTSN을 그대로 사용하시면 됩니다.", launcherVersion), 0x40)
\t\treturn
\t}
\tbody := strings.TrimSpace(rel.Body)
\tif len([]rune(body)) > 480 {
\t\tbody = string([]rune(body)[:480]) + "…"
\t}
\tcontent := fmt.Sprintf("현재 버전 v%s  →  최신 버전 %s\\n\\n새 버전이 있습니다. 지금 업데이트하시겠습니까?", launcherVersion, rel.Tag)
\tif body != "" {
\t\tcontent += "\\n\\n" + body
\t}
\tif !askUpdate("새로운 JTSN을 사용할 수 있습니다", content) {
\t\treturn
\t}
\tupdateCorePID = corePID
\tif err := runUpdateProgress(rel); err != nil {
\t\tmessage("업데이트에 실패했습니다. 실행 중인 버전은 그대로 유지됩니다.\\n\\n"+err.Error(), 0x10)
\t}
}

'''
    t = t.replace(marker, func_text + marker, 1)

INSTALLER.write_text(t, encoding="utf-8")

print("v5.67 source patch applied")
