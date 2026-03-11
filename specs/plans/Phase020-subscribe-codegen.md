# ✅ 완료 Phase 020: @subscribe 메시지 타입 + 코드젠 분리

## 목표

1. `@subscribe` 함수의 메시지 페이로드를 Go struct로 타입 선언
2. `@subscribe` 문법에 메시지 타입명 추가: `@subscribe "topic" TypeName`
3. 함수 시그니처에서 파라미터 추출: `func Name(TypeName message) {}`
4. Subscribe 함수 코드젠을 HTTP 함수와 분리: `func(ctx context.Context, message T) error`
5. Subscribe 함수 내부 시퀀스의 에러 처리를 `return fmt.Errorf(...)` 패턴으로 변경

## 변경 파일 목록

### 1. 파서

| 파일 | 변경 |
|---|---|
| `parser/types.go` | `SubscribeInfo.MessageType` 추가, `ServiceFunc.Param` 추가, `ParamInfo` struct 추가, `StructInfo`/`StructField` struct 추가 |
| `parser/parser.go` | `@subscribe` 파싱에 TypeName 추출, `ParseFile()`에서 함수 파라미터 추출 (`Param`), struct 선언 수집 (`collectStructs`) |

### 2. 검증

| 파일 | 변경 |
|---|---|
| `validator/validator.go` | subscribe 파라미터 검증 (누락, 변수명, 타입 존재, 필드 존재) |

### 3. 코드젠

| 파일 | 변경 |
|---|---|
| `generator/go_target.go` | `GenerateFunc()` subscribe 분기: 시그니처, 에러 처리, ctx 전달 분리 |
| `generator/go_templates.go` | subscribe용 템플릿 추가 (에러 시 `return fmt.Errorf(...)`) |

### 4. 테스트

| 파일 | 변경 |
|---|---|
| `parser/parser_test.go` | @subscribe TypeName 파싱, 함수 파라미터 파싱, struct 수집 |
| `validator/validator_test.go` | 파라미터 누락 ERROR, 변수명 ERROR, 타입 미존재 ERROR, 필드 미존재 ERROR |
| `generator/generator_test.go` | subscribe 함수 시그니처, return err 패턴, ctx 전달, message 직접 접근 |

### 5. 문서

| 파일 | 변경 |
|---|---|
| `artifacts/manual-for-ai.md` | @subscribe 문법 변경, 메시지 타입 선언, 코드젠 분리, 테스트 수 갱신 |
| `artifacts/manual-for-human.md` | @subscribe 섹션 전면 갱신, 코드젠 예시 변경 |
| `README.md` | subscribe 행 문법 변경, 코드젠 기능 갱신 |
| `CLAUDE.md` | @subscribe 문법 변경, 코드젠 기능 갱신 |

## 상세 설계

### 1-1. types.go 변경

```go
type SubscribeInfo struct {
    Topic       string // "order.completed"
    MessageType string // "OnOrderCompletedMessage"
}

type ParamInfo struct {
    TypeName string // "OnOrderCompletedMessage"
    VarName  string // "message"
}

type ServiceFunc struct {
    Name      string
    FileName  string
    Domain    string
    Sequences []Sequence
    Imports   []string
    Subscribe *SubscribeInfo
    Param     *ParamInfo     // nil이면 HTTP (파라미터 없음)
    Structs   []StructInfo   // .ssac 파일에 선언된 Go struct 목록
}

type StructInfo struct {
    Name   string        // "OnOrderCompletedMessage"
    Fields []StructField
}

type StructField struct {
    Name string // "OrderID"
    Type string // "int64"
}
```

### 1-2. parser.go 변경

**@subscribe 파싱 변경**:

```go
case strings.HasPrefix(line, "@subscribe "):
    rest := strings.TrimSpace(line[11:])
    topic, rest := extractQuoted(rest)
    msgType := strings.TrimSpace(rest)
    seq = &Sequence{Type: "subscribe", Topic: topic, Target: msgType}
```

`Target` 필드를 MessageType 전달에 임시 사용 (subscribe는 시퀀스에서 제거되므로 충돌 없음).

