package generator

import (
	"strings"
	"testing"

	"github.com/geul-org/ssac/parser"
	"github.com/geul-org/ssac/validator"
)

func TestGenerateGet(t *testing.T) {
	sf := parser.ServiceFunc{
		Name: "GetCourse", FileName: "get_course.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqGet, Model: "Course.FindByID", Inputs: map[string]string{"CourseID": "request.CourseID"}, Result: &parser.Result{Type: "Course", Var: "course"}},
			{Type: parser.SeqResponse, Fields: map[string]string{"course": "course"}},
		},
	}
	code := mustGenerate(t, sf, nil)
	assertContains(t, code, `course, err := h.CourseModel.FindByID(courseID)`)
	assertContains(t, code, `courseID := c.Query("CourseID")`)
	assertContains(t, code, `"course": course`)
}

func TestGeneratePost(t *testing.T) {
	sf := parser.ServiceFunc{
		Name: "CreateSession", FileName: "create_session.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqPost, Model: "Session.Create", Inputs: map[string]string{"ProjectID": "request.ProjectID", "Command": "request.Command"}, Result: &parser.Result{Type: "Session", Var: "session"}},
			{Type: parser.SeqResponse, Fields: map[string]string{"session": "session"}},
		},
	}
	code := mustGenerate(t, sf, nil)
	assertContains(t, code, `session, err := h.SessionModel.WithTx(tx).Create(command, projectID)`)
	assertContains(t, code, `"session": session`)
	assertContains(t, code, `h.DB.BeginTx`)
	assertContains(t, code, `tx.Commit()`)
}

func TestGeneratePut(t *testing.T) {
	sf := parser.ServiceFunc{
		Name: "UpdateCourse", FileName: "update_course.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqPut, Model: "Course.Update", Inputs: map[string]string{"Title": "request.Title", "ID": "course.ID"}},
		},
	}
	code := mustGenerate(t, sf, nil)
	assertContains(t, code, `err = h.CourseModel.WithTx(tx).Update(course.ID, title)`)
	assertContains(t, code, `h.DB.BeginTx`)
}

func TestGenerateDelete(t *testing.T) {
	sf := parser.ServiceFunc{
		Name: "CancelReservation", FileName: "cancel_reservation.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqGet, Model: "Reservation.FindByID", Inputs: map[string]string{"ID": "request.ID"}, Result: &parser.Result{Type: "Reservation", Var: "reservation"}},
			{Type: parser.SeqDelete, Model: "Reservation.Cancel", Inputs: map[string]string{"ID": "reservation.ID"}},
		},
	}
	code := mustGenerate(t, sf, nil)
	assertContains(t, code, `err = h.ReservationModel.WithTx(tx).Cancel(reservation.ID)`)
}

func TestGenerateEmpty(t *testing.T) {
	sf := parser.ServiceFunc{
		Name: "GetCourse", FileName: "get_course.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqGet, Model: "Course.FindByID", Inputs: map[string]string{"ID": "request.ID"}, Result: &parser.Result{Type: "Course", Var: "course"}},
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
			{Type: parser.SeqGet, Model: "Course.FindByTitle", Inputs: map[string]string{"Title": "request.Title"}, Result: &parser.Result{Type: "Course", Var: "existing"}},
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
			{Type: parser.SeqGet, Model: "Reservation.FindByID", Inputs: map[string]string{"ID": "request.ID"}, Result: &parser.Result{Type: "Reservation", Var: "reservation"}},
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
	assertContains(t, code, `authz.Check(authz.CheckRequest{Action: "delete", Resource: "project"`)
	assertContains(t, code, `ID: project.ID`)
	assertContains(t, code, `Owner: project.OwnerID`)
	assertContains(t, code, `http.StatusForbidden`)
	assertNotContains(t, code, `authz.Input{`)
}

func TestGenerateCallWithResult(t *testing.T) {
	sf := parser.ServiceFunc{
		Name: "Login", FileName: "login.go",
		Imports: []string{"myapp/auth"},
		Sequences: []parser.Sequence{
			{Type: parser.SeqGet, Model: "User.FindByEmail", Inputs: map[string]string{"Email": "request.Email"}, Result: &parser.Result{Type: "User", Var: "user"}},
			{Type: parser.SeqCall, Model: "auth.VerifyPassword", Inputs: map[string]string{"Email": "user.Email", "Password": "request.Password"}, Result: &parser.Result{Type: "Token", Var: "token"}},
			{Type: parser.SeqResponse, Fields: map[string]string{"token": "token"}},
		},
	}
	code := mustGenerate(t, sf, nil)
	assertContains(t, code, `auth.VerifyPassword(auth.VerifyPasswordRequest{`)
	assertContains(t, code, `Email: user.Email`)
	assertContains(t, code, `Password: password`)
	assertContains(t, code, `http.StatusInternalServerError`)
}

func TestGenerateCallWithoutResult(t *testing.T) {
	sf := parser.ServiceFunc{
		Name: "Cancel", FileName: "cancel.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqCall, Model: "notification.Send", Inputs: map[string]string{"ID": "reservation.ID", "Status": `"cancelled"`}},
		},
	}
	code := mustGenerate(t, sf, nil)
	assertContains(t, code, `notification.Send(notification.SendRequest{`)
	assertContains(t, code, `ID: reservation.ID`)
	assertContains(t, code, `Status: "cancelled"`)
	assertContains(t, code, `_, err :=`)
	assertContains(t, code, `http.StatusInternalServerError`)
}

