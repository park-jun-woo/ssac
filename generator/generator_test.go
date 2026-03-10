package generator

import (
	"os"
	"strings"
	"testing"

	"github.com/geul-org/ssac/parser"
	"github.com/geul-org/ssac/validator"
)

func TestGenerateGet(t *testing.T) {
	sf := parser.ServiceFunc{
		Name: "GetCourse", FileName: "get_course.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqGet, Model: "Course.FindByID", Args: []parser.Arg{{Source: "request", Field: "CourseID"}}, Result: &parser.Result{Type: "Course", Var: "course"}},
			{Type: parser.SeqResponse, Fields: map[string]string{"course": "course"}},
		},
	}
	code := mustGenerate(t, sf, nil)
	assertContains(t, code, `course, err := courseModel.FindByID(courseID)`)
	assertContains(t, code, `courseID := c.Query("CourseID")`)
	assertContains(t, code, `"course": course`)
}

func TestGeneratePost(t *testing.T) {
	sf := parser.ServiceFunc{
		Name: "CreateSession", FileName: "create_session.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqPost, Model: "Session.Create", Args: []parser.Arg{{Source: "request", Field: "ProjectID"}, {Source: "request", Field: "Command"}}, Result: &parser.Result{Type: "Session", Var: "session"}},
			{Type: parser.SeqResponse, Fields: map[string]string{"session": "session"}},
		},
	}
	code := mustGenerate(t, sf, nil)
	assertContains(t, code, `session, err := sessionModel.Create(projectID, command)`)
	assertContains(t, code, `"session": session`)
}

func TestGeneratePut(t *testing.T) {
	sf := parser.ServiceFunc{
		Name: "UpdateCourse", FileName: "update_course.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqPut, Model: "Course.Update", Args: []parser.Arg{{Source: "request", Field: "Title"}, {Source: "course", Field: "ID"}}},
		},
	}
	code := mustGenerate(t, sf, nil)
	assertContains(t, code, `err := courseModel.Update(title, course.ID)`)
}

func TestGenerateDelete(t *testing.T) {
	sf := parser.ServiceFunc{
		Name: "CancelReservation", FileName: "cancel_reservation.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqGet, Model: "Reservation.FindByID", Args: []parser.Arg{{Source: "request", Field: "ID"}}, Result: &parser.Result{Type: "Reservation", Var: "reservation"}},
			{Type: parser.SeqDelete, Model: "Reservation.Cancel", Args: []parser.Arg{{Source: "reservation", Field: "ID"}}},
		},
	}
	code := mustGenerate(t, sf, nil)
	assertContains(t, code, `err = reservationModel.Cancel(reservation.ID)`)
}

func TestGenerateEmpty(t *testing.T) {
	sf := parser.ServiceFunc{
		Name: "GetCourse", FileName: "get_course.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqGet, Model: "Course.FindByID", Args: []parser.Arg{{Source: "request", Field: "ID"}}, Result: &parser.Result{Type: "Course", Var: "course"}},
			{Type: parser.SeqEmpty, Target: "course", Message: "코스를 찾을 수 없습니다"},
			{Type: parser.SeqResponse, Fields: map[string]string{"course": "course"}},
		},
	}
	code := mustGenerate(t, sf, nil)
	assertContains(t, code, `if course == nil`)
	assertContains(t, code, `코스를 찾을 수 없습니다`)
}

func TestGenerateExists(t *testing.T) {
	sf := parser.ServiceFunc{
		Name: "CreateCourse", FileName: "create_course.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqGet, Model: "Course.FindByTitle", Args: []parser.Arg{{Source: "request", Field: "Title"}}, Result: &parser.Result{Type: "Course", Var: "existing"}},
			{Type: parser.SeqExists, Target: "existing", Message: "이미 존재합니다"},
		},
	}
	code := mustGenerate(t, sf, nil)
	assertContains(t, code, `if existing != nil`)
	assertContains(t, code, `이미 존재합니다`)
}

