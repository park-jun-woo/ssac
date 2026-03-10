✅ 완료

# Phase 010: CanTransition 반환 타입 bool → error

## 목표

`@state` 코드젠의 `CanTransition` 호출을 `!bool` 패턴에서 `err != nil` 패턴으로 변경한다.
상태 전이 실패 시 구체적 사유를 `err.Error()`에서 가져온다.

```go
// 변경 전
if !reservationstate.CanTransition(reservationstate.Input{...}, "cancel") {
    c.JSON(http.StatusConflict, gin.H{"error": "취소할 수 없습니다"})

// 변경 후
if err := reservationstate.CanTransition(reservationstate.Input{...}, "cancel"); err != nil {
    c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
```

## 변경 파일

| 파일 | 내용 |
|---|---|
| `generator/go_templates.go` | state 템플릿: `!bool` → `err := ...; err != nil`, `err.Error()` |
| `generator/generator_test.go` | `TestGenerateState` assertion 업데이트 |

## 검증

```bash
go test ./parser/... ./validator/... ./generator/... -count=1
ssac gen specs/dummy-study/ /tmp/ssac-phase10-check/
```

## 의존성

- 수정지시서v2/004
- fullend stategen.go 대응 필요 (SSaC 외부)
