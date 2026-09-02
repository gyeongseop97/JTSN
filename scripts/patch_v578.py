from pathlib import Path
import re


def replace_one(text: str, pattern: str, replacement: str, label: str, flags=0) -> str:
    matches = list(re.finditer(pattern, text, flags))
    if len(matches) != 1:
        raise RuntimeError(f"{label}: expected exactly one match, got {len(matches)}")
    return re.sub(pattern, replacement, text, count=1, flags=flags)

# app/main.go: version + latest-only patch note + all-history prepend
app_path = Path("app/main.go")
app = app_path.read_text(encoding="utf-8")
app = replace_one(app, r'const appVersion = "5\.77"', 'const appVersion = "5.78"', 'app version')
app = replace_one(
    app,
    r'const latestPatchNotes = `v5\.77.*?`\n\nconst allPatchNotes',
    '''const latestPatchNotes = `v5.78\n\n• 파일/폴더 우클릭의 ‘JTSN 새 폴더에 넣기’ 실행 경로를 안정화\n• 버전별 Core EXE 대신 고정 JTSN.exe 런처를 호출하도록 변경\n• 기존 우클릭 메뉴도 업데이트 시 자동으로 현재 설치 경로로 복구`\n\nconst allPatchNotes''',
    'latest patch notes',
    flags=re.S,
)
app = replace_one(
    app,
    r'(const allPatchNotes = `잡툴사니 · JTSN 패치노트\n\n)',
    r'''\1v5.78\n• 파일/폴더 우클릭 ‘새 폴더에 넣기’ 먹통 문제 수정\n• 우클릭 명령이 고정 JTSN.exe 런처를 사용하도록 변경\n• 기존 등록 메뉴를 업데이트 시 자동 복구하도록 보강\n\n''',
    'all patch notes header',
)
app = app.replace('v5.76• 퀵런처', 'v5.76\n• 퀵런처')
app_path.write_text(app, encoding="utf-8")

# app/bundle.go: register context menu against stable launcher, not versioned core exe
bundle_path = Path("app/bundle.go")
bundle = bundle_path.read_text(encoding="utf-8")
new_register = r'''func bundleExplorerCommandTarget() string {
	if launcher, err := installedLauncherPath(); err == nil && strings.TrimSpace(launcher) != "" {
		return launcher
	}
	// Development/portable fallback. Installed builds should always resolve the
	// stable launcher above so future core-version changes cannot break Explorer.
	if exe, err := os.Executable(); err == nil {
		return exe
	}
	return ""
}

func registerBundleExplorerMenu() {
	target := bundleExplorerCommandTarget()
	if target == "" {
		errorBox("JTSN 실행 파일 경로를 확인할 수 없습니다.")
		return
	}
	command := fmt.Sprintf("\\\"%s\\\" --bundle-shell \\\"%%1\\\"", target)
	keys := []string{`HKCU\\Software\\Classes\\*\\shell\\JTSNBundle`, `HKCU\\Software\\Classes\\Directory\\shell\\JTSNBundle`}
	for _, key := range keys {
		commands := [][]string{{"add", key, "/v", "MUIVerb", "/d", "JTSN 새 폴더에 넣기", "/f"}, {"add", key, "/v", "MultiSelectModel", "/d", "Player", "/f"}, {"add", key + `\\command`, "/ve", "/d", command, "/f"}}
		for _, args := range commands {
			cmd := exec.Command("reg.exe", args...)
			cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
			if out, e := cmd.CombinedOutput(); e != nil {
				errorBox("탐색기 메뉴 등록 실패: " + e.Error() + "\\n" + string(out))
				return
			}
		}
	}
	info("탐색기 우클릭 메뉴에 ‘JTSN 새 폴더에 넣기’를 등록했습니다.\\n\\n이제 버전이 업데이트되어도 고정 JTSN 런처를 통해 계속 동작합니다.\\nWindows 11에서는 ‘더 많은 옵션 표시’ 안에 나타날 수 있습니다.")
}
'''
bundle = replace_one(
    bundle,
    r'func registerBundleExplorerMenu\(\) \{.*?\n\}\n\n// runBundleShellImmediate',
    new_register + '\n// runBundleShellImmediate',
    'bundle context menu registration',
    flags=re.S,
)
bundle_path.write_text(bundle, encoding="utf-8")

# installer/main.go: bump version and repair existing registered menu on each install/update
installer_path = Path("installer/main.go")
installer = installer_path.read_text(encoding="utf-8")
installer = replace_one(installer, r'launcherVersion = "5\.77"', 'launcherVersion = "5.78"', 'launcher version')
repair_func = r'''
func repairBundleExplorerMenuIfRegistered() {
	target := installedPath()
	command := fmt.Sprintf("\\\"%s\\\" --bundle-shell \\\"%%1\\\"", target)
	keys := []string{`HKCU\\Software\\Classes\\*\\shell\\JTSNBundle`, `HKCU\\Software\\Classes\\Directory\\shell\\JTSNBundle`}
	for _, key := range keys {
		query := exec.Command("reg.exe", "query", key)
		query.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
		if err := query.Run(); err != nil {
			// Respect users who never enabled the Explorer integration.
			continue
		}
		_ = runHidden("reg.exe", "add", key, "/v", "MUIVerb", "/d", "JTSN 새 폴더에 넣기", "/f")
		_ = runHidden("reg.exe", "add", key, "/v", "MultiSelectModel", "/d", "Player", "/f")
		_ = runHidden("reg.exe", "add", key+`\\command`, "/ve", "/d", command, "/f")
	}
}
'''
installer = replace_one(
    installer,
    r'(func refreshBranding\(\) \{.*?registerUninstall\(installedPath\(\)\)\n)(\t_ = runHidden\("ie4uinit\.exe", "-show"\)\n\})',
    r'\1\trepairBundleExplorerMenuIfRegistered()\n\2' + repair_func,
    'refresh branding repair hook',
    flags=re.S,
)
installer_path.write_text(installer, encoding="utf-8")

print("v5.78 patch applied")