func TestGenerateState(t *testing.T) {
	sf := parser.ServiceFunc{
		Name: "CancelReservation", FileName: "cancel_reservation.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqGet, Model: "Reservation.FindByID", Args: []parser.Arg{{Source: "request", Field: "ID"}}, Result: &parser.Result{Type: "Reservation", Var: "reservation"}},
			{Type: parser.SeqState, DiagramID: "reservation", Inputs: map[string]string{"status": "reservation.Status"}, Transition: "cancel", Message: "취소할 수 없습니다"},
		},
	}
	code := mustGenerate(t, sf, nil)
	assertContains(t, code, `err := reservationstate.CanTransition(reservationstate.Input{`)
	assertContains(t, code, `Status: reservation.Status`)
	assertContains(t, code, `"cancel"`)
	assertContains(t, code, `err != nil`)
	assertContains(t, code, `err.Error()`)
}

func TestGenerateAuth(t *testing.T) {
	sf := parser.ServiceFunc{
		Name: "DeleteProject", FileName: "delete_project.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqAuth, Action: "delete", Resource: "project", Inputs: map[string]string{"id": "project.ID", "owner": "project.OwnerID"}, Message: "권한 없음"},
		},
	}
	code := mustGenerate(t, sf, nil)
	assertContains(t, code, `authz.Check(currentUser, "delete", "project", authz.Input{`)
	assertContains(t, code, `ID: project.ID`)
	assertContains(t, code, `Owner: project.OwnerID`)
	assertContains(t, code, `currentUser := c.MustGet("currentUser")`)
}

func TestGenerateCallWithResult(t *testing.T) {
	sf := parser.ServiceFunc{
		Name: "Login", FileName: "login.go",
		Imports: []string{"myapp/auth"},
		Sequences: []parser.Sequence{
			{Type: parser.SeqGet, Model: "User.FindByEmail", Args: []parser.Arg{{Source: "request", Field: "Email"}}, Result: &parser.Result{Type: "User", Var: "user"}},
			{Type: parser.SeqCall, Model: "auth.VerifyPassword", Args: []parser.Arg{{Source: "user", Field: "Email"}, {Source: "request", Field: "Password"}}, Result: &parser.Result{Type: "Token", Var: "token"}},
			{Type: parser.SeqResponse, Fields: map[string]string{"token": "token"}},
		},
	}
	code := mustGenerate(t, sf, nil)
	assertContains(t, code, `auth.VerifyPassword(auth.VerifyPasswordRequest{`)
	assertContains(t, code, `user.Email, password`)
	assertContains(t, code, `http.StatusInternalServerError`)
}

func TestGenerateCallWithoutResult(t *testing.T) {
	sf := parser.ServiceFunc{
		Name: "Cancel", FileName: "cancel.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqCall, Model: "notification.Send", Args: []parser.Arg{{Source: "reservation", Field: "ID"}, {Literal: "cancelled"}}},
		},
	}
	code := mustGenerate(t, sf, nil)
	assertContains(t, code, `notification.Send(notification.SendRequest{`)
	assertContains(t, code, `reservation.ID, "cancelled"`)
	assertContains(t, code, `_, err :=`)
	assertContains(t, code, `http.StatusUnauthorized`)
}

func TestGenerateResponse(t *testing.T) {
	sf := parser.ServiceFunc{
		Name: "GetCourse", FileName: "get_course.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqGet, Model: "Course.FindByID", Args: []parser.Arg{{Source: "request", Field: "ID"}}, Result: &parser.Result{Type: "Course", Var: "course"}},
			{Type: parser.SeqGet, Model: "User.FindByID", Args: []parser.Arg{{Source: "course", Field: "InstructorID"}}, Result: &parser.Result{Type: "User", Var: "instructor"}},
			{Type: parser.SeqResponse, Fields: map[string]string{"course": "course", "instructor_name": "instructor.Name"}},
		},
	}
	code := mustGenerate(t, sf, nil)
	assertContains(t, code, `"course":`)
	assertContains(t, code, `"instructor_name":`)
	assertContains(t, code, `instructor.Name`)
}

func TestGenerateCurrentUser(t *testing.T) {
	sf := parser.ServiceFunc{
		Name: "ListMy", FileName: "list_my.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqGet, Model: "Item.ListByUser", Args: []parser.Arg{{Source: "currentUser", Field: "ID"}}, Result: &parser.Result{Type: "[]Item", Var: "items"}},
			{Type: parser.SeqResponse, Fields: map[string]string{"items": "items"}},
		},
	}
	code := mustGenerate(t, sf, nil)
	assertContains(t, code, `currentUser := c.MustGet("currentUser")`)
	assertContains(t, code, `itemModel.ListByUser(currentUser.ID)`)
}