func TestGenerateCallErrStatus(t *testing.T) {
	sf := parser.ServiceFunc{
		Name: "Login", FileName: "login.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqCall, Model: "auth.VerifyPassword", Inputs: map[string]string{"Email": "request.Email", "Password": "request.Password"}, ErrStatus: 401},
		},
	}
	code := mustGenerate(t, sf, nil)
	assertContains(t, code, `http.StatusUnauthorized`)
	assertNotContains(t, code, `http.StatusInternalServerError`)
}

func TestGenerateCallErrStatusFromSymbolTable(t *testing.T) {
	// @call 대상 함수에 @error 401 어노테이션 → .ssac 명시 없으면 401 사용
	st := &validator.SymbolTable{
		Models: map[string]validator.ModelSymbol{
			"auth._func": {Methods: map[string]validator.MethodInfo{
				"VerifyPassword": {ErrStatus: 401},
			}},
		},
		Operations: map[string]validator.OperationSymbol{},
		DDLTables:  map[string]validator.DDLTable{},
	}
	sf := parser.ServiceFunc{
		Name: "Login", FileName: "login.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqCall, Model: "auth.VerifyPassword", Inputs: map[string]string{"Email": "request.Email", "Password": "request.Password"}},
		},
	}
	code := mustGenerate(t, sf, st)
	assertContains(t, code, `http.StatusUnauthorized`)
	assertNotContains(t, code, `http.StatusInternalServerError`)
}

func TestGenerateCallErrStatusSsacOverridesAnnotation(t *testing.T) {
	// .ssac 파일 명시값(500)이 @error 어노테이션(401)보다 우선
	st := &validator.SymbolTable{
		Models: map[string]validator.ModelSymbol{
			"auth._func": {Methods: map[string]validator.MethodInfo{
				"VerifyPassword": {ErrStatus: 401},
			}},
		},
		Operations: map[string]validator.OperationSymbol{},
		DDLTables:  map[string]validator.DDLTable{},
	}
	sf := parser.ServiceFunc{
		Name: "Login", FileName: "login.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqCall, Model: "auth.VerifyPassword", Inputs: map[string]string{"Email": "request.Email"}, ErrStatus: 500},
		},
	}
	code := mustGenerate(t, sf, st)
	assertContains(t, code, `http.StatusInternalServerError`)
	assertNotContains(t, code, `http.StatusUnauthorized`)
}

func TestGenerateCallBareVariable(t *testing.T) {
	sf := parser.ServiceFunc{
		Name: "Register", FileName: "register.go",
		Imports: []string{"myapp/auth"},
		Sequences: []parser.Sequence{
			{Type: parser.SeqCall, Model: "auth.HashPassword", Inputs: map[string]string{"Password": "request.Password"}, Result: &parser.Result{Type: "string", Var: "hashedPassword"}},
			{Type: parser.SeqPost, Model: "User.Create", Inputs: map[string]string{"Email": "request.Email", "HashedPassword": "hashedPassword", "Role": "request.Role"}, Result: &parser.Result{Type: "User", Var: "user"}},
			{Type: parser.SeqResponse, Fields: map[string]string{"user": "user"}},
		},
	}
	code := mustGenerate(t, sf, nil)
	// @call named field
	assertContains(t, code, `auth.HashPassword(auth.HashPasswordRequest{Password: password})`)
	// bare variable: no trailing dot
	assertContains(t, code, `h.UserModel.WithTx(tx).Create(email, hashedPassword, role)`)
	assertNotContains(t, code, `hashedPassword.`)
}

