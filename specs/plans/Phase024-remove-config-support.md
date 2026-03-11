# Phase 024: config.* 지원 전면 제거

## 목표

`config.*` 입력값 지원을 전면 제거한다. 인프라 설정(SMTP, DB 등)은 func 내부에서 직접 `os.Getenv()`로 읽으며, SSaC @call은 비즈니스 데이터만 전달한다.

`config.*` 사용 시 validator ERROR로 거부. generator에서 config 관련 코드젠 전체 삭제.

## 변경 파일 목록

### 1. validator/validator.go

- `validateVariableFlow()` declared에서 `"config": true` 제거
- `validateVariableFlow()` Inputs 검증에서 `config` 허용 제거 + `config.*` 사용 시 ERROR 추가
- `resolveCallInputType()`에서 `config.` 분기 삭제
- `validateConfigTypes()` 함수 + `configSupportedTypes` 변수 전체 삭제
- `ValidateWithSymbols()`에서 `validateConfigTypes` 호출 제거
- `reservedSources`의 `"config": true`는 유지 (result 변수명 충돌 방지)

### 2. generator/go_target.go

- `inputValueToCode(val string, targetType string)` → `inputValueToCode(val string)` 시그니처 복원, config 블록 삭제
- `buildInputFieldsFromMap(inputs, paramTypes)` → `buildInputFieldsFromMap(inputs)` 단순화
- `lookupCallParamTypes()` 함수 삭제
- `buildTemplateData()`의 @call 분기에서 paramTypes 조회 제거
- `buildArgsCodeFromInputs()`, `buildPublishPayload()` 호출에서 targetType 인자 제거
- `needsConfig()` 함수 삭제
- `collectImports()`/`collectSubscribeImports()`에서 config import 분기 삭제
- `argToCode()`에서 `a.Source == "config"` 분기 삭제
- `collectUsedVars()`에서 `config.` prefix 분기 삭제
- `toUpperSnake()` 함수 삭제 (config 전용이었으므로)

### 3. generator/generator_test.go

삭제 (5개):
- `TestGenerateConfigGet`
- `TestGenerateConfigGetSubscribe`
- `TestGenerateConfigGetInt`
- `TestGenerateConfigGetInt64`
- `TestGenerateConfigGetBool`

### 4. validator/validator_test.go

삭제 (1개):
- `TestValidateConfigUnsupportedType`

추가 (1개):
- `TestValidateConfigInputRejected` — `config.*` 입력값 → ERROR 확인

### 5. 문서 (CLAUDE.md, README.md, manual-for-ai.md, manual-for-human.md)

config.* 코드젠 관련 내용 제거, config.* 사용 금지 설명으로 교체.

## 의존성

- Phase023 완료 상태 (config 타입 변환 코드가 존재하는 상태에서 롤백)

## 검증 방법

```bash
go test ./parser/... ./validator/... ./generator/... -count=1
```

기존 163 테스트 - 6개(config 삭제) + 1개(config 거부) = 158 테스트 목표.

## 상태: ✅ 완료
