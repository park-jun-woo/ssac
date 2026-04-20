# pkg/cache

TTL 기반 키-값 캐시 유틸 — 성능을 위한 휘발성 데이터 저장.

## 개요

`pkg/cache`는 SSaC 생성 코드가 `@call cache.Get/Set/Delete` 시퀀스로 호출하는 캐시 유틸 패키지다.
`CacheModel` 인터페이스 뒤에 in-memory 맵 구현과 PostgreSQL 테이블 구현을 두 개 제공하며,
모든 값은 JSON으로 직렬화되어 문자열로 저장/반환된다. 서비스 부팅 시 `cache.Init(model)`로
기본 구현체를 주입하면 이후 패키지 레벨의 `Get/Set/Delete` 래퍼가 해당 모델을 사용한다.

## 모델/인터페이스

`CacheModel` (cache_model.go)

| 메서드 | 시그니처 | 설명 |
|---|---|---|
| Set | `Set(ctx, key string, value any, ttl time.Duration) error` | JSON 직렬화 후 저장. 기존 키는 덮어쓰기 |
| Get | `Get(ctx, key string) (string, error)` | 만료 전이면 JSON 문자열 반환, 미존재/만료면 `""` |
| Delete | `Delete(ctx, key string) error` | 키 제거 (존재하지 않아도 에러 없음) |

`Init(model CacheModel)`로 패키지 기본 모델을 주입한다 (default_model.go).

## 구현체

### memoryCache (new_memory_cache.go)

생성자: `NewMemoryCache() CacheModel`

- in-process `map[string]memoryEntry` + `sync.RWMutex`
- 만료 처리: `Get` 시 `time.Now().After(entry.expiresAt)` 비교해 만료되었으면 `""` 반환 (lazy expiration, 백그라운드 청소 없음)
- 재시작 시 데이터 소멸 — 개발/테스트 또는 단일 인스턴스 배포용

### postgresCache (new_postgres_cache.go)

생성자: `NewPostgresCache(ctx, db *sql.DB) (CacheModel, error)`

- `fullend_cache (key TEXT PK, value TEXT, expires_at TIMESTAMPTZ)` 테이블을 `CREATE TABLE IF NOT EXISTS`로 자동 생성
- 만료 처리: `SELECT ... WHERE expires_at > NOW()` — SQL이 직접 만료 필터링, `sql.ErrNoRows` 시 `""` 반환
- `Set`은 `INSERT ... ON CONFLICT (key) DO UPDATE` UPSERT
- 영속/다중 인스턴스 공유 저장소. 만료된 행은 자연 퇴출되지 않으므로 필요 시 배치 정리 별도

## 공개 API

| 함수 | Request | Response | 비고 |
|---|---|---|---|
| `Get(ctx, GetRequest)` | `{Key string}` | `{Value string}` | 미존재/만료 → `Value=""`, err=nil |
| `Set(ctx, SetRequest)` | `{Key, Value string; TTL int64}` | `{}` | TTL은 초 단위. 값은 문자열로 받아 JSON 인코딩 |
| `Delete(ctx, DeleteRequest)` | `{Key string}` | `{}` | 키 없어도 성공 |

에러 조건:
- Memory: 실질적 에러 없음 (마샬링 실패 시만)
- Postgres: 커넥션/쿼리 실패 시 `error` 반환. `sql.ErrNoRows`는 성공으로 흡수되어 `Value=""`로 치환

## 사용 예시

부팅 시 주입:

```go
// main.go
db, _ := sql.Open("postgres", dsn)
model, err := cache.NewPostgresCache(ctx, db)
if err != nil { log.Fatal(err) }
cache.Init(model)
```

SSaC DSL에서 @call 시퀀스:

```go
// @call cache.GetResponse cached = cache.Get({Key: request.Key})
// @empty cached.Value "cache miss"
// @response cached

// @call cache.Set({Key: request.Key, Value: payload.JSON, TTL: 300})
// @response ok
```

Go에서 직접 호출:

```go
resp, err := cache.Get(ctx, cache.GetRequest{Key: "user:42"})
_, err  = cache.Set(ctx, cache.SetRequest{Key: "user:42", Value: body, TTL: 600})
_, err  = cache.Delete(ctx, cache.DeleteRequest{Key: "user:42"})
```