func TestGenerateResponse(t *testing.T) {
	sf := parser.ServiceFunc{
		Name: "GetCourse", FileName: "get_course.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqGet, Model: "Course.FindByID", Inputs: map[string]string{"ID": "request.ID"}, Result: &parser.Result{Type: "Course", Var: "course"}},
			{Type: parser.SeqGet, Model: "User.FindByID", Inputs: map[string]string{"InstructorID": "course.InstructorID"}, Result: &parser.Result{Type: "User", Var: "instructor"}},
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
			{Type: parser.SeqGet, Model: "Item.ListByUser", Inputs: map[string]string{"ID": "currentUser.ID"}, Result: &parser.Result{Type: "[]Item", Var: "items"}},
			{Type: parser.SeqResponse, Fields: map[string]string{"items": "items"}},
		},
	}
	code := mustGenerate(t, sf, nil)
	assertContains(t, code, `currentUser := c.MustGet("currentUser")`)
	assertContains(t, code, `h.ItemModel.ListByUser(currentUser.ID)`)
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
			{Type: parser.SeqGet, Model: "Reservation.ListByUserID", Inputs: map[string]string{"UserID": "currentUser.ID", "Opts": "query"}, Result: &parser.Result{Type: "[]Reservation", Var: "reservations"}},
			{Type: parser.SeqResponse, Fields: map[string]string{"reservations": "reservations"}},
		},
	}
	code := mustGenerate(t, sf, st)
	assertContains(t, code, `opts := model.ParseQueryOpts(c, model.QueryOptsConfig{`)
	assertContains(t, code, `PaginationConfig{Style: "offset"`)
	assertContains(t, code, `h.ReservationModel.ListByUserID(currentUser.ID, opts)`)
	assertContains(t, code, `reservations, total, err`)
	assertContains(t, code, `"total":`)
}

func TestGenerateWithPathParam(t *testing.T) {
	st := &validator.SymbolTable{
		Models:    map[string]validator.ModelSymbol{},
		DDLTables: map[string]validator.DDLTable{},
		Operations: map[string]validator.OperationSymbol{
			"GetCourse": {
				PathParams:    []validator.PathParam{{Name: "CourseID", GoType: "int64"}},
				RequestFields: map[string]bool{"CourseID": true},
			},
		},
	}
	sf := parser.ServiceFunc{
		Name: "GetCourse", FileName: "get_course.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqGet, Model: "Course.FindByID", Inputs: map[string]string{"CourseID": "request.CourseID"}, Result: &parser.Result{Type: "Course", Var: "course"}},
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
			{Type: parser.SeqPost, Model: "Session.Create", Inputs: map[string]string{"ProjectID": "request.ProjectID", "Command": "request.Command"}, Result: &parser.Result{Type: "Session", Var: "session"}},
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
			{Type: parser.SeqGet, Model: "User.FindByEmail", Inputs: map[string]string{"Email": "request.Email"}, Result: &parser.Result{Type: "User", Var: "user"}},
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
			{Type: parser.SeqGet, Model: "Reservation.FindByID", Inputs: map[string]string{"ReservationID": "request.ReservationID"}, Result: &parser.Result{Type: "Reservation", Var: "reservation"}},
			{Type: parser.SeqEmpty, Target: "reservation", Message: "예약을 찾을 수 없습니다"},
			{Type: parser.SeqState, DiagramID: "reservation", Inputs: map[string]string{"status": "reservation.Status"}, Transition: "cancel", Message: "취소할 수 없습니다"},
			{Type: parser.SeqCall, Model: "billing.CalculateRefund", Inputs: map[string]string{"ID": "reservation.ID", "StartAt": "reservation.StartAt", "EndAt": "reservation.EndAt"}, Result: &parser.Result{Type: "Refund", Var: "refund"}},
			{Type: parser.SeqPut, Model: "Reservation.UpdateStatus", Inputs: map[string]string{"ReservationID": "request.ReservationID", "Status": `"cancelled"`}},
			{Type: parser.SeqGet, Model: "Reservation.FindByID", Inputs: map[string]string{"ReservationID": "request.ReservationID"}, Result: &parser.Result{Type: "Reservation", Var: "reservation"}},
			{Type: parser.SeqResponse, Fields: map[string]string{"reservation": "reservation", "refund": "refund"}},
		},
	}
	code := mustGenerate(t, sf, nil)

	// auth
	assertContains(t, code, `authz.Check(authz.CheckRequest{`)
	// get
	assertContains(t, code, `reservation, err := h.ReservationModel.WithTx(tx).FindByID`)
	// empty
	assertContains(t, code, `if reservation == nil`)
	// state
	assertContains(t, code, `reservationstate.CanTransition`)
	// call
	assertContains(t, code, `billing.CalculateRefund`)
	// put
	assertContains(t, code, `h.ReservationModel.WithTx(tx).UpdateStatus`)
	// response
	assertContains(t, code, `"reservation":`)
	assertContains(t, code, `"refund":`)
}

