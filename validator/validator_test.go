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
			Type:   parser.SeqGet,
			Model:  "Course.FindByID",
			Inputs: map[string]string{"CourseID": "request.CourseID"},
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
			Inputs: map[string]string{"Title": "request.Title"},
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
			{Type: parser.SeqGet, Model: "Course.FindByID", Inputs: map[string]string{"ID": "request.ID"}, Result: &parser.Result{Type: "Course", Var: "course"}},
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

func TestValidateCallPrimitiveReturnType(t *testing.T) {
	for _, typ := range []string{"string", "int", "int64", "bool", "float64", "time.Time"} {
		t.Run(typ, func(t *testing.T) {
			funcs := []parser.ServiceFunc{{
				Name: "Login", FileName: "login.go",
				Sequences: []parser.Sequence{{
					Type:   parser.SeqCall,
					Model:  "auth.IssueToken",
					Inputs: map[string]string{"UserID": "user.ID"},
					Result: &parser.Result{Type: typ, Var: "token"},
				}},
			}}
			errs := Validate(funcs)
			assertHasError(t, errs, "기본 타입")
			assertHasError(t, errs, "Response struct 타입을 지정하세요")
		})
	}
}

func TestValidateCallStructReturnTypeOK(t *testing.T) {
	funcs := []parser.ServiceFunc{{
		Name: "Login", FileName: "login.go",
		Sequences: []parser.Sequence{{
			Type:   parser.SeqCall,
			Model:  "auth.IssueToken",
			Inputs: map[string]string{"UserID": "user.ID"},
			Result: &parser.Result{Type: "TokenResponse", Var: "token"},
		}},
	}}
	errs := Validate(funcs)
	for _, e := range errs {
		if contains(e.Message, "기본 타입") {
			t.Errorf("unexpected primitive type error: %s", e.Message)
		}
	}
}

