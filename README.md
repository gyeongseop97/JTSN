# JTSN · 잡툴사니

Windows 업무용 도구 모음입니다.

## 설치 및 업데이트

1. [Releases](https://github.com/gyeongseop97/JTSN/releases/latest)에서 최신 `JTSN_v*.exe`를 받습니다.
2. v5.51부터 실행 중 새 GitHub Release를 자동으로 확인합니다.
3. 업데이트에 동의하면 EXE와 SHA-256 체크섬을 내려받아 검증하고 자동 교체·재실행합니다.

모든 업데이트 파일은 이 공개 저장소의 GitHub Releases에서만 내려받습니다.

## 릴리스 방법

소스의 `appVersion`, `latestPatchNotes`, `RELEASE_NOTES.md`를 갱신한 뒤 같은 버전의 태그를 푸시합니다.

```text
v5.52
```

GitHub Actions가 Windows 실행 파일과 SHA-256 파일을 빌드하여 Release에 자동 게시합니다.
