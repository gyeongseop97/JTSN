# JTSN · 잡툴사니

Windows 업무용 도구 모음입니다.

## 설치 및 업데이트

1. [Releases](https://github.com/gyeongseop97/JTSN/releases/latest)에서 최신 `JTSN_Setup_v*.exe`를 받습니다.
2. v5.51부터 실행 중 새 GitHub Release를 자동으로 확인합니다.
3. 업데이트에 동의하면 EXE와 SHA-256 체크섬을 내려받아 검증하고 자동 교체·재실행합니다.

모든 업데이트 파일은 이 공개 저장소의 GitHub Releases에서만 내려받습니다.

## 릴리스 방법

`installer/main.go`의 `launcherVersion`을 갱신하고 같은 버전의 태그를 푸시합니다.

```text
v5.52
```

정상 동작을 확인한 Windows 실행 파일과 SHA-256 파일을 Release에 게시합니다.
