✅ 완료

# Phase 007: WARNING 억제 (`!` 접미사)

## 목표

모든 시퀀스 타입에 `!` 접미사를 붙이면 해당 시퀀스의 WARNING을 억제한다.

```go
// @delete! Room.DeleteAll()       — 0-arg 전체 삭제 WARNING 억제
// @get! Course course = ...       — stale 등 WARNING 억제
```

## 설계

### DSL 문법 확장

`@type!` — `!`는 "의도적"을 의미. WARNING만 억제하고 ERROR는 그대로 발생.

### 파서

`@delete!`를 파싱할 때 `!`를 감지하여 `Sequence.SuppressWarn = true` 설정 후 `!`를 제거하여 타입은 `"delete"`로 저장.

### 검증기

WARNING 발행 전 `seq.SuppressWarn`을 체크. `true`면 해당 WARNING을 건너뜀.

현재 WARNING 발생 지점:
- `validateStaleResponse` — put/delete 후 갱신 없이 response에 사용
- `validateReservedSourceConflict` — ERROR만, 해당 없음
- `validateCurrentUserType` — currentUser 사용 시 model/에 CurrentUser 타입 없음
- `validateRequiredFields` — @delete 0-arg
- `validateRequest` — OpenAPI request에 있지만 SSaC에서 미사용
- `validateResponse` — OpenAPI response에 있지만 SSaC @response에 없음

`SuppressWarn`은 해당 시퀀스의 WARNING만 억제. 함수 전체가 아님.

## 변경 파일

| 파일 | 내용 |
|---|---|
| `parser/types.go` | `Sequence`에 `SuppressWarn bool` 필드 추가 |
| `parser/parser.go` | `@type!` 파싱 시 `!` 감지, `SuppressWarn` 설정 |
| `parser/parser_test.go` | `!` 접미사 파싱 테스트 |
| `validator/validator.go` | WARNING 발행 전 `SuppressWarn` 체크 |
| `validator/validator_test.go` | `SuppressWarn` 억제 테스트 |
| `CLAUDE.md` | DSL 문법에 `!` 접미사 설명 추가 |

## 검증

```bash
go test ./parser/... ./validator/... ./generator/... -count=1
```

## 의존성

- Phase 006 (예약 소스, CurrentUser WARNING)
- 수정지시서v2/001 (@delete 0-arg WARNING)
