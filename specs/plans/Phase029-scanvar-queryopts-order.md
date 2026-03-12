✅ 완료

# Phase029: scanXxx 변수명 충돌 + ListByXxx 인자 순서 수정

## 목표

두 가지 codegen 버그를 수정한다:

1. **BUG001**: fullend의 `scanXxx` 함수에서 scanner 파라미터 `s`가 모델 변수와 충돌 (S로 시작하는 모델). → ssac 범위 외 (fullend `internal/gluegen/model_impl.go`), 여기서는 **스킵**.
2. **BUG002**: ssac의 `buildArgsCodeFromInputs`가 key를 알파벳순 정렬하여 `opts`(QueryOpts)가 비즈니스 인자보다 앞에 배치됨. → `opts`를 항상 마지막으로 이동.

## 변경 파일

| 파일 | 변경 내용 |
|---|---|
| `generator/go_target.go` | `buildArgsCodeFromInputs`에서 `query` 값을 가진 인자를 마지막으로 배치 |
| `generator/generator_test.go` | 기존 `TestGenerateWithQueryOpts` assertion 수정 (올바른 인자 순서) |

## 상세 변경

### 1. `generator/go_target.go` — `buildArgsCodeFromInputs`

현재: key를 알파벳순 정렬 → `Opts` < `UserID` → `opts, currentUser.ID`

변경: 정렬 후 `query` 값을 가진 키를 뒤로 이동.

```go
func buildArgsCodeFromInputs(inputs map[string]string) string {
    if len(inputs) == 0 {
        return ""
    }
    keys := make([]string, 0, len(inputs))
    var queryKey string
    for k := range inputs {
        if inputs[k] == "query" {
            queryKey = k
        } else {
            keys = append(keys, k)
        }
    }
    sort.Strings(keys)
    if queryKey != "" {
        keys = append(keys, queryKey)
    }

    var parts []string
    for _, k := range keys {
        parts = append(parts, inputValueToCode(inputs[k]))
    }
    return strings.Join(parts, ", ")
}
```

### 2. `generator/generator_test.go` — assertion 수정

```go
// before
assertContains(t, code, `reservationModel.ListByUserID(opts, currentUser.ID)`)

// after
assertContains(t, code, `reservationModel.ListByUserID(currentUser.ID, opts)`)
```

## BUG001 (scanXxx) — ssac 범위 외

`scanXxx` 함수는 fullend 프로젝트 `internal/gluegen/model_impl.go:297`에서 생성된다.
scanner 파라미터 `s` → `row`로 변경하는 것은 fullend 프로젝트에서 처리해야 한다.

## 의존성

없음.

## 검증

```bash
go test ./generator/... -count=1
```