func TestValidateCallNoResultOK(t *testing.T) {
	funcs := []parser.ServiceFunc{{
		Name: "Notify", FileName: "notify.go",
		Sequences: []parser.Sequence{{
			Type:   parser.SeqCall,
			Model:  "notification.Send",
			Inputs: map[string]string{"ID": "reservation.ID"},
		}},
	}}
	errs := Validate(funcs)
	for _, e := range errs {
		if contains(e.Message, "기본 타입") {
			t.Errorf("unexpected primitive type error: %s", e.Message)
		}
	}
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
			{Type: parser.SeqGet, Model: "Course.FindByID", Inputs: map[string]string{"ID": "request.ID"}, Result: &parser.Result{Type: "Course", Var: "course"}},
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
			{Type: parser.SeqGet, Model: "Item.ListByUser", Inputs: map[string]string{"ID": "currentUser.ID"}, Result: &parser.Result{Type: "[]Item", Var: "items"}},
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
			{Type: parser.SeqGet, Model: "User.FindByID", Inputs: map[string]string{"InstructorID": "course.InstructorID"}, Result: &parser.Result{Type: "User", Var: "user"}},
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
			{Type: parser.SeqGet, Model: "Reservation.FindByID", Inputs: map[string]string{"ID": "request.ID"}, Result: &parser.Result{Type: "Reservation", Var: "reservation"}},
			{Type: parser.SeqPut, Model: "Reservation.UpdateStatus", Inputs: map[string]string{"ID": "request.ID", "Status": `"cancelled"`}},
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
			{Type: parser.SeqGet, Model: "Reservation.FindByID", Inputs: map[string]string{"ID": "request.ID"}, Result: &parser.Result{Type: "Reservation", Var: "reservation"}},
			{Type: parser.SeqPut, Model: "Reservation.UpdateStatus", Inputs: map[string]string{"ID": "request.ID", "Status": `"cancelled"`}},
			{Type: parser.SeqGet, Model: "Reservation.FindByID", Inputs: map[string]string{"ID": "request.ID"}, Result: &parser.Result{Type: "Reservation", Var: "reservation"}},
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
			{Type: parser.SeqGet, Model: "Course.FindByID", Inputs: map[string]string{"ID": "request.ID"}, Result: &parser.Result{Type: "Course", Var: "course"}},
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
			{Type: parser.SeqGet, Model: "Course.FindByID", Inputs: map[string]string{"ID": "request.ID"}, Result: &parser.Result{Type: "Course", Var: "course"}},
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
			{Type: parser.SeqCall, Model: "auth.VerifyPassword", Inputs: map[string]string{"Password": "request.Password"}, Result: &parser.Result{Type: "Token", Var: "token"}},
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
			{Type: parser.SeqGet, Model: "Course.FindByID", Inputs: map[string]string{"UnknownField": "request.UnknownField"}, Result: &parser.Result{Type: "Course", Var: "course"}},
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
			{Type: parser.SeqGet, Model: "Course.FindByID", Inputs: map[string]string{"CourseID": "request.CourseID"}, Result: &parser.Result{Type: "Course", Var: "course"}},
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
			{Type: parser.SeqGet, Model: "Course.FindByID", Inputs: map[string]string{"CourseID": "request.CourseID"}, Result: &parser.Result{Type: "Course", Var: "course"}},
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
			{Type: parser.SeqGet, Model: "Course.FindByID", Inputs: map[string]string{"CourseID": "request.CourseID"}, Result: &parser.Result{Type: "Course", Var: "course"}},
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
			{Type: parser.SeqGet, Model: "Course.FindByID", Inputs: map[string]string{"CourseID": "request.CourseID"}, Result: &parser.Result{Type: "Course", Var: "course"}},
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
			{Type: parser.SeqGet, Model: "Item.List", Inputs: map[string]string{"Dummy": "request.dummy"}, Result: &parser.Result{Type: "[]Item", Var: "items"}},
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
			{Type: parser.SeqGet, Model: "Reservation.FindByID", Inputs: map[string]string{"ID": "request.ID"}, Result: &parser.Result{Type: "Reservation", Var: "reservation"}},
			{Type: parser.SeqPut, Model: "Reservation.UpdateStatus", Inputs: map[string]string{"ID": "request.ID", "Status": `"cancelled"`}},
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
					{Type: parser.SeqGet, Model: "User.FindByID", Inputs: map[string]string{"ID": "request.ID"}, Result: &parser.Result{Type: "User", Var: name}},
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
			{Type: parser.SeqGet, Model: "User.FindByID", Inputs: map[string]string{"ID": "request.ID"}, Result: &parser.Result{Type: "User", Var: "user"}},
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
			{Type: parser.SeqGet, Model: "Reservation.List", Result: &parser.Result{Type: "[]Reservation", Var: "reservations"}},
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
			{Type: parser.SeqGet, Model: "Course.FindByID", Inputs: map[string]string{"ID": "request.ID", "Opts": "query"}, Result: &parser.Result{Type: "Course", Var: "course"}},
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
			{Type: parser.SeqGet, Model: "Reservation.List", Inputs: map[string]string{"Opts": "query"}, Result: &parser.Result{Type: "[]Reservation", Var: "reservations"}},
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

func assertNoErrors(t *testing.T, errs []ValidationError) {
	t.Helper()
	var errors []ValidationError
	for _, e := range errs {
		if e.Level != "WARNING" {
			errors = append(errors, e)
		}
	}
	if len(errors) > 0 {
		t.Errorf("expected no errors, got %d: %v", len(errors), errors)
	}
}

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

// --- 외부 검증: 패키지 접두사 모델 ---

func TestValidatePackageModelMethodExists(t *testing.T) {
	st := &SymbolTable{
		Models: map[string]ModelSymbol{
			"session.Session": {Methods: map[string]MethodInfo{"Get": {}, "Set": {}, "Delete": {}}},
		},
		Operations: map[string]OperationSymbol{},
		DDLTables:  map[string]DDLTable{},
	}
	funcs := []parser.ServiceFunc{{
		Name: "GetSession", FileName: "get_session.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqGet, Package: "session", Model: "Session.Get", Inputs: map[string]string{"token": "request.Token"}, Result: &parser.Result{Type: "Session", Var: "session"}},
		},
	}}
	errs := ValidateWithSymbols(funcs, st)
	assertNoErrors(t, errs)
}