func TestGenerateReAssign(t *testing.T) {
	sf := parser.ServiceFunc{
		Name: "CancelReservation", FileName: "cancel_reservation.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqGet, Model: "Reservation.FindByID", Inputs: map[string]string{"ID": "request.ID"}, Result: &parser.Result{Type: "Reservation", Var: "reservation"}},
			{Type: parser.SeqPut, Model: "Reservation.UpdateStatus", Inputs: map[string]string{"ID": "request.ID", "Status": `"cancelled"`}},
			{Type: parser.SeqGet, Model: "Reservation.FindByID", Inputs: map[string]string{"ID": "request.ID"}, Result: &parser.Result{Type: "Reservation", Var: "reservation"}},
			{Type: parser.SeqResponse, Fields: map[string]string{"reservation": "reservation"}},
		},
	}
	code := mustGenerate(t, sf, nil)
	// 첫 번째 @get: :=
	assertContains(t, code, `reservation, err := h.ReservationModel.WithTx(tx).FindByID`)
	// 두 번째 @get: = (재선언)
	assertContains(t, code, `reservation, err = h.ReservationModel.WithTx(tx).FindByID`)
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
	assertContains(t, code, `authz.CheckRequest{`)
}

func TestGenerateLiteralArg(t *testing.T) {
	sf := parser.ServiceFunc{
		Name: "Cancel", FileName: "cancel.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqPut, Model: "Reservation.UpdateStatus", Inputs: map[string]string{"ID": "request.ID", "Status": `"cancelled"`}},
		},
	}
	code := mustGenerate(t, sf, nil)
	assertContains(t, code, `h.ReservationModel.WithTx(tx).UpdateStatus(id, "cancelled")`)
}

func TestGeneratePostBodySingleField(t *testing.T) {
	st := &validator.SymbolTable{
		Models:    map[string]validator.ModelSymbol{},
		DDLTables: map[string]validator.DDLTable{
			"proposals": {Columns: map[string]string{"bid_amount": "int64", "gig_id": "int64", "freelancer_id": "int64"}},
		},
		Operations: map[string]validator.OperationSymbol{
			"SubmitProposal": {
				PathParams: []validator.PathParam{{Name: "ID", GoType: "int64"}},
			},
		},
	}
	sf := parser.ServiceFunc{
		Name: "SubmitProposal", FileName: "submit_proposal.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqGet, Model: "Gig.FindByID", Inputs: map[string]string{"ID": "request.ID"}, Result: &parser.Result{Type: "Gig", Var: "gig"}},
			{Type: parser.SeqPost, Model: "Proposal.Create", Inputs: map[string]string{"GigID": "gig.ID", "FreelancerID": "currentUser.ID", "BidAmount": "request.BidAmount"}, Result: &parser.Result{Type: "Proposal", Var: "proposal"}},
			{Type: parser.SeqResponse, Fields: map[string]string{"proposal": "proposal"}},
		},
	}
	code := mustGenerate(t, sf, st)
	// BidAmount는 path param이 아니므로 JSON body에서 읽어야 함
	assertContains(t, code, `ShouldBindJSON(&req)`)
	assertContains(t, code, `BidAmount int64`)
	assertNotContains(t, code, `c.Query("BidAmount")`)
}

func TestGenerateResponseDirect(t *testing.T) {
	sf := parser.ServiceFunc{
		Name: "ListGigs", FileName: "list_gigs.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqGet, Model: "Gig.List", Inputs: map[string]string{"Query": "query"}, Result: &parser.Result{Type: "Gig", Var: "gigPage", Wrapper: "Page"}},
			{Type: parser.SeqResponse, Target: "gigPage"},
		},
	}
	code := mustGenerate(t, sf, nil)
	assertContains(t, code, `c.JSON(__RESPONSE_STATUS__, gigPage)`)
	assertNotContains(t, code, `c.JSON(__RESPONSE_STATUS__, gin.H`)
	assertNotContains(t, code, `pagination`)
}

func TestGeneratePageNoHasTotal(t *testing.T) {
	st := &validator.SymbolTable{
		Models:     map[string]validator.ModelSymbol{},
		DDLTables:  map[string]validator.DDLTable{},
		Operations: map[string]validator.OperationSymbol{
			"ListGigs": {XPagination: &validator.XPagination{Style: "offset", DefaultLimit: 20, MaxLimit: 100}},
		},
	}
	sf := parser.ServiceFunc{
		Name: "ListGigs", FileName: "list_gigs.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqGet, Model: "Gig.List", Inputs: map[string]string{"Query": "query"}, Result: &parser.Result{Type: "Gig", Var: "gigPage", Wrapper: "Page"}},
			{Type: parser.SeqResponse, Target: "gigPage"},
		},
	}
	code := mustGenerate(t, sf, st)
	// Page[T]이면 3-tuple 아니라 단일 반환
	assertNotContains(t, code, "total")
	assertContains(t, code, `gigPage, err :=`)
}

