package validator

import (
	"testing"

	"github.com/geul-org/ssac/parser"
)

// --- 내부 검증: 필수 필드 ---

func TestValidateGetMissingResult(t *testing.T) {
	funcs := []parser.ServiceFunc{{
		Name: "GetCourse", FileName: "get_course.go",
		Sequences: []parser.Sequence{{
			Type:  parser.SeqGet,
			Model: "Course.FindByID",
			Args:  []parser.Arg{{Source: "request", Field: "CourseID"}},
			// Result 누락
		}},
	}}
	errs := Validate(funcs)
	assertHasError(t, errs, "Result 누락")
}

func TestValidatePutHasResult(t *testing.T) {
	funcs := []parser.ServiceFunc{{
		Name: "UpdateCourse", FileName: "update_course.go",
		Sequences: []parser.Sequence{{
			Type:   parser.SeqPut,
			Model:  "Course.Update",
			Args:   []parser.Arg{{Source: "request", Field: "Title"}},
			Result: &parser.Result{Type: "Course", Var: "course"}, // 있으면 안 됨
		}},
	}}
	errs := Validate(funcs)
	assertHasError(t, errs, "Result는 nil이어야 함")
}

func TestValidateEmptyMissingMessage(t *testing.T) {
	funcs := []parser.ServiceFunc{{
		Name: "GetCourse", FileName: "get_course.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqGet, Model: "Course.FindByID", Args: []parser.Arg{{Source: "request", Field: "ID"}}, Result: &parser.Result{Type: "Course", Var: "course"}},
			{Type: parser.SeqEmpty, Target: "course"}, // Message 누락
		},
	}}
	errs := Validate(funcs)
	assertHasError(t, errs, "Message 누락")
}

func TestValidateStateMissingFields(t *testing.T) {
	funcs := []parser.ServiceFunc{{
		Name: "Cancel", FileName: "cancel.go",
		Sequences: []parser.Sequence{{
			Type: parser.SeqState,
			// DiagramID, Inputs, Transition, Message 전부 누락
		}},
	}}
	errs := Validate(funcs)
	assertHasError(t, errs, "DiagramID 누락")
	assertHasError(t, errs, "Inputs 누락")
	assertHasError(t, errs, "Transition 누락")
	assertHasError(t, errs, "Message 누락")
}

func TestValidateAuthMissingFields(t *testing.T) {
	funcs := []parser.ServiceFunc{{
		Name: "Delete", FileName: "delete.go",
		Sequences: []parser.Sequence{{
			Type: parser.SeqAuth,
			// Action, Resource, Message 전부 누락
		}},
	}}
	errs := Validate(funcs)
	assertHasError(t, errs, "Action 누락")
	assertHasError(t, errs, "Resource 누락")
	assertHasError(t, errs, "Message 누락")
}

func TestValidateCallMissingModel(t *testing.T) {
	funcs := []parser.ServiceFunc{{
		Name: "Login", FileName: "login.go",
		Sequences: []parser.Sequence{{
			Type: parser.SeqCall,
			// Model 누락
		}},
	}}
	errs := Validate(funcs)
	assertHasError(t, errs, "Model 누락")
}

func TestValidateResponseEmptyFieldsAllowed(t *testing.T) {
	funcs := []parser.ServiceFunc{{
		Name: "DeleteRoom", FileName: "delete_room.go",
		Sequences: []parser.Sequence{{
			Type: parser.SeqResponse,
			// 빈 Fields — DELETE 등에서 허용
		}},
	}}
	errs := Validate(funcs)
	if len(errs) > 0 {
		t.Errorf("expected no errors, got %d: %v", len(errs), errs)
	}
}

// --- 내부 검증: 변수 흐름 ---

func TestValidateUndeclaredVariable(t *testing.T) {
	funcs := []parser.ServiceFunc{{
		Name: "GetCourse", FileName: "get_course.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqEmpty, Target: "course", Message: "not found"}, // course 선언 전 사용
		},
	}}
	errs := Validate(funcs)
	assertHasError(t, errs, `"course" 변수가 아직 선언되지 않았습니다`)
}

