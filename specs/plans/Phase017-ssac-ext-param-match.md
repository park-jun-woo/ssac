# Phase 017: .ssac 확장자 도입 + 패키지 모델 파라미터 매칭 검증

## 목표

1. SSaC 서비스 파일 확장자를 `.go` → `.ssac`으로 변경하여 Go 빌드 충돌 해소
2. 패키지 접두사 @model의 SSaC 파라미터명 ↔ Go interface 파라미터명 일치 검증 추가

## 변경 파일 목록

### 1. .ssac 확장자 도입

| 파일 | 변경 |
|---|---|
| `parser/parser.go:17` | `ParseDir()`: `.go` → `.ssac` 필터 |
| `generator/generator.go:41` | `outName`: `TrimSuffix(sf.FileName, ".go")` → `TrimSuffix(sf.FileName, ".ssac")` |
| `parser/parser_test.go` | 테스트에서 서비스 파일 생성 시 `.go` → `.ssac` (4곳: line 322, 346, 520, 635) |
| `specs/dummy-study/service/**/*.go` | 7개 파일 확장자 `.go` → `.ssac` 이름 변경 |

**변경하지 않는 곳:**
- `validator/symbol.go:300` (`loadGoInterfaces`) — model/ 디렉토리의 Go interface 파일이므로 `.go` 유지
- `validator/symbol.go:432` (`loadPackageGoInterfaces`) — 패키지 Go interface 파일이므로 `.go` 유지
- `generator/go_target.go:20` (`FileExtension()`) — 생성 코드는 `.go`이므로 유지
- `v1/testdata/` — 아카이브, 수정 금지

### 2. 패키지 모델 파라미터 매칭 검증

| 파일 | 변경 |
|---|---|
| `validator/symbol.go` | `MethodInfo`에 `Params []string` 필드 추가 |
| `validator/symbol.go` | `loadPackageGoInterfaces()`: interface 메서드 파라미터명 수집 (`context.Context` 제외) |
| `validator/validator.go` | `validateModel()`의 패키지 모델 분기에 파라미터 매칭 로직 추가 |

### 3. 테스트

| 파일 | 변경 |
|---|---|
| `parser/parser_test.go` | 기존 ParseDir 테스트 파일 확장자 `.ssac`으로 변경 |
| `validator/validator_test.go` | 파라미터 매칭 테스트 추가 |

## 상세 설계

### 2-1. MethodInfo 확장

```go
type MethodInfo struct {
    Cardinality string   // "one", "many", "exec"
    Params      []string // interface 파라미터명 (context.Context 제외)
}
```

### 2-2. loadPackageGoInterfaces 변경

현재 메서드 이름만 저장:
```go
ms.Methods[method.Names[0].Name] = MethodInfo{}
```

변경 후 파라미터명도 수집:
```go
funcType := method.Type.(*ast.FuncType)
var params []string
for _, param := range funcType.Params.List {
    // context.Context 타입 스킵
    if isContextType(param.Type) {
        continue
    }
    for _, name := range param.Names {
        params = append(params, name.Name)
    }
}
ms.Methods[method.Names[0].Name] = MethodInfo{Params: params}
```

`isContextType()` 헬퍼: `ast.SelectorExpr`에서 `X.Name == "context"` && `Sel.Name == "Context"` 확인.

### 2-3. validateModel 파라미터 매칭

패키지 모델 분기 (`seq.Package != ""`)에서 메서드 존재 확인 후:

```go
mi := ms.Methods[methodName]
if len(mi.Params) > 0 {
    ssacKeys := keysOf(seq.Inputs)
    ifaceParams := setOf(mi.Params)

    // SSaC에 있지만 interface에 없는 키 → ERROR
    for _, key := range ssacKeys {
        if !ifaceParams[key] {
            // ERROR: 파라미터 불일치 + interface 파라미터 목록 안내
        }
    }
    // interface에 있지만 SSaC에 없는 파라미터 → ERROR
    for _, param := range mi.Params {
        if !ssacKeysSet[param] {
            // ERROR: 파라미터 누락 + SSaC 파라미터 목록 안내
        }
    }
}
```

`mi.Params`가 비어있으면 (interface 파라미터 정보 없음) 검증 스킵.

### 2-4. ERROR 메시지 형식

```
[ERROR] SSaC ↔ Interface: GetSession — @model session.Session.Get 파라미터 불일치. SSaC에 "token"이 있지만 interface에 없습니다. interface 파라미터: [key]
[ERROR] SSaC ↔ Interface: CreateSession — @model session.Session.Set 파라미터 누락. interface에 "ttl"이 필요하지만 SSaC에 없습니다. SSaC 파라미터: [key, value]
```

### 2-5. context.Context 스킵 규칙

- interface 파라미터 타입이 `context.Context`이면 매칭 대상에서 제외
- Go 언어 보편 관례 (gin, echo, net/http 등 프레임워크가 주입)
- interface에 `ctx context.Context`가 없어도 에러 아님 (사용자 선택)

### 2-6. 적용 범위

- `.ssac` 확장자: 모든 SSaC 서비스 파일
- 파라미터 매칭: 패키지 접두사 @model만 해당 (`seq.Package != ""`)
- DDL 모델 (접두사 없음)은 기존 방식 유지 (fullend crosscheck 담당)

## 테스트 계획

| 테스트 | 검증 내용 |
|---|---|
| `TestParseDirSsacExtension` | `.ssac` 파일만 파싱, `.go` 파일 무시 |
| `TestValidatePackageParamMatch` | SSaC 파라미터 = interface 파라미터 → OK |
| `TestValidatePackageParamExtra` | SSaC에 interface에 없는 파라미터 → ERROR |
| `TestValidatePackageParamMissing` | interface에 있지만 SSaC에 없는 파라미터 → ERROR |
| `TestValidatePackageParamSkipContext` | `context.Context` 파라미터 제외 확인 |

## 의존성

- 수정지시서 016 (패키지 접두사 @model + Go interface 로드)
- `go/ast`: `*ast.FuncType`, `*ast.SelectorExpr` — 이미 사용 중

## 검증 방법

```bash
go test ./parser/... ./validator/... ./generator/... -count=1
```
