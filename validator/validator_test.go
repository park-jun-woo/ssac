package validator

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/geul-org/ssac/parser"
)

func specsDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "testdata", "backend-service")
}

func dummyStudyRoot() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "specs", "dummy-study")
}

func dummyStudyServiceDir() string {
	return filepath.Join(dummyStudyRoot(), "service")
}

// --- 정상 케이스: 기획서 예시는 검증 통과 ---

func TestValidateCreateSession(t *testing.T) {
	sf, _ := parser.ParseFile(filepath.Join(specsDir(), "create_session.go"))
	errs := Validate([]parser.ServiceFunc{*sf})
	if len(errs) != 0 {
		t.Errorf("CreateSession 검증 실패 (에러 없어야 함):")
		for _, e := range errs {
			t.Errorf("  %s", e)
		}
	}
}

func TestValidateDeleteProject(t *testing.T) {
	sf, _ := parser.ParseFile(filepath.Join(specsDir(), "delete_project.go"))
	errs := Validate([]parser.ServiceFunc{*sf})
	if len(errs) != 0 {
		t.Errorf("DeleteProject 검증 실패 (에러 없어야 함):")
		for _, e := range errs {
			t.Errorf("  %s", e)
		}
	}
}

// --- 필수 필드 누락 ---

func TestValidateAuthorizeMissingFields(t *testing.T) {
	sf := parser.ServiceFunc{
		Name:     "Test",
		FileName: "test.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqAuthorize}, // action, resource, id 모두 누락
		},
	}
	errs := Validate([]parser.ServiceFunc{sf})
	assertContainsError(t, errs, "@action", "누락")
	assertContainsError(t, errs, "@resource", "누락")
	assertContainsError(t, errs, "@id", "누락")
}

func TestValidateGetMissingModel(t *testing.T) {
	sf := parser.ServiceFunc{
		Name:     "Test",
		FileName: "test.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqGet}, // model, result 누락
		},
	}
	errs := Validate([]parser.ServiceFunc{sf})
	assertContainsError(t, errs, "@model", "누락")
	assertContainsError(t, errs, "@result", "누락")
}

func TestValidatePostMissingModel(t *testing.T) {
	sf := parser.ServiceFunc{
		Name:     "Test",
		FileName: "test.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqPost}, // model, result 누락
		},
	}
	errs := Validate([]parser.ServiceFunc{sf})
	assertContainsError(t, errs, "@model", "누락")
	assertContainsError(t, errs, "@result", "누락")
}

func TestValidatePutDeleteMissingModel(t *testing.T) {
	sf := parser.ServiceFunc{
		Name:     "Test",
		FileName: "test.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqPut},
			{Type: parser.SeqDelete},
		},
	}
	errs := Validate([]parser.ServiceFunc{sf})
	if count := countErrors(errs, "@model"); count != 2 {
		t.Errorf("@model 누락 에러 2개 예상, got %d", count)
	}
}

func TestValidateGuardMissingTarget(t *testing.T) {
	sf := parser.ServiceFunc{
		Name:     "Test",
		FileName: "test.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqGuardNil},    // target 누락
			{Type: parser.SeqGuardExists}, // target 누락
		},
	}
	errs := Validate([]parser.ServiceFunc{sf})
	if count := countErrors(errs, "@sequence"); count != 2 {
		t.Errorf("guard 대상 누락 에러 2개 예상, got %d", count)
	}
}

func TestValidatePasswordMissingParams(t *testing.T) {
	sf := parser.ServiceFunc{
		Name:     "Test",
		FileName: "test.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqPassword, Params: []parser.Param{{Name: "hash"}}}, // 1개만
		},
	}
	errs := Validate([]parser.ServiceFunc{sf})
	assertContainsError(t, errs, "@param", "2개 필요")
}

func TestValidateCallMissingBoth(t *testing.T) {
	sf := parser.ServiceFunc{
		Name:     "Test",
		FileName: "test.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqCall}, // component, func 둘 다 없음
		},
	}
	errs := Validate([]parser.ServiceFunc{sf})
	assertContainsError(t, errs, "@component/@func", "둘 다 누락")
}

func TestValidateCallBothSet(t *testing.T) {
	sf := parser.ServiceFunc{
		Name:     "Test",
		FileName: "test.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqCall, Component: "a", Func: "b"}, // 둘 다 있음
		},
	}
	errs := Validate([]parser.ServiceFunc{sf})
	assertContainsError(t, errs, "@component/@func", "둘 다 지정")
}

// --- @model 형식 검증 ---

