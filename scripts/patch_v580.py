from pathlib import Path
import re


def replace_one(text: str, pattern: str, replacement: str, label: str, flags=0) -> str:
    matches = list(re.finditer(pattern, text, flags))
    if len(matches) != 1:
        raise RuntimeError(f"{label}: expected exactly one match, got {len(matches)}")
    m = matches[0]
    return text[:m.start()] + replacement + text[m.end():]

# ---- app/main.go: version + patch notes ----
main_path = Path("app/main.go")
main = main_path.read_text(encoding="utf-8")
main = main.replace('const appVersion = "5.79"', 'const appVersion = "5.80"', 1)
main = replace_one(
    main,
    r'const latestPatchNotes = `v5\.79.*?`\n\nconst allPatchNotes',
    '''const latestPatchNotes = `v5.80\n\n• 파일 1개만 우클릭해 ‘새 폴더에 넣기’ 할 때 발생하던 오류 수정\n• 탐색기 우클릭 호출 수집 로직을 단일/다중 선택 모두 안정적으로 처리하도록 개선\n• 이전 우클릭 호출이 다음 실행에 섞이지 않도록 큐 수집 범위를 현재 실행 묶음으로 제한`\n\nconst allPatchNotes''',
    'latest patch notes',
    flags=re.S,
)
main = main.replace(
    'const allPatchNotes = `잡툴사니 · JTSN 패치노트\n\n',
    'const allPatchNotes = `잡툴사니 · JTSN 패치노트\n\nv5.80\n• 파일 1개 우클릭 새 폴더 이동 오류 수정\n• 단일/다중 선택 우클릭 호출 수집 로직 안정화\n• 현재 실행 이전의 큐 항목이 섞이지 않도록 배치 범위 제한\n\n',
    1,
)
main_path.write_text(main, encoding="utf-8")

# ---- app/bundle.go: robust shell batching ----
bundle_path = Path("app/bundle.go")
bundle = bundle_path.read_text(encoding="utf-8")

