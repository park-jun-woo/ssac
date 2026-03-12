✅ 완료

# Phase027: @auth Role 자동 추가 + validator 자체 점검

## 목표

1. **@auth 코드젠 — Role 자동 추가**: `@auth` 시퀀스에서 currentUser를 참조하면 `Role: currentUser.Role` 필드를 자동 생성
2. **validator 자체 점검**: 수정지시서 Part 2(inputs 변수 참조 검증)는 이미 구현됨 확인. 추가 누락 검증 규칙 식별 및 보완

## 수정지시서 Part 2 — 이미 구현됨

`validateVariableFlow()` (validator.go:195-208)에서 모든 시퀀스의 Inputs 값을 순회하며 미선언 변수 참조를 ERROR 처리한다. 테스트도 존재 (`TestValidateUndeclaredInInputs`). 추가 작업 없음.

## Part 1: @auth 코드젠 — Role 자동 추가

### 현재 동작

```go
authz.Check(authz.CheckRequest{Action: "PublishGig", Resource: "gig", ResourceID: gig.ID, UserID: currentUser.ID})
```

### 기대 동작

```go
authz.Check(authz.CheckRequest{Action: "PublishGig", Resource: "gig", ResourceID: gig.ID, Role: currentUser.Role, UserID: currentUser.ID})
```

### 변경 파일

#### `generator/go_target.go` — `buildInputFieldsFromMap` 호출 전 Role 삽입

`buildSequenceData()` 함수에서 `seq.Type == parser.SeqAuth`이고 Inputs에 `currentUser.*` 참조가 있으면, `Role` 키가 없을 때 `Role: currentUser.Role`을 Inputs에 자동 추가한다.

**주의**: 원본 `seq.Inputs`를 수정하면 안 되므로, 복사본을 만들어서 Role을 추가한 뒤 `buildInputFieldsFromMap`에 전달한다.

**조건**:
- `seq.Type == parser.SeqAuth`
- Inputs 값 중 `currentUser.`로 시작하는 항목이 1개 이상
- Inputs에 `Role` 키가 아직 없음

**추가**: `inputs["Role"] = "currentUser.Role"`

#### `generator/go_templates.go`

변경 없음. `{{.InputFields}}`에 Role이 포함되어 나옴.

### 테스트 변경 (`generator/generator_test.go`)

- `TestGenerateAuthCallStyle` (line 686): `assertContains(t, code, "Role: currentUser.Role")` 추가
- `TestGenerateAuth` (line 105): inputs에 `currentUser.*` 없으므로 Role 미생성 확인 — 기존 동작 유지
- `TestGenerateAuthNoCurrentUser` (line 702): `assertNotContains(t, code, "Role:")` 추가 (currentUser 없으면 Role도 없음)

## Part 3: validator 자체 점검 — 추가 누락 규칙

코드와 DSL 스펙을 대조한 결과, 아래 검증이 누락되어 있다:

### 3-1. `@response` Target(간단쓰기) 변수 존재 검증

**현재**: `validateVariableFlow`의 `seq.Target` 검증이 모든 시퀀스 타입에 적용되므로 `@response varName`의 변수 미선언도 이미 잡힘. → **추가 작업 없음**

### 3-2. `@publish` 에서 `query` 사용 검증

**현재 갭**: `@publish` Inputs에 `query`를 사용해도 에러가 안 남. `query`는 HTTP QueryOpts 전용이므로 `@publish`에서는 무의미.

**수정**: `validateVariableFlow`에서 `@publish` 시퀀스의 Inputs에 `val == "query"` 또는 `strings.HasPrefix(val, "query.")` 이면 ERROR.

**수정 위치**: `validator/validator.go` — `validateVariableFlow` Inputs 검증 루프 내

### 3-3. subscribe 함수에서 `query` 사용 검증

**현재 갭**: subscribe 함수의 어떤 시퀀스에서든 `query`를 사용해도 에러 없음. subscribe에는 HTTP query string이 없으므로 무의미.

**수정**: `validateSubscribeRules`에서 subscribe 함수 내 `query` 사용 시 ERROR.

**수정 위치**: `validator/validator.go` — `validateSubscribeRules` 함수 내

### 3-4. result 변수명 중복 선언 WARNING

**현재 갭**: 동일 함수 내에서 두 시퀀스가 같은 result 변수명을 쓰면 silent override. 의도적 재할당도 있지만 실수일 가능성이 높다.

**판단**: 코드젠에서 `ReAssign` 플래그로 처리 중이므로 유효한 패턴. → **추가 작업 없음** (오탐 위험)

### 요약

| 항목 | 조치 |
|---|---|
| Part 1: @auth Role 자동 추가 | 구현 |
| Part 2: inputs 변수 참조 검증 | 이미 구현됨, 스킵 |
| 3-2: @publish query 사용 금지 | 구현 |
| 3-3: subscribe query 사용 금지 | 구현 |

## 변경 파일 목록

| 파일 | 변경 |
|---|---|
| `generator/go_target.go` | @auth Role 자동 삽입 로직 |
| `generator/generator_test.go` | Role 생성/미생성 테스트 |
| `validator/validator.go` | @publish/subscribe query 사용 검증 |
| `validator/validator_test.go` | query 사용 금지 테스트 |

## 검증

```bash
go test ./generator/... ./validator/... -count=1
```