func TestValidateVariableFlowValid(t *testing.T) {
	funcs := []parser.ServiceFunc{{
		Name: "GetCourse", FileName: "get_course.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqGet, Model: "Course.FindByID", Args: []parser.Arg{{Source: "request", Field: "ID"}}, Result: &parser.Result{Type: "Course", Var: "course"}},
			{Type: parser.SeqEmpty, Target: "course", Message: "not found"},
			{Type: parser.SeqResponse, Fields: map[string]string{"course": "course"}},
		},
	}}
	errs := Validate(funcs)
	if len(errs) > 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
}

func TestValidateUndeclaredInResponse(t *testing.T) {
	funcs := []parser.ServiceFunc{{
		Name: "GetCourse", FileName: "get_course.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqResponse, Fields: map[string]string{"course": "course"}},
		},
	}}
	errs := Validate(funcs)
	assertHasError(t, errs, `"course" 변수가 아직 선언되지 않았습니다`)
}

func TestValidateUndeclaredInInputs(t *testing.T) {
	funcs := []parser.ServiceFunc{{
		Name: "Cancel", FileName: "cancel.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqState, DiagramID: "reservation", Inputs: map[string]string{"status": "reservation.Status"}, Transition: "cancel", Message: "fail"},
		},
	}}
	errs := Validate(funcs)
	assertHasError(t, errs, `"reservation" 변수가 아직 선언되지 않았습니다`)
}

func TestValidateCurrentUserNoError(t *testing.T) {
	funcs := []parser.ServiceFunc{{
		Name: "GetMy", FileName: "get_my.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqGet, Model: "Item.ListByUser", Args: []parser.Arg{{Source: "currentUser", Field: "ID"}}, Result: &parser.Result{Type: "[]Item", Var: "items"}},
			{Type: parser.SeqResponse, Fields: map[string]string{"items": "items"}},
		},
	}}
	errs := Validate(funcs)
	if len(errs) > 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
}

func TestValidateArgVarRef(t *testing.T) {
	funcs := []parser.ServiceFunc{{
		Name: "GetDetail", FileName: "get_detail.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqGet, Model: "User.FindByID", Args: []parser.Arg{{Source: "course", Field: "InstructorID"}}, Result: &parser.Result{Type: "User", Var: "user"}},
		},
	}}
	errs := Validate(funcs)
	assertHasError(t, errs, `"course" 변수가 아직 선언되지 않았습니다`)
}

// --- 내부 검증: stale 데이터 ---

func TestValidateStaleResponse(t *testing.T) {
	funcs := []parser.ServiceFunc{{
		Name: "Cancel", FileName: "cancel.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqGet, Model: "Reservation.FindByID", Args: []parser.Arg{{Source: "request", Field: "ID"}}, Result: &parser.Result{Type: "Reservation", Var: "reservation"}},
			{Type: parser.SeqPut, Model: "Reservation.UpdateStatus", Args: []parser.Arg{{Source: "request", Field: "ID"}, {Literal: "cancelled"}}},
			{Type: parser.SeqResponse, Fields: map[string]string{"reservation": "reservation"}},
		},
	}}
	errs := Validate(funcs)
	assertHasWarning(t, errs, "갱신 없이 response에 사용됩니다")
}