func TestValidatePackageModelMethodMissing(t *testing.T) {
	st := &SymbolTable{
		Models: map[string]ModelSymbol{
			"session.Session": {Methods: map[string]MethodInfo{"Get": {}, "Set": {}, "Delete": {}}},
		},
		Operations: map[string]OperationSymbol{},
		DDLTables:  map[string]DDLTable{},
	}
	funcs := []parser.ServiceFunc{{
		Name: "GetSession", FileName: "get_session.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqGet, Package: "session", Model: "Session.Gett", Inputs: map[string]string{"token": "request.Token"}, Result: &parser.Result{Type: "Session", Var: "session"}},
		},
	}}
	errs := ValidateWithSymbols(funcs, st)
	assertHasError(t, errs, `메서드 "Gett" 없음`)
	assertHasError(t, errs, "사용 가능: Delete, Get, Set")
}

func TestValidatePackageModelNoInterface(t *testing.T) {
	st := &SymbolTable{
		Models:     map[string]ModelSymbol{},
		Operations: map[string]OperationSymbol{},
		DDLTables:  map[string]DDLTable{},
	}
	funcs := []parser.ServiceFunc{{
		Name: "GetSession", FileName: "get_session.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqGet, Package: "session", Model: "Session.Get", Inputs: map[string]string{"token": "request.Token"}, Result: &parser.Result{Type: "Session", Var: "session"}},
		},
	}}
	errs := ValidateWithSymbols(funcs, st)
	assertHasWarning(t, errs, "패키지 interface를 찾을 수 없습니다")
}

func TestValidatePackageModelSkipDDL(t *testing.T) {
	// 패키지 모델은 DDL 모델 체크를 하지 않음 — "Session" DDL 테이블 없어도 에러 아님
	st := &SymbolTable{
		Models: map[string]ModelSymbol{
			"session.Session": {Methods: map[string]MethodInfo{"Get": {}}},
		},
		Operations: map[string]OperationSymbol{},
		DDLTables:  map[string]DDLTable{}, // Session DDL 없음
	}
	funcs := []parser.ServiceFunc{{
		Name: "GetSession", FileName: "get_session.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqGet, Package: "session", Model: "Session.Get", Inputs: map[string]string{"token": "request.Token"}, Result: &parser.Result{Type: "Session", Var: "session"}},
		},
	}}
	errs := ValidateWithSymbols(funcs, st)
	assertNoErrors(t, errs)
}

// --- 외부 검증: 패키지 모델 파라미터 매칭 ---

func TestValidatePackageParamMatch(t *testing.T) {
	st := &SymbolTable{
		Models: map[string]ModelSymbol{
			"session.Session": {Methods: map[string]MethodInfo{"Get": {Params: []string{"key"}}}},
		},
		Operations: map[string]OperationSymbol{},
		DDLTables:  map[string]DDLTable{},
	}
	funcs := []parser.ServiceFunc{{
		Name: "GetSession", FileName: "get_session.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqGet, Package: "session", Model: "Session.Get", Inputs: map[string]string{"key": "request.Token"}, Result: &parser.Result{Type: "Session", Var: "session"}},
		},
	}}
	errs := ValidateWithSymbols(funcs, st)
	assertNoErrors(t, errs)
}

