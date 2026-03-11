# ✅ 완료 Phase 021: @response 단일 행 구조체 형식 파싱 지원

## 목표

`@response { field: var }` 한 줄 형식을 정상 파싱. 현재는 `{ field: var }` 전체를 변수명으로 인식하여 에러 발생.

## 변경 파일 목록

| 파일 | 변경 |
|---|---|
| `parser/parser.go` | `parseLine()`에서 `{`로 시작 + `}`로 끝나는 경우 단일 행 구조체로 파싱 |
| `parser/parser_test.go` | 단일 행 @response 테스트 추가 |

## 상세 설계

`parseLine()`의 @response 분기에서 `trimmed == "{"` 체크와 shorthand 분기 사이에 단일 행 분기 추가:

```go
trimmed := strings.TrimSpace(strings.TrimPrefix(line, tag))
if trimmed == "{" {
    return nil, true, nil // multiline 시작
}
// 단일 행 구조체: @response { field: var, ... }
if strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}") {
    inner := trimmed[1 : len(trimmed)-1]
    return &Sequence{
        Type:         SeqResponse,
        Fields:       parseResponseFields(strings.Split(inner, ",")),
        SuppressWarn: suppressWarn,
    }, false, nil
}
// shorthand: @response varName
if trimmed != "" {
    return &Sequence{...Target: trimmed...}, false, nil
}
```

`parseResponseFields()`는 이미 `[]string` → `map[string]string` 변환을 수행하므로 그대로 재사용.

단, 현재 `parseResponseFields`는 `[]string`을 인자로 받는데, 단일 행에서는 쉼표로 split한 결과를 넘기면 된다.

## 테스트 계획

| 테스트 | 검증 내용 |
|---|---|
| `TestParseResponseSingleLine` | `@response { user: user }` → Fields 정상 파싱 |
| `TestParseResponseSingleLineMultiFields` | `@response { user: user, name: user.Name }` → 2개 필드 |

## 검증 방법

```bash
go test ./parser/... -count=1
```