func TestValidateStaleResponseRefetched(t *testing.T) {
	funcs := []parser.ServiceFunc{{
		Name: "Cancel", FileName: "cancel.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqGet, Model: "Reservation.FindByID", Args: []parser.Arg{{Source: "request", Field: "ID"}}, Result: &parser.Result{Type: "Reservation", Var: "reservation"}},
			{Type: parser.SeqPut, Model: "Reservation.UpdateStatus", Args: []parser.Arg{{Source: "request", Field: "ID"}, {Literal: "cancelled"}}},
			{Type: parser.SeqGet, Model: "Reservation.FindByID", Args: []parser.Arg{{Source: "request", Field: "ID"}}, Result: &parser.Result{Type: "Reservation", Var: "reservation"}},
			{Type: parser.SeqResponse, Fields: map[string]string{"reservation": "reservation"}},
		},
	}}
	errs := Validate(funcs)
	for _, e := range errs {
		if e.IsWarning() {
			t.Errorf("unexpected warning: %s", e.Message)
		}
	}
}

// --- 외부 검증: 모델 ---

func TestValidateModelNotFound(t *testing.T) {
	st := &SymbolTable{
		Models:     map[string]ModelSymbol{},
		Operations: map[string]OperationSymbol{},
	}
	funcs := []parser.ServiceFunc{{
		Name: "GetCourse", FileName: "get_course.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqGet, Model: "Course.FindByID", Args: []parser.Arg{{Source: "request", Field: "ID"}}, Result: &parser.Result{Type: "Course", Var: "course"}},
			{Type: parser.SeqResponse, Fields: map[string]string{"course": "course"}},
		},
	}}
	errs := ValidateWithSymbols(funcs, st)
	assertHasError(t, errs, `"Course" 모델을 찾을 수 없습니다`)
}

func TestValidateMethodNotFound(t *testing.T) {
	st := &SymbolTable{
		Models: map[string]ModelSymbol{
			"Course": {Methods: map[string]MethodInfo{"Create": {Cardinality: "one"}}},
		},
		Operations: map[string]OperationSymbol{},
	}
	funcs := []parser.ServiceFunc{{
		Name: "GetCourse", FileName: "get_course.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqGet, Model: "Course.FindByID", Args: []parser.Arg{{Source: "request", Field: "ID"}}, Result: &parser.Result{Type: "Course", Var: "course"}},
			{Type: parser.SeqResponse, Fields: map[string]string{"course": "course"}},
		},
	}}
	errs := ValidateWithSymbols(funcs, st)
	assertHasError(t, errs, `"FindByID" 메서드가 없습니다`)
}

func TestValidateCallSkipsModelCheck(t *testing.T) {
	st := &SymbolTable{
		Models:     map[string]ModelSymbol{},
		Operations: map[string]OperationSymbol{},
	}
	funcs := []parser.ServiceFunc{{
		Name: "Login", FileName: "login.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqCall, Model: "auth.VerifyPassword", Args: []parser.Arg{{Source: "request", Field: "Password"}}, Result: &parser.Result{Type: "Token", Var: "token"}},
			{Type: parser.SeqResponse, Fields: map[string]string{"token": "token"}},
		},
	}}
	errs := ValidateWithSymbols(funcs, st)
	// @call은 외부 패키지이므로 모델 체크 안 함 → 모델 관련 에러 없어야 함
	for _, e := range errs {
		if !e.IsWarning() && (contains(e.Message, "모델") || contains(e.Message, "메서드")) {
			t.Errorf("unexpected model error for @call: %s", e.Message)
		}
	}
}

// --- 외부 검증: request ---

func TestValidateRequestFieldNotInOpenAPI(t *testing.T) {
	st := &SymbolTable{
		Models: map[string]ModelSymbol{
			"Course": {Methods: map[string]MethodInfo{"FindByID": {Cardinality: "one"}}},
		},
		Operations: map[string]OperationSymbol{
			"GetCourse": {
				RequestFields:  map[string]bool{"CourseID": true},
				ResponseFields: map[string]bool{"course": true},
			},
		},
	}
	funcs := []parser.ServiceFunc{{
		Name: "GetCourse", FileName: "get_course.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqGet, Model: "Course.FindByID", Args: []parser.Arg{{Source: "request", Field: "UnknownField"}}, Result: &parser.Result{Type: "Course", Var: "course"}},
			{Type: parser.SeqResponse, Fields: map[string]string{"course": "course"}},
		},
	}}
	errs := ValidateWithSymbols(funcs, st)
	assertHasError(t, errs, `OpenAPI request에 "UnknownField" 필드가 없습니다`)
}