func TestValidatePackageParamExtra(t *testing.T) {
	st := &SymbolTable{
		Models: map[string]ModelSymbol{
			"session.Session": {Methods: map[string]MethodInfo{"Get": {Params: []string{"key"}}}},
		},
		Operations: map[string]OperationSymbol{},
		DDLTables:  map[string]DDLTable{},
	}
	funcs := []parser.ServiceFunc{{
		Name: "GetSession", FileName: "get_session.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqGet, Package: "session", Model: "Session.Get", Inputs: map[string]string{"token": "request.Token"}, Result: &parser.Result{Type: "Session", Var: "session"}},
		},
	}}
	errs := ValidateWithSymbols(funcs, st)
	assertHasError(t, errs, `SSaC에 "token"가 있지만 interface에 없습니다`)
	assertHasError(t, errs, "interface 파라미터: [key]")
}

func TestValidatePackageParamMissing(t *testing.T) {
	st := &SymbolTable{
		Models: map[string]ModelSymbol{
			"session.Session": {Methods: map[string]MethodInfo{"Set": {Params: []string{"key", "value", "ttl"}}}},
		},
		Operations: map[string]OperationSymbol{},
		DDLTables:  map[string]DDLTable{},
	}
	funcs := []parser.ServiceFunc{{
		Name: "CreateSession", FileName: "create_session.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqPost, Package: "session", Model: "Session.Set", Inputs: map[string]string{"key": "userID", "value": "userData"}, Result: &parser.Result{Type: "Session", Var: "result"}},
		},
	}}
	errs := ValidateWithSymbols(funcs, st)
	assertHasError(t, errs, `interface에 "ttl"가 필요하지만 SSaC에 없습니다`)
	assertHasError(t, errs, "SSaC 파라미터: [key, value]")
}

func TestValidatePackageParamSkipContext(t *testing.T) {
	// Params에 context.Context가 제외된 상태로 저장됨을 전제
	st := &SymbolTable{
		Models: map[string]ModelSymbol{
			"session.Session": {Methods: map[string]MethodInfo{"Get": {Params: []string{"key"}}}},
		},
		Operations: map[string]OperationSymbol{},
		DDLTables:  map[string]DDLTable{},
	}
	funcs := []parser.ServiceFunc{{
		Name: "GetSession", FileName: "get_session.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqGet, Package: "session", Model: "Session.Get", Inputs: map[string]string{"key": "request.Token"}, Result: &parser.Result{Type: "Session", Var: "session"}},
		},
	}}
	errs := ValidateWithSymbols(funcs, st)
	assertNoErrors(t, errs)
}

// --- 외부 검증: pagination type ---

func TestValidatePaginationOffsetWithPage(t *testing.T) {
	st := &SymbolTable{
		Models:     map[string]ModelSymbol{"Gig": {Methods: map[string]MethodInfo{"List": {Cardinality: "many"}}}},
		DDLTables:  map[string]DDLTable{},
		Operations: map[string]OperationSymbol{
			"ListGigs": {XPagination: &XPagination{Style: "offset"}},
		},
	}
	funcs := []parser.ServiceFunc{{
		Name: "ListGigs", FileName: "list_gigs.go",
		Sequences: []parser.Sequence{{
			Type: parser.SeqGet, Model: "Gig.List",
			Result: &parser.Result{Type: "Gig", Var: "gigPage", Wrapper: "Page"},
		}},
	}}
	errs := ValidateWithSymbols(funcs, st)
	assertNoErrors(t, errs)
}

func TestValidatePaginationOffsetWithSlice(t *testing.T) {
	st := &SymbolTable{
		Models:     map[string]ModelSymbol{"Gig": {Methods: map[string]MethodInfo{"List": {Cardinality: "many"}}}},
		DDLTables:  map[string]DDLTable{},
		Operations: map[string]OperationSymbol{
			"ListGigs": {XPagination: &XPagination{Style: "offset"}},
		},
	}
	funcs := []parser.ServiceFunc{{
		Name: "ListGigs", FileName: "list_gigs.go",
		Sequences: []parser.Sequence{{
			Type: parser.SeqGet, Model: "Gig.List",
			Result: &parser.Result{Type: "[]Gig", Var: "gigs"},
		}},
	}}
	errs := ValidateWithSymbols(funcs, st)
	assertHasError(t, errs, "Page[T]가 아닙니다")
}

