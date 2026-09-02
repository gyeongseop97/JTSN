from pathlib import Path


def replace_exact(text: str, old: str, new: str, label: str, count: int = 1) -> str:
    actual = text.count(old)
    if actual != count:
        raise RuntimeError(f"{label}: expected {count} match(es), got {actual}")
    return text.replace(old, new, count)

# app/main.go: bump version and keep automatic popup latest-only.
app_path = Path("app/main.go")
app = app_path.read_text(encoding="utf-8")
app = replace_exact(app, 'const appVersion = "5.78"', 'const appVersion = "5.79"', 'app version')
old_latest = '''const latestPatchNotes = `v5.78

• 파일/폴더 우클릭의 ‘JTSN 새 폴더에 넣기’ 실행 경로를 안정화
• 버전별 Core EXE 대신 고정 JTSN.exe 런처를 호출하도록 변경
• 기존 우클릭 메뉴도 업데이트 시 자동으로 현재 설치 경로로 복구`'''
new_latest = '''const latestPatchNotes = `v5.79

• 파일/폴더 우클릭 ‘새 폴더에 넣기’가 C:\\Windows를 대상으로 잡는 문제 수정
• 탐색기 우클릭 레지스트리 경로를 정상 형식으로 교정하고 기존 메뉴를 자동 복구
• 선택 파일 경로가 절대경로가 아닐 경우 이동을 중단하도록 안전장치 추가`'''
app = replace_exact(app, old_latest, new_latest, 'latest patch notes')
header = 'const allPatchNotes = `잡툴사니 · JTSN 패치노트\n\n'
insert = '''const allPatchNotes = `잡툴사니 · JTSN 패치노트

v5.79
• 파일/폴더 우클릭 ‘새 폴더에 넣기’가 C:\\Windows에 새 폴더를 만들려던 문제 수정
• 우클릭 레지스트리 키 경로의 잘못된 백슬래시 표기를 수정
• 기존 등록 메뉴를 현재 JTSN.exe 런처 경로로 자동 복구
• 상대경로 인자가 들어오면 이동하지 않도록 안전장치 추가

'''
app = replace_exact(app, header, insert, 'all patch notes header')
app_path.write_text(app, encoding="utf-8")

# app/bundle.go: use valid Registry path separators and reject relative shell paths.
bundle_path = Path("app/bundle.go")
bundle = bundle_path.read_text(encoding="utf-8")
old_keys = r'''keys := []string{`HKCU\\Software\\Classes\\*\\shell\\JTSNBundle`, `HKCU\\Software\\Classes\\Directory\\shell\\JTSNBundle`}'''
new_keys = r'''keys := []string{`HKCU\Software\Classes\*\shell\JTSNBundle`, `HKCU\Software\Classes\Directory\shell\JTSNBundle`}'''
bundle = replace_exact(bundle, old_keys, new_keys, 'bundle registry keys')
bundle = replace_exact(bundle, r'''key + `\\command`''', r'''key + `\command`''', 'bundle command subkey')
old_loop = '''\tfor _, path := range paths {
\t\tpath = filepath.Clean(strings.TrimSpace(path))
\t\tst, err := os.Stat(path)
\t\tkey := strings.ToLower(path)
\t\tif err != nil || seen[key] {
\t\t\tcontinue
\t\t}
\t\tseen[key] = true
\t\tentries = append(entries, bundleEntry{Path: path, IsDir: st.IsDir()})
\t}'''
new_loop = '''\tfor _, path := range paths {
\t\traw := strings.TrimSpace(strings.Trim(path, `"`))
\t\t// Explorer must provide a full source path. Never resolve a relative
\t\t// argument against JTSN's process working directory (often C:\\Windows).
\t\tif raw == "" || !filepath.IsAbs(raw) {
\t\t\tcontinue
\t\t}
\t\tpath = filepath.Clean(raw)
\t\tst, err := os.Stat(path)
\t\tkey := strings.ToLower(path)
\t\tif err != nil || seen[key] {
\t\t\tcontinue
\t\t}
\t\tseen[key] = true
\t\tentries = append(entries, bundleEntry{Path: path, IsDir: st.IsDir()})
\t}'''
bundle = replace_exact(bundle, old_loop, new_loop, 'bundle shell absolute path guard')
bundle = replace_exact(
    bundle,
    'errorBox("이동할 파일이나 폴더를 찾을 수 없습니다.")',
    'errorBox("이동할 파일이나 폴더의 전체 경로를 받지 못했습니다.\\n\\n탐색기 우클릭 메뉴를 다시 등록한 뒤 시도해 주세요.")',
    'bundle invalid path message',
)
bundle_path.write_text(bundle, encoding="utf-8")

# installer/main.go: repair the actually registered Explorer keys after update.
installer_path = Path("installer/main.go")
installer = installer_path.read_text(encoding="utf-8")
installer = replace_exact(installer, 'launcherVersion = "5.78"', 'launcherVersion = "5.79"', 'launcher version')
installer = replace_exact(installer, old_keys, new_keys, 'installer registry keys')
installer = replace_exact(installer, r'''key+`\\command`''', r'''key+`\command`''', 'installer command subkey')
installer_path.write_text(installer, encoding="utf-8")

print("v5.79 bundle shell path repair applied")