**ParseFile()에서 subscribe 추출 변경**:

```go
if seq.Type == "subscribe" {
    sf.Subscribe = &SubscribeInfo{
        Topic:       seq.Topic,
        MessageType: seq.Target, // parseLine에서 Target에 저장
    }
    continue
}
```

**함수 파라미터 추출**:

`ParseFile()`에서 `ast.FuncDecl.Type.Params`를 검사:

```go
if fn.Type.Params != nil && fn.Type.Params.List != nil {
    for _, param := range fn.Type.Params.List {
        if len(param.Names) > 0 {
            typeName := exprToString(param.Type)
            varName := param.Names[0].Name
            sf.Param = &ParamInfo{TypeName: typeName, VarName: varName}
        }
    }
}
```

`exprToString()` 헬퍼:

```go
func exprToString(expr ast.Expr) string {
    switch t := expr.(type) {
    case *ast.Ident:
        return t.Name
    case *ast.SelectorExpr:
        return exprToString(t.X) + "." + t.Sel.Name
    default:
        return ""
    }
}
```

**struct 수집** (`collectStructs`):

```go
func collectStructs(f *ast.File) []StructInfo {
    var structs []StructInfo
    for _, decl := range f.Decls {
        gd, ok := decl.(*ast.GenDecl)
        if !ok {
            continue
        }
        for _, spec := range gd.Specs {
            ts, ok := spec.(*ast.TypeSpec)
            if !ok {
                continue
            }
            st, ok := ts.Type.(*ast.StructType)
            if !ok {
                continue
            }
            si := StructInfo{Name: ts.Name.Name}
            for _, field := range st.Fields.List {
                if len(field.Names) > 0 {
                    si.Fields = append(si.Fields, StructField{
                        Name: field.Names[0].Name,
                        Type: exprToString(field.Type),
                    })
                }
            }
            structs = append(structs, si)
        }
    }
    return structs
}
```

`ParseFile()`에서:
```go
structs := collectStructs(f)
// ...
sf.Structs = structs
```

주의: struct는 파일 레벨이므로 같은 파일의 모든 함수가 동일한 Structs를 공유한다.

### 2-1. 검증 규칙

`validateSubscribeRules()`에 추가:

```go
// subscribe 함수에 파라미터 필수
if sf.Subscribe != nil && sf.Param == nil {
    errs = append(errs, errCtx{sf.FileName, sf.Name, -1}.err(
        "@subscribe", "@subscribe 함수에 파라미터가 필요합니다 — func Name(TypeName message) {}"))
}

// 파라미터 변수명은 반드시 "message"
if sf.Param != nil && sf.Param.VarName != "message" {
    errs = append(errs, errCtx{sf.FileName, sf.Name, -1}.err(
        "@subscribe", fmt.Sprintf("파라미터 변수명은 \"message\"여야 합니다 — 현재: %q", sf.Param.VarName)))
}

// MessageType이 파일 내 struct로 존재하는지
if sf.Subscribe != nil && sf.Subscribe.MessageType != "" {
    found := false
    for _, si := range sf.Structs {
        if si.Name == sf.Subscribe.MessageType {
            found = true
            break
        }
    }
    if !found {
        errs = append(errs, errCtx{sf.FileName, sf.Name, -1}.err(
            "@subscribe", fmt.Sprintf("메시지 타입 %q이 파일 내에 struct로 선언되지 않았습니다", sf.Subscribe.MessageType)))
    }
}
```

**message.Field 검증** — `validateVariableFlow()`에서 확장:

subscribe 함수에서 `message.X`를 사용할 때, X가 struct 필드에 존재하는지 확인:

```go
// Inputs value 검증 (기존 로직 후)
if sf.Subscribe != nil && strings.HasPrefix(val, "message.") {
    field := val[len("message."):]
    if !hasStructField(sf.Structs, sf.Subscribe.MessageType, field) {
        errs = append(errs, ctx.err("@"+seq.Type, fmt.Sprintf(
            "message.%s — 메시지 타입 %q에 %q 필드가 없습니다", field, sf.Subscribe.MessageType, field)))
    }
}
```