func TestGenerateQueryArg(t *testing.T) {
	st := &validator.SymbolTable{
		Models:     map[string]validator.ModelSymbol{},
		Operations: map[string]validator.OperationSymbol{
			"ListMyReservations": {
				XPagination: &validator.XPagination{Style: "offset", DefaultLimit: 20, MaxLimit: 100},
			},
		},
		DDLTables: map[string]validator.DDLTable{},
	}
	sf := parser.ServiceFunc{
		Name: "ListMyReservations", FileName: "list_my_reservations.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqGet, Model: "Reservation.ListByUserID", Args: []parser.Arg{{Source: "currentUser", Field: "ID"}, {Source: "query"}}, Result: &parser.Result{Type: "[]Reservation", Var: "reservations"}},
			{Type: parser.SeqResponse, Fields: map[string]string{"reservations": "reservations"}},
		},
	}
	code := mustGenerate(t, sf, st)
	assertContains(t, code, `opts := QueryOpts{}`)
	assertContains(t, code, `reservationModel.ListByUserID(currentUser.ID, opts)`)
	assertContains(t, code, `reservations, total, err`)
	assertContains(t, code, `"total":`)
}

func TestGenerateWithPathParam(t *testing.T) {
	st := &validator.SymbolTable{
		Models:    map[string]validator.ModelSymbol{},
		DDLTables: map[string]validator.DDLTable{},
		Operations: map[string]validator.OperationSymbol{
			"GetCourse": {
				PathParams:     []validator.PathParam{{Name: "CourseID", GoType: "int64"}},
				RequestFields:  map[string]bool{"CourseID": true},
				ResponseFields: map[string]bool{"course": true},
			},
		},
	}
	sf := parser.ServiceFunc{
		Name: "GetCourse", FileName: "get_course.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqGet, Model: "Course.FindByID", Args: []parser.Arg{{Source: "request", Field: "CourseID"}}, Result: &parser.Result{Type: "Course", Var: "course"}},
			{Type: parser.SeqResponse, Fields: map[string]string{"course": "course"}},
		},
	}
	code := mustGenerate(t, sf, st)
	assertContains(t, code, `c.Param("CourseID")`)
	assertContains(t, code, `strconv.ParseInt`)
}

func TestGenerateWithJSONBody(t *testing.T) {
	st := &validator.SymbolTable{
		Models: map[string]validator.ModelSymbol{},
		DDLTables: map[string]validator.DDLTable{
			"sessions": {Columns: map[string]string{"project_id": "int64", "command": "string"}},
		},
		Operations: map[string]validator.OperationSymbol{},
	}
	sf := parser.ServiceFunc{
		Name: "CreateSession", FileName: "create_session.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqPost, Model: "Session.Create", Args: []parser.Arg{{Source: "request", Field: "ProjectID"}, {Source: "request", Field: "Command"}}, Result: &parser.Result{Type: "Session", Var: "session"}},
			{Type: parser.SeqResponse, Fields: map[string]string{"session": "session"}},
		},
	}
	code := mustGenerate(t, sf, st)
	assertContains(t, code, `ShouldBindJSON(&req)`)
	assertContains(t, code, `ProjectID int64`)
}

func TestGenerateDomainPackage(t *testing.T) {
	sf := parser.ServiceFunc{
		Name: "Login", FileName: "login.go", Domain: "auth",
		Sequences: []parser.Sequence{
			{Type: parser.SeqGet, Model: "User.FindByEmail", Args: []parser.Arg{{Source: "request", Field: "Email"}}, Result: &parser.Result{Type: "User", Var: "user"}},
			{Type: parser.SeqResponse, Fields: map[string]string{"user": "user"}},
		},
	}
	code := mustGenerate(t, sf, nil)
	assertContains(t, code, "package auth")
}