func TestGeneratePublish(t *testing.T) {
	sf := parser.ServiceFunc{
		Name: "CompleteOrder", FileName: "complete_order.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqGet, Model: "Order.FindByID", Inputs: map[string]string{"ID": "request.OrderID"}, Result: &parser.Result{Type: "Order", Var: "order"}},
			{Type: parser.SeqPublish, Topic: "order.completed", Inputs: map[string]string{"OrderID": "order.ID", "Email": "order.Email"}},
			{Type: parser.SeqResponse, Fields: map[string]string{"order": "order"}},
		},
	}
	code := mustGenerate(t, sf, nil)
	assertContains(t, code, `queue.Publish(c.Request.Context(), "order.completed"`)
	assertContains(t, code, `"OrderID": order.ID`)
	assertContains(t, code, `order.Email`)
	assertContains(t, code, `"queue"`)
}

func TestGeneratePublishWithOptions(t *testing.T) {
	sf := parser.ServiceFunc{
		Name: "AbandonCart", FileName: "abandon_cart.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqPublish, Topic: "cart.abandoned", Inputs: map[string]string{"CartID": "cart.ID"}, Options: map[string]string{"delay": "1800"}},
		},
	}
	code := mustGenerate(t, sf, nil)
	assertContains(t, code, `queue.Publish(c.Request.Context(), "cart.abandoned"`)
	assertContains(t, code, `queue.WithDelay(1800)`)
}

func TestGeneratePackageModelCall(t *testing.T) {
	sf := parser.ServiceFunc{
		Name: "GetSession", FileName: "get_session.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqGet, Package: "session", Model: "Session.Get", Inputs: map[string]string{"token": "request.Token"}, Result: &parser.Result{Type: "Session", Var: "session"}},
			{Type: parser.SeqResponse, Fields: map[string]string{"session": "session"}},
		},
	}
	code := mustGenerate(t, sf, nil)
	assertContains(t, code, `h.SessionModel.Get(token)`)
	assertContains(t, code, `"session": session`)
}

func TestGenerateAuthCallStyle(t *testing.T) {
	sf := parser.ServiceFunc{
		Name: "AcceptProposal", FileName: "accept_proposal.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqGet, Model: "Gig.FindByID", Inputs: map[string]string{"ID": "request.GigID"}, Result: &parser.Result{Type: "Gig", Var: "gig"}},
			{Type: parser.SeqAuth, Action: "AcceptProposal", Resource: "gig", Inputs: map[string]string{"UserID": "currentUser.ID", "ResourceID": "gig.ClientID"}, Message: "Not authorized"},
		},
	}
	code := mustGenerate(t, sf, nil)
	assertContains(t, code, `authz.Check(authz.CheckRequest{Action: "AcceptProposal", Resource: "gig"`)
	assertContains(t, code, `ResourceID: gig.ClientID`)
	assertContains(t, code, `Role: currentUser.Role`)
	assertContains(t, code, `UserID: currentUser.ID`)
	assertContains(t, code, `http.StatusForbidden`)
	assertNotContains(t, code, `authz.Input{`)
}

func TestGenerateAuthAlwaysHasCurrentUser(t *testing.T) {
	sf := parser.ServiceFunc{
		Name: "CheckAccess", FileName: "check_access.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqAuth, Action: "read", Resource: "public", Inputs: map[string]string{"Key": "request.APIKey"}, Message: "Forbidden"},
		},
	}
	code := mustGenerate(t, sf, nil)
	// @auth는 항상 currentUser 추출 + UserID, Role 포함
	assertContains(t, code, `c.MustGet("currentUser")`)
	assertContains(t, code, `UserID: currentUser.ID`)
	assertContains(t, code, `Role: currentUser.Role`)
	assertContains(t, code, `authz.Check(authz.CheckRequest{`)
}

func TestGenerateUnusedVar(t *testing.T) {
	sf := parser.ServiceFunc{
		Name: "ProcessOrder", FileName: "process_order.go",
		Imports: []string{"myapp/billing"},
		Sequences: []parser.Sequence{
			{Type: parser.SeqGet, Model: "Order.FindByID", Inputs: map[string]string{"ID": "request.OrderID"}, Result: &parser.Result{Type: "Order", Var: "order"}},
			{Type: parser.SeqCall, Model: "billing.HoldEscrow", Inputs: map[string]string{"Amount": "order.Budget"}, Result: &parser.Result{Type: "Escrow", Var: "escrow"}},
			{Type: parser.SeqResponse, Fields: map[string]string{"order": "order"}},
		},
	}
	code := mustGenerate(t, sf, nil)
	// escrow는 response에서 미참조 → _, err already declared → =
	assertContains(t, code, `_, err = billing.HoldEscrow(billing.HoldEscrowRequest{`)
	// order는 response에서 참조 → 변수명 유지
	assertContains(t, code, `order, err := h.OrderModel.FindByID`)
}