`hasStructField()` 헬퍼:

```go
func hasStructField(structs []StructInfo, typeName, fieldName string) bool {
    for _, si := range structs {
        if si.Name == typeName {
            for _, f := range si.Fields {
                if f.Name == fieldName {
                    return true
                }
            }
            return false
        }
    }
    return false
}
```

### 3-1. 코드젠 분리

**`GenerateFunc()` 분기**:

subscribe 함수와 HTTP 함수를 분리:

```go
func (g *GoTarget) GenerateFunc(sf parser.ServiceFunc, st *validator.SymbolTable) ([]byte, error) {
    if sf.Subscribe != nil {
        return g.generateSubscribeFunc(sf, st)
    }
    return g.generateHTTPFunc(sf, st)
}
```

기존 `GenerateFunc` 본문을 `generateHTTPFunc()`으로 이동.

**`generateSubscribeFunc()` 구현**:

```go
func (g *GoTarget) generateSubscribeFunc(sf parser.ServiceFunc, st *validator.SymbolTable) ([]byte, error) {
    var buf bytes.Buffer

    pkgName := "service"
    if sf.Domain != "" {
        pkgName = sf.Domain
    }
    buf.WriteString("package " + pkgName + "\n\n")

    // imports: context, fmt + sequence별 필요 패키지
    imports := collectSubscribeImports(sf)
    if len(imports) > 0 {
        buf.WriteString("import (\n")
        for _, imp := range imports {
            fmt.Fprintf(&buf, "\t%q\n", imp)
        }
        buf.WriteString(")\n\n")
    }

    // func signature
    msgType := sf.Subscribe.MessageType
    fmt.Fprintf(&buf, "func %s(ctx context.Context, message %s) error {\n", sf.Name, msgType)

    // sequences — subscribe 전용 템플릿 사용
    errDeclared := false
    declaredVars := map[string]bool{}
    resultTypes := map[string]string{}
    for _, seq := range sf.Sequences {
        if seq.Result != nil {
            resultTypes[seq.Result.Var] = seq.Result.Type
        }
    }

    for i, seq := range sf.Sequences {
        data := buildTemplateData(seq, &errDeclared, declaredVars, resultTypes, st, sf.Name)
        tmplName := subscribeTemplateName(seq)
        var seqBuf bytes.Buffer
        if err := goTemplates.ExecuteTemplate(&seqBuf, tmplName, data); err != nil {
            return nil, fmt.Errorf("sequence[%d] %s 템플릿 실행 실패: %w", i, seq.Type, err)
        }
        buf.Write(seqBuf.Bytes())
        buf.WriteString("\n")
    }

    buf.WriteString("\treturn nil\n")
    buf.WriteString("}\n")

    formatted, err := format.Source(buf.Bytes())
    if err != nil {
        return buf.Bytes(), fmt.Errorf("gofmt 실패: %w\n--- raw ---\n%s", err, buf.String())
    }
    return formatted, nil
}
```

**`subscribeTemplateName()`**: subscribe 함수 내 시퀀스는 `sub_`접두사 템플릿 사용:

```go
func subscribeTemplateName(seq parser.Sequence) string {
    switch seq.Type {
    case parser.SeqCall:
        if seq.Result != nil {
            return "sub_call_with_result"
        }
        return "sub_call_no_result"
    case parser.SeqPublish:
        return "sub_publish"
    case parser.SeqEmpty:
        return "sub_empty"
    case parser.SeqExists:
        return "sub_exists"
    default:
        // get, post, put, delete — 모델 호출 동일, 에러만 다름
        return "sub_" + seq.Type
    }
}
```

**`collectSubscribeImports()`**:

