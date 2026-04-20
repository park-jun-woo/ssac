# pkg/queue

`@publish` / `@subscribe` 시퀀스 런타임. Memory 또는 Postgres 백엔드 기반 메시지 큐.

## 개요

SSaC에서 `@publish "topic" {k:v}`와 `@subscribe "topic"` 시퀀스의 코드젠 타겟 런타임. `Init()`으로 백엔드를 선택하고(`"memory"` 또는 `"postgres"`), `Subscribe()`로 토픽 핸들러를 등록한 뒤 `Start()`로 처리를 시작한다. Memory 백엔드는 `Publish` 호출 시 같은 프로세스 핸들러를 동기 호출하고, Postgres 백엔드는 `fullend_queue` 테이블에 INSERT 후 1초 간격 폴링 루프가 `FOR UPDATE SKIP LOCKED`로 대기 메시지를 꺼내 핸들러에 디스패치한다. 전역 싱글톤 상태를 사용하므로 테스트는 `resetQueue()` 헬퍼로 초기화한다.

## 백엔드 비교

| 항목 | memory | postgres |
|---|---|---|
| 초기화 | `Init(ctx, "memory", nil)` | `Init(ctx, "postgres", db)` (fullend_queue 테이블 auto-create) |
| Publish | 핸들러 동기 호출 | `fullend_queue` INSERT |
| Delivery | 즉시, 프로세스 내부 | 1초 폴링, 여러 프로세스 가능 (`FOR UPDATE SKIP LOCKED`) |
| Delay | 무시됨(즉시 실행) | `deliver_at` 컬럼으로 지연 |
| Priority | 무시됨 | `priority` 컬럼으로 정렬(high→normal→low) |
| 영속성 | 없음 | DB 저장, `status`/`processed_at` 추적 |

## 공개 API

### `Init(ctx context.Context, backend string, db *sql.DB) error`

백엔드 초기화. `backend`는 `"memory"` 또는 `"postgres"`. Postgres는 `db` 필수이며 `fullend_queue` 테이블과 pending 인덱스를 auto-create 한다.

### `Subscribe(topic string, handler func(ctx context.Context, msg []byte) error)`

토픽 핸들러 등록. 동일 토픽에 여러 핸들러를 순차 호출한다. 핸들러가 error를 반환하면 해당 메시지는 `status='failed'`로 마킹된다(postgres).

### `Publish(ctx context.Context, topic string, payload any, opts ...PublishOption) error`

JSON marshal 후 백엔드로 발행. 초기화 전 호출 시 `ErrNotInitialized` 반환. 구독자 없는 토픽에 발행해도 에러 아님.

### `Start(ctx context.Context) error`

메시지 처리 시작. Memory는 ctx 취소까지 블록만 함(실제 디스패치는 Publish 시점). Postgres는 1초 간격 폴링 루프 실행. ctx 취소 시 정상 종료.

### `Close() error`

폴링 루프 중지 후 전역 상태 해제. Start가 반환할 때까지 대기.

### Publish Options

| 함수 | 설명 |
|---|---|
| `WithDelay(seconds int) PublishOption` | 지연 전달 시간(초). postgres만 적용, memory는 즉시 실행 |
| `WithPriority(p string) PublishOption` | 우선순위 `"high"` / `"normal"`(기본) / `"low"`. postgres만 정렬에 반영 |

### 에러

| 이름 | 조건 |
|---|---|
| `ErrNotInitialized` | `Init` 이전에 `Publish` 호출 |
| `ErrUnknownBackend` | 지원하지 않는 백엔드 문자열 |

### DB 스키마 (postgres)

```sql
CREATE TABLE fullend_queue (
    id           BIGSERIAL PRIMARY KEY,
    topic        TEXT NOT NULL,
    payload      JSONB NOT NULL,
    priority     TEXT NOT NULL DEFAULT 'normal',
    status       TEXT NOT NULL DEFAULT 'pending',  -- pending/done/failed
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deliver_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processed_at TIMESTAMPTZ
);
CREATE INDEX idx_fullend_queue_pending
  ON fullend_queue (topic, status, deliver_at) WHERE status = 'pending';
```

## 사용 예시

### SSaC `@publish` (HTTP 함수 내)

```go
// @publish "user.registered" {UserID: user.ID, Email: user.Email}
// @publish "email.send"      {To: user.Email} {delay: 1800, priority: "high"}
```

코드젠 결과:

```go
if err := queue.Publish(c.Request.Context(), "user.registered", struct {
    UserID int64
    Email  string
}{UserID: user.ID, Email: user.Email}); err != nil { /* 500 */ }

if err := queue.Publish(c.Request.Context(), "email.send", struct {
    To string
}{To: user.Email}, queue.WithDelay(1800), queue.WithPriority("high")); err != nil { /* 500 */ }
```

### SSaC `@subscribe`

```ssac
// Message payload struct
type UserRegistered struct {
    UserID int64
    Email  string
}

// @subscribe "user.registered"
// @call _ = mail.SendEmail({To: message.Email, Subject: "환영"})
func OnUserRegistered(ctx context.Context, message UserRegistered) error { ... }
```

`message` 예약 소스로 큐 페이로드 필드 접근, 에러 반환 시 `return fmt.Errorf(...)`, 성공 시 `return nil`.

### Go 직접 사용

```go
import (
    "context"
    "github.com/park-jun-woo/ssac/pkg/queue"
)

ctx := context.Background()
queue.Init(ctx, "postgres", db)
defer queue.Close()

queue.Subscribe("user.registered", func(ctx context.Context, msg []byte) error {
    var ev struct{ UserID int64; Email string }
    json.Unmarshal(msg, &ev)
    return handleRegistered(ev)
})

go queue.Start(ctx)  // postgres 폴링 시작

queue.Publish(ctx, "user.registered",
    map[string]any{"UserID": 42, "Email": "a@b.com"},
    queue.WithDelay(60),
    queue.WithPriority("high"),
)
```
