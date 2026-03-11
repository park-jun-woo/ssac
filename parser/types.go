package parser

// ServiceFunc는 하나의 서비스 함수 선언이다.
type ServiceFunc struct {
	Name      string         // 함수명 (e.g. "GetCourse")
	FileName  string         // 원본 파일명
	Domain    string         // 도메인 폴더명 (e.g. "auth", 없으면 "")
	Sequences []Sequence     // 시퀀스 목록
	Imports   []string       // Go import 경로
	Subscribe *SubscribeInfo // nil이면 HTTP 트리거
	Param     *ParamInfo     // 함수 파라미터 (subscribe 함수용)
	Structs   []StructInfo   // .ssac 파일에 선언된 Go struct 목록
}

// SubscribeInfo는 큐 구독 트리거 정보다.
type SubscribeInfo struct {
	Topic       string // "order.completed"
	MessageType string // "OnOrderCompletedMessage"
}

// ParamInfo는 함수 파라미터 정보다.
type ParamInfo struct {
	TypeName string // "OnOrderCompletedMessage"
	VarName  string // "message"
}

// StructInfo는 .ssac 파일에 선언된 Go struct 정보다.
type StructInfo struct {
	Name   string        // "OnOrderCompletedMessage"
	Fields []StructField
}

// StructField는 struct 필드 정보다.
type StructField struct {
	Name string // "OrderID"
	Type string // "int64"
}

// Sequence는 하나의 시퀀스 라인이다.
type Sequence struct {
	Type string // "get", "post", "put", "delete", "empty", "exists", "state", "auth", "call", "response"

	// get/post/put/delete/call 공통: 함수 호출
	Package string // "session" (패키지 접두사, 없으면 "")
	Model   string // "Course.FindByID" 또는 "auth.VerifyPassword"
	Args    []Arg  // 호출 인자

	// get/post/call: 대입
	Result *Result // 결과 바인딩 (nil이면 대입 없음)

	// empty/exists: guard
	Target string // "course" 또는 "course.InstructorID"

	// state: 상태 전이
	DiagramID  string            // "reservation"
	Inputs     map[string]string // {status: "reservation.Status"}
	Transition string            // "cancel"

	// publish: 이벤트 발행
	Topic   string            // "order.completed"
	Options map[string]string // {delay: "1800"} (선택)
	// Inputs 재사용: payload

	// auth: 권한 검사
	Action   string // "delete"
	Resource string // "project"
	// Inputs 재사용     // {id: "project.ID", owner: "project.OwnerID"}

	// response: 필드 매핑
	Fields map[string]string // {course: "course", instructor_name: "instructor.Name"}

	// 공통
	Message      string // 에러 메시지
	ErrStatus    int    // @call 에러 HTTP 상태 코드 (0이면 기본값 500)
	SuppressWarn bool   // @type! — WARNING 억제
}

// Arg는 함수 호출 인자다.
type Arg struct {
	Source  string // "request", 변수명, 또는 "" (리터럴)
	Field   string // "CourseID", "ID" 등
	Literal string // "cancelled" 등 (Source가 ""일 때)
}

// Result는 결과 바인딩이다.
type Result struct {
	Type    string // "Course", "Reservation" (내부 타입)
	Var     string // "course", "reservations"
	Wrapper string // "Page", "Cursor", "" (제네릭 래퍼)
}

// sequence 타입 상수
const (
	SeqGet      = "get"
	SeqPost     = "post"
	SeqPut      = "put"
	SeqDelete   = "delete"
	SeqEmpty    = "empty"
	SeqExists   = "exists"
	SeqState    = "state"
	SeqAuth     = "auth"
	SeqCall     = "call"
	SeqPublish  = "publish"
	SeqResponse = "response"
)

var ValidSequenceTypes = map[string]bool{
	SeqGet:      true,
	SeqPost:     true,
	SeqPut:      true,
	SeqDelete:   true,
	SeqEmpty:    true,
	SeqExists:   true,
	SeqState:    true,
	SeqAuth:     true,
	SeqCall:     true,
	SeqPublish:  true,
	SeqResponse: true,
}