```go
func collectSubscribeImports(sf parser.ServiceFunc) []string {
    seen := map[string]bool{
        "context": true,
        "fmt":     true,
    }
    for _, seq := range sf.Sequences {
        if seq.Type == parser.SeqState {
            seen["states/"+seq.DiagramID+"state"] = true
        }
        if seq.Type == parser.SeqAuth {
            seen["authz"] = true
        }
        if seq.Type == parser.SeqPublish {
            seen["queue"] = true
        }
    }
    needsCU := needsCurrentUser(sf.Sequences)
    if needsCU {
        seen["model"] = true
    }
    for _, imp := range sf.Imports {
        seen[imp] = true
    }
    var imports []string
    order := []string{"context", "fmt"}
    for _, imp := range order {
        if seen[imp] {
            imports = append(imports, imp)
            delete(seen, imp)
        }
    }
    var dynamic []string
    for imp := range seen {
        dynamic = append(dynamic, imp)
    }
    sort.Strings(dynamic)
    imports = append(imports, dynamic...)
    return imports
}
```

### 3-2. Subscribe 전용 템플릿

HTTP 함수와 Subscribe 함수의 차이:

| 항목 | HTTP | Subscribe |
|---|---|---|
| 시그니처 | `func(c *gin.Context)` | `func(ctx context.Context, message T) error` |
| 에러 처리 | `c.JSON(500, ...) + return` | `return fmt.Errorf("message: %w", err)` |
| @publish ctx | `c.Request.Context()` | `ctx` |
| @empty | `c.JSON(404, ...) + return` | `return fmt.Errorf("message")` |
| @exists | `c.JSON(409, ...) + return` | `return fmt.Errorf("message")` |
| @state | `c.JSON(409, ...) + return` | `return err` (state는 이미 error 반환) |
| @auth | `c.JSON(403, ...) + return` | `return fmt.Errorf("message")` |
| 함수 끝 | (암묵적 return) | `return nil` |

```go
// subscribe 함수 내 get/post — 모델 호출 동일, 에러만 다름
{{- define "sub_get" -}}
    {{.Result.Var}}, {{if .HasTotal}}total, {{end}}err {{if .ReAssign}}={{else}}:={{end}} {{.ModelCall}}({{.ArgsCode}})
    if err != nil {
        return fmt.Errorf("{{.Message}}: %w", err)
    }
{{end}}

{{- define "sub_post" -}}
    {{.Result.Var}}, err {{if .ReAssign}}={{else}}:={{end}} {{.ModelCall}}({{.ArgsCode}})
    if err != nil {
        return fmt.Errorf("{{.Message}}: %w", err)
    }
{{end}}

{{- define "sub_put" -}}
    err {{if .FirstErr}}:={{else}}={{end}} {{.ModelCall}}({{.ArgsCode}})
    if err != nil {
        return fmt.Errorf("{{.Message}}: %w", err)
    }
{{end}}

{{- define "sub_delete" -}}
    err {{if .FirstErr}}:={{else}}={{end}} {{.ModelCall}}({{.ArgsCode}})
    if err != nil {
        return fmt.Errorf("{{.Message}}: %w", err)
    }
{{end}}

{{- define "sub_empty" -}}
    if {{.Target}} {{.ZeroCheck}} {
        return fmt.Errorf("{{.Message}}")
    }
{{end}}

{{- define "sub_exists" -}}
    if {{.Target}} {{.ExistsCheck}} {
        return fmt.Errorf("{{.Message}}")
    }
{{end}}

{{- define "sub_state" -}}
    if err := {{.DiagramID}}state.CanTransition({{.DiagramID}}state.Input{ {{.InputFields}} }, "{{.Transition}}"); err != nil {
        return err
    }
{{end}}

{{- define "sub_auth" -}}
    if err := authz.Check(currentUser, "{{.Action}}", "{{.Resource}}", authz.Input{ {{.InputFields}} }); err != nil {
        return fmt.Errorf("{{.Message}}: %w", err)
    }
{{end}}

{{- define "sub_call_with_result" -}}
    {{.Result.Var}}, err {{if .ReAssign}}={{else}}:={{end}} {{.PkgName}}.{{.FuncMethod}}({{.PkgName}}.{{.FuncMethod}}Request{ {{.InputFields}} })
    if err != nil {
        return fmt.Errorf("{{.Message}}: %w", err)
    }
{{end}}

{{- define "sub_call_no_result" -}}
    if _, err {{if .FirstErr}}:={{else}}={{end}} {{.PkgName}}.{{.FuncMethod}}({{.PkgName}}.{{.FuncMethod}}Request{ {{.InputFields}} }); err != nil {
        return fmt.Errorf("{{.Message}}: %w", err)
    }
{{end}}

{{- define "sub_publish" -}}
    err {{if .FirstErr}}:={{else}}={{end}} queue.Publish(ctx, "{{.Topic}}", map[string]any{
{{.InputFields}}
    }{{.OptionCode}})
    if err != nil {
        return fmt.Errorf("{{.Message}}: %w", err)
    }
{{end}}
```