func TestValidateReverseRequestMissing(t *testing.T) {
	st := &SymbolTable{
		Models: map[string]ModelSymbol{
			"Course": {Methods: map[string]MethodInfo{"FindByID": {Cardinality: "one"}}},
		},
		Operations: map[string]OperationSymbol{
			"GetCourse": {
				RequestFields:  map[string]bool{"CourseID": true, "Description": true},
				ResponseFields: map[string]bool{"course": true},
			},
		},
	}
	funcs := []parser.ServiceFunc{{
		Name: "GetCourse", FileName: "get_course.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqGet, Model: "Course.FindByID", Args: []parser.Arg{{Source: "request", Field: "CourseID"}}, Result: &parser.Result{Type: "Course", Var: "course"}},
			{Type: parser.SeqResponse, Fields: map[string]string{"course": "course"}},
		},
	}}
	errs := ValidateWithSymbols(funcs, st)
	assertHasWarning(t, errs, `OpenAPI request에 "Description" 필드가 있지만 SSaC에서 사용하지 않습니다`)
}

func TestValidateReverseRequestPathParamSkip(t *testing.T) {
	st := &SymbolTable{
		Models: map[string]ModelSymbol{
			"Course": {Methods: map[string]MethodInfo{"FindByID": {Cardinality: "one"}}},
		},
		Operations: map[string]OperationSymbol{
			"GetCourse": {
				RequestFields:  map[string]bool{"CourseID": true},
				ResponseFields: map[string]bool{"course": true},
				PathParams:     []PathParam{{Name: "CourseID", GoType: "int64"}},
			},
		},
	}
	funcs := []parser.ServiceFunc{{
		Name: "GetCourse", FileName: "get_course.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqGet, Model: "Course.FindByID", Args: []parser.Arg{{Source: "request", Field: "CourseID"}}, Result: &parser.Result{Type: "Course", Var: "course"}},
			{Type: parser.SeqResponse, Fields: map[string]string{"course": "course"}},
		},
	}}
	errs := ValidateWithSymbols(funcs, st)
	for _, e := range errs {
		if e.IsWarning() && contains(e.Message, "CourseID") {
			t.Errorf("path param should be skipped in reverse check: %s", e.Message)
		}
	}
}

// --- 외부 검증: response ---

func TestValidateResponseFieldNotInOpenAPI(t *testing.T) {
	st := &SymbolTable{
		Models: map[string]ModelSymbol{
			"Course": {Methods: map[string]MethodInfo{"FindByID": {Cardinality: "one"}}},
		},
		Operations: map[string]OperationSymbol{
			"GetCourse": {
				RequestFields:  map[string]bool{"CourseID": true},
				ResponseFields: map[string]bool{"course": true},
				PathParams:     []PathParam{{Name: "CourseID", GoType: "int64"}},
			},
		},
	}
	funcs := []parser.ServiceFunc{{
		Name: "GetCourse", FileName: "get_course.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqGet, Model: "Course.FindByID", Args: []parser.Arg{{Source: "request", Field: "CourseID"}}, Result: &parser.Result{Type: "Course", Var: "course"}},
			{Type: parser.SeqResponse, Fields: map[string]string{"course": "course", "extra": "course.Name"}},
		},
	}}
	errs := ValidateWithSymbols(funcs, st)
	assertHasError(t, errs, `OpenAPI response에 "extra" 필드가 없습니다`)
}

