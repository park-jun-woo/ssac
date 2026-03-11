# ✅ 완료 Phase 019: @publish 시퀀스 + @subscribe 트리거 신설

## 목표

1. `@publish` — 11번째 시퀀스 타입. 함수 내부에서 이벤트를 비동기 발행.
2. `@subscribe` — 함수 레벨 트리거. 큐 이벤트 수신 시 함수를 실행.
3. `message` — 구독 함수의 입력 변수 (`request`의 큐 버전).

## 변경 파일 목록

### 1. 파서

| 파일 | 변경 |
|---|---|
| `parser/types.go` | `ServiceFunc.Subscribe` 필드 추가, `Sequence.Topic`/`Options` 필드 추가, `SeqPublish` 상수 추가 |
| `parser/parser.go` | `parsePublish()` 함수 추가, `parseLine()`에 `@publish` 분기 추가, `parseComments()`에서 `@subscribe` 감지 → `ServiceFunc` 반환 시 설정 |

### 2. 검증

| 파일 | 변경 |
|---|---|
| `validator/validator.go` | `validateRequiredFields()`에 `@publish` 분기, `validateFunc()`에 subscribe 관련 검증 추가 |

### 3. 코드젠

| 파일 | 변경 |
|---|---|
| `generator/go_target.go` | `templateData`에 `Topic`/`Options`/`OptionCode` 필드 추가, `buildTemplateData()`에 `@publish` 데이터 설정, `collectImports()`에 `queue` 임포트 추가 |
| `generator/go_templates.go` | `@publish` 템플릿 추가 (기본 + options) |

### 4. 테스트

| 파일 | 변경 |
|---|---|
| `parser/parser_test.go` | @publish 파싱, @subscribe 파싱 테스트 |
| `validator/validator_test.go` | subscribe+response ERROR, subscribe+request ERROR, HTTP+message ERROR, publish 필수 검증 |
| `generator/generator_test.go` | @publish 코드젠 테스트 |

### 5. 문서

| 파일 | 변경 |
|---|---|
| `artifacts/manual-for-ai.md` | DSL 문법에 `@publish`/`@subscribe` 추가, `message` 예약 소스 추가, 시퀀스 11개로 변경, 필수 요소 테이블에 publish 추가, 코드젠 섹션에 publish/subscribe 추가, 테스트 수 갱신 |
| `artifacts/manual-for-human.md` | `@publish`/`@subscribe` 문법/예시/검증 규칙/코드젠 섹션 추가, `message` 예약 소스 추가, 테스트 수 갱신 |
| `README.md` | 시퀀스 타입 목록에 `@publish` 추가, 기능 설명에 큐 이벤트 발행/구독 추가 |
| `CLAUDE.md` | DSL 문법 섹션에 `@publish`/`@subscribe` 추가, 예약 소스에 `message` 추가, 테스트 수 갱신 |

## 상세 설계

### 1-1. types.go 변경

```go
type ServiceFunc struct {
    Name      string
    FileName  string
    Domain    string
    Sequences []Sequence
    Imports   []string
    Subscribe *SubscribeInfo // nil이면 HTTP 트리거
}

type SubscribeInfo struct {
    Topic string // "order.completed"
}
```

Sequence에 추가:
```go
// publish: 이벤트 발행
Topic   string            // "order.completed"
Options map[string]string // {delay: "1800"} (선택)
// Inputs 재사용: payload
```

상수:
```go
SeqPublish = "publish"
```

`ValidSequenceTypes`에 `SeqPublish: true` 추가.

### 1-2. parser.go 변경

**@subscribe 파싱**:

`parseLine()`에서 `@subscribe` 감지 시 특수 Sequence 반환 (Type="subscribe", Topic 설정). `ParseFile()`에서 추출 후 `ServiceFunc.Subscribe`에 설정하고 `Sequences`에서 제거.

주의: `@subscribe`는 시퀀스가 아니라 함수 메타데이터이므로, `ParseFile()`에서 반드시 필터링해야 한다. 필터링하지 않으면 `validateRequiredFields()`의 `default` 분기에서 "알 수 없는 타입" ERROR가 발생한다.

```go
// ParseFile()에서 subscribe 추출
var filtered []Sequence
for _, seq := range sequences {
    if seq.Type == "subscribe" {
        sf.Subscribe = &SubscribeInfo{Topic: seq.Topic}
        continue
    }
    filtered = append(filtered, seq)
}
sequences = filtered
```

`parseLine()`에 분기 추가:
```go
case strings.HasPrefix(line, "@subscribe "):
    topic, _ := extractQuoted(line[11:])
    seq = &Sequence{Type: "subscribe", Topic: topic}
```

**@publish 파싱**:

