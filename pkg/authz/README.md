# pkg/authz

OPA(Open Policy Agent) 정책 기반 인가 검사 패키지.

## 개요

`@auth` 시퀀스의 런타임 구현. 애플리케이션 부팅 시 OPA Rego 정책 파일과 `OwnershipMapping` 목록을 프로세스 전역에 로드한 뒤, 핸들러마다 `Check`를 호출해 action/resource/claims 기반 인가를 평가한다. 검사 직전에 DB에서 리소스 소유자 정보(`data.owners`)를 조회해 OPA 입력에 주입하므로, 정책은 "이 리소스의 owner == claims.user_id" 같은 조건을 선언적으로 쓸 수 있다. SSaC 코드젠은 `@auth "action" "resource" {...}` 시퀀스를 `authz.Check(authz.CheckRequest{...})` 호출(403 에러)로 변환한다.

## 초기화 흐름

1. 앱 시작 시 `authz.Init(db, ownerships)` 1회 호출.
2. `Init`은 `OPA_POLICY_PATH` 환경변수의 Rego 정책 파일을 읽어 `globalPolicy`에, `*sql.DB`를 `globalDB`에, 매핑 목록을 `globalOwnerships`에 저장한다.
3. `DISABLE_AUTHZ=1`이면 정책 로딩을 스킵한다(테스트/로컬 개발용).
4. 요청 핸들러는 `Check`를 호출 → `loadOwners`가 `globalOwnerships`를 순회하며 각 매핑에 대해 `SELECT <Column> FROM <Table> WHERE id = $1`을 실행해 `map[resource]map[id]ownerID`를 빌드 → OPA `data.owners`로 주입 → `data.authz.allow` 쿼리 평가.

## 공개 API

### Init

글로벌 인가 상태(정책, DB, ownership 매핑)를 초기화한다.

시그니처: `func Init(db *sql.DB, ownerships []OwnershipMapping) error`

동작:
- `DISABLE_AUTHZ=1` → 정책 로딩 생략 후 nil 반환.
- `OPA_POLICY_PATH` 미설정 → 에러.
- 정책 파일 읽기 실패 → 에러.

### Check

OPA 정책을 평가해 인가를 검사한다.

Request (`CheckRequest`):

| 필드 | 타입 | 설명 |
|---|---|---|
| Ctx | context.Context | OPA 평가와 DB 조회에 전파되는 요청 컨텍스트. nil이면 `context.Background()` 폴백 |
| Tx | *sql.Tx | (선택) ownership 조회를 실행할 트랜잭션. 비nil이면 같은 핸들러에서 앞서 insert/update한 행이 보임(MVCC snapshot 일관성). nil이면 `globalDB` 사용 |
| Action | string | OPA input.action — 수행할 동작 이름 |
| Resource | string | OPA input.resource — 리소스 종류 |
| ResourceID | int64 | OPA input.resource_id — 리소스 PK. ownership 조회의 `WHERE id = $1` |
| Claim | any | OPA input.claims로 전달될 임의 구조체(JSON 태그로 rego 키 제어). nil이면 빈 map으로 정규화 |

Response (`CheckResponse`): 비어있음.

에러 조건:
- `globalPolicy == ""` (Init 미호출) → `authz not initialized`
- ownership DB 조회 실패 → `load owners: <wrapped>`
- OPA 평가 실패 → `OPA eval failed: <wrapped>`
- `data.authz.allow`가 없거나 false → `forbidden` (코드젠 계층에서 403 매핑)

### OwnershipMapping

`@ownership` 주석에서 파생된 리소스-테이블 매핑 구조체.

| 필드 | 타입 | 예 |
|---|---|---|
| Resource | string | `"gig"`, `"proposal"` |
| Table | string | `"gigs"`, `"proposals"` |
| Column | string | `"client_id"`, `"freelancer_id"` |

## 사용 예시

SSaC 시퀀스에서의 `@auth`:

```go
// 제안 수락: currentUser가 gig의 client_id 소유자인지 확인
// @auth "AcceptProposal" "gig" {ResourceID: request.GigID} "gig을 수락할 권한이 없습니다"
// @put Proposal.Accept({ID: request.ProposalID})
// @response!
```

생성되는 Go 코드(개요):

```go
if _, err := authz.Check(authz.CheckRequest{
    Ctx:        c.Request.Context(),
    Tx:         tx,                   // 트랜잭션 핸들러에서는 자동 전달
    Action:     "AcceptProposal",
    Resource:   "gig",
    ResourceID: gigID,
    Claim:      currentUser,          // JSON 태그로 rego claims 매핑
}); err != nil {
    c.JSON(403, gin.H{"error": "gig을 수락할 권한이 없습니다"})
    return
}
```

앱 부팅:

```go
if err := authz.Init(db, []authz.OwnershipMapping{
    {Resource: "gig",      Table: "gigs",      Column: "client_id"},
    {Resource: "proposal", Table: "proposals", Column: "freelancer_id"},
}); err != nil {
    log.Fatal(err)
}
```

대응하는 Rego 정책:

```rego
package authz

default allow := false

allow if {
    input.action == "AcceptProposal"
    input.resource == "gig"
    data.owners.gig[sprintf("%d", [input.resource_id])] == input.claims.user_id
}
```

## 외부 의존성

- `github.com/open-policy-agent/opa/v1/rego` — 정책 평가
- `github.com/open-policy-agent/opa/v1/storage/inmem` — `data.owners` 인메모리 스토어
- `database/sql` — ownership 조회 (`*sql.DB`, `*sql.Tx` 양쪽 지원)
