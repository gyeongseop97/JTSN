from pathlib import Path

p = Path("app/main.go")
s = p.read_text(encoding="utf-8")

marker = "const allPatchNotes = `잡툴사니 · JTSN 패치노트\n\nv5.63"
if "const allPatchNotes = `잡툴사니 · JTSN 패치노트\n\nv5.67" not in s:
    if marker not in s:
        raise SystemExit("allPatchNotes marker not found")
    replacement = '''const allPatchNotes = `잡툴사니 · JTSN 패치노트

v5.67
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
    s = s.replace(marker, replacement, 1)

p.write_text(s, encoding="utf-8")
print("allPatchNotes updated through v5.67")