func TestValidateReverseResponseMissing(t *testing.T) {
	st := &SymbolTable{
		Models: map[string]ModelSymbol{
			"Course": {Methods: map[string]MethodInfo{"FindByID": {Cardinality: "one"}}},
		},
		Operations: map[string]OperationSymbol{
			"GetCourse": {
				RequestFields:  map[string]bool{"CourseID": true},
				ResponseFields: map[string]bool{"course": true, "instructor": true},
				PathParams:     []PathParam{{Name: "CourseID", GoType: "int64"}},
			},
		},
	}
	funcs := []parser.ServiceFunc{{
		Name: "GetCourse", FileName: "get_course.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqGet, Model: "Course.FindByID", Args: []parser.Arg{{Source: "request", Field: "CourseID"}}, Result: &parser.Result{Type: "Course", Var: "course"}},
			{Type: parser.SeqResponse, Fields: map[string]string{"course": "course"}},
		},
	}}
	errs := ValidateWithSymbols(funcs, st)
	assertHasError(t, errs, `OpenAPI response에 "instructor" 필드가 있지만 SSaC @response에 없습니다`)
}

func TestValidateReverseResponsePaginationTotal(t *testing.T) {
	st := &SymbolTable{
		Models: map[string]ModelSymbol{
			"Item": {Methods: map[string]MethodInfo{"List": {Cardinality: "many"}}},
		},
		Operations: map[string]OperationSymbol{
			"ListItems": {
				RequestFields:  map[string]bool{},
				ResponseFields: map[string]bool{"items": true, "total": true},
				XPagination:    &XPagination{Style: "offset", DefaultLimit: 20, MaxLimit: 100},
			},
		},
	}
	funcs := []parser.ServiceFunc{{
		Name: "ListItems", FileName: "list_items.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqGet, Model: "Item.List", Args: []parser.Arg{{Source: "request", Field: "dummy"}}, Result: &parser.Result{Type: "[]Item", Var: "items"}},
			{Type: parser.SeqResponse, Fields: map[string]string{"items": "items"}},
		},
	}}
	errs := ValidateWithSymbols(funcs, st)
	for _, e := range errs {
		if contains(e.Message, "total") {
			t.Errorf("total should be skipped with x-pagination: %s", e.Message)
		}
	}
}

// --- 내부 검증: SuppressWarn ---

func TestValidateDeleteNoArgsSuppressed(t *testing.T) {
	funcs := []parser.ServiceFunc{{
		Name: "DeleteAll", FileName: "delete_all.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqDelete, Model: "Room.DeleteAll", SuppressWarn: true},
		},
	}}
	errs := Validate(funcs)
	for _, e := range errs {
		if e.IsWarning() && contains(e.Message, "전체 삭제") {
			t.Errorf("expected WARNING to be suppressed: %s", e.Message)
		}
	}
}

func TestValidateStaleResponseSuppressed(t *testing.T) {
	funcs := []parser.ServiceFunc{{
		Name: "Cancel", FileName: "cancel.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqGet, Model: "Reservation.FindByID", Args: []parser.Arg{{Source: "request", Field: "ID"}}, Result: &parser.Result{Type: "Reservation", Var: "reservation"}},
			{Type: parser.SeqPut, Model: "Reservation.UpdateStatus", Args: []parser.Arg{{Source: "request", Field: "ID"}, {Literal: "cancelled"}}},
			{Type: parser.SeqResponse, Fields: map[string]string{"reservation": "reservation"}, SuppressWarn: true},
		},
	}}
	errs := Validate(funcs)
	for _, e := range errs {
		if e.IsWarning() && contains(e.Message, "갱신 없이") {
			t.Errorf("expected stale WARNING to be suppressed: %s", e.Message)
		}
	}
}

// --- 내부 검증: 예약 소스 ---

func TestValidateReservedSourceConflict(t *testing.T) {
	for _, name := range []string{"request", "currentUser", "config"} {
		t.Run(name, func(t *testing.T) {
			funcs := []parser.ServiceFunc{{
				Name: "Test", FileName: "test.go",
				Sequences: []parser.Sequence{
					{Type: parser.SeqGet, Model: "User.FindByID", Args: []parser.Arg{{Source: "request", Field: "ID"}}, Result: &parser.Result{Type: "User", Var: name}},
				},
			}}
			errs := Validate(funcs)
			assertHasError(t, errs, "예약 소스이므로 result 변수명으로 사용할 수 없습니다")
		})
	}
}

