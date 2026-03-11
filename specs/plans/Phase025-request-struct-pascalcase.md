# Phase 025: snake_case 네이밍 변환 전면 수정

## 목표

`ettle/strcase` 라이브러리를 도입하고, 코드젠 전체에서 snake_case 입력을 올바른 Go 네이밍으로 변환한다.
- PascalCase 필요한 곳: `ucFirst`/변환없음 → `strcase.ToGoPascal()`
- camelCase 필요한 곳: `lcFirst` → `strcase.ToGoCamel()`

## 현재 문제

1. **ucFirst**: `ucFirst("bid_amount")` → `Bid_amount` (snake_case 미처리)
2. **lcFirst**: `lcFirst("bid_amount")` → `bid_amount` (camelCase 변환 안 됨)
3. **변환 없음**: `rp.name` 그대로 struct 필드에 사용 → unexported → JSON 바인딩 불가

## 변경 파일 목록

### 1. generator/generator.go — toPascalCase() 추가

`commonInitialisms` 맵을 활용한 snake_case → PascalCase 변환 함수.
delimiter(`_`, `-`, ` `) 분리 → 각 워드 capitalize → Go initialism 적용.

| 입력 | 결과 |
|---|---|
| `email` | `Email` |
| `bid_amount` | `BidAmount` |
| `id` | `ID` |
| `client_id` | `ClientID` |
| `video_url` | `VideoURL` |
| `CourseID` | `CourseID` (이미 PascalCase면 유지) |

### 2. generator/go_target.go — PascalCase 필요한 곳 (4곳)

| 행 | 함수 | 현재 | 변경 |
|---|---|---|---|
| 669 | buildJSONBodyParams (struct 필드) | `rp.name` (변환 없음) | `toPascalCase(rp.name)` |
| 501 | buildInputFieldsFromMap | `ucFirst(k)` | `toPascalCase(k)` |
| 546 | buildPublishPayload | `ucFirst(k)` | `toPascalCase(k)` |
| 1141 | resolveArgParamName (변수.Field) | `source + ucFirst(a.Field)` | `source + toPascalCase(a.Field)` |

**buildTemplateData의 `ucFirst(parts[1])`은 유지** — @call 함수명은 이미 PascalCase이므로 ucFirst로 충분.

### 3. generator/go_target.go — camelCase 필요한 곳 (9곳)

모두 `lcFirst(x)` → `lcFirst(toPascalCase(x))` 패턴.

| 행 | 함수 | 현재 | 변경 |
|---|---|---|---|
| 477 | argToCode (request.Field) | `lcFirst(a.Field)` | `lcFirst(toPascalCase(a.Field))` |
| 512 | inputValueToCode (request.Field) | `lcFirst(val[8:])` | `lcFirst(toPascalCase(val[8:]))` |
| 650 | 단일 param 추출 (c.Query) | `lcFirst(rp.name)` | `lcFirst(toPascalCase(rp.name))` |
| 677 | buildJSONBodyParams (varName) | `lcFirst(rp.name)` | `lcFirst(toPascalCase(rp.name))` |
| 678 | buildJSONBodyParams (req.Field) | `rp.name` (변환 없음) | `toPascalCase(rp.name)` |
| 737 | generatePathParamCode | `lcFirst(pp.Name)` | `lcFirst(toPascalCase(pp.Name))` |
| 1113 | derivedParam Name (models_gen) | `lcFirst(k)` | `lcFirst(toPascalCase(k))` |
| 1138 | resolveArgParamName (request/currentUser) | `lcFirst(a.Field)` | `lcFirst(toPascalCase(a.Field))` |
| 1143 | resolveArgParamName (fallback) | `lcFirst(a.Field)` | `lcFirst(toPascalCase(a.Field))` |

### 4. 변경 불필요 (유지)

| 행 | 위치 | 이유 |
|---|---|---|
| 342 | `ucFirst(parts[1])` @call 함수명 | DSL 규칙상 PascalCase 보장 |
| 346 | `lcFirst(parts[0])` 모델명 | 파서가 PascalCase 강제 |
| 1135 | `lcFirst(a.Literal)` | 리터럴 → 이름 (edge case, 영향 없음) |

## 의존성

- Phase024 완료 상태

## 검증 방법

### 테스트 추가 (2개)

| 테스트 | 검증 내용 |
|---|---|
| `TestGenerateRequestStructExported` | `request.email` → struct `Email string \`json:"email"\`` |
| `TestGenerateRequestStructSnakeCase` | `request.bid_amount` → struct `BidAmount int32 \`json:"bid_amount"\``, 변수 `bidAmount := req.BidAmount` |

### 기존 테스트

기존 테스트는 `request.CourseID` 등 PascalCase 입력이라 `toPascalCase`와 `ucFirst`/`lcFirst` 결과 동일 → 변경 불필요.

```bash
go test ./parser/... ./validator/... ./generator/... -count=1
```

158 + 2 = 160 테스트 목표.

## 상태: ✅ 완료