func TestValidatePaginationCursorMismatch(t *testing.T) {
	st := &SymbolTable{
		Models:     map[string]ModelSymbol{"Gig": {Methods: map[string]MethodInfo{"List": {Cardinality: "many"}}}},
		DDLTables:  map[string]DDLTable{},
		Operations: map[string]OperationSymbol{
			"ListGigs": {XPagination: &XPagination{Style: "cursor"}},
		},
	}
	funcs := []parser.ServiceFunc{{
		Name: "ListGigs", FileName: "list_gigs.go",
		Sequences: []parser.Sequence{{
			Type: parser.SeqGet, Model: "Gig.List",
			Result: &parser.Result{Type: "Gig", Var: "gigPage", Wrapper: "Page"},
		}},
	}}
	errs := ValidateWithSymbols(funcs, st)
	assertHasError(t, errs, "Cursor[T]가 아닙니다")
}

func TestValidateNoPaginationWithWrapper(t *testing.T) {
	st := &SymbolTable{
		Models:     map[string]ModelSymbol{"Gig": {Methods: map[string]MethodInfo{"List": {Cardinality: "many"}}}},
		DDLTables:  map[string]DDLTable{},
		Operations: map[string]OperationSymbol{
			"ListGigs": {},
		},
	}
	funcs := []parser.ServiceFunc{{
		Name: "ListGigs", FileName: "list_gigs.go",
		Sequences: []parser.Sequence{{
			Type: parser.SeqGet, Model: "Gig.List",
			Result: &parser.Result{Type: "Gig", Var: "gigPage", Wrapper: "Page"},
		}},
	}}
	errs := ValidateWithSymbols(funcs, st)
	assertHasError(t, errs, "x-pagination이 없지만")
}

func TestValidateResponseDirectPageFieldsMatch(t *testing.T) {
	st := &SymbolTable{
		Models:    map[string]ModelSymbol{"Gig": {Methods: map[string]MethodInfo{"List": {Cardinality: "many"}}}},
		DDLTables: map[string]DDLTable{},
		Operations: map[string]OperationSymbol{
			"ListGigs": {
				XPagination:    &XPagination{Style: "offset"},
				ResponseFields: map[string]bool{"items": true, "total": true},
			},
		},
	}
	funcs := []parser.ServiceFunc{{
		Name: "ListGigs", FileName: "list_gigs.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqGet, Model: "Gig.List", Inputs: map[string]string{"Query": "query"}, Result: &parser.Result{Type: "Gig", Var: "gigPage", Wrapper: "Page"}},
			{Type: parser.SeqResponse, Target: "gigPage"},
		},
	}}
	errs := ValidateWithSymbols(funcs, st)
	assertNoErrors(t, errs)
}

func TestValidateResponseDirectPageFieldsMissing(t *testing.T) {
	st := &SymbolTable{
		Models:    map[string]ModelSymbol{"Gig": {Methods: map[string]MethodInfo{"List": {Cardinality: "many"}}}},
		DDLTables: map[string]DDLTable{},
		Operations: map[string]OperationSymbol{
			"ListGigs": {
				XPagination:    &XPagination{Style: "offset"},
				ResponseFields: map[string]bool{"data": true, "count": true},
			},
		},
	}
	funcs := []parser.ServiceFunc{{
		Name: "ListGigs", FileName: "list_gigs.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqGet, Model: "Gig.List", Inputs: map[string]string{"Query": "query"}, Result: &parser.Result{Type: "Gig", Var: "gigPage", Wrapper: "Page"}},
			{Type: parser.SeqResponse, Target: "gigPage"},
		},
	}}
	errs := ValidateWithSymbols(funcs, st)
	assertHasError(t, errs, `"items" 필드가 없습니다`)
}

// --- 내부 검증: @publish / @subscribe ---

func TestValidatePublishTopicMissing(t *testing.T) {
	funcs := []parser.ServiceFunc{{
		Name: "Publish", FileName: "publish.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqPublish, Inputs: map[string]string{"OrderID": "order.ID"}},
		},
	}}
	errs := Validate(funcs)
	assertHasError(t, errs, "Topic 누락")
}

