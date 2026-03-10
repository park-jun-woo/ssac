# ssac

## 프로젝트 루트
~/.clari/repos/ssac

## 프로젝트 개요
Service Sequences as Code — Go 주석 기반 선언적 서비스 로직을 파싱하여 Go+gin 구현 코드를 생성하는 CLI 도구.

## CLI

```
ssac parse [dir]              # 주석 파싱 결과 출력 (기본: specs/backend/service/)
ssac validate [dir]           # 내부 검증 또는 외부 SSOT 교차 검증 (자동 감지)
ssac gen <service-dir> <out>  # validate → codegen → gofmt (심볼 테이블 있으면 타입 변환 + 모델 인터페이스 생성)
```

## 계획 작성 원칙

구현 전 `specs/plans/`에 계획 md를 작성한다.

- 파일명: `PhaseNNN-TITLE.md` (예: `Phase001-CLISkeleton.md`)
- 구현 코드를 쓰기 전에 계획을 먼저 작성하고 승인을 받는다
- 계획에는 다음을 포함한다:
  - 목표: 무엇을 만드는가
  - 변경 파일 목록: 어떤 파일을 생성/수정하는가
  - 의존성: 외부 패키지, 형제 프로젝트 API
  - 검증 방법: 어떻게 확인하는가
- 계획이 승인되면 구현하고, 완료 후 계획 상단에 `✅ 완료` 표시

## 기술 스택

- Go 1.24+, module: `github.com/geul-org/ssac`
- 파싱: `go/ast`, `go/parser`
- 코드젠: `text/template`, `go/format`
- 생성 코드 타겟: `github.com/gin-gonic/gin`
- 외부 의존성: `gopkg.in/yaml.v3` (OpenAPI 파싱)

## DSL 문법 (v2 — 한 줄 표현식)

```go
// @get Type var = Model.Method(args...)          — 리소스 조회 (result 필수)
// @post Type var = Model.Method(args...)         — 리소스 생성 (result 필수)
// @put Model.Method(args...)                     — 리소스 수정 (result 없음)
// @delete Model.Method(args...)                  — 리소스 삭제 (result 없음)
// @empty target "message"                        — nil이면 종료 (404)
// @exists target "message"                       — 존재하면 종료 (409)
// @state diagramID {inputs} "transition" "msg"   — 상태 전이 검사 (409)
// @auth "action" "resource" {inputs} "message"   — 권한 검사 (403)
// @call Type var = pkg.Func(args...)             — 외부 함수 호출 (result 있음/없음)
// @response { field: var, field: var.Member }    — 응답 (멀티라인 블록)
// @type! — 모든 시퀀스에 ! 접미사로 WARNING 억제 (e.g. @delete!, @response!)
```

Args 형식: `source.Field` 또는 `"literal"`
- `request.CourseID` — HTTP 요청 파라미터 (예약 소스)
- `course.InstructorID` — 이전 결과 변수의 필드
- `currentUser.ID` — 인증 컨텍스트 (예약 소스)
- `config.APIKey` — 환경 설정 (예약 소스)
- `"cancelled"` — 문자열 리터럴

예약 소스 (Reserved Sources): `request`, `currentUser`, `config`, `query`
- 사용자가 선언하지 않는 특수 소스. result 변수명으로 사용 불가 (validator ERROR)
- `request.Field` → 코드젠에서 `lcFirst(Field)` 로컬 변수로 치환
- `currentUser.Field` → `currentUser.Field` 실제 변수 유지
- `config.Field` → `config.Field` 실제 변수 유지
- `query` → 코드젠에서 `opts` (QueryOpts) 변수로 변환. OpenAPI x-extensions와 교차 검증

타입별 필수 요소:

| 타입 | 필수 |
|---|---|
| get | Model, Result (Args 선택) |
| post | Model, Result, Args |
| put | Model, Args |
| delete | Model, Args (0-arg WARNING, `@delete!`로 억제) |
| empty, exists | Target, Message |
| state | DiagramID, Inputs, Transition, Message |
| auth | Action, Resource, Message |
| call | Model (pkg.Func 형식) |
| response | (없음, Fields 선택) |

## 디렉토리

```
cmd/ssac/main.go                 # CLI 진입점
parser/                          # 주석 → []ServiceFunc
  types.go                       #   IR 구조체 (ServiceFunc, Sequence, Arg, Result)
  parser.go                      #   한 줄 표현식 파서
validator/                       # 내부 + 외부 SSOT 검증
  validator.go                   #   검증 규칙
  symbol.go                      #   심볼 테이블 (DDL, OpenAPI, sqlc, model)
  errors.go                      #   ValidationError
generator/                       # Target 인터페이스 기반 코드젠 (다중 언어 확장 가능)
  target.go                      #   Target 인터페이스 + DefaultTarget()
  go_target.go                   #   GoTarget: Go+gin 코드 생성 구현
  go_templates.go                #   Go+gin 템플릿
  generator.go                   #   하위 호환 래퍼 (Generate, GenerateWith) + 유틸
specs/                           # 선언 (입력, SSOT)
  dummy-study/                   #   스터디룸 예약 더미 프로젝트
    service/  db/queries/  api/  model/
  plans/                         #   구현 계획서
artifacts/                       # 문서
  manual-for-human.md            #   상세 매뉴얼 (인간용)
  manual-for-ai.md               #   컴팩트 레퍼런스 (AI용)
testdata/                        # 테스트 fixture
v1/                              # 아카이브된 v1 코드 (참조용, 삭제 금지)
files/                           # 기초 자료
  SSaC.md                        #   기획서
```