func TestValidateModelFormat(t *testing.T) {
	sf := parser.ServiceFunc{
		Name:     "Test",
		FileName: "test.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqGet, Model: "Project", Result: &parser.Result{Var: "p", Type: "P"}}, // dot 없음
		},
	}
	errs := Validate([]parser.ServiceFunc{sf})
	assertContainsError(t, errs, "@model", "Model.Method")
}

// --- 변수 흐름 검증 ---

func TestValidateVarFlowOK(t *testing.T) {
	sf := parser.ServiceFunc{
		Name:     "Test",
		FileName: "test.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqGet, Model: "P.Find", Result: &parser.Result{Var: "project", Type: "P"},
				Params: []parser.Param{{Name: "ID", Source: "request"}}},
			{Type: parser.SeqGuardNil, Target: "project"},
		},
	}
	errs := Validate([]parser.ServiceFunc{sf})
	if len(errs) != 0 {
		t.Errorf("에러 없어야 함:")
		for _, e := range errs {
			t.Errorf("  %s", e)
		}
	}
}

func TestValidateVarFlowGuardBeforeDecl(t *testing.T) {
	sf := parser.ServiceFunc{
		Name:     "Test",
		FileName: "test.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqGuardNil, Target: "project"}, // project 아직 선언 안 됨
			{Type: parser.SeqGet, Model: "P.Find", Result: &parser.Result{Var: "project", Type: "P"},
				Params: []parser.Param{{Name: "ID", Source: "request"}}},
		},
	}
	errs := Validate([]parser.ServiceFunc{sf})
	assertContainsError(t, errs, "guard", "선언되지 않았습니다")
}

func TestValidateVarFlowParamRef(t *testing.T) {
	sf := parser.ServiceFunc{
		Name:     "Test",
		FileName: "test.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqCall, Func: "doSomething",
				Params: []parser.Param{{Name: "project"}}}, // project 미선언
		},
	}
	errs := Validate([]parser.ServiceFunc{sf})
	assertContainsError(t, errs, "@param", "project")
}

func TestValidateVarFlowDotNotation(t *testing.T) {
	sf := parser.ServiceFunc{
		Name:     "Test",
		FileName: "test.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqCall, Component: "notify",
				Params: []parser.Param{{Name: "user.Email"}}}, // user 미선언
		},
	}
	errs := Validate([]parser.ServiceFunc{sf})
	assertContainsError(t, errs, "@param", "user")
}

func TestValidateVarFlowDotNotationAfterDecl(t *testing.T) {
	sf := parser.ServiceFunc{
		Name:     "Test",
		FileName: "test.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqGet, Model: "U.Find", Result: &parser.Result{Var: "user", Type: "U"},
				Params: []parser.Param{{Name: "ID", Source: "request"}}},
			{Type: parser.SeqCall, Component: "notify",
				Params: []parser.Param{{Name: "user.Email"}}}, // user 선언됨 → OK
		},
	}
	errs := Validate([]parser.ServiceFunc{sf})
	if len(errs) != 0 {
		t.Errorf("에러 없어야 함:")
		for _, e := range errs {
			t.Errorf("  %s", e)
		}
	}
}

func TestValidateVarFlowResponseVar(t *testing.T) {
	sf := parser.ServiceFunc{
		Name:     "Test",
		FileName: "test.go",
		Sequences: []parser.Sequence{
			{Type: "response json", Vars: []string{"session"}}, // session 미선언
		},
	}
	errs := Validate([]parser.ServiceFunc{sf})
	assertContainsError(t, errs, "@var", "session")
}

func TestValidateVarFlowLiteralNotChecked(t *testing.T) {
	sf := parser.ServiceFunc{
		Name:     "Test",
		FileName: "test.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqCall, Component: "notify",
				Params: []parser.Param{{Name: `"리터럴 값"`}}}, // 리터럴은 검증 안 함
		},
	}
	errs := Validate([]parser.ServiceFunc{sf})
	if len(errs) != 0 {
		t.Errorf("리터럴은 변수 검증 안 해야 함:")
		for _, e := range errs {
			t.Errorf("  %s", e)
		}
	}
}

func TestValidateVarFlowRequestNotChecked(t *testing.T) {
	sf := parser.ServiceFunc{
		Name:     "Test",
		FileName: "test.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqGet, Model: "P.Find", Result: &parser.Result{Var: "p", Type: "P"},
				Params: []parser.Param{{Name: "ProjectID", Source: "request"}}}, // request는 검증 안 함
		},
	}
	errs := Validate([]parser.ServiceFunc{sf})
	if len(errs) != 0 {
		t.Errorf("request source는 변수 검증 안 해야 함:")
		for _, e := range errs {
			t.Errorf("  %s", e)
		}
	}
}

// === 외부 검증 (심볼 테이블) ===

// --- 심볼 테이블 로드 ---