func TestValidatePublishPayloadMissing(t *testing.T) {
	funcs := []parser.ServiceFunc{{
		Name: "Publish", FileName: "publish.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqPublish, Topic: "order.completed"},
		},
	}}
	errs := Validate(funcs)
	assertHasError(t, errs, "Payload 누락")
}

func TestValidateSubscribeWithResponse(t *testing.T) {
	funcs := []parser.ServiceFunc{{
		Name: "OnOrder", FileName: "on_order.go",
		Subscribe: &parser.SubscribeInfo{Topic: "order.completed", MessageType: "Msg"},
		Param:     &parser.ParamInfo{TypeName: "Msg", VarName: "message"},
		Structs:   []parser.StructInfo{{Name: "Msg", Fields: []parser.StructField{{Name: "OrderID", Type: "int64"}}}},
		Sequences: []parser.Sequence{
			{Type: parser.SeqGet, Model: "Order.FindByID", Inputs: map[string]string{"ID": "message.OrderID"}, Result: &parser.Result{Type: "Order", Var: "order"}},
			{Type: parser.SeqResponse, Fields: map[string]string{"order": "order"}},
		},
	}}
	errs := Validate(funcs)
	assertHasError(t, errs, "@subscribe 함수에 @response를 사용할 수 없습니다")
}

func TestValidateSubscribeWithRequest(t *testing.T) {
	funcs := []parser.ServiceFunc{{
		Name: "OnOrder", FileName: "on_order.go",
		Subscribe: &parser.SubscribeInfo{Topic: "order.completed", MessageType: "Msg"},
		Param:     &parser.ParamInfo{TypeName: "Msg", VarName: "message"},
		Structs:   []parser.StructInfo{{Name: "Msg", Fields: []parser.StructField{{Name: "OrderID", Type: "int64"}}}},
		Sequences: []parser.Sequence{
			{Type: parser.SeqGet, Model: "Order.FindByID", Inputs: map[string]string{"ID": "request.OrderID"}, Result: &parser.Result{Type: "Order", Var: "order"}},
		},
	}}
	errs := Validate(funcs)
	assertHasError(t, errs, "@subscribe 함수에서 request를 사용할 수 없습니다")
}

func TestValidateHTTPWithMessage(t *testing.T) {
	funcs := []parser.ServiceFunc{{
		Name: "GetOrder", FileName: "get_order.go",
		// Subscribe nil → HTTP trigger
		Sequences: []parser.Sequence{
			{Type: parser.SeqGet, Model: "Order.FindByID", Inputs: map[string]string{"ID": "message.OrderID"}, Result: &parser.Result{Type: "Order", Var: "order"}},
		},
	}}
	errs := Validate(funcs)
	assertHasError(t, errs, "HTTP 함수에서 message를 사용할 수 없습니다")
}

func TestValidateMessageReservedSource(t *testing.T) {
	funcs := []parser.ServiceFunc{{
		Name: "Test", FileName: "test.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqGet, Model: "User.FindByID", Inputs: map[string]string{"ID": "request.ID"}, Result: &parser.Result{Type: "User", Var: "message"}},
		},
	}}
	errs := Validate(funcs)
	assertHasError(t, errs, "예약 소스이므로 result 변수명으로 사용할 수 없습니다")
}

func TestValidateSubscribeMessageVariable(t *testing.T) {
	// subscribe 함수에서 message는 선언 없이 사용 가능
	funcs := []parser.ServiceFunc{{
		Name: "OnOrder", FileName: "on_order.go",
		Subscribe: &parser.SubscribeInfo{Topic: "order.completed", MessageType: "Msg"},
		Param:     &parser.ParamInfo{TypeName: "Msg", VarName: "message"},
		Structs:   []parser.StructInfo{{Name: "Msg", Fields: []parser.StructField{{Name: "OrderID", Type: "int64"}}}},
		Sequences: []parser.Sequence{
			{Type: parser.SeqGet, Model: "Order.FindByID", Inputs: map[string]string{"ID": "message.OrderID"}, Result: &parser.Result{Type: "Order", Var: "order"}},
		},
	}}
	errs := Validate(funcs)
	for _, e := range errs {
		if contains(e.Message, `"message" 변수가 아직 선언되지 않았습니다`) {
			t.Errorf("message should be pre-declared in subscribe func: %s", e.Message)
		}
	}
}

