# 수정요구: DDLTable에 FK 관계 및 인덱스 정보 추가

## 요청자
fullend (github.com/park-jun-woo/fullend)

## 배경

fullend의 교차 검증에서 다음 규칙을 구현하려면 ssac의 DDLTable에 추가 정보가 필요하다:

1. **OpenAPI x-include ↔ DDL FK**: x-include.allowed 리소스가 실제 FK 관계로 연결된 테이블인지 검증
2. **OpenAPI x-sort ↔ DDL Index**: x-sort.allowed 컬럼에 인덱스가 있는지 성능 경고

현재 `parseDDLTables()` (symbol.go:436-439)에서 FOREIGN KEY, CREATE INDEX를 명시적으로 skip하고 있다.

## 요청 변경 사항

### 1. DDLTable 구조체 확장

```go
// 현재
type DDLTable struct {
    Columns map[string]string // snake_case 컬럼명 → Go 타입
}

// 요청
type DDLTable struct {
    Columns    map[string]string    // snake_case 컬럼명 → Go 타입
    ForeignKeys []ForeignKey        // FK 관계 목록
    Indexes    []Index              // 인덱스 목록
}

type ForeignKey struct {
    Column     string // 이 테이블의 컬럼 (e.g. "user_id")
    RefTable   string // 참조 테이블 (e.g. "users")
    RefColumn  string // 참조 컬럼 (e.g. "id")
}

type Index struct {
    Name    string   // 인덱스 이름 (e.g. "idx_reservations_room_time")
    Columns []string // 인덱스 컬럼 목록 (e.g. ["room_id", "start_at", "end_at"])
}
```

### 2. parseDDLTables 수정

현재 skip하는 두 가지를 파싱:

**인라인 FK (컬럼 정의 내)**
```sql
user_id BIGINT NOT NULL REFERENCES users(id)
```
→ 컬럼 파싱 시 `REFERENCES` 키워드 감지 → ForeignKey 추가

**독립 FK (CONSTRAINT 절)**
```sql
CONSTRAINT fk_user FOREIGN KEY (user_id) REFERENCES users(id)
```
→ FOREIGN KEY 라인 파싱 → ForeignKey 추가

**CREATE INDEX 문**
```sql
CREATE INDEX idx_reservations_room_time ON reservations (room_id, start_at, end_at);
```
→ DDL 파일 내 CREATE INDEX 파싱 → 해당 테이블의 Indexes에 추가

### 3. 기존 동작 영향 없음

- Columns 맵은 그대로 유지
- LoadSymbolTable() 반환 타입 변경 없음
- ValidateWithSymbols()는 FK/Index를 사용하지 않으므로 영향 없음
- 새 필드는 fullend의 crosscheck에서만 참조

## 대상 파일

| 파일 | 변경 |
|---|---|
| `validator/symbol.go` | DDLTable, ForeignKey, Index 타입 추가 |
| `validator/symbol.go` | parseDDLTables() 내 FK/INDEX 파싱 로직 추가 |

## 검증 방법

기존 dummy-study DDL로 확인:

```sql
-- reservations.sql
user_id BIGINT NOT NULL REFERENCES users(id)    → FK{Column:"user_id", RefTable:"users", RefColumn:"id"}
room_id BIGINT NOT NULL REFERENCES rooms(id)    → FK{Column:"room_id", RefTable:"rooms", RefColumn:"id"}

CREATE INDEX idx_reservations_room_time ON reservations (room_id, start_at, end_at);
→ Index{Name:"idx_reservations_room_time", Columns:["room_id","start_at","end_at"]}

CREATE INDEX idx_reservations_user ON reservations (user_id);
→ Index{Name:"idx_reservations_user", Columns:["user_id"]}
```

- `go test ./...` 통과
- 기존 ssac validate 동작 변화 없음
