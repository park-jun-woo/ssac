✅ 완료

# Phase 014: CRUD args를 {Key: value} 문법으로 통일

## 목표

`@get`, `@post`, `@put`, `@delete`의 args를 `@call`/`@state`/`@auth`와 동일한 `{Key: value}` 형태로 통일한다.

```go
// 변경 전 (positional)
// @get Course course = Course.FindByID(request.CourseID)
// @put Reservation.UpdateStatus(request.ReservationID, "cancelled")
// @post User user = User.Create(request.Email, hashedPasswordResp.HashedPassword, request.Role)

// 변경 후 ({Key: value})
// @get Course course = Course.FindByID({CourseID: request.CourseID})
// @put Reservation.UpdateStatus({ReservationID: request.ReservationID, Status: "cancelled"})
// @post User user = User.Create({Email: request.Email, HashedPassword: hashedPasswordResp.HashedPassword, Role: request.Role})
```

모든 시퀀스 타입의 인자가 `{Key: value}` 통일:
- `@get`, `@post`, `@put`, `@delete` — model method 호출 (positional 생성)
- `@call` — external pkg Request struct (named field 생성)
- `@state`, `@auth` — Input struct (named field 생성)

## 변경 파일

| 파일 | 내용 |
|---|---|
| `parser/parser.go` | CRUD 파싱: `({Key: val, ...})` → `seq.Inputs` map에 저장 |
| `parser/parser_test.go` | CRUD 파싱 테스트 업데이트 |
| `generator/go_target.go` | `buildArgsCode()`: `seq.Inputs` 기반으로 value만 추출하여 positional 생성 |
| `generator/go_target.go` | `collectRequestParams()`: `seq.Inputs` value에서 request 참조 수집 |
| `generator/go_target.go` | model interface 파생: `seq.Inputs` 기반 파라미터 도출 |
| `generator/generator_test.go` | CRUD 코드젠 테스트 업데이트 |
| `validator/validator.go` | 변수 흐름 검증: `seq.Inputs` value 기반 (이미 지원) |
| `specs/dummy-study/service/**/*.go` | 기존 CRUD args 마이그레이션 |

## 설계

### 파서 변경

CRUD의 args 부분이 `({...})`이면 `parseInputs()`로 파싱하여 `seq.Inputs`에 저장. `seq.Args`는 비워둔다.

```go
// @get Course course = Course.FindByID({CourseID: request.CourseID})
// → seq.Inputs = {"CourseID": "request.CourseID"}
// → seq.Args = nil
```

`parseCallExpr` → `parseModelExpr` 통합: CRUD도 `({...})` 지원.

### 코드젠 변경

CRUD는 positional 함수 호출이므로 `seq.Inputs`의 value만 순서대로 추출:

```go
func buildArgsCodeFromInputs(inputs map[string]string) string {
    // key 알파벳 순 정렬 → value를 inputValueToCode()로 변환
}
```

`@call`은 기존대로 `buildInputFieldsFromMap()` → `Key: value` 형식.

### model interface 파생

`deriveInterfaces()`에서 CRUD의 `seq.Inputs`를 사용. Key가 파라미터명, value에서 타입 추론.

## 검증

```bash
go test ./parser/... ./validator/... ./generator/... -count=1
ssac gen specs/dummy-study/ /tmp/ssac-phase14-check/
```

## 의존성

- 수정지시서v2/008 (방향 전환: result 추출 → args 통일)
- Phase 013 (@call Inputs 기반)