func TestValidateSubscribeNoParam(t *testing.T) {
	funcs := []parser.ServiceFunc{{
		Name: "OnOrder", FileName: "on_order.go",
		Subscribe: &parser.SubscribeInfo{Topic: "order.completed", MessageType: "Msg"},
		Structs:   []parser.StructInfo{{Name: "Msg", Fields: []parser.StructField{{Name: "OrderID", Type: "int64"}}}},
		// Param nil — 파라미터 누락
		Sequences: []parser.Sequence{
			{Type: parser.SeqGet, Model: "Order.FindByID", Inputs: map[string]string{"ID": "message.OrderID"}, Result: &parser.Result{Type: "Order", Var: "order"}},
		},
	}}
	errs := Validate(funcs)
	assertHasError(t, errs, "@subscribe 함수에 파라미터가 필요합니다")
}

func TestValidateSubscribeWrongVarName(t *testing.T) {
	funcs := []parser.ServiceFunc{{
		Name: "OnOrder", FileName: "on_order.go",
		Subscribe: &parser.SubscribeInfo{Topic: "order.completed", MessageType: "Msg"},
		Param:     &parser.ParamInfo{TypeName: "Msg", VarName: "msg"},
		Structs:   []parser.StructInfo{{Name: "Msg", Fields: []parser.StructField{{Name: "OrderID", Type: "int64"}}}},
		Sequences: []parser.Sequence{
			{Type: parser.SeqGet, Model: "Order.FindByID", Inputs: map[string]string{"ID": "message.OrderID"}, Result: &parser.Result{Type: "Order", Var: "order"}},
		},
	}}
	errs := Validate(funcs)
	assertHasError(t, errs, "파라미터 변수명은 \"message\"여야 합니다")
}

func TestValidateSubscribeTypeNotFound(t *testing.T) {
	funcs := []parser.ServiceFunc{{
		Name: "OnOrder", FileName: "on_order.go",
		Subscribe: &parser.SubscribeInfo{Topic: "order.completed", MessageType: "NonExistentMsg"},
		Param:     &parser.ParamInfo{TypeName: "NonExistentMsg", VarName: "message"},
		Structs:   []parser.StructInfo{}, // struct 없음
		Sequences: []parser.Sequence{
			{Type: parser.SeqGet, Model: "Order.FindByID", Inputs: map[string]string{"ID": "message.OrderID"}, Result: &parser.Result{Type: "Order", Var: "order"}},
		},
	}}
	errs := Validate(funcs)
	assertHasError(t, errs, "struct로 선언되지 않았습니다")
}

func TestValidateSubscribeFieldNotFound(t *testing.T) {
	funcs := []parser.ServiceFunc{{
		Name: "OnOrder", FileName: "on_order.go",
		Subscribe: &parser.SubscribeInfo{Topic: "order.completed", MessageType: "Msg"},
		Param:     &parser.ParamInfo{TypeName: "Msg", VarName: "message"},
		Structs:   []parser.StructInfo{{Name: "Msg", Fields: []parser.StructField{{Name: "OrderID", Type: "int64"}}}},
		Sequences: []parser.Sequence{
			{Type: parser.SeqGet, Model: "Order.FindByID", Inputs: map[string]string{"ID": "message.Email"}, Result: &parser.Result{Type: "Order", Var: "order"}},
		},
	}}
	errs := Validate(funcs)
	assertHasError(t, errs, "메시지 타입 \"Msg\"에 \"Email\" 필드가 없습니다")
}