func TestGenerateUsedVar(t *testing.T) {
	sf := parser.ServiceFunc{
		Name: "ProcessOrder", FileName: "process_order.go",
		Imports: []string{"myapp/billing"},
		Sequences: []parser.Sequence{
			{Type: parser.SeqGet, Model: "Order.FindByID", Inputs: map[string]string{"ID": "request.OrderID"}, Result: &parser.Result{Type: "Order", Var: "order"}},
			{Type: parser.SeqCall, Model: "billing.HoldEscrow", Inputs: map[string]string{"Amount": "order.Budget"}, Result: &parser.Result{Type: "Escrow", Var: "escrow"}},
			{Type: parser.SeqResponse, Fields: map[string]string{"order": "order", "escrow": "escrow"}},
		},
	}
	code := mustGenerate(t, sf, nil)
	// escrow는 response에서 참조됨 → 변수명 유지
	assertContains(t, code, `escrow, err := billing.HoldEscrow(billing.HoldEscrowRequest{`)
	assertContains(t, code, `order, err := h.OrderModel.FindByID`)
}

func TestGenerateUnusedVarErrAlreadyDeclared(t *testing.T) {
	// 2번째 @get에서 Unused + err already declared → _, err =
	sf := parser.ServiceFunc{
		Name: "DoSomething", FileName: "do_something.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqGet, Model: "User.FindByID", Inputs: map[string]string{"ID": "request.UserID"}, Result: &parser.Result{Type: "User", Var: "user"}},
			{Type: parser.SeqGet, Model: "Token.Generate", Inputs: map[string]string{"UserID": "user.ID"}, Result: &parser.Result{Type: "Token", Var: "token"}},
			{Type: parser.SeqResponse, Fields: map[string]string{"user": "user"}},
		},
	}
	code := mustGenerate(t, sf, nil)
	// token은 미참조 + err already declared → _, err =
	assertContains(t, code, `_, err = h.TokenModel.Generate(user.ID)`)
	// user는 참조됨 → user, err :=
	assertContains(t, code, `user, err := h.UserModel.FindByID`)
}

func TestGenerateUnusedVarFirstErr(t *testing.T) {
	// 첫 시퀀스에서 Unused → _, err := (err 첫 선언)
	sf := parser.ServiceFunc{
		Name: "DoSomething", FileName: "do_something.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqGet, Model: "Token.Generate", Inputs: map[string]string{"Key": "request.Key"}, Result: &parser.Result{Type: "Token", Var: "token"}},
			{Type: parser.SeqResponse, Fields: map[string]string{}},
		},
	}
	code := mustGenerate(t, sf, nil)
	// token은 미참조 + err 첫 선언 → _, err :=
	assertContains(t, code, `_, err := h.TokenModel.Generate`)
}

func TestGenerateEmptyErrStatus(t *testing.T) {
	sf := parser.ServiceFunc{
		Name:     "ActivateWorkflow",
		FileName: "activate_workflow.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqGet, Model: "Org.FindByID", Inputs: map[string]string{"ID": "request.OrgID"}, Result: &parser.Result{Type: "Org", Var: "org"}},
			{Type: parser.SeqEmpty, Target: "org", Message: "Insufficient credits", ErrStatus: 402},
		},
	}
	code := mustGenerate(t, sf, nil)
	assertContains(t, code, "http.StatusPaymentRequired")
	assertNotContains(t, code, "http.StatusNotFound")
}

func TestGenerateExistsErrStatus(t *testing.T) {
	sf := parser.ServiceFunc{
		Name:     "Register",
		FileName: "register.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqGet, Model: "User.FindByEmail", Inputs: map[string]string{"Email": "request.Email"}, Result: &parser.Result{Type: "*User", Var: "user"}},
			{Type: parser.SeqExists, Target: "user", Message: "Already registered", ErrStatus: 422},
		},
	}
	code := mustGenerate(t, sf, nil)
	assertContains(t, code, "http.StatusUnprocessableEntity")
	assertNotContains(t, code, "http.StatusConflict")
}

func TestGenerateStateErrStatus(t *testing.T) {
	sf := parser.ServiceFunc{
		Name:     "ActivateWorkflow",
		FileName: "activate_workflow.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqGet, Model: "Workflow.FindByID", Inputs: map[string]string{"ID": "request.WorkflowID"}, Result: &parser.Result{Type: "Workflow", Var: "workflow"}},
			{Type: parser.SeqState, DiagramID: "workflow", Inputs: map[string]string{"Status": "workflow.Status"}, Transition: "Activate", Message: "Cannot transition", ErrStatus: 422},
		},
	}
	code := mustGenerate(t, sf, nil)
	assertContains(t, code, "http.StatusUnprocessableEntity")
	assertNotContains(t, code, "http.StatusConflict")
}