```go
func parsePublish(rest string) (*Sequence, error) {
    // "topic" {payload} [{options}]
    topic, rest := extractQuoted(rest)
    payload, rest, err := extractInputs(rest)
    rest = strings.TrimSpace(rest)
    var options map[string]string
    if strings.HasPrefix(rest, "{") {
        options, _, err = extractInputs(rest)
    }
    return &Sequence{
        Type:    SeqPublish,
        Topic:   topic,
        Inputs:  payload,
        Options: options,
    }, nil
}
```

`parseLine()`에 분기 추가:
```go
case strings.HasPrefix(line, "@publish "):
    seq, err = parsePublish(line[9:])
```

### 2-1. 검증 규칙

`validateRequiredFields()`에 추가:
```go
case parser.SeqPublish:
    if seq.Topic == "" {
        errs = append(errs, ctx.err("@publish", "Topic 누락"))
    }
    if len(seq.Inputs) == 0 {
        errs = append(errs, ctx.err("@publish", "Payload 누락"))
    }
```

`validateFunc()`에서 subscribe 관련 검증 함수 호출:
```go
errs = append(errs, validateSubscribeRules(sf)...)
```

`validateSubscribeRules()`:
- `sf.Subscribe != nil` && 시퀀스에 `@response` 있음 → ERROR
- `sf.Subscribe != nil` && Inputs에서 `request.*` 사용 → ERROR
- `sf.Subscribe == nil` && Inputs에서 `message.*` 사용 → ERROR

`validateVariableFlow()`에 `message` 예약 변수 추가 (subscribe 함수일 때만):
```go
if sf.Subscribe != nil {
    declared["message"] = true
}
```

`reservedSources`에 `message` 추가 — result 변수명으로 사용 방지:
```go
var reservedSources = map[string]bool{
    "request": true, "currentUser": true, "config": true,
    "query": true, "message": true,
}
```

### 3-1. 코드젠

**`templateData` 확장**:
```go
type templateData struct {
    // ... 기존 필드 ...
    Topic      string // @publish: "order.completed"
    OptionCode string // @publish: ", queue.WithDelay(1800)" 또는 ""
}
```

**`buildTemplateData()`에 `@publish` 처리**:
```go
case parser.SeqPublish:
    d.Topic = seq.Topic
    d.InputFields = buildPublishPayload(seq.Inputs)
    d.OptionCode = buildPublishOptions(seq.Options)
```

`buildPublishPayload()`: `map[string]string` → `"OrderID": order.ID, "Email": order.Email` 형식 (키는 ucFirst, 값은 inputValueToCode 변환).

`buildPublishOptions()`: options가 비어있으면 빈 문자열, delay가 있으면 `, queue.WithDelay(1800)` 등.

**`ctx` 문제 해결**: gin 핸들러에서 `c *gin.Context`를 사용하므로, `@publish` 템플릿에서 `c.Request.Context()` 를 직접 사용한다:
```go
queue.Publish(c.Request.Context(), "topic", ...)
```

**`collectImports()` 확장**: `@publish` 시퀀스가 있으면 `"queue"` 임포트 추가:
```go
if seq.Type == parser.SeqPublish {
    seen["queue"] = true
}
```

**`@publish` 템플릿**:
```go
{{- define "publish" -}}
	if err {{if .FirstErr}}:={{else}}={{end}} queue.Publish(c.Request.Context(), "{{.Topic}}", map[string]any{
		{{.InputFields}}
	}{{.OptionCode}}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "{{.Message}}"})
		return
	}
{{end}}
```

**`@publish` err 추적**: `if err :=` 인라인이 아닌 `err =/err :=` + `if err != nil` 패턴 사용 (다른 시퀀스와 일관성 유지). `buildTemplateData()`의 err 선언 추적에 추가:
```go
case parser.SeqPublish:
    if !*errDeclared {
        d.FirstErr = true
        *errDeclared = true
    }
```

## 테스트 계획

| 테스트 | 검증 내용 |
|---|---|
| `TestParsePublish` | topic, payload 파싱 |
| `TestParsePublishWithOptions` | topic, payload, options 파싱 |
| `TestParseSubscribe` | Subscribe 필드 설정, topic 파싱 |
| `TestParseSubscribeWithSequences` | subscribe + 후속 시퀀스 파싱 |
| `TestValidatePublishTopicMissing` | topic 누락 → ERROR |
| `TestValidatePublishPayloadMissing` | payload 누락 → ERROR |
| `TestValidateSubscribeWithResponse` | subscribe 함수에 @response → ERROR |
| `TestValidateSubscribeWithRequest` | subscribe 함수에서 request 사용 → ERROR |
| `TestValidateHTTPWithMessage` | HTTP 함수에서 message 사용 → ERROR |
| `TestGeneratePublish` | queue.Publish 코드젠 |
| `TestGeneratePublishWithOptions` | queue.WithDelay 옵션 코드젠 |

## 의존성

- 기존 파서 인프라: `extractQuoted()`, `extractInputs()`, `parseInputs()`
- `queue` 패키지: 코드젠 대상 (fullend에서 제공)

## 검증 방법

```bash
go test ./parser/... ./validator/... ./generator/... -count=1
```