## 외부 검증 프로젝트 구조

`ssac validate <project-root>` 시 자동 감지:
- `<root>/service/**/*.go` — sequence spec (재귀 탐색, 도메인 폴더 지원)
- `<root>/db/*.sql` — DDL (CREATE TABLE → 컬럼 타입, FK, Index)
- `<root>/db/queries/*.sql` — sqlc 쿼리 (파일명→모델, `-- name: Method :cardinality`)
- `<root>/api/openapi.yaml` — OpenAPI 3.0 (operationId=함수명, x-pagination/sort/filter/include)
- `<root>/model/*.go` — Go interface→model, `// @dto`→DDL 없는 DTO. @call은 외부 패키지이므로 교차검증 스킵

## 코드젠 기능

생성 코드는 gin 프레임워크 사용:
- 함수 시그니처: `func Name(c *gin.Context)`
- Path params: `c.Param()` + 타입 변환
- Request body: `c.ShouldBindJSON(&req)` (2+ request 파라미터) 또는 `c.Query()`
- currentUser: `c.MustGet("currentUser").(*model.CurrentUser)` — @auth 또는 args에서 currentUser 참조 시 자동 생성

심볼 테이블(외부 SSOT)이 있을 때 추가되는 기능:

- **타입 변환 코드젠**: DDL 컬럼 타입 기반 request 파라미터 변환 (int64→`strconv.ParseInt`, time.Time→`time.Parse`, 400 early return)
- **Guard 값 타입**: result 타입에 따른 zero value 비교 (int→`== 0`/`> 0`, pointer→`== nil`/`!= nil`)
- **Stale 데이터 경고**: put/delete 후 갱신 없이 response에 사용하면 WARNING
- **QueryOpts**: SSaC에 `query` 예약 소스가 명시된 메서드에만 `opts QueryOpts` 전달 (암묵적 삽입 없음)
- **List 3-tuple 반환**: many + QueryOpts → `result, total, err :=` (count 포함)
- **모델 인터페이스 파생**: 3 SSOT 교차(sqlc 카디널리티 + SSaC Args + OpenAPI x-확장) → `<outDir>/model/models_gen.go`
- **도메인 폴더 구조**: `service/auth/login.go` → `Domain="auth"` → `outDir/auth/login.go`, `package auth`
- **@call 코드젠**: `@call pkg.Func(args)` → `pkg.Func(pkg.FuncRequest{args...})` (unkeyed positional). result 없음→`_, err` guard형(401), 있음→value형(500)
- **@state 코드젠**: `{id}state.CanTransition({id}state.Input{...}, "transition")`, import `"states/{id}state"`
- **@auth 코드젠**: `authz.Check(currentUser, "action", "resource", authz.Input{...})`
- **Spec 파일 imports**: spec 파일의 Go import 선언이 생성 코드에 전달됨

## 더미 프로젝트

ssac 자체 더미: `specs/dummy-study/` (내부 테스트용, 외부 검증 프로젝트 구조)

fullend 더미 (SSaC 소비자, 통합 검증용):

| 프로젝트 | SSOT (specs) | 생성 산출물 (artifacts) |
|---|---|---|
| dummy-study | `~/.clari/repos/fullend/specs/dummy-study/` | `~/.clari/repos/fullend/artifacts/dummy-study/` |
| dummy-lesson | `~/.clari/repos/fullend/specs/dummy-lesson/` | `~/.clari/repos/fullend/artifacts/dummy-lesson/` |

각 더미 프로젝트 구조:
- `specs/<project>/frontend/*.html` — STML 페이지
- `specs/<project>/frontend/components/*.tsx` — 커스텀 컴포넌트
- `specs/<project>/api/openapi.yaml` — OpenAPI 스펙
- `artifacts/<project>/frontend/src/*.tsx` — 생성된 React 페이지
- `artifacts/<project>/backend/` — 생성된 Go 백엔드

## Coding Conventions

- gofmt 준수, 에러 즉시 처리 (early return)
- 파일명: snake_case, 변수/함수: camelCase, 타입: PascalCase
- 테스트: `go test ./parser/... ./validator/... ./generator/... -count=1`
- 테스트용 fixture는 `testdata/`에 배치. `/tmp` 등 외부 경로 사용 금지.

## Git 규칙

- Co-Authored-By 금지. 커밋 메시지에 AI 이름을 절대 포함하지 않는다.
- remote: `https://github.com/geul-org/ssac.git`
- 라이선스: MIT