func TestGenerateAuthErrStatus(t *testing.T) {
	sf := parser.ServiceFunc{
		Name:     "Execute",
		FileName: "execute.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqAuth, Action: "Execute", Resource: "workflow", Inputs: map[string]string{"UserID": "currentUser.ID"}, Message: "Token expired", ErrStatus: 401},
		},
	}
	code := mustGenerate(t, sf, nil)
	assertContains(t, code, "http.StatusUnauthorized")
	assertNotContains(t, code, "http.StatusForbidden")
}

func TestGenerateRequestStructExported(t *testing.T) {
	sf := parser.ServiceFunc{
		Name: "CreateUser", FileName: "create_user.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqPost, Model: "User.Create", Inputs: map[string]string{"email": "request.email"}, Result: &parser.Result{Type: "User", Var: "user"}},
			{Type: parser.SeqResponse, Fields: map[string]string{"user": "user"}},
		},
	}
	code := mustGenerate(t, sf, nil)
	assertContains(t, code, "Email string `json:\"email\"`")
	assertContains(t, code, "email := req.Email")
}

func TestGenerateRequestStructSnakeCase(t *testing.T) {
	st := &validator.SymbolTable{
		DDLTables: map[string]validator.DDLTable{
			"bids": {Columns: map[string]string{"bid_amount": "int32", "id": "int64"}},
		},
		Operations: map[string]validator.OperationSymbol{},
		Models:     map[string]validator.ModelSymbol{},
	}
	sf := parser.ServiceFunc{
		Name: "PlaceBid", FileName: "place_bid.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqPost, Model: "Bid.Place", Inputs: map[string]string{"bid_amount": "request.bid_amount", "id": "request.id"}, Result: &parser.Result{Type: "Bid", Var: "bid"}},
			{Type: parser.SeqResponse, Fields: map[string]string{"bid": "bid"}},
		},
	}
	code := mustGenerate(t, sf, st)
	assertContains(t, code, "`json:\"bid_amount\"`")
	assertContains(t, code, "BidAmount int32")
	assertContains(t, code, "`json:\"id\"`")
	assertContains(t, code, "ID ")
	assertContains(t, code, "bidAmount := req.BidAmount")
	assertContains(t, code, "id := req.ID")
}

// --- BUG005: sort allowlist ---

func TestGenerateSortAllowlist(t *testing.T) {
	st := &validator.SymbolTable{
		Models:     map[string]validator.ModelSymbol{},
		DDLTables:  map[string]validator.DDLTable{},
		Operations: map[string]validator.OperationSymbol{
			"ListGigs": {
				XPagination: &validator.XPagination{Style: "offset", DefaultLimit: 20, MaxLimit: 100},
				XSort:       &validator.XSort{Allowed: []string{"created_at", "title", "price"}, Default: "created_at"},
			},
		},
	}
	sf := parser.ServiceFunc{
		Name: "ListGigs", FileName: "list_gigs.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqGet, Model: "Gig.List", Inputs: map[string]string{"Query": "query"}, Result: &parser.Result{Type: "[]Gig", Var: "gigs"}},
			{Type: parser.SeqResponse, Fields: map[string]string{"gigs": "gigs"}},
		},
	}
	code := mustGenerate(t, sf, st)
	// model.ParseQueryOpts로 sort allowlist 포함 config 생성
	assertContains(t, code, `model.ParseQueryOpts(c, model.QueryOptsConfig{`)
	assertContains(t, code, `&model.SortConfig{`)
	assertContains(t, code, `"created_at"`)
	assertContains(t, code, `"title"`)
	assertContains(t, code, `"price"`)
	assertContains(t, code, `Default: "created_at"`)
	// 수동 파싱 패턴 없어야 함
	assertNotContains(t, code, `allowedSort`)
	assertNotContains(t, code, `c.Query("sort")`)
}

func TestGenerateSortNoXSort(t *testing.T) {
	st := &validator.SymbolTable{
		Models:     map[string]validator.ModelSymbol{},
		DDLTables:  map[string]validator.DDLTable{},
		Operations: map[string]validator.OperationSymbol{
			"ListGigs": {
				XPagination: &validator.XPagination{Style: "offset", DefaultLimit: 20, MaxLimit: 100},
			},
		},
	}
	sf := parser.ServiceFunc{
		Name: "ListGigs", FileName: "list_gigs.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqGet, Model: "Gig.List", Inputs: map[string]string{"Query": "query"}, Result: &parser.Result{Type: "[]Gig", Var: "gigs"}},
			{Type: parser.SeqResponse, Fields: map[string]string{"gigs": "gigs"}},
		},
	}
	code := mustGenerate(t, sf, st)
	// x-sort 없으면 SortConfig 없음
	assertNotContains(t, code, `Sort:`)
	assertNotContains(t, code, `SortConfig`)
}