func TestGenerateFullExample(t *testing.T) {
	sf := parser.ServiceFunc{
		Name: "CancelReservation", FileName: "cancel_reservation.go",
		Imports: []string{"myapp/billing"},
		Sequences: []parser.Sequence{
			{Type: parser.SeqAuth, Action: "cancel", Resource: "reservation", Inputs: map[string]string{"id": "request.ReservationID"}, Message: "권한 없음"},
			{Type: parser.SeqGet, Model: "Reservation.FindByID", Args: []parser.Arg{{Source: "request", Field: "ReservationID"}}, Result: &parser.Result{Type: "Reservation", Var: "reservation"}},
			{Type: parser.SeqEmpty, Target: "reservation", Message: "예약을 찾을 수 없습니다"},
			{Type: parser.SeqState, DiagramID: "reservation", Inputs: map[string]string{"status": "reservation.Status"}, Transition: "cancel", Message: "취소할 수 없습니다"},
			{Type: parser.SeqCall, Model: "billing.CalculateRefund", Args: []parser.Arg{{Source: "reservation", Field: "ID"}, {Source: "reservation", Field: "StartAt"}, {Source: "reservation", Field: "EndAt"}}, Result: &parser.Result{Type: "Refund", Var: "refund"}},
			{Type: parser.SeqPut, Model: "Reservation.UpdateStatus", Args: []parser.Arg{{Source: "request", Field: "ReservationID"}, {Literal: "cancelled"}}},
			{Type: parser.SeqGet, Model: "Reservation.FindByID", Args: []parser.Arg{{Source: "request", Field: "ReservationID"}}, Result: &parser.Result{Type: "Reservation", Var: "reservation"}},
			{Type: parser.SeqResponse, Fields: map[string]string{"reservation": "reservation", "refund": "refund"}},
		},
	}
	code := mustGenerate(t, sf, nil)

	// auth
	assertContains(t, code, `authz.Check(currentUser`)
	// get
	assertContains(t, code, `reservation, err := reservationModel.FindByID`)
	// empty
	assertContains(t, code, `if reservation == nil`)
	// state
	assertContains(t, code, `reservationstate.CanTransition`)
	// call
	assertContains(t, code, `billing.CalculateRefund`)
	// put
	assertContains(t, code, `reservationModel.UpdateStatus`)
	// response
	assertContains(t, code, `"reservation":`)
	assertContains(t, code, `"refund":`)
}

func TestGenerateReAssign(t *testing.T) {
	sf := parser.ServiceFunc{
		Name: "CancelReservation", FileName: "cancel_reservation.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqGet, Model: "Reservation.FindByID", Args: []parser.Arg{{Source: "request", Field: "ID"}}, Result: &parser.Result{Type: "Reservation", Var: "reservation"}},
			{Type: parser.SeqPut, Model: "Reservation.UpdateStatus", Args: []parser.Arg{{Source: "request", Field: "ID"}, {Literal: "cancelled"}}},
			{Type: parser.SeqGet, Model: "Reservation.FindByID", Args: []parser.Arg{{Source: "request", Field: "ID"}}, Result: &parser.Result{Type: "Reservation", Var: "reservation"}},
			{Type: parser.SeqResponse, Fields: map[string]string{"reservation": "reservation"}},
		},
	}
	code := mustGenerate(t, sf, nil)
	// 첫 번째 @get: :=
	assertContains(t, code, `reservation, err := reservationModel.FindByID`)
	// 두 번째 @get: = (재선언)
	assertContains(t, code, `reservation, err = reservationModel.FindByID`)
}

func TestGenerateAuthInputsRequestConversion(t *testing.T) {
	sf := parser.ServiceFunc{
		Name: "CreateReservation", FileName: "create_reservation.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqAuth, Action: "create", Resource: "reservation", Inputs: map[string]string{"id": "request.RoomID"}, Message: "권한 없음"},
		},
	}
	code := mustGenerate(t, sf, nil)
	// request.RoomID → roomID
	assertContains(t, code, `ID: roomID`)
	assertNotContains(t, code, `request.RoomID`)
}

func TestGenerateLiteralArg(t *testing.T) {
	sf := parser.ServiceFunc{
		Name: "Cancel", FileName: "cancel.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqPut, Model: "Reservation.UpdateStatus", Args: []parser.Arg{{Source: "request", Field: "ID"}, {Literal: "cancelled"}}},
		},
	}
	code := mustGenerate(t, sf, nil)
	assertContains(t, code, `reservationModel.UpdateStatus(id, "cancelled")`)
}

