# pkg/session

TTL 기반 키-값 세션 저장소 — 로그인/장바구니 등 사용자 상태 보관.

## 개요

`pkg/session`은 SSaC 생성 코드가 `@call session.Get/Set/Delete` 시퀀스로 호출하는
세션 유틸 패키지다. 인터페이스·TTL·JSON 직렬화 구조는 `pkg/cache`와 동일하지만,
의미가 다르다: 캐시는 "성능 최적화용 휘발성", 세션은 "사용자에 귀속된 상태"를 다룬다.
이 때문에 생성자·테이블명·인터페이스가 별도로 분리되어 있으며, 두 저장소를 서로 다른
백엔드로 운영할 수 있다 (예: cache=Redis 대체 가능, session=Postgres 영속).

## 모델/인터페이스

`SessionModel` (session_model.go)

| 메서드 | 시그니처 | 설명 |
|---|---|---|
| Set | `Set(ctx, key string, value any, ttl time.Duration) error` | `value any` — 구조체/맵을 그대로 받아 JSON으로 직렬화 |
| Get | `Get(ctx, key string) (string, error)` | JSON 문자열 반환. 미존재/만료면 `""` |
| Delete | `Delete(ctx, key string) error` | 키 제거 |

`Init(model SessionModel)`로 패키지 기본 모델을 주입한다 (default_model.go).

`value any`로 받는 설계 덕분에 구조체/맵을 편하게 저장할 수 있고, 읽을 때는 JSON
문자열이 돌아오므로 호출 측에서 원하는 타입으로 `json.Unmarshal`한다.
`test_memory_session_struct_value_test.go`가 이 동작(맵 저장 → 비어있지 않은 JSON 문자열 조회)을 검증한다.

## 구현체

### memorySession (new_memory_session.go)

생성자: `NewMemorySession() SessionModel`

- in-process `map[string]memoryEntry` + `sync.RWMutex`
- 만료 처리: `Get` 시 `time.Now().After(expiresAt)` 비교 (lazy expiration)
- 재시작 시 데이터 소멸. 단일 프로세스 개발/테스트용

### postgresSession (new_postgres_session.go)

생성자: `NewPostgresSession(ctx, db *sql.DB) (SessionModel, error)`

- `fullend_sessions (key TEXT PK, value TEXT, expires_at TIMESTAMPTZ)` 테이블을 `CREATE TABLE IF NOT EXISTS`로 자동 생성
- 만료 처리: `SELECT ... WHERE expires_at > NOW()` SQL 조건으로 필터
- `Set`은 `INSERT ... ON CONFLICT (key) DO UPDATE` UPSERT
- 다중 인스턴스/영속 세션 저장소에 적합

## 공개 API

| 함수 | Request | Response | 비고 |
|---|---|---|---|
| `Get(ctx, GetRequest)` | `{Key string}` | `{Value string}` | 미존재/만료 → `Value=""`, err=nil |
| `Set(ctx, SetRequest)` | `{Key, Value string; TTL int64}` | `{}` | TTL은 초 단위 |
| `Delete(ctx, DeleteRequest)` | `{Key string}` | `{}` | 키 없어도 성공 |

공개 래퍼의 `SetRequest.Value`는 `string`이지만, 내부 `SessionModel.Set`은 `value any`를
받는다. Go 코드에서 구조체 세션을 직접 저장하려면 주입된 모델을 호출한다:
`sessionModel.Set(ctx, key, userStruct, ttl)`.

에러 조건:
- Memory: 실질적 에러 없음
- Postgres: 커넥션/쿼리 실패 시 `error` 반환. `sql.ErrNoRows`는 성공으로 흡수

## 사용 예시

부팅 시 주입:

```go
model, err := session.NewPostgresSession(ctx, db)
if err != nil { log.Fatal(err) }
session.Init(model)
```

SSaC DSL에서 @call 시퀀스:

```go
// @call session.GetResponse sess = session.Get({Key: request.SID})
// @empty sess.Value "session expired"
// @response sess

// @call session.Set({Key: request.SID, Value: token, TTL: 3600})
// @response ok
```

Go에서 직접 호출:

```go
resp, err := session.Get(ctx, session.GetRequest{Key: sid})
_, err  = session.Set(ctx, session.SetRequest{Key: sid, Value: tokenJSON, TTL: 3600})
_, err  = session.Delete(ctx, session.DeleteRequest{Key: sid})
```
