package parser

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func specsDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "testdata", "backend-service")
}

func TestParseCreateSession(t *testing.T) {
	sf, err := ParseFile(filepath.Join(specsDir(), "create_session.go"))
	if err != nil {
		t.Fatalf("파싱 실패: %v", err)
	}
	if sf == nil {
		t.Fatal("ServiceFunc가 nil")
	}

	if sf.Name != "CreateSession" {
		t.Errorf("함수명: got %q, want %q", sf.Name, "CreateSession")
	}
	if sf.FileName != "create_session.go" {
		t.Errorf("파일명: got %q, want %q", sf.FileName, "create_session.go")
	}
	if len(sf.Sequences) != 4 {
		t.Fatalf("sequence 수: got %d, want 4", len(sf.Sequences))
	}

	// sequence 0: get
	s := sf.Sequences[0]
	assertStr(t, "seq[0].Type", s.Type, "get")
	assertStr(t, "seq[0].Model", s.Model, "Project.FindByID")
	assertParams(t, "seq[0].Params", s.Params, []Param{{Name: "ProjectID", Source: "request"}})
	assertResult(t, "seq[0].Result", s.Result, "project", "Project")

	// sequence 1: guard nil
	s = sf.Sequences[1]
	assertStr(t, "seq[1].Type", s.Type, "guard nil")
	assertStr(t, "seq[1].Target", s.Target, "project")
	assertStr(t, "seq[1].Message", s.Message, "프로젝트가 존재하지 않습니다")

	// sequence 2: post
	s = sf.Sequences[2]
	assertStr(t, "seq[2].Type", s.Type, "post")
	assertStr(t, "seq[2].Model", s.Model, "Session.Create")
	assertParams(t, "seq[2].Params", s.Params, []Param{
		{Name: "ProjectID", Source: "request"},
		{Name: "Command", Source: "request"},
	})
	assertResult(t, "seq[2].Result", s.Result, "session", "Session")

	// sequence 3: response json
	s = sf.Sequences[3]
	assertStr(t, "seq[3].Type", s.Type, "response json")
	assertStrSlice(t, "seq[3].Vars", s.Vars, []string{"session"})
}

func TestParseDeleteProject(t *testing.T) {
	sf, err := ParseFile(filepath.Join(specsDir(), "delete_project.go"))
	if err != nil {
		t.Fatalf("파싱 실패: %v", err)
	}
	if sf == nil {
		t.Fatal("ServiceFunc가 nil")
	}

	if sf.Name != "DeleteProject" {
		t.Errorf("함수명: got %q, want %q", sf.Name, "DeleteProject")
	}
	if len(sf.Sequences) != 9 {
		t.Fatalf("sequence 수: got %d, want 9", len(sf.Sequences))
	}

	// sequence 0: authorize
	s := sf.Sequences[0]
	assertStr(t, "seq[0].Type", s.Type, "authorize")
	assertStr(t, "seq[0].Action", s.Action, "delete")
	assertStr(t, "seq[0].Resource", s.Resource, "project")
	assertStr(t, "seq[0].ID", s.ID, "ProjectID")

	// sequence 1: get
	s = sf.Sequences[1]
	assertStr(t, "seq[1].Type", s.Type, "get")
	assertStr(t, "seq[1].Model", s.Model, "Project.FindByID")

	// sequence 2: guard nil
	s = sf.Sequences[2]
	assertStr(t, "seq[2].Type", s.Type, "guard nil")
	assertStr(t, "seq[2].Target", s.Target, "project")

	// sequence 3: get (SessionCount)
	s = sf.Sequences[3]
	assertStr(t, "seq[3].Type", s.Type, "get")
	assertStr(t, "seq[3].Model", s.Model, "Session.CountByProjectID")
	assertResult(t, "seq[3].Result", s.Result, "sessionCount", "int")

	// sequence 4: guard exists
	s = sf.Sequences[4]
	assertStr(t, "seq[4].Type", s.Type, "guard exists")
	assertStr(t, "seq[4].Target", s.Target, "sessionCount")
	assertStr(t, "seq[4].Message", s.Message, "하위 세션이 존재하여 삭제할 수 없습니다")

	// sequence 5: call @component
	s = sf.Sequences[5]
	assertStr(t, "seq[5].Type", s.Type, "call")
	assertStr(t, "seq[5].Component", s.Component, "notification")
	assertParams(t, "seq[5].Params", s.Params, []Param{
		{Name: "project.OwnerEmail"},
		{Name: `"프로젝트가 삭제됩니다"`},
	})

	// sequence 6: call @func
	s = sf.Sequences[6]
	assertStr(t, "seq[6].Type", s.Type, "call")
	assertStr(t, "seq[6].Func", s.Func, "cleanupProjectResources")
	assertParams(t, "seq[6].Params", s.Params, []Param{{Name: "project"}})
	assertResult(t, "seq[6].Result", s.Result, "cleaned", "bool")

	// sequence 7: delete
	s = sf.Sequences[7]
	assertStr(t, "seq[7].Type", s.Type, "delete")
	assertStr(t, "seq[7].Model", s.Model, "Project.Delete")

	// sequence 8: response json
	s = sf.Sequences[8]
	assertStr(t, "seq[8].Type", s.Type, "response json")
}

