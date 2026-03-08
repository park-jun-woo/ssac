# SSaC — AI Compact Reference

## CLI

```
ssac parse [dir]              # 주석 파싱 결과 출력 (기본: specs/backend/service/)
ssac validate [dir]           # 내부 검증 또는 외부 SSOT 교차 검증 (자동 감지)
ssac gen <service-dir> <out>  # validate → codegen → gofmt (심볼 테이블 있으면 타입 변환 + 모델 인터페이스 생성)
```

## 기술 스택

Go 1.24+, `go/ast`(파싱), `text/template`(코드젠), `gopkg.in/yaml.v3`(OpenAPI)

## DSL 문법

```go
// @sequence <type>        — 블록 시작. 10종: authorize|get|guard nil|guard exists|post|put|delete|password|call|response
// @model <Model.Method>   — 리소스 모델.메서드 (get/post/put/delete)
// @param <Name> <source> [-> column]  — source: request, currentUser, 변수명, "리터럴". -> column: 명시적 DDL 컬럼 매핑
// @result <var> <Type>    — 결과 바인딩 (get/post 필수, call 선택)
// @message "msg"          — 커스텀 에러 메시지 (선택, 기본값 자동생성)
// @var <name>             — response에서 반환할 변수
// @action @resource @id   — authorize 전용 (3개 모두 필수)
// @component | @func      — call 전용 (택일 필수)
```

타입별 필수 태그:

| 타입 | 필수 |
|---|---|
| authorize | @action, @resource, @id |
| get, post | @model, @result |
| put, delete | @model |
| guard nil/exists | target (sequence 라인에 변수명) |
| password | @param 2개 (hash, plain) |
| call | @component 또는 @func (택일) |
| response | (없음, @var는 선택) |

## 디렉토리

```
cmd/ssac/main.go                 # CLI 진입점
parser/                          # 주석 → []ServiceFunc
validator/                       # 내부 + 외부 SSOT 검증
generator/                       # Target 인터페이스 기반 코드젠 (다중 언어 확장 가능)
  target.go                      #   Target 인터페이스 + DefaultTarget()
  go_target.go                   #   GoTarget: Go 코드 생성 구현
  go_templates.go                #   Go 템플릿
  generator.go                   #   하위 호환 래퍼 (Generate, GenerateWith) + 유틸
specs/                           # 선언 (입력, SSOT)
  dummy-study/                   #   스터디룸 예약 더미 프로젝트
    service/  db/queries/  api/  model/
  plans/                         #   구현 계획서
artifacts/                       # 문서
  manual-for-human.md            #   상세 매뉴얼
  manual-for-ai.md               #   컴팩트 레퍼런스
testdata/                        # 테스트 fixture
files/                           # 기초 자료
```

## 외부 검증 프로젝트 구조

`ssac validate <project-root>` 시 자동 감지:
- `<root>/service/*.go` — sequence spec
- `<root>/db/*.sql` — DDL (CREATE TABLE → 컬럼 타입)
- `<root>/db/queries/*.sql` — sqlc 쿼리 (파일명→모델, `-- name: Method :cardinality`)
- `<root>/api/openapi.yaml` — OpenAPI 3.0 (operationId=함수명, x-pagination/sort/filter/include)
- `<root>/model/*.go` — Go interface→component, func→@func, `// @dto`→DDL 테이블 없는 DTO 등록

## 코드젠 기능

심볼 테이블(외부 SSOT)이 있을 때 추가되는 기능:

- **타입 변환**: DDL 컬럼 타입 기반 request 파라미터 변환 (int64→`strconv.ParseInt`, time.Time→`time.Parse`, 실패 시 400 early return)
- **`-> column` 매핑**: `@param PaymentMethod request -> method` — 자동 변환 대신 명시적 DDL 컬럼 매핑
- **Guard 값 타입**: result 타입에 따른 zero value 비교 (int→`== 0`/`> 0`, pointer→`== nil`/`!= nil`)
- **currentUser/config source**: `@param Name currentUser` → `currentUser.Name`
- **Stale 데이터 경고**: put/delete 후 갱신 없이 response 사용 시 WARNING
- **@dto 태그**: `// @dto` 주석이 달린 struct → DDL 테이블 매칭 건너뜀
- **DDL FK/Index 파싱**: REFERENCES(인라인/독립), CREATE INDEX → `DDLTable.ForeignKeys`, `DDLTable.Indexes`
- **모델 인터페이스 파생**: 3 SSOT 교차 → `<outDir>/model/models_gen.go`
  - sqlc: 메서드명, 카디널리티 (:one→`*T`, :many→`[]T`, :exec→`error`)
  - SSaC: 비즈니스 파라미터 (실제 사용된 메서드만 포함)
  - OpenAPI x-: 인프라 파라미터 (x-pagination → `opts QueryOpts` 추가)

단수화 규칙 (sqlc 파일명 → 모델명): `ies`→`y`, `sses`→`ss`, `xes`→`x`, 나머지 `s` 제거

## OpenAPI x- 확장

OpenAPI 엔드포인트에 인프라 파라미터를 선언한다. SSaC spec에는 비즈니스 파라미터만 선언하고, 인프라 파라미터는 x-에만 선언한다.

```yaml
/api/reservations:
  get:
    operationId: ListReservations
    x-pagination:                    # 페이지네이션
      style: offset                  # offset | cursor
      defaultLimit: 20
      maxLimit: 100
    x-sort:                          # 정렬
      allowed: [start_at, created_at]
      default: start_at
      direction: desc                # asc | desc
    x-filter:                        # 필터
      allowed: [status, room_id]
    x-include:                       # 관계 포함
      allowed: [room, user]
```

코드젠 영향:
- x- 있는 operation의 모델 메서드에 `opts QueryOpts` 파라미터 추가
- `:many` + x-pagination → 반환 타입에 total count 포함: `([]T, int, error)`
- `QueryOpts` struct 자동 생성 (Limit, Offset, Cursor, SortCol, SortDir, Filters, Includes)

## Coding Conventions

- gofmt 준수, 에러 즉시 처리 (early return)
- 파일명: snake_case, 변수/함수: camelCase, 타입: PascalCase
- 테스트: `go test ./parser/... ./validator/... ./generator/... -count=1`