func TestValidateSubscribeFieldOK(t *testing.T) {
	funcs := []parser.ServiceFunc{{
		Name: "OnOrder", FileName: "on_order.go",
		Subscribe: &parser.SubscribeInfo{Topic: "order.completed", MessageType: "Msg"},
		Param:     &parser.ParamInfo{TypeName: "Msg", VarName: "message"},
		Structs:   []parser.StructInfo{{Name: "Msg", Fields: []parser.StructField{{Name: "OrderID", Type: "int64"}, {Name: "Email", Type: "string"}}}},
		Sequences: []parser.Sequence{
			{Type: parser.SeqGet, Model: "Order.FindByID", Inputs: map[string]string{"ID": "message.OrderID"}, Result: &parser.Result{Type: "Order", Var: "order"}},
			{Type: parser.SeqPut, Model: "Order.Update", Inputs: map[string]string{"Email": "message.Email"}},
		},
	}}
	errs := Validate(funcs)
	for _, e := range errs {
		if contains(e.Message, "필드가 없습니다") {
			t.Errorf("unexpected field error: %s", e.Message)
		}
	}
}

// --- 외부 검증: Go 예약어 ---

func TestValidateGoReservedWordInInputs(t *testing.T) {
	st := &SymbolTable{
		Models:     map[string]ModelSymbol{"Transaction": {Methods: map[string]MethodInfo{"Create": {Cardinality: "exec"}}}},
		Operations: map[string]OperationSymbol{},
		DDLTables: map[string]DDLTable{
			"transactions": {Columns: map[string]string{"type": "string", "amount": "int64", "gig_id": "int64"}},
		},
	}
	funcs := []parser.ServiceFunc{{
		Name: "CreateTransaction", FileName: "create_transaction.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqPost, Model: "Transaction.Create", Inputs: map[string]string{"amount": "request.Amount", "gigID": "request.GigID", "type": "request.Type"}, Result: &parser.Result{Type: "Transaction", Var: "tx"}},
		},
	}}
	errs := ValidateWithSymbols(funcs, st)
	assertHasError(t, errs, `DDL column "type" in table "transactions" is a Go reserved word`)
}

func TestValidateGoReservedWordNoConflict(t *testing.T) {
	st := &SymbolTable{
		Models:     map[string]ModelSymbol{"Transaction": {Methods: map[string]MethodInfo{"Create": {Cardinality: "exec"}}}},
		Operations: map[string]OperationSymbol{},
		DDLTables: map[string]DDLTable{
			"transactions": {Columns: map[string]string{"tx_type": "string", "amount": "int64"}},
		},
	}
	funcs := []parser.ServiceFunc{{
		Name: "CreateTransaction", FileName: "create_transaction.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqPost, Model: "Transaction.Create", Inputs: map[string]string{"amount": "request.Amount", "txType": "request.TxType"}, Result: &parser.Result{Type: "Transaction", Var: "tx"}},
		},
	}}
	errs := ValidateWithSymbols(funcs, st)
	assertNoErrors(t, errs)
}

func TestValidateGoReservedWordRange(t *testing.T) {
	st := &SymbolTable{
		Models:     map[string]ModelSymbol{"Schedule": {Methods: map[string]MethodInfo{"Create": {Cardinality: "exec"}}}},
		Operations: map[string]OperationSymbol{},
		DDLTables: map[string]DDLTable{
			"schedules": {Columns: map[string]string{"range": "string", "name": "string"}},
		},
	}
	funcs := []parser.ServiceFunc{{
		Name: "CreateSchedule", FileName: "create_schedule.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqPost, Model: "Schedule.Create", Inputs: map[string]string{"range": "request.Range", "name": "request.Name"}, Result: &parser.Result{Type: "Schedule", Var: "schedule"}},
		},
	}}
	errs := ValidateWithSymbols(funcs, st)
	assertHasError(t, errs, `DDL column "range" in table "schedules" is a Go reserved word`)
}