// --- BUG007: auth Claims ---

func TestGenerateAuthClaims(t *testing.T) {
	sf := parser.ServiceFunc{
		Name: "AcceptProposal", FileName: "accept_proposal.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqGet, Model: "Gig.FindByID", Inputs: map[string]string{"ID": "request.GigID"}, Result: &parser.Result{Type: "Gig", Var: "gig"}},
			{Type: parser.SeqAuth, Action: "AcceptProposal", Resource: "gig", Inputs: map[string]string{"UserID": "currentUser.ID", "ResourceID": "gig.ClientID"}, Message: "Not authorized"},
		},
	}
	code := mustGenerate(t, sf, nil)
	// 템플릿 고정 UserID, Role — inputs에 명시해도 중복 없음
	assertContains(t, code, `UserID: currentUser.ID`)
	assertContains(t, code, `Role: currentUser.Role`)
	// UserID가 1번만 나오는지 확인 (중복 방지)
	if strings.Count(code, "UserID:") != 1 {
		t.Errorf("expected UserID: to appear exactly once, got %d\n--- code ---\n%s", strings.Count(code, "UserID:"), code)
	}
}

func TestGenerateAuthAlwaysIncludesUserIDRole(t *testing.T) {
	sf := parser.ServiceFunc{
		Name: "CheckAccess", FileName: "check_access.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqAuth, Action: "read", Resource: "public", Inputs: map[string]string{"Key": "request.APIKey"}, Message: "Forbidden"},
		},
	}
	code := mustGenerate(t, sf, nil)
	// @auth 템플릿에 항상 UserID, Role 포함
	assertContains(t, code, `UserID: currentUser.ID`)
	assertContains(t, code, `Role: currentUser.Role`)
}

func TestGenerateSubscribeAuthClaims(t *testing.T) {
	sf := parser.ServiceFunc{
		Name: "OnTest", FileName: "on_test.go",
		Subscribe: &parser.SubscribeInfo{Topic: "test", MessageType: "TestMsg"},
		Param:     &parser.ParamInfo{TypeName: "TestMsg", VarName: "message"},
		Sequences: []parser.Sequence{
			{Type: parser.SeqAuth, Action: "process", Resource: "order", Inputs: map[string]string{"UserID": "currentUser.ID"}, Message: "Not authorized"},
		},
	}
	code := mustGenerate(t, sf, nil)
	assertContains(t, code, `UserID: currentUser.ID`)
}

// --- BUG012: 미사용 import 제거 ---

func TestGenerateNoUnusedImportDatabaseSQL(t *testing.T) {
	// @post가 있으면 tx 코드가 생성되지만, database/sql은 handler.go에만 필요
	sf := parser.ServiceFunc{
		Name: "CreateSession", FileName: "create_session.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqPost, Model: "Session.Create", Inputs: map[string]string{"UserID": "request.UserID"}, Result: &parser.Result{Type: "Session", Var: "session"}},
			{Type: parser.SeqResponse, Fields: map[string]string{"session": "session"}},
		},
	}
	code := mustGenerate(t, sf, nil)
	assertNotContains(t, code, `"database/sql"`)
	assertContains(t, code, `h.DB.BeginTx`)
}

func TestGenerateNoUnusedImportStrconv(t *testing.T) {
	// string 타입 request param만 있으면 strconv 불필요
	sf := parser.ServiceFunc{
		Name: "GetCourse", FileName: "get_course.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqGet, Model: "Course.FindByID", Inputs: map[string]string{"ID": "request.CourseID"}, Result: &parser.Result{Type: "Course", Var: "course"}},
			{Type: parser.SeqResponse, Fields: map[string]string{"course": "course"}},
		},
	}
	code := mustGenerate(t, sf, nil)
	assertNotContains(t, code, `"strconv"`)
}

func TestGenerateKeepsStrconvWhenUsed(t *testing.T) {
	// int64 path param이 있으면 strconv.ParseInt 생성 → strconv 유지
	st := &validator.SymbolTable{
		Models:    map[string]validator.ModelSymbol{},
		DDLTables: map[string]validator.DDLTable{},
		Operations: map[string]validator.OperationSymbol{
			"GetCourse": {PathParams: []validator.PathParam{{Name: "ID", GoType: "int64"}}},
		},
	}
	sf := parser.ServiceFunc{
		Name: "GetCourse", FileName: "get_course.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqGet, Model: "Course.FindByID", Inputs: map[string]string{"ID": "request.ID"}, Result: &parser.Result{Type: "Course", Var: "course"}},
			{Type: parser.SeqResponse, Fields: map[string]string{"course": "course"}},
		},
	}
	code := mustGenerate(t, sf, st)
	assertContains(t, code, `"strconv"`)
	assertContains(t, code, `strconv.ParseInt`)
}
