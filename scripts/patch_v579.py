from pathlib import Path
import re


def replace_exact(text: str, old: str, new: str, label: str, count: int = 1) -> str:
    actual = text.count(old)
    if actual != count:
        raise RuntimeError(f"{label}: expected {count} match(es), got {actual}")
    return text.replace(old, new, count)


def replace_regex_once(text: str, pattern: str, replacement: str, label: str) -> str:
    matches = list(re.finditer(pattern, text, re.S))
    if len(matches) != 1:
        raise RuntimeError(f"{label}: expected exactly one match, got {len(matches)}")
    return re.sub(pattern, lambda _m: replacement, text, count=1, flags=re.S)

# app/main.go: bump version and keep automatic popup latest-only.
app_path = Path("app/main.go")
app = app_path.read_text(encoding="utf-8")
app = replace_exact(app, 'const appVersion = "5.78"', 'const appVersion = "5.79"', 'app version')
old_latest = '''const latestPatchNotes = `v5.78

• 파일/폴더 우클릭의 ‘JTSN 새 폴더에 넣기’ 실행 경로를 안정화
• 버전별 Core EXE 대신 고정 JTSN.exe 런처를 호출하도록 변경
• 기존 우클릭 메뉴도 업데이트 시 자동으로 현재 설치 경로로 복구`'''
new_latest = '''const latestPatchNotes = `v5.79

• 파일/폴더 우클릭 ‘새 폴더에 넣기’가 C:\\Windows에 폴더를 만들려던 문제 수정
• 업데이트 시 탐색기 우클릭 명령을 현재 JTSN.exe 경로로 강제 재등록
• 파일 전체 경로를 받지 못하면 이동하지 않도록 안전장치 추가`'''
app = replace_exact(app, old_latest, new_latest, 'latest patch notes')
header = 'const allPatchNotes = `잡툴사니 · JTSN 패치노트\n\n'
insert = '''const allPatchNotes = `잡툴사니 · JTSN 패치노트

v5.79
• 파일/폴더 우클릭 ‘새 폴더에 넣기’가 C:\\Windows에 새 폴더를 만들려던 문제 수정
• 업데이트 시 기존 우클릭 명령을 삭제하고 현재 JTSN.exe 경로로 다시 등록
• 선택 파일의 전체 경로가 아닌 값은 이동 대상에서 제외하도록 안전장치 추가

'''
app = replace_exact(app, header, insert, 'all patch notes header')
app_path.write_text(app, encoding="utf-8")

# app/bundle.go: never resolve Explorer shell arguments against the process cwd.
bundle_path = Path("app/bundle.go")
bundle = bundle_path.read_text(encoding="utf-8")
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
\t\t// Explorer must provide the full source path. A relative argument would
\t\t// otherwise be resolved against JTSN's cwd (which can be C:\\Windows).
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
    'errorBox("선택한 파일/폴더의 전체 경로를 받지 못했습니다.\\n\\nJTSN이 우클릭 메뉴를 자동 복구한 뒤 다시 시도해 주세요.")',
    'bundle invalid path message',
)
bundle_path.write_text(bundle, encoding="utf-8")

# installer/main.go: on update, replace the existing shell verb completely.
# This removes any stale command that still points to an old core executable.
installer_path = Path("installer/main.go")
installer = installer_path.read_text(encoding="utf-8")
installer = replace_exact(installer, 'launcherVersion = "5.78"', 'launcherVersion = "5.79"', 'launcher version')
new_repair = r'''func repairBundleExplorerMenuIfRegistered() {
	target := installedPath()
	command := fmt.Sprintf("\"%s\" --bundle-shell \"%%1\"", target)
	keys := []string{`HKCU\Software\Classes\*\shell\JTSNBundle`, `HKCU\Software\Classes\Directory\shell\JTSNBundle`}
	for _, key := range keys {
		// Recreate the verb instead of editing it in-place. This guarantees that
		// an old version-specific core command cannot survive an update.
		_ = runHidden("reg.exe", "delete", key, "/f")
		_ = runHidden("reg.exe", "add", key, "/v", "MUIVerb", "/d", "JTSN 새 폴더에 넣기", "/f")
		_ = runHidden("reg.exe", "add", key, "/v", "MultiSelectModel", "/d", "Player", "/f")
		_ = runHidden("reg.exe", "add", key, "/v", "Icon", "/d", target+",0", "/f")
		_ = runHidden("reg.exe", "add", key+`\command`, "/ve", "/d", command, "/f")
	}
}'''
installer = replace_regex_once(
    installer,
    r'func repairBundleExplorerMenuIfRegistered\(\) \{.*?\n\}\n\nfunc removeDesktopShortcut',
    new_repair + '\n\nfunc removeDesktopShortcut',
    'force Explorer verb recreation',
)
installer_path.write_text(installer, encoding="utf-8")

print("v5.79 bundle shell path repair applied")