### 3-3. inputValueToCode에 message 소스 추가

`inputValueToCode()`에서 `message.Field`는 변환 없이 그대로 출력:

```go
func inputValueToCode(val string) string {
    if val == "query" { return "opts" }
    if strings.HasPrefix(val, "request.") { return lcFirst(val[len("request."):]) }
    // message.Field, currentUser.Field, config.Field, 일반 변수 → 그대로
    return val
}
```

현재 이미 그대로 통과하므로 변경 불필요. `message.OrderID`는 그대로 `message.OrderID`로 출력된다.

## 테스트 계획

### Parser

| 테스트 | 검증 내용 |
|---|---|
| `TestParseSubscribeWithType` | `@subscribe "topic" TypeName` → MessageType 추출 |
| `TestParseSubscribeParam` | `func Name(TypeName message) {}` → Param 추출 |
| `TestParseStructs` | .ssac 파일의 Go struct 선언 수집 |

### Validator

| 테스트 | 검증 내용 |
|---|---|
| `TestValidateSubscribeNoParam` | subscribe 함수에 파라미터 없음 → ERROR |
| `TestValidateSubscribeWrongVarName` | 파라미터 변수명이 message가 아님 → ERROR |
| `TestValidateSubscribeTypeNotFound` | 메시지 타입이 struct로 선언되지 않음 → ERROR |
| `TestValidateSubscribeFieldNotFound` | message.X 필드가 struct에 없음 → ERROR |
| `TestValidateSubscribeFieldOK` | message.X 필드가 struct에 있음 → OK |

### Generator

| 테스트 | 검증 내용 |
|---|---|
| `TestGenerateSubscribeFunc` | 시그니처 `func(ctx context.Context, message T) error`, `return nil` |
| `TestGenerateSubscribeGet` | 모델 호출 + `return fmt.Errorf(...)` 에러 처리 |
| `TestGenerateSubscribePublish` | `queue.Publish(ctx, ...)` — ctx 직접 사용 |
| `TestGenerateSubscribeEmpty` | `return fmt.Errorf("message")` 패턴 |

## 의존성

- 기존 `go/ast` 파싱 인프라 (이미 사용 중)
- Phase019의 `@subscribe`/`@publish` 기반

## 주의사항

1. **struct 범위**: 같은 .ssac 파일의 모든 함수가 파일 레벨 Structs를 공유. 별도 파일의 struct는 이번 Phase에서는 지원하지 않음 (같은 파일에 선언 필수).
2. **기존 @subscribe 테스트 수정**: Phase019 테스트는 TypeName 없이 `@subscribe "topic"` 형태. 새 문법에 맞게 수정 필요.
3. **err 추적**: subscribe 함수 내부에서도 get/post는 `:=` (FirstErr=true), put/delete는 기존 패턴 유지. `generateSubscribeFunc()`의 `errDeclared` 초기값은 false.
4. **HTTP 함수 기존 코드 영향 없음**: `GenerateFunc()` 분기로 기존 HTTP 코드젠은 `generateHTTPFunc()`으로 그대로 유지.
5. **subscribe 함수의 request 파라미터 수집 불필요**: `collectRequestParams()`는 HTTP 함수 전용. subscribe 함수는 request 없음.

## 검증 방법

```bash
go test ./parser/... ./validator/... ./generator/... -count=1
```