func TestGenerateModelInterface(t *testing.T) {
	st := &validator.SymbolTable{
		Models: map[string]validator.ModelSymbol{
			"Course": {Methods: map[string]validator.MethodInfo{
				"FindByID": {Cardinality: "one"},
			}},
		},
		DDLTables: map[string]validator.DDLTable{
			"courses": {Columns: map[string]string{"id": "int64", "title": "string"}},
		},
		Operations: map[string]validator.OperationSymbol{},
	}
	funcs := []parser.ServiceFunc{{
		Name: "GetCourse", FileName: "get_course.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqGet, Model: "Course.FindByID", Args: []parser.Arg{{Source: "request", Field: "CourseID"}}, Result: &parser.Result{Type: "Course", Var: "course"}},
			{Type: parser.SeqResponse, Fields: map[string]string{"course": "course"}},
		},
	}}

	outDir := t.TempDir()
	if err := GenerateModelInterfaces(funcs, st, outDir); err != nil {
		t.Fatal(err)
	}

	data, err := readFile(t, outDir+"/model/models_gen.go")
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, data, "type CourseModel interface")
	assertContains(t, data, "FindByID(")
}

func TestGenerateModelInterfaceQueryOptsExplicit(t *testing.T) {
	st := &validator.SymbolTable{
		Models: map[string]validator.ModelSymbol{
			"Reservation": {Methods: map[string]validator.MethodInfo{
				"ListByUserID": {Cardinality: "many"},
			}},
			"User": {Methods: map[string]validator.MethodInfo{
				"FindByID": {Cardinality: "one"},
			}},
		},
		DDLTables: map[string]validator.DDLTable{
			"reservations": {Columns: map[string]string{"id": "int64", "user_id": "int64"}},
			"users":        {Columns: map[string]string{"id": "int64"}},
		},
		Operations: map[string]validator.OperationSymbol{},
	}
	funcs := []parser.ServiceFunc{{
		Name: "ListMyReservations", FileName: "list_my_reservations.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqGet, Model: "User.FindByID", Args: []parser.Arg{{Source: "currentUser", Field: "ID"}}, Result: &parser.Result{Type: "User", Var: "user"}},
			{Type: parser.SeqGet, Model: "Reservation.ListByUserID", Args: []parser.Arg{{Source: "currentUser", Field: "ID"}, {Source: "query"}}, Result: &parser.Result{Type: "[]Reservation", Var: "reservations"}},
			{Type: parser.SeqResponse, Fields: map[string]string{"reservations": "reservations"}},
		},
	}}

	outDir := t.TempDir()
	if err := GenerateModelInterfaces(funcs, st, outDir); err != nil {
		t.Fatal(err)
	}

	data, err := readFile(t, outDir+"/model/models_gen.go")
	if err != nil {
		t.Fatal(err)
	}
	// query arg가 있는 메서드에만 opts QueryOpts 추가
	assertContains(t, data, "ListByUserID(id int64, opts QueryOpts)")
	// query arg가 없는 메서드에는 opts 없음
	assertNotContains(t, data, "FindByID(id int64, opts QueryOpts)")
	assertContains(t, data, "FindByID(id int64)")
}

// --- helpers ---

func mustGenerate(t *testing.T, sf parser.ServiceFunc, st *validator.SymbolTable) string {
	t.Helper()
	code, err := GenerateFunc(sf, st)
	if err != nil {
		t.Fatalf("GenerateFunc failed: %v", err)
	}
	return string(code)
}

func assertContains(t *testing.T, code, substr string) {
	t.Helper()
	if !strings.Contains(code, substr) {
		t.Errorf("expected code to contain %q\n--- code ---\n%s", substr, code)
	}
}

func assertNotContains(t *testing.T, code, substr string) {
	t.Helper()
	if strings.Contains(code, substr) {
		t.Errorf("expected code NOT to contain %q\n--- code ---\n%s", substr, code)
	}
}

func readFile(t *testing.T, path string) (string, error) {
	t.Helper()
	data, err := os.ReadFile(path)
	return string(data), err
}
