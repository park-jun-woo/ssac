package parser

// ServiceFunc는 파일 단위 파싱 결과다.
type ServiceFunc struct {
	Name      string     // 함수명 (e.g. "CreateSession")
	FileName  string     // 원본 파일명 (e.g. "create_session.go")
	Domain    string     // 도메인 폴더명 (e.g. "auth"). 빈 문자열이면 루트.
	Imports   []string   // spec 파일의 import 경로 ("net/http" 제외)
	Sequences []Sequence // 순서 보존된 sequence 리스트
}

// Sequence는 개별 sequence 블록이다.
type Sequence struct {
	Type    string   // authorize, get, guard nil, guard exists, guard state, post, put, delete, call, response
	Model   string   // @model 값 (e.g. "Project.FindByID")
	Params  []Param  // @param 리스트
	Result  *Result  // @result (없을 수 있음)
	Message string   // @message (없으면 빈 문자열)
	Target  string   // guard 대상 변수 (e.g. "project")
	Vars    []string // @var 리스트 (response용)
	// authorize 전용
	Action   string // @action
	Resource string // @resource
	ID       string // @id
	// call 전용
	Func    string // @func (funcName only, e.g. "hashPassword")
	Package string // @func package (e.g. "auth")
}

// Param은 @param 태그의 파싱 결과다.
type Param struct {
	Name   string // 파라미터명 (e.g. "ProjectID")
	Source string // 소스 (e.g. "request", 변수명, 리터럴)
	Column string // 명시적 DDL 컬럼 매핑 (e.g. "method"), 비어있으면 자동 추론
}

// Result는 @result 태그의 파싱 결과다.
type Result struct {
	Var   string // 변수명 (e.g. "project")
	Type  string // 타입명 (e.g. "Project")
	Field string // Response struct 필드명 (e.g. "AccessToken"). 비어있으면 ucFirst(Var) 사용.
}

// sequence 타입 상수
const (
	SeqAuthorize   = "authorize"
	SeqGet         = "get"
	SeqGuardNil    = "guard nil"
	SeqGuardExists = "guard exists"
	SeqPost        = "post"
	SeqPut         = "put"
	SeqDelete      = "delete"
	SeqCall        = "call"
	SeqGuardState  = "guard state"
	SeqResponse    = "response"
)

var ValidSequenceTypes = map[string]bool{
	SeqAuthorize:   true,
	SeqGet:         true,
	SeqGuardNil:    true,
	SeqGuardExists: true,
	SeqPost:        true,
	SeqPut:         true,
	SeqDelete:      true,
	SeqCall:        true,
	SeqGuardState:  true,
	SeqResponse:    true,
}