new_block = r'''func normalizeBundleShellPaths(paths []string) []string {
	out := make([]string, 0, len(paths))
	seen := map[string]bool{}
	for _, path := range paths {
		raw := strings.TrimSpace(strings.Trim(path, `"`))
		if raw == "" || !filepath.IsAbs(raw) {
			continue
		}
		clean := filepath.Clean(raw)
		if _, err := os.Stat(clean); err != nil {
			continue
		}
		key := strings.ToLower(clean)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, clean)
	}
	return out
}

// runBundleShellImmediate is intentionally independent of the main window.
// Explorer context-menu launches therefore finish without creating another
// JTSN window or taskbar item.
func runBundleShellImmediate(paths []string) {
	direct := normalizeBundleShellPaths(paths)
	if len(direct) == 0 {
		errorBox("선택한 파일/폴더의 전체 경로를 받지 못했습니다.\n\n탐색기에서 파일을 다시 선택한 뒤 시도해 주세요.")
		return
	}

	collected, leader := collectBundleShellInvocations(direct)
	if !leader {
		// A leader process is already collecting this Explorer multi-selection.
		return
	}
	paths = normalizeBundleShellPaths(collected)
	if len(paths) == 0 {
		// A single-item invocation must never fail merely because the queue could
		// not be read back. The leader still has the original absolute path.
		paths = direct
	}

	entries := make([]bundleEntry, 0, len(paths))
	for _, path := range paths {
		st, err := os.Stat(path)
		if err != nil {
			continue
		}
		entries = append(entries, bundleEntry{Path: path, IsDir: st.IsDir()})
	}
	if len(entries) == 0 {
		errorBox("이동할 파일이나 폴더를 찾을 수 없습니다.")
		return
	}

	base := filepath.Dir(entries[0].Path)
	if base == "" || !filepath.IsAbs(base) {
		errorBox("새 폴더를 만들 원본 위치를 확인하지 못했습니다.")
		return
	}
	dest := uniqueBundlePath(filepath.Join(base, "새 폴더"))
	if err := os.MkdirAll(dest, 0755); err != nil {
		errorBox("새 폴더를 만들지 못했습니다.\n\n" + err.Error())
		return
	}

	failures := make([]string, 0)
	moved := 0
	for _, entry := range entries {
		target := filepath.Join(dest, filepath.Base(entry.Path))
		if _, err := os.Stat(target); err == nil {
			target = uniqueBundleItemPath(target, entry.IsDir)
		}
		if err := moveBundlePath(entry.Path, target); err != nil {
			failures = append(failures, filepath.Base(entry.Path)+": "+err.Error())
			continue
		}
		moved++
	}
	if moved == 0 {
		// Do not leave an empty "새 폴더" behind when the source could not move.
		_ = os.Remove(dest)
	}
	if len(failures) > 0 {
		message := strings.Join(failures, "\n")
		if len([]rune(message)) > 1200 {
			message = string([]rune(message)[:1200]) + "\n…"
		}
		errorBox(fmt.Sprintf("%d개 항목을 이동하지 못했습니다.\n\n%s", len(failures), message))
	}
}

// Explorer may start a classic context-menu command once per selected item.
// The first process becomes the leader; the other short-lived processes only
// enqueue their absolute path. The leader collects the current burst once.
func collectBundleShellInvocations(paths []string) ([]string, bool) {
	paths = normalizeBundleShellPaths(paths)
	if len(paths) == 0 {
		return nil, true
	}

	cache, err := os.UserCacheDir()
	if err != nil || cache == "" {
		cache = os.TempDir()
	}
	queueDir := filepath.Join(cache, "JTSN", "bundle-shell-queue")
	_ = os.MkdirAll(queueDir, 0755)

	batchStarted := time.Now()
	mutex, _, mutexErr := procCreateMutexW.Call(0, 1, uintptr(unsafe.Pointer(p16("Local\\JTSN_Bundle_Shell_Batch_v2"))))
	if mutex != 0 {
		defer procCloseHandleMain.Call(mutex)
	}
	leader := mutexErr != syscall.Errno(183)

	// The leader cleans only files that predate this burst. Followers are
	// created after the mutex exists, so their fresh requests are preserved.
	if leader {
		files, _ := filepath.Glob(filepath.Join(queueDir, "*.json"))
		for _, file := range files {
			info, statErr := os.Stat(file)
			if statErr != nil || info.ModTime().Before(batchStarted.Add(-500*time.Millisecond)) {
				_ = os.Remove(file)
			}
		}
	}

	payload, _ := json.Marshal(paths)
	requestPath := filepath.Join(queueDir, fmt.Sprintf("%d-%d.json", os.Getpid(), time.Now().UnixNano()))
	if err := os.WriteFile(requestPath, payload, 0600); err != nil {
		if leader {
			return paths, true
		}
		return nil, false
	}
	if !leader {
		return nil, false
	}

	// Wait until Explorer's burst of per-item launches has gone quiet. A single
	// file therefore costs only a short debounce, while multi-select remains one
	// operation and one destination folder.
	lastCount := -1
	stableRounds := 0
	deadline := time.Now().Add(1800 * time.Millisecond)
	for time.Now().Before(deadline) && stableRounds < 3 {
		time.Sleep(140 * time.Millisecond)
		files, _ := filepath.Glob(filepath.Join(queueDir, "*.json"))
		current := 0
		for _, file := range files {
			info, statErr := os.Stat(file)
			if statErr == nil && !info.ModTime().Before(batchStarted.Add(-500*time.Millisecond)) {
				current++
			}
		}
		if current == lastCount {
			stableRounds++
		} else {
			lastCount = current
			stableRounds = 0
		}
	}

	all := make([]string, 0)
	files, _ := filepath.Glob(filepath.Join(queueDir, "*.json"))
	for _, file := range files {
		info, statErr := os.Stat(file)
		if statErr != nil {
			continue
		}
		if info.ModTime().Before(batchStarted.Add(-500 * time.Millisecond)) {
			_ = os.Remove(file)
			continue
		}
		data, readErr := os.ReadFile(file)
		if readErr == nil {
			var queued []string
			if json.Unmarshal(data, &queued) == nil {
				all = append(all, queued...)
			}
		}
		_ = os.Remove(file)
	}
	return normalizeBundleShellPaths(all), true
}
'''

bundle = replace_one(
    bundle,
    r'// runBundleShellImmediate is intentionally independent of the main window\..*?func collectBundleShellInvocations\(paths \[\]string\) \[\]string \{.*?\n\}\n\nfunc runBundleMove\(\)',
    new_block + '\nfunc runBundleMove()',
    'bundle shell block',
    flags=re.S,
)
bundle_path.write_text(bundle, encoding="utf-8")

# ---- installer/main.go: bump launcher version ----
installer_path = Path("installer/main.go")
installer = installer_path.read_text(encoding="utf-8")
installer = installer.replace('launcherVersion = "5.79"', 'launcherVersion = "5.80"', 1)
installer_path.write_text(installer, encoding="utf-8")

print("v5.80 single-item bundle shell fix applied")
