# Phase 5: Generator Target 추상화

수정지시서 004 기반. generator를 Target 인터페이스로 재구조화하여 다중 언어 코드젠 슬롯을 만든다.

## 목표

- parser IR (`ServiceFunc`, `Sequence`)은 이미 언어 중립 → generator만 분리하면 다중 언어 지원 가능
- 기존 외부 API (`Generate`, `GenerateFunc`, `GenerateModelInterfaces`) 시그니처 불변 → 테스트 수정 없음
- Go 코드 생성 로직을 `GoTarget` struct로 이동

## 작업 순서

### Step 1: Target 인터페이스 정의

`generator/target.go` 신규 생성:

```go
type Target interface {
    GenerateFunc(sf parser.ServiceFunc, st *validator.SymbolTable) ([]byte, error)
    GenerateModelInterfaces(funcs []parser.ServiceFunc, st *validator.SymbolTable, outDir string) error
    FileExtension() string
}
```

### Step 2: GoTarget으로 기존 로직 이동

| 현재 파일 | 이동 후 |
|---|---|
| `generator.go` (GenerateFunc 본문, buildTemplateData, collectTypedRequestParams, generateExtractCode, buildJSONBodyParams, resolveParam, resolveParamRef, lookupDDLType, collectImports, hasConversionErr, buildParamArgs, defaultMessage, zeroValueChecks, templateData) | `go_target.go` |
| `model_interface.go` (GenerateModelInterfaces 본문, collectModelUsages, deriveInterfaces, resolveParamName, resolveParamType, deriveReturnType, renderInterfaces, renderParams, hasQueryOpts, needsTimeImport, sortedKeys, sortedMethodKeys) | `go_target.go` |
| `templates.go` | `go_templates.go` (파일명만 변경) |

`GoTarget` struct에 `GenerateFunc`, `GenerateModelInterfaces`, `FileExtension` 메서드 구현.
내부 헬퍼 함수는 패키지 내부 함수로 유지 (GoTarget 메서드로 만들 필요 없음).

### Step 3: generator.go를 하위 호환 래퍼로 축소

```go
func Generate(funcs, outDir, st) error       → GenerateWith(DefaultTarget(), ...)
func GenerateFunc(sf, st) ([]byte, error)    → DefaultTarget().GenerateFunc(sf, st)
func GenerateModelInterfaces(funcs, st, out) → DefaultTarget().GenerateModelInterfaces(...)
func GenerateWith(t Target, funcs, outDir, st) error  // 신규 진입점
func DefaultTarget() Target                           // → &GoTarget{}
```

### Step 4: 범용 유틸 분리

`generator.go`에 남길 함수:
- `lcFirst()` — 언어 중립 유틸
- `toSnakeCase()` — 언어 중립 유틸
- `GenerateWith()`, `DefaultTarget()` — Target 디스패치

### Step 5: 검증

- 기존 51개 테스트 전부 통과 (API 불변)
- `var _ Target = (*GoTarget)(nil)` 컴파일 체크
- `go build ./cmd/ssac/` 성공

## 하지 않는 것

- registry 패턴 (`RegisterTarget`, `GetTarget`) — 현재 불필요, 차후 필요 시 추가
- CLI에 `--target` 플래그 — fullend가 `GenerateWith()`로 직접 선택
- Java/Node.js Target 구현 — 이 Phase는 구조만 만듦

## 리스크

- 낮음: 순수 리팩토링, 기능 추가 없음, 외부 API 불변
- 파일 이동만으로 기존 테스트 통과 확인이 검증 기준