func TestValidateReservedSourceNonReservedOK(t *testing.T) {
	funcs := []parser.ServiceFunc{{
		Name: "Test", FileName: "test.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqGet, Model: "User.FindByID", Args: []parser.Arg{{Source: "request", Field: "ID"}}, Result: &parser.Result{Type: "User", Var: "user"}},
		},
	}}
	errs := Validate(funcs)
	for _, e := range errs {
		if contains(e.Message, "예약 소스") {
			t.Errorf("unexpected reserved source error: %s", e.Message)
		}
	}
}

// --- 외부 검증: query 교차 검증 ---

func TestValidateQueryUsageMissing(t *testing.T) {
	st := &SymbolTable{
		Models:     map[string]ModelSymbol{},
		Operations: map[string]OperationSymbol{
			"ListReservations": {
				XPagination: &XPagination{Style: "offset", DefaultLimit: 20, MaxLimit: 100},
			},
		},
		DDLTables: map[string]DDLTable{},
	}
	funcs := []parser.ServiceFunc{{
		Name: "ListReservations", FileName: "list_reservations.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqGet, Model: "Reservation.List", Args: []parser.Arg{}, Result: &parser.Result{Type: "[]Reservation", Var: "reservations"}},
		},
	}}
	errs := ValidateWithSymbols(funcs, st)
	assertHasWarning(t, errs, "query가 사용되지 않았습니다")
}

func TestValidateQueryUsageNoOpenAPI(t *testing.T) {
	st := &SymbolTable{
		Models:     map[string]ModelSymbol{},
		Operations: map[string]OperationSymbol{},
		DDLTables:  map[string]DDLTable{},
	}
	funcs := []parser.ServiceFunc{{
		Name: "GetCourse", FileName: "get_course.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqGet, Model: "Course.FindByID", Args: []parser.Arg{{Source: "request", Field: "ID"}, {Source: "query"}}, Result: &parser.Result{Type: "Course", Var: "course"}},
		},
	}}
	errs := ValidateWithSymbols(funcs, st)
	assertHasError(t, errs, "OpenAPI에 x-pagination")
}

func TestValidateQueryUsageMatch(t *testing.T) {
	st := &SymbolTable{
		Models:     map[string]ModelSymbol{},
		Operations: map[string]OperationSymbol{
			"ListReservations": {
				XPagination: &XPagination{Style: "offset", DefaultLimit: 20, MaxLimit: 100},
			},
		},
		DDLTables: map[string]DDLTable{},
	}
	funcs := []parser.ServiceFunc{{
		Name: "ListReservations", FileName: "list_reservations.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqGet, Model: "Reservation.List", Args: []parser.Arg{{Source: "query"}}, Result: &parser.Result{Type: "[]Reservation", Var: "reservations"}},
		},
	}}
	errs := ValidateWithSymbols(funcs, st)
	for _, e := range errs {
		if contains(e.Message, "query") {
			t.Errorf("unexpected query validation error: %s", e.Message)
		}
	}
}

// --- helpers ---

func assertHasError(t *testing.T, errs []ValidationError, substr string) {
	t.Helper()
	for _, e := range errs {
		if !e.IsWarning() && contains(e.Message, substr) {
			return
		}
	}
	t.Errorf("expected error containing %q, got %v", substr, errs)
}

func assertHasWarning(t *testing.T, errs []ValidationError, substr string) {
	t.Helper()
	for _, e := range errs {
		if e.IsWarning() && contains(e.Message, substr) {
			return
		}
	}
	t.Errorf("expected warning containing %q, got %v", substr, errs)
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstr(s, substr)
}

func searchSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