func TestLoadSymbolTable(t *testing.T) {
	st, err := LoadSymbolTable(dummyStudyRoot())
	if err != nil {
		t.Fatalf("심볼 테이블 로드 실패: %v", err)
	}

	// sqlc models
	for _, model := range []string{"User", "Room", "Reservation"} {
		if _, ok := st.Models[model]; !ok {
			t.Errorf("모델 %q 없음", model)
		}
	}

	// sqlc methods
	userMethods := []string{"FindByEmail", "FindByID"}
	for _, m := range userMethods {
		if !st.Models["User"].HasMethod(m) {
			t.Errorf("User.%s 없음", m)
		}
	}

	roomMethods := []string{"FindByID", "Delete", "Update"}
	for _, m := range roomMethods {
		if !st.Models["Room"].HasMethod(m) {
			t.Errorf("Room.%s 없음", m)
		}
	}

	resMethods := []string{"FindByID", "FindConflict", "Create", "ListByUserID", "CountByRoomID", "UpdateStatus"}
	for _, m := range resMethods {
		if !st.Models["Reservation"].HasMethod(m) {
			t.Errorf("Reservation.%s 없음", m)
		}
	}

	// OpenAPI operations
	for _, op := range []string{"Login", "CreateReservation", "GetReservation", "ListMyReservations", "CancelReservation", "UpdateRoom", "DeleteRoom"} {
		if _, ok := st.Operations[op]; !ok {
			t.Errorf("operation %q 없음", op)
		}
	}

	// Login request fields
	loginOp := st.Operations["Login"]
	for _, f := range []string{"Email", "Password"} {
		if !loginOp.RequestFields[f] {
			t.Errorf("Login request에 %q 없음", f)
		}
	}

	// Login response fields
	if !loginOp.ResponseFields["token"] {
		t.Error("Login response에 token 없음")
	}

	// CreateReservation request
	createOp := st.Operations["CreateReservation"]
	for _, f := range []string{"RoomID", "StartAt", "EndAt"} {
		if !createOp.RequestFields[f] {
			t.Errorf("CreateReservation request에 %q 없음", f)
		}
	}

	// Components
	if !st.Components["notification"] {
		t.Error("component notification 없음")
	}

	// Go interface → Model로도 등록
	if _, ok := st.Models["Notification"]; !ok {
		t.Error("Notification 모델 없음 (interface에서)")
	}
	if _, ok := st.Models["Session"]; !ok {
		t.Error("Session 모델 없음 (interface에서)")
	}

	// Funcs
	if !st.Funcs["calculateRefund"] {
		t.Error("func calculateRefund 없음")
	}
}

// --- 외부 검증 정상 케이스: dummy-study 전체 통과 ---

func TestValidateWithSymbolsDummyStudy(t *testing.T) {
	st, err := LoadSymbolTable(dummyStudyRoot())
	if err != nil {
		t.Fatalf("심볼 테이블 로드 실패: %v", err)
	}

	funcs, err := parser.ParseDir(dummyStudyServiceDir())
	if err != nil {
		t.Fatalf("파싱 실패: %v", err)
	}

	errs := ValidateWithSymbols(funcs, st)
	for _, e := range errs {
		if !e.IsWarning() {
			t.Errorf("dummy-study 외부 검증 실패 (에러 없어야 함): %s", e)
		}
	}
}

// --- 외부 검증 실패 케이스: 존재하지 않는 모델 ---

func TestValidateWithSymbolsUnknownModel(t *testing.T) {
	st := &SymbolTable{
		Models:     map[string]ModelSymbol{},
		Operations: map[string]OperationSymbol{},
		Components: map[string]bool{},
		Funcs:      map[string]bool{},
	}
	sf := parser.ServiceFunc{
		Name:     "Test",
		FileName: "test.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqGet, Model: "Unknown.FindByID",
				Result: &parser.Result{Var: "x", Type: "X"},
				Params: []parser.Param{{Name: "ID", Source: "request"}}},
		},
	}
	errs := ValidateWithSymbols([]parser.ServiceFunc{sf}, st)
	assertContainsError(t, errs, "@model", "Unknown")
}

// --- 외부 검증 실패 케이스: 존재하지 않는 메서드 ---

func TestValidateWithSymbolsUnknownMethod(t *testing.T) {
	st := &SymbolTable{
		Models: map[string]ModelSymbol{
			"User": {Methods: map[string]MethodInfo{"FindByID": {}}},
		},
		Operations: map[string]OperationSymbol{},
		Components: map[string]bool{},
		Funcs:      map[string]bool{},
	}
	sf := parser.ServiceFunc{
		Name:     "Test",
		FileName: "test.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqGet, Model: "User.FindByName",
				Result: &parser.Result{Var: "u", Type: "User"},
				Params: []parser.Param{{Name: "Name", Source: "request"}}},
		},
	}
	errs := ValidateWithSymbols([]parser.ServiceFunc{sf}, st)
	assertContainsError(t, errs, "@model", "FindByName")
}

