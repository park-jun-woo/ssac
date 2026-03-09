✅ 완료

# Phase 9: guard state 시퀀스 타입 추가

수정지시서 008 기반. 11번째 시퀀스 타입 `guard state`를 추가하여 상태 전이 가능 여부를 검사하는 코드를 생성한다.

## 목표

- `guard state {stateDiagramID}` 파싱 + 코드젠 + 검증
- 생성 코드: `{target}state.CanTransition(entity.Field, "FuncName")`
- 기존 10개 시퀀스 동작 변경 없음

## 작업 순서

### Step 1: parser — guard state 파싱

`parser/types.go`:
- `SeqGuardState = "guard state"` 상수 추가
- `ValidSequenceTypes`에 등록

`parser/parser.go`:
- `parseSequenceType`: `guard state` 이미 기존 `guard nil`/`guard exists`와 같은 2단어 패턴이므로 기존 로직으로 자동 처리됨
- `parseGuardTarget`: `guard state course` → Target `"course"` — 기존 로직으로 자동 처리됨
- `@param`은 기존 parseParam으로 파싱됨: `course.Published` → `Param{Name: "course.Published"}`

파싱 결과:
```
Sequence.Type = "guard state"
Sequence.Target = "course"           // stateDiagramID
Sequence.Params[0].Name = "course.Published"  // entity.Field (기존 dot notation)
```

### Step 2: validator — guard state 검증

`validator/validator.go` `validateRequiredFields`:
```go
case parser.SeqGuardState:
    if seq.Target == "" {
        errs = append(errs, ctx.err("@sequence", "guard state 대상 누락"))
    }
    if len(seq.Params) != 1 {
        errs = append(errs, ctx.err("@param", fmt.Sprintf("guard state는 @param 1개 필요, %d개 있음", len(seq.Params))))
    } else if !strings.Contains(seq.Params[0].Name, ".") {
        errs = append(errs, ctx.err("@param", "entity.Field 형식이어야 함"))
    }
```

`validateVariableFlow`: guard state의 Target은 stateDiagramID이므로 변수 참조가 아님.
→ `guard state`일 때는 Target 변수 검증을 건너뛴다. 대신 `@param`의 entity 부분이 선언된 변수인지 검증 (기존 paramVarRef 로직으로 자동 처리).

### Step 3: generator — guard state 코드젠

`generator/go_target.go` `templateData`에 추가 필드:
```go
// guard state
Entity      string // @param의 entity 부분 (e.g. "course")
StatusField string // @param의 field 부분 (e.g. "Published")
FuncName    string // 현재 함수명 (e.g. "PublishCourse")
```

`buildTemplateData`에 guard state 처리 추가:
```go
if seq.Type == parser.SeqGuardState && len(seq.Params) > 0 {
    parts := strings.SplitN(seq.Params[0].Name, ".", 2)
    d.Entity = parts[0]
    if len(parts) > 1 {
        d.StatusField = parts[1]
    }
}
```

`GenerateFunc`에서 `funcName`을 `templateData`에 전달:
```go
data.FuncName = sf.Name
```

`templateName` 함수: `guard state` 반환 (기존 로직으로 자동).

`generator/go_templates.go`에 guard state 템플릿 추가:
```
{{- define "guard state" -}}
	// guard state
	if !{{.Target}}state.CanTransition({{.Entity}}.{{.StatusField}}, "{{.FuncName}}") {
		http.Error(w, "{{.Message}}", http.StatusConflict)
		return
	}
{{end}}
```

`collectImports`에 guard state import 추가:
- `guard state` 시퀀스가 있으면 import에 `"{module}/states/{target}state"` 추가
- module 경로는 `go.mod`에서 읽거나, 심볼 테이블에서 가져올 수 있어야 함
- 단, SSaC는 소비자 프로젝트의 module 경로를 모름 → **import는 placeholder로 생성**: `"states/{target}state"`
- 실제 module prefix는 fullend가 후처리로 추가 (또는 `ssac gen`에 `--module` 플래그 추가 — 후속 Phase)

**결정**: import에 `{target}state`만 추가 (패키지 alias 없이). Go에서 import path 없이 패키지명만으로는 컴파일 불가이므로, placeholder import `"states/{target}state"` 를 추가한다. fullend가 실제 module prefix를 붙여준다.

### Step 4: 기본 메시지

`defaultMessage` 함수에 guard state 분기 추가:
```go
case parser.SeqGuardState:
    return "상태 전이가 허용되지 않습니다"
```

### Step 5: 테스트

#### parser_test.go — `TestParseGuardState`

testdata fixture 또는 인라인:
```go
// @sequence get
// @model Course.FindByID
// @param CourseID request
// @result course Course
//
// @sequence guard nil course
//
// @sequence guard state course
// @param course.Published
//
// @sequence put
// @model Course.Publish
// @param CourseID request
//
// @sequence response json
func PublishCourse(w http.ResponseWriter, r *http.Request) {}
```

검증:
- `seq.Type == "guard state"`
- `seq.Target == "course"`
- `seq.Params[0].Name == "course.Published"`

#### validator_test.go — `TestValidateGuardState`

- 정상: guard state course + @param course.Published → 에러 없음
- 에러: @param 없음 → 에러
- 에러: @param에 dot 없음 → 에러
- 에러: @param의 entity가 미선언 변수 → 에러

#### generator_test.go — `TestGenerateGuardState`

- `CanTransition(course.Published, "PublishCourse")` 포함 확인
- import에 `states/coursestate` 포함 확인

## 변경 파일 목록

| 파일 | 변경 유형 |
|---|---|
| `parser/types.go` | 수정: `SeqGuardState` 상수 + `ValidSequenceTypes` 등록 |
| `parser/parser_test.go` | 추가: `TestParseGuardState` |
| `validator/validator.go` | 수정: `validateRequiredFields`에 guard state 분기, `validateVariableFlow`에서 guard state Target 제외 |
| `validator/validator_test.go` | 추가: `TestValidateGuardState` |
| `generator/go_target.go` | 수정: `templateData`에 Entity/StatusField/FuncName, `buildTemplateData` guard state 처리, `collectImports` guard state import, `defaultMessage` 분기 |
| `generator/go_templates.go` | 추가: `guard state` 템플릿 |
| `generator/generator_test.go` | 추가: `TestGenerateGuardState` |

## 하지 않는 것

- 상태 머신 패키지 생성 — fullend 책임
- `--module` 플래그 — 후속 Phase
- stateDiagramID 실제 존재 여부 검증 — fullend 교차 검증

## 검증

```bash
go test ./parser/... ./validator/... ./generator/... -count=1

# guard state course + @param course.Published
# → coursestate.CanTransition(course.Published, "PublishCourse")
# → import "states/coursestate"
```

## 리스크

- **낮음**: 순수 추가. 기존 시퀀스 동작 변경 없음.
- import placeholder(`states/{target}state`)는 ssac 단독 빌드 시 컴파일 불가 → fullend 후처리 필요. 이는 설계 의도.
- `guard state`의 Target은 변수가 아닌 stateDiagramID이므로 `validateVariableFlow`에서 제외 필요.
