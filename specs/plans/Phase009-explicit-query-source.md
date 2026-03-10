✅ 완료

# Phase 009: `query` 예약 소스 — QueryOpts 명시적 선언

## 목표

QueryOpts의 암묵적 삽입을 제거하고, `query`를 예약 소스로 추가하여 SSaC spec에서 명시적으로 선언하게 한다.

현재: codegen이 OpenAPI x-extensions + List 메서드 추론으로 `opts`를 몰래 삽입
목표: spec에 `query`가 명시되어야만 QueryOpts가 전달됨

```go
// 변경 전 — query가 보이지 않음
// @get []Reservation reservations = Reservation.ListByUserID(currentUser.ID)

// 변경 후 — query가 명시적
// @get []Reservation reservations = Reservation.ListByUserID(currentUser.ID, query)
```

## 설계

### `query` 예약 소스

`request`, `currentUser`, `config`과 동일한 예약 소스. result 변수명으로 사용 불가.

- 파서: `query`는 `Arg{Source: "query"}`로 파싱됨 (Field 없음, bare variable)
- codegen: `query` → `opts` 변수 (QueryOpts 자동 생성·파싱 코드는 유지)
- 모델 인터페이스: `query` 인자가 있는 메서드에만 `opts QueryOpts` 추가

### 암묵적 삽입 제거

현재 codegen의 암묵적 opts 삽입 로직을 제거:
- `GenerateFunc`: `QueryOpts auto-append for List methods` 블록 제거
- `needsQueryOpts`: args에 `query` 소스가 있는지로 판단 (OpenAPI + isListMethod 추론 제거)
- `deriveInterfaces`: `HasQueryOpts`를 args에 `query`가 있는지로 판단 (Operation 추론 제거)
- `hasTotal`: `query` 인자 + `[]Type` 결과일 때 true

### Validator 교차 검증

- OpenAPI에 x-extensions 있는데 SSaC에 `query` 없으면 **WARNING**
- SSaC에 `query` 있는데 OpenAPI에 x-extensions 없으면 **ERROR**

### 기존 수정지시서003 수정(slice 추론)과의 관계

수정지시서003의 slice 타입 추론 수정은 이 Phase로 **대체**됨. `query` 명시 여부로 판단하므로 타입 추론 불필요.

## 변경 파일

| 파일 | 내용 |
|---|---|
| `parser/parser.go` | `query` bare arg 파싱 (기존 파서로 이미 처리됨, 변경 없을 수 있음) |
| `validator/validator.go` | `query`를 reservedSources에 추가, 교차 검증 추가 |
| `generator/go_target.go` | 암묵적 opts 삽입 제거, `query` arg 기반으로 전환 |
| `generator/go_target.go` | `needsQueryOpts`: args에 query 있는지 확인 |
| `generator/go_target.go` | `deriveInterfaces`: args에 query 있으면 HasQueryOpts |
| `generator/go_target.go` | `argToCode`: `query` → `opts` 변환 |
| `generator/generator_test.go` | 기존 QueryOpts 테스트 업데이트, 명시적 query 테스트 추가 |
| `validator/validator_test.go` | query 교차 검증 테스트 추가 |
| `specs/dummy-study/service/list_my_reservations.go` | `query` 인자 추가 |
| `CLAUDE.md` | 예약 소스에 `query` 추가 |

## 검증

```bash
go test ./parser/... ./validator/... ./generator/... -count=1
ssac gen specs/dummy-study/ /tmp/ssac-phase9-check/
```

## 의존성

- 수정지시서v2/003 (QueryOpts 비-List 제외 → 이 Phase로 대체)