// --- 외부 검증 실패 케이스: OpenAPI request 필드 누락 ---

func TestValidateWithSymbolsMissingRequestField(t *testing.T) {
	st := &SymbolTable{
		Models: map[string]ModelSymbol{
			"User": {Methods: map[string]MethodInfo{"FindByID": {}}},
		},
		Operations: map[string]OperationSymbol{
			"Test": {
				RequestFields:  map[string]bool{"Email": true},
				ResponseFields: map[string]bool{},
			},
		},
		Components: map[string]bool{},
		Funcs:      map[string]bool{},
	}
	sf := parser.ServiceFunc{
		Name:     "Test",
		FileName: "test.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqGet, Model: "User.FindByID",
				Result: &parser.Result{Var: "u", Type: "User"},
				Params: []parser.Param{{Name: "Phone", Source: "request"}}}, // Phone은 OpenAPI에 없음
		},
	}
	errs := ValidateWithSymbols([]parser.ServiceFunc{sf}, st)
	assertContainsError(t, errs, "@param", "Phone")
}

// --- 외부 검증 실패 케이스: OpenAPI response 필드 누락 ---

func TestValidateWithSymbolsMissingResponseField(t *testing.T) {
	st := &SymbolTable{
		Models:     map[string]ModelSymbol{},
		Operations: map[string]OperationSymbol{
			"Test": {
				RequestFields:  map[string]bool{},
				ResponseFields: map[string]bool{"user": true},
			},
		},
		Components: map[string]bool{},
		Funcs:      map[string]bool{},
	}
	sf := parser.ServiceFunc{
		Name:     "Test",
		FileName: "test.go",
		Sequences: []parser.Sequence{
			{Type: "response json", Vars: []string{"user", "token"}}, // token은 response에 없음
		},
	}
	errs := ValidateWithSymbols([]parser.ServiceFunc{sf}, st)
	assertContainsError(t, errs, "@var", "token")
}

// --- 외부 검증 실패 케이스: 존재하지 않는 component ---

func TestValidateWithSymbolsUnknownComponent(t *testing.T) {
	st := &SymbolTable{
		Models:     map[string]ModelSymbol{},
		Operations: map[string]OperationSymbol{},
		Components: map[string]bool{"notification": true},
		Funcs:      map[string]bool{},
	}
	sf := parser.ServiceFunc{
		Name:     "Test",
		FileName: "test.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqCall, Component: "emailer",
				Params: []parser.Param{{Name: `"hello"`}}},
		},
	}
	errs := ValidateWithSymbols([]parser.ServiceFunc{sf}, st)
	assertContainsError(t, errs, "@component", "emailer")
}

// --- 외부 검증 실패 케이스: 존재하지 않는 func ---

func TestValidateWithSymbolsUnknownFunc(t *testing.T) {
	st := &SymbolTable{
		Models:     map[string]ModelSymbol{},
		Operations: map[string]OperationSymbol{},
		Components: map[string]bool{},
		Funcs:      map[string]bool{"calculateRefund": true},
	}
	sf := parser.ServiceFunc{
		Name:     "Test",
		FileName: "test.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqCall, Func: "unknownFunc",
				Params: []parser.Param{{Name: `"x"`}}},
		},
	}
	errs := ValidateWithSymbols([]parser.ServiceFunc{sf}, st)
	assertContainsError(t, errs, "@func", "unknownFunc")
}

// --- 에러 메시지 형식 ---

func TestValidationErrorFormat(t *testing.T) {
	e := ValidationError{
		FileName: "test.go",
		FuncName: "DoSomething",
		SeqIndex: 2,
		Tag:      "@model",
		Message:  "누락",
	}
	got := e.Error()
	want := "ERROR: test.go:DoSomething:seq[2] @model — 누락"
	if got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

// --- 헬퍼 ---

func assertContainsError(t *testing.T, errs []ValidationError, tag, msgSubstr string) {
	t.Helper()
	for _, e := range errs {
		if e.Tag == tag && strings.Contains(e.Message, msgSubstr) {
			return
		}
	}
	t.Errorf("에러 없음: tag=%q, message contains %q. 전체 에러:", tag, msgSubstr)
	for _, e := range errs {
		t.Errorf("  %s", e)
	}
}

func countErrors(errs []ValidationError, tag string) int {
	n := 0
	for _, e := range errs {
		if e.Tag == tag {
			n++
		}
	}
	return n
}