func TestParseDir(t *testing.T) {
	funcs, err := ParseDir(specsDir())
	if err != nil {
		t.Fatalf("디렉토리 파싱 실패: %v", err)
	}
	if len(funcs) != 2 {
		t.Errorf("함수 수: got %d, want 2", len(funcs))
	}
}

// --- 유닛 테스트: parseTag ---

func TestParseTag(t *testing.T) {
	tests := []struct {
		line     string
		wantTag  string
		wantVal  string
	}{
		{"@sequence get", "sequence", "get"},
		{"@model Project.FindByID", "model", "Project.FindByID"},
		{"@param ProjectID request", "param", "ProjectID request"},
		{"@result project Project", "result", "project Project"},
		{"@message \"커스텀 메시지\"", "message", "\"커스텀 메시지\""},
		{"@var session", "var", "session"},
		{"@action delete", "action", "delete"},
		{"@sequence guard nil project", "sequence", "guard nil project"},
		{"@component notification", "component", "notification"},
		{"@func cleanupProjectResources", "func", "cleanupProjectResources"},
		{"@id ProjectID", "id", "ProjectID"},
		{"@resource project", "resource", "project"},
		// 값 없는 태그
		{"@sequence", "sequence", ""},
	}

	for _, tt := range tests {
		tag, val := parseTag(tt.line)
		if tag != tt.wantTag || val != tt.wantVal {
			t.Errorf("parseTag(%q) = (%q, %q), want (%q, %q)", tt.line, tag, val, tt.wantTag, tt.wantVal)
		}
	}
}

// --- 유닛 테스트: parseSequenceType ---

func TestParseSequenceType(t *testing.T) {
	tests := []struct {
		value string
		want  string
	}{
		{"get", "get"},
		{"post", "post"},
		{"put", "put"},
		{"delete", "delete"},
		{"authorize", "authorize"},
		{"password", "password"},
		{"call", "call"},
		{"guard nil project", "guard nil"},
		{"guard exists sessionCount", "guard exists"},
		{"response json", "response json"},
		{"response redirect", "response redirect"},
		{"response view", "response view"},
		// guard + 유효하지 않은 서브타입 → guard만
		{"guard something", "guard"},
	}

	for _, tt := range tests {
		got := parseSequenceType(tt.value)
		if got != tt.want {
			t.Errorf("parseSequenceType(%q) = %q, want %q", tt.value, got, tt.want)
		}
	}
}

// --- 유닛 테스트: parseParam ---

func TestParseParam(t *testing.T) {
	tests := []struct {
		value string
		want  Param
	}{
		{"ProjectID request", Param{Name: "ProjectID", Source: "request"}},
		{"Command request", Param{Name: "Command", Source: "request"}},
		// 변수 참조 (source 없음)
		{"project", Param{Name: "project"}},
		// dot notation
		{"project.OwnerEmail", Param{Name: "project.OwnerEmail"}},
		// 따옴표 리터럴
		{`"프로젝트가 삭제됩니다"`, Param{Name: `"프로젝트가 삭제됩니다"`}},
		{`"hello world"`, Param{Name: `"hello world"`}},
		// -> column 매핑
		{"PaymentMethod request -> method", Param{Name: "PaymentMethod", Source: "request", Column: "method"}},
		{"Status request -> status_code", Param{Name: "Status", Source: "request", Column: "status_code"}},
		// -> 없으면 Column 비어있음
		{"Name request", Param{Name: "Name", Source: "request", Column: ""}},
	}

	for _, tt := range tests {
		got := parseParam(tt.value)
		if got.Name != tt.want.Name || got.Source != tt.want.Source || got.Column != tt.want.Column {
			t.Errorf("parseParam(%q) = %+v, want %+v", tt.value, got, tt.want)
		}
	}
}

