# JTSN · 잡툴사니

Windows 업무용 도구 모음입니다.

## 설치 및 업데이트

1. [Releases](https://github.com/gyeongseop97/JTSN/releases/latest)에서 최신 `JTSN_Setup_v*.exe`를 받습니다.
2. 업데이트는 새로운 버전이 저장소에 업로드되면 사용자가 프로그램 재실행 시 최신 버전으로 업데이트 진행합니다.

모든 업데이트 파일은 이 공개 저장소의 GitHub Releases에서만 내려받습니다.

## 소스 구성

- `installer/`: 현재 v5.60 설치 및 업데이트 프로그램
- `app/`: 이전 ChatGPT 작업에서 회수한 메인 프로그램의 마지막 빌드 가능 원본

`app/`의 회수 기준은 v5.1입니다. 현재 배포 중인 v5.60 실행 파일은
`installer/core/`에 그대로 보존되어 있으며, v5.1 이후 기능은 순차적으로
소스에 복원해야 합니다. 회수 소스를 v5.60으로 잘못 표시하지 않습니다.
