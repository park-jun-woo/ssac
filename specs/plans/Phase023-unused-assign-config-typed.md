# Phase 023: 미사용 변수 _ 시 := → = 전환 + config.Get 타입 변환

## 목표

Phase022에서 도입한 미사용 변수 `_` 처리와 `config.Get()` 코드젠의 두 가지 컴파일 에러를 수정한다.

## 문제

### A. `_, err :=` → `_, err =` 전환

`_`(blank identifier)는 새 변수가 아니므로, `err`이 이미 선언된 상태에서 `_, err :=`는 Go 컴파일 에러(`no new variables on left side of :=`).

현재 get/post/call_with_result 템플릿:
```
{{if .Unused}}_{{else}}{{.Result.Var}}{{end}}, err {{if .ReAssign}}={{else}}:={{end}}
```

`ReAssign`은 **result 변수**의 재선언 여부를 추적하지, `err`의 선언 여부를 추적하지 않음.
- Unused=true → `_` 출력 → `_`는 새 변수 아님
- `err`이 이전 시퀀스에서 이미 선언됨 → `:=` 불가
- 결과: `_, err :=` → 컴파일 에러

**수정**: Unused=true일 때, `err`이 이미 선언된 상태면 `ReAssign=true`로 강제하여 `_, err =` 생성.

### B. `config.Get()` 타입 변환

`config.Get()`은 항상 `string` 반환. @call의 Request 필드가 `int`, `int64`, `bool` 등일 때 타입 불일치 컴파일 에러.

**수정**: `inputValueToCode()`에 `targetType` 파라미터를 추가하고, 타입에 따라 `config.GetInt()`, `config.GetInt64()`, `config.GetBool()` 등 사용.

## 변경 파일 목록

### 1. generator/go_target.go

#### A. Unused 시 := → = 전환

`buildTemplateData()`에서 err 선언 추적 블록(404-426행) **이전**에 `errWasDeclared`를 캡처. Unused=true일 때 errWasDeclared면 ReAssign 강제:

```go
// result var reassign tracking (기존 396-401행)
if seq.Result != nil {
    if declaredVars[seq.Result.Var] {
        d.ReAssign = true
    }
    declaredVars[seq.Result.Var] = true
}

// ★ 추가: Unused + err already declared → force ReAssign
// (Unused는 호출측에서 설정하므로, 여기서는 errDeclared 상태만 캡처)
```

실제로 `Unused`는 `buildTemplateData` 바깥(generateHTTPFunc/generateSubscribeFunc)에서 설정됨. 따라서 `buildTemplateData`에서 처리할 수 없음.

**대안**: `templateData`에 `ErrDeclared bool` 필드 추가. `buildTemplateData` 진입 시점의 `*errDeclared` 값을 저장. 호출측에서 `Unused && ErrDeclared` 조합으로 `ReAssign` 오버라이드:

```go
// buildTemplateData 내부, 최상단에:
d.ErrDeclared = *errDeclared

// generateHTTPFunc/generateSubscribeFunc에서:
data := buildTemplateData(seq, errDeclared, ...)
if seq.Result != nil && !usedVars[seq.Result.Var] {
    data.Unused = true
    if data.ErrDeclared {
        data.ReAssign = true  // _, err = (no new vars with :=)
    }
}
```

#### B. config.Get 타입 변환

`inputValueToCode(val string)` → `inputValueToCode(val string, targetType string)` 시그니처 변경:

```go
func inputValueToCode(val string, targetType string) string {
    // ...
    if strings.HasPrefix(val, "config.") {
        key := val[len("config."):]
        upperKey := toUpperSnake(key)
        switch targetType {
        case "int":
            return `config.GetInt("` + upperKey + `")`
        case "int32":
            return `int32(config.GetInt("` + upperKey + `"))`
        case "int64":
            return `config.GetInt64("` + upperKey + `")`
        case "bool":
            return `config.GetBool("` + upperKey + `")`
        default:
            return `config.Get("` + upperKey + `")`
        }
    }
    // ...
}
```

호출측 변경:
- `buildInputFieldsFromMap(inputs, paramTypes)` — `paramTypes map[string]string` 파라미터 추가. @call 시 `MethodInfo.ParamTypes` 전달, 그 외는 `nil`.
- `buildArgsCodeFromInputs(inputs)` — targetType `""` 전달 (CRUD는 positional args, 별도 타입 추적 불필요)
- `buildPublishPayload(inputs)` — targetType `""` 전달

`buildTemplateData()`의 @call 분기에서 ParamTypes 조회:

```go
case parser.SeqCall:
    if len(seq.Inputs) > 0 {
        var paramTypes map[string]string
        if st != nil {
            // seq.Model = "pkg.Func" → look up MethodInfo.ParamTypes
            pkgModel := seq.Package + "." + strings.SplitN(seq.Model, ".", 2)[0]
            if ms, ok := st.Models[pkgModel]; ok {
                method := strings.SplitN(seq.Model, ".", 2)[1]
                if mi, ok := ms.Methods[method]; ok {
                    paramTypes = mi.ParamTypes
                }
            }
        }
        d.InputFields = buildInputFieldsFromMap(seq.Inputs, paramTypes)
    }
```

### 2. generator/go_templates.go

변경 없음. 기존 `{{if .ReAssign}}={{else}}:={{end}}` 로직이 `ReAssign` 오버라이드로 정확히 동작.

### 3. validator/validator.go — config 타입 검증

`validateCallInputTypes()`에서 config 값이 변환 불가능한 타입(`map[string]string` 등)에 대입될 때 ERROR:

```go
// resolveCallInputType()에서 config.* → "string" 반환 (현재 동작)
// validateCallInputTypes()에서 config 값 + 지원 안 되는 target 타입 → ERROR
```

`config.*` 값이 `string`, `int`, `int32`, `int64`, `bool` 외의 타입 필드에 대입되면:
```
ERROR: config.Key는 타입 map[string]string으로 변환할 수 없습니다
```

## 의존성

- Phase022 완료 상태 (Unused, config.Get, ParamTypes 모두 구현됨)
- fullend `pkg/config` 패키지에 `GetInt`, `GetInt64`, `GetBool` 함수 추가 필요 (별도 작업)

## 검증 방법

### 테스트 추가

| 테스트 | 파일 | 검증 내용 |
|---|---|---|
| `TestGenerateUnusedVarErrAlreadyDeclared` | generator_test.go | 2번째 시퀀스에서 Unused → `_, err =` (= 사용) |
| `TestGenerateUnusedVarFirstErr` | generator_test.go | 첫 시퀀스에서 Unused → `_, err :=` (:= 사용) |
| `TestGenerateConfigGetInt` | generator_test.go | @call input config.* + int 필드 → `config.GetInt("KEY")` |
| `TestGenerateConfigGetInt64` | generator_test.go | @call input config.* + int64 필드 → `config.GetInt64("KEY")` |
| `TestGenerateConfigGetBool` | generator_test.go | @call input config.* + bool 필드 → `config.GetBool("KEY")` |
| `TestValidateConfigUnsupportedType` | validator_test.go | config.* + map[string]string → ERROR |

### 기존 테스트

```bash
go test ./parser/... ./validator/... ./generator/... -count=1
```

기존 148 테스트 + 신규 6개 = 154 테스트 목표.

## 상태: ✅ 완료