// --- 유닛 테스트: parseResult ---

func TestParseResult(t *testing.T) {
	tests := []struct {
		value    string
		wantVar  string
		wantType string
	}{
		{"project Project", "project", "Project"},
		{"session Session", "session", "Session"},
		{"sessionCount int", "sessionCount", "int"},
		{"cleaned bool", "cleaned", "bool"},
		// 타입 없는 경우
		{"project", "project", ""},
	}

	for _, tt := range tests {
		got := parseResult(tt.value)
		if got.Var != tt.wantVar || got.Type != tt.wantType {
			t.Errorf("parseResult(%q) = (%q, %q), want (%q, %q)", tt.value, got.Var, got.Type, tt.wantVar, tt.wantType)
		}
	}
}

// --- 유닛 테스트: trimQuotes ---

func TestTrimQuotes(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{`"hello"`, "hello"},
		{`"프로젝트가 존재하지 않습니다"`, "프로젝트가 존재하지 않습니다"},
		{"no quotes", "no quotes"},
		{`""`, ""},
		{`"one side only`, `"one side only`},
	}

	for _, tt := range tests {
		got := trimQuotes(tt.input)
		if got != tt.want {
			t.Errorf("trimQuotes(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// --- 엣지 케이스: sequence 주석이 없는 파일 ---

func TestParseFileNoSequence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.go")
	content := `package service

import "net/http"

// 이 함수에는 sequence 주석이 없다.
func NoSequence(w http.ResponseWriter, r *http.Request) {}
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	sf, err := ParseFile(path)
	if err != nil {
		t.Fatalf("파싱 실패: %v", err)
	}
	if sf != nil {
		t.Errorf("sequence 없는 파일인데 ServiceFunc 반환됨: %+v", sf)
	}
}

// --- 엣지 케이스: 잘못된 Go 파일 ---

func TestParseFileInvalidGo(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.go")
	if err := os.WriteFile(path, []byte("not valid go code{{{"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := ParseFile(path)
	if err == nil {
		t.Error("잘못된 Go 파일인데 에러가 없음")
	}
}

// --- 엣지 케이스: 존재하지 않는 디렉토리 ---

func TestParseDirNotFound(t *testing.T) {
	_, err := ParseDir("/nonexistent/path")
	if err == nil {
		t.Error("존재하지 않는 디렉토리인데 에러가 없음")
	}
}

// --- 엣지 케이스: 빈 디렉토리 ---

func TestParseDirEmpty(t *testing.T) {
	dir := t.TempDir()
	funcs, err := ParseDir(dir)
	if err != nil {
		t.Fatalf("빈 디렉토리 파싱 실패: %v", err)
	}
	if len(funcs) != 0 {
		t.Errorf("빈 디렉토리인데 함수 반환됨: %d개", len(funcs))
	}
}

// --- 엣지 케이스: password, put 타입 ---

func TestParsePasswordAndPut(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "login.go")
	content := `package service

import "net/http"

// @sequence get
// @model User.FindByEmail
// @param Email request
// @result user User

// @sequence guard nil user
// @message "사용자를 찾을 수 없습니다"

// @sequence password
// @param user.PasswordHash
// @param Password request

// @sequence put
// @model Session.Update
// @param SessionID request
// @param user.ID

// @sequence response json
// @var user
func Login(w http.ResponseWriter, r *http.Request) {}
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	sf, err := ParseFile(path)
	if err != nil {
		t.Fatalf("파싱 실패: %v", err)
	}
	if sf == nil {
		t.Fatal("ServiceFunc가 nil")
	}

	assertStr(t, "Name", sf.Name, "Login")
	if len(sf.Sequences) != 5 {
		t.Fatalf("sequence 수: got %d, want 5", len(sf.Sequences))
	}

	// password
	s := sf.Sequences[2]
	assertStr(t, "password.Type", s.Type, "password")
	assertParams(t, "password.Params", s.Params, []Param{
		{Name: "user.PasswordHash"},
		{Name: "Password", Source: "request"},
	})

	// put
	s = sf.Sequences[3]
	assertStr(t, "put.Type", s.Type, "put")
	assertStr(t, "put.Model", s.Model, "Session.Update")
	assertParams(t, "put.Params", s.Params, []Param{
		{Name: "SessionID", Source: "request"},
		{Name: "user.ID"},
	})
	if s.Result != nil {
		t.Errorf("put.Result: got %+v, want nil", s.Result)
	}

	// response
	s = sf.Sequences[4]
	assertStr(t, "response.Type", s.Type, "response json")
	assertStrSlice(t, "response.Vars", s.Vars, []string{"user"})
}

// --- 엣지 케이스: 다수 @var ---

func TestParseMultipleVars(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "detail.go")
	content := `package service

import "net/http"

// @sequence get
// @model Project.FindByID
// @param ProjectID request
// @result project Project

// @sequence get
// @model Session.ListByProjectID
// @param ProjectID request
// @result sessions []Session

// @sequence response json
// @var project
// @var sessions
func GetProjectDetail(w http.ResponseWriter, r *http.Request) {}
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	sf, err := ParseFile(path)
	if err != nil {
		t.Fatalf("파싱 실패: %v", err)
	}

	s := sf.Sequences[2]
	assertStr(t, "response.Type", s.Type, "response json")
	assertStrSlice(t, "response.Vars", s.Vars, []string{"project", "sessions"})
}

// --- guard state 파싱 ---

func TestParseGuardState(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "publish_course.go")
	content := `package service

import "net/http"

// @sequence get
// @model Course.FindByID
// @param CourseID request
// @result course Course

// @sequence guard nil course

// @sequence guard state course
// @param course.Published

// @sequence put
// @model Course.Publish
// @param CourseID request

// @sequence response json
func PublishCourse(w http.ResponseWriter, r *http.Request) {}
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	sf, err := ParseFile(path)
	if err != nil {
		t.Fatalf("파싱 실패: %v", err)
	}
	if sf == nil {
		t.Fatal("ServiceFunc가 nil")
	}

	if len(sf.Sequences) != 5 {
		t.Fatalf("sequence 수: got %d, want 5", len(sf.Sequences))
	}

	// sequence 2: guard state
	s := sf.Sequences[2]
	assertStr(t, "guard state.Type", s.Type, "guard state")
	assertStr(t, "guard state.Target", s.Target, "course")
	if len(s.Params) != 1 {
		t.Fatalf("guard state params: got %d, want 1", len(s.Params))
	}
	assertStr(t, "guard state.Params[0].Name", s.Params[0].Name, "course.Published")
}

// --- 도메인 폴더 재귀 탐색 ---

func TestParseDirRecursive(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	domainDir := filepath.Join(filepath.Dir(file), "..", "testdata", "domain-service")

	funcs, err := ParseDir(domainDir)
	if err != nil {
		t.Fatalf("도메인 디렉토리 파싱 실패: %v", err)
	}

	if len(funcs) != 2 {
		t.Fatalf("함수 수: got %d, want 2", len(funcs))
	}

	// WalkDir은 알파벳순이므로 course/create_course.go가 먼저
	assertStr(t, "funcs[0].Name", funcs[0].Name, "CreateCourse")
	assertStr(t, "funcs[0].Domain", funcs[0].Domain, "course")

	assertStr(t, "funcs[1].Name", funcs[1].Name, "Login")
	assertStr(t, "funcs[1].Domain", funcs[1].Domain, "")
}

// --- 헬퍼 ---

func assertStr(t *testing.T, label, got, want string) {
	t.Helper()
	if got != want {
		t.Errorf("%s: got %q, want %q", label, got, want)
	}
}

func assertStrSlice(t *testing.T, label string, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("%s: len got %d, want %d", label, len(got), len(want))
		return
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("%s[%d]: got %q, want %q", label, i, got[i], want[i])
		}
	}
}

func assertParams(t *testing.T, label string, got, want []Param) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("%s: len got %d, want %d", label, len(got), len(want))
		return
	}
	for i := range got {
		if got[i].Name != want[i].Name {
			t.Errorf("%s[%d].Name: got %q, want %q", label, i, got[i].Name, want[i].Name)
		}
		if got[i].Source != want[i].Source {
			t.Errorf("%s[%d].Source: got %q, want %q", label, i, got[i].Source, want[i].Source)
		}
	}
}

func assertResult(t *testing.T, label string, got *Result, wantVar, wantType string) {
	t.Helper()
	if got == nil {
		t.Errorf("%s: got nil", label)
		return
	}
	if got.Var != wantVar {
		t.Errorf("%s.Var: got %q, want %q", label, got.Var, wantVar)
	}
	if got.Type != wantType {
		t.Errorf("%s.Type: got %q, want %q", label, got.Type, wantType)
	}
}
