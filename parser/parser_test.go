package parser

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseGet(t *testing.T) {
	src := `package service

// @get Course course = Course.FindByID({CourseID: request.CourseID})
func GetCourse(c *gin.Context) {}
`
	sfs := parseTestFile(t, src)
	if len(sfs) != 1 {
		t.Fatalf("expected 1 func, got %d", len(sfs))
	}
	seq := sfs[0].Sequences[0]
	assertEqual(t, "Type", seq.Type, SeqGet)
	assertEqual(t, "Model", seq.Model, "Course.FindByID")
	if seq.Result == nil {
		t.Fatal("expected result")
	}
	assertEqual(t, "Result.Type", seq.Result.Type, "Course")
	assertEqual(t, "Result.Var", seq.Result.Var, "course")
	if len(seq.Inputs) != 1 {
		t.Fatalf("expected 1 input, got %d", len(seq.Inputs))
	}
	assertEqual(t, "Inputs.CourseID", seq.Inputs["CourseID"], "request.CourseID")
}

func TestParseGetMultiArgs(t *testing.T) {
	src := `package service

// @get []Reservation reservations = Reservation.ListByUserAndRoom({UserID: currentUser.ID, RoomID: request.RoomID})
func ListReservations(c *gin.Context) {}
`
	sfs := parseTestFile(t, src)
	seq := sfs[0].Sequences[0]
	assertEqual(t, "Result.Type", seq.Result.Type, "[]Reservation")
	if len(seq.Inputs) != 2 {
		t.Fatalf("expected 2 inputs, got %d", len(seq.Inputs))
	}
	assertEqual(t, "Inputs.UserID", seq.Inputs["UserID"], "currentUser.ID")
	assertEqual(t, "Inputs.RoomID", seq.Inputs["RoomID"], "request.RoomID")
}

func TestParsePost(t *testing.T) {
	src := `package service

// @post Session session = Session.Create({ProjectID: request.ProjectID, Command: request.Command})
func CreateSession(c *gin.Context) {}
`
	sfs := parseTestFile(t, src)
	seq := sfs[0].Sequences[0]
	assertEqual(t, "Type", seq.Type, SeqPost)
	assertEqual(t, "Result.Type", seq.Result.Type, "Session")
	assertEqual(t, "Result.Var", seq.Result.Var, "session")
	if len(seq.Inputs) != 2 {
		t.Fatalf("expected 2 inputs, got %d", len(seq.Inputs))
	}
}

func TestParsePut(t *testing.T) {
	src := `package service

// @put Course.Update({Title: request.Title, ID: course.ID})
func UpdateCourse(c *gin.Context) {}
`
	sfs := parseTestFile(t, src)
	seq := sfs[0].Sequences[0]
	assertEqual(t, "Type", seq.Type, SeqPut)
	assertEqual(t, "Model", seq.Model, "Course.Update")
	if seq.Result != nil {
		t.Fatal("expected no result for @put")
	}
	if len(seq.Inputs) != 2 {
		t.Fatalf("expected 2 inputs, got %d", len(seq.Inputs))
	}
	assertEqual(t, "Inputs.ID", seq.Inputs["ID"], "course.ID")
}

func TestParseDelete(t *testing.T) {
	src := `package service

// @delete Reservation.Cancel({ID: reservation.ID})
func CancelReservation(c *gin.Context) {}
`
	sfs := parseTestFile(t, src)
	seq := sfs[0].Sequences[0]
	assertEqual(t, "Type", seq.Type, SeqDelete)
	if seq.Result != nil {
		t.Fatal("expected no result for @delete")
	}
	assertEqual(t, "Inputs.ID", seq.Inputs["ID"], "reservation.ID")
}

func TestParseLiteralArg(t *testing.T) {
	src := `package service

// @put Reservation.UpdateStatus({ReservationID: request.ReservationID, Status: "cancelled"})
func CancelReservation(c *gin.Context) {}
`
	sfs := parseTestFile(t, src)
	seq := sfs[0].Sequences[0]
	assertEqual(t, "Inputs.Status", seq.Inputs["Status"], `"cancelled"`)
}

func TestParseEmpty(t *testing.T) {
	src := `package service

// @empty course "코스를 찾을 수 없습니다"
func GetCourse(c *gin.Context) {}
`
	sfs := parseTestFile(t, src)
	seq := sfs[0].Sequences[0]
	assertEqual(t, "Type", seq.Type, SeqEmpty)
	assertEqual(t, "Target", seq.Target, "course")
	assertEqual(t, "Message", seq.Message, "코스를 찾을 수 없습니다")
}

func TestParseEmptyMember(t *testing.T) {
	src := `package service

// @empty course.InstructorID "강사가 지정되지 않았습니다"
func GetCourse(c *gin.Context) {}
`
	sfs := parseTestFile(t, src)
	seq := sfs[0].Sequences[0]
	assertEqual(t, "Target", seq.Target, "course.InstructorID")
}

func TestParseExists(t *testing.T) {
	src := `package service

// @exists existing "이미 존재합니다"
func CreateCourse(c *gin.Context) {}
`
	sfs := parseTestFile(t, src)
	seq := sfs[0].Sequences[0]
	assertEqual(t, "Type", seq.Type, SeqExists)
	assertEqual(t, "Target", seq.Target, "existing")
	assertEqual(t, "Message", seq.Message, "이미 존재합니다")
}

func TestParseState(t *testing.T) {
	src := `package service

// @state reservation {status: reservation.Status} "cancel" "취소할 수 없습니다"
func CancelReservation(c *gin.Context) {}
`
	sfs := parseTestFile(t, src)
	seq := sfs[0].Sequences[0]
	assertEqual(t, "Type", seq.Type, SeqState)
	assertEqual(t, "DiagramID", seq.DiagramID, "reservation")
	assertEqual(t, "Transition", seq.Transition, "cancel")
	assertEqual(t, "Message", seq.Message, "취소할 수 없습니다")
	if seq.Inputs["status"] != "reservation.Status" {
		t.Errorf("expected Inputs[status]=%q, got %q", "reservation.Status", seq.Inputs["status"])
	}
}

func TestParseStateMultiInputs(t *testing.T) {
	src := `package service

// @state course {status: course.Status, createdAt: course.CreatedAt} "publish" "발행할 수 없습니다"
func PublishCourse(c *gin.Context) {}
`
	sfs := parseTestFile(t, src)
	seq := sfs[0].Sequences[0]
	if len(seq.Inputs) != 2 {
		t.Fatalf("expected 2 inputs, got %d", len(seq.Inputs))
	}
	assertEqual(t, "Inputs[status]", seq.Inputs["status"], "course.Status")
	assertEqual(t, "Inputs[createdAt]", seq.Inputs["createdAt"], "course.CreatedAt")
}

func TestParseAuth(t *testing.T) {
	src := `package service

// @auth "delete" "project" {id: project.ID, owner: project.OwnerID} "권한 없음"
func DeleteProject(c *gin.Context) {}
`
	sfs := parseTestFile(t, src)
	seq := sfs[0].Sequences[0]
	assertEqual(t, "Type", seq.Type, SeqAuth)
	assertEqual(t, "Action", seq.Action, "delete")
	assertEqual(t, "Resource", seq.Resource, "project")
	assertEqual(t, "Message", seq.Message, "권한 없음")
	assertEqual(t, "Inputs[id]", seq.Inputs["id"], "project.ID")
	assertEqual(t, "Inputs[owner]", seq.Inputs["owner"], "project.OwnerID")
}

func TestParseAuthEmptyInputs(t *testing.T) {
	src := `package service

// @auth "view" "dashboard" {} "권한 없음"
func ViewDashboard(c *gin.Context) {}
`
	sfs := parseTestFile(t, src)
	seq := sfs[0].Sequences[0]
	assertEqual(t, "Action", seq.Action, "view")
	if len(seq.Inputs) != 0 {
		t.Errorf("expected empty inputs, got %d", len(seq.Inputs))
	}
}

func TestParseCallWithResult(t *testing.T) {
	src := `package service

// @call Token token = auth.VerifyPassword({Email: user.Email, Password: request.Password})
func Login(c *gin.Context) {}
`
	sfs := parseTestFile(t, src)
	seq := sfs[0].Sequences[0]
	assertEqual(t, "Type", seq.Type, SeqCall)
	assertEqual(t, "Model", seq.Model, "auth.VerifyPassword")
	if seq.Result == nil {
		t.Fatal("expected result")
	}
	assertEqual(t, "Result.Type", seq.Result.Type, "Token")
	assertEqual(t, "Result.Var", seq.Result.Var, "token")
	if len(seq.Inputs) != 2 {
		t.Fatalf("expected 2 inputs, got %d", len(seq.Inputs))
	}
	assertEqual(t, "Inputs.Email", seq.Inputs["Email"], "user.Email")
	assertEqual(t, "Inputs.Password", seq.Inputs["Password"], "request.Password")
}

func TestParseCallWithoutResult(t *testing.T) {
	src := `package service

// @call notification.Send({ID: reservation.ID, Status: "cancelled"})
func CancelReservation(c *gin.Context) {}
`
	sfs := parseTestFile(t, src)
	seq := sfs[0].Sequences[0]
	assertEqual(t, "Type", seq.Type, SeqCall)
	assertEqual(t, "Model", seq.Model, "notification.Send")
	if seq.Result != nil {
		t.Fatal("expected no result")
	}
	if len(seq.Inputs) != 2 {
		t.Fatalf("expected 2 inputs, got %d", len(seq.Inputs))
	}
	assertEqual(t, "Inputs.ID", seq.Inputs["ID"], "reservation.ID")
	assertEqual(t, "Inputs.Status", seq.Inputs["Status"], `"cancelled"`)
}

func TestParseResponse(t *testing.T) {
	src := `package service

// @get Course course = Course.FindByID({CourseID: request.CourseID})
// @get User instructor = User.FindByID({InstructorID: course.InstructorID})
// @response {
//   course: course,
//   instructor_name: instructor.Name,
//   status: "success"
// }
func GetCourse(c *gin.Context) {}
`
	sfs := parseTestFile(t, src)
	if len(sfs[0].Sequences) != 3 {
		t.Fatalf("expected 3 sequences, got %d", len(sfs[0].Sequences))
	}
	seq := sfs[0].Sequences[2]
	assertEqual(t, "Type", seq.Type, SeqResponse)
	if len(seq.Fields) != 3 {
		t.Fatalf("expected 3 fields, got %d", len(seq.Fields))
	}
	assertEqual(t, "Fields[course]", seq.Fields["course"], "course")
	assertEqual(t, "Fields[instructor_name]", seq.Fields["instructor_name"], "instructor.Name")
	assertEqual(t, "Fields[status]", seq.Fields["status"], `"success"`)
}

func TestParseImports(t *testing.T) {
	src := `package service

import "myapp/auth"

// @get User user = User.FindByEmail({Email: request.Email})
func Login(c *gin.Context) {}
`
	sfs := parseTestFile(t, src)
	if len(sfs[0].Imports) != 1 {
		t.Fatalf("expected 1 import, got %d", len(sfs[0].Imports))
	}
	assertEqual(t, "Import", sfs[0].Imports[0], "myapp/auth")
}

func TestParseImportsExcludeNetHTTP(t *testing.T) {
	src := `package service

import (
	"net/http"
	"myapp/billing"
)

// @get User user = User.FindByID({UserID: request.UserID})
func GetUser(c *gin.Context) {}
`
	sfs := parseTestFile(t, src)
	if len(sfs[0].Imports) != 1 {
		t.Fatalf("expected 1 import, got %d", len(sfs[0].Imports))
	}
	assertEqual(t, "Import", sfs[0].Imports[0], "myapp/billing")
}

func TestParseFlatServiceError(t *testing.T) {
	dir := t.TempDir()

	src := `package service

// @get User user = User.FindByEmail({Email: request.Email})
// @response {
//   user: user
// }
func Login() {}
`
	os.WriteFile(filepath.Join(dir, "login.ssac"), []byte(src), 0644)

	_, err := ParseDir(dir)
	if err == nil {
		t.Fatal("expected error for flat service/ file, got nil")
	}
	if !strings.Contains(err.Error(), "도메인 서브 폴더를 사용하세요") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestParseDomainFolder(t *testing.T) {
	dir := t.TempDir()
	authDir := filepath.Join(dir, "auth")
	os.MkdirAll(authDir, 0755)

	src := `package service

// @get User user = User.FindByEmail({Email: request.Email})
// @response {
//   user: user
// }
func Login(c *gin.Context) {}
`
	os.WriteFile(filepath.Join(authDir, "login.ssac"), []byte(src), 0644)

	funcs, err := ParseDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(funcs) != 1 {
		t.Fatalf("expected 1 func, got %d", len(funcs))
	}
	assertEqual(t, "Domain", funcs[0].Domain, "auth")
}

func TestParseFullExample(t *testing.T) {
	src := `package service

import "myapp/auth"

// @auth "cancel" "reservation" {id: request.ReservationID} "권한 없음"
// @get Reservation reservation = Reservation.FindByID({ReservationID: request.ReservationID})
// @empty reservation "예약을 찾을 수 없습니다"
// @state reservation {status: reservation.Status} "cancel" "취소할 수 없습니다"
// @call Refund refund = billing.CalculateRefund({ID: reservation.ID, StartAt: reservation.StartAt, EndAt: reservation.EndAt})
// @put Reservation.UpdateStatus({ReservationID: request.ReservationID, Status: "cancelled"})
// @get Reservation reservation = Reservation.FindByID({ReservationID: request.ReservationID})
// @response {
//   reservation: reservation,
//   refund: refund
// }
func CancelReservation(c *gin.Context) {}
`
	sfs := parseTestFile(t, src)
	sf := sfs[0]
	assertEqual(t, "Name", sf.Name, "CancelReservation")

	if len(sf.Sequences) != 8 {
		t.Fatalf("expected 8 sequences, got %d", len(sf.Sequences))
	}

	// @auth
	assertEqual(t, "seq0.Type", sf.Sequences[0].Type, SeqAuth)
	assertEqual(t, "seq0.Action", sf.Sequences[0].Action, "cancel")

	// @get
	assertEqual(t, "seq1.Type", sf.Sequences[1].Type, SeqGet)
	assertEqual(t, "seq1.Model", sf.Sequences[1].Model, "Reservation.FindByID")

	// @empty
	assertEqual(t, "seq2.Type", sf.Sequences[2].Type, SeqEmpty)

	// @state
	assertEqual(t, "seq3.Type", sf.Sequences[3].Type, SeqState)
	assertEqual(t, "seq3.DiagramID", sf.Sequences[3].DiagramID, "reservation")

	// @call
	assertEqual(t, "seq4.Type", sf.Sequences[4].Type, SeqCall)
	assertEqual(t, "seq4.Model", sf.Sequences[4].Model, "billing.CalculateRefund")
	if seq4r := sf.Sequences[4].Result; seq4r == nil {
		t.Fatal("expected call result")
	} else {
		assertEqual(t, "seq4.Result.Type", seq4r.Type, "Refund")
	}

	// @put
	assertEqual(t, "seq5.Type", sf.Sequences[5].Type, SeqPut)

	// @get (re-fetch)
	assertEqual(t, "seq6.Type", sf.Sequences[6].Type, SeqGet)

	// @response
	assertEqual(t, "seq7.Type", sf.Sequences[7].Type, SeqResponse)
	assertEqual(t, "seq7.Fields[reservation]", sf.Sequences[7].Fields["reservation"], "reservation")
	assertEqual(t, "seq7.Fields[refund]", sf.Sequences[7].Fields["refund"], "refund")
}

func TestParseMultipleFuncs(t *testing.T) {
	src := `package service

// @get Course course = Course.FindByID({CourseID: request.CourseID})
// @response {
//   course: course
// }
func GetCourse(c *gin.Context) {}

// @post Course course = Course.Create({Title: request.Title})
// @response {
//   course: course
// }
func CreateCourse(c *gin.Context) {}
`
	sfs := parseTestFile(t, src)
	if len(sfs) != 2 {
		t.Fatalf("expected 2 funcs, got %d", len(sfs))
	}
	assertEqual(t, "Func0", sfs[0].Name, "GetCourse")
	assertEqual(t, "Func1", sfs[1].Name, "CreateCourse")
}

// --- SuppressWarn (!) ---

func TestParseSuppressWarnDelete(t *testing.T) {
	src := `package service

// @delete! Room.DeleteAll()
func DeleteAll() {}
`
	sfs := parseTestFile(t, src)
	seq := sfs[0].Sequences[0]
	assertEqual(t, "Type", seq.Type, SeqDelete)
	assertEqual(t, "Model", seq.Model, "Room.DeleteAll")
	if !seq.SuppressWarn {
		t.Error("expected SuppressWarn=true")
	}
}

func TestParseSuppressWarnGet(t *testing.T) {
	src := `package service

// @get! Course course = Course.FindByID({CourseID: request.CourseID})
func GetCourse() {}
`
	sfs := parseTestFile(t, src)
	seq := sfs[0].Sequences[0]
	assertEqual(t, "Type", seq.Type, SeqGet)
	assertEqual(t, "Model", seq.Model, "Course.FindByID")
	if !seq.SuppressWarn {
		t.Error("expected SuppressWarn=true")
	}
}

func TestParseNoSuppressWarn(t *testing.T) {
	src := `package service

// @get Course course = Course.FindByID({CourseID: request.CourseID})
func GetCourse() {}
`
	sfs := parseTestFile(t, src)
	seq := sfs[0].Sequences[0]
	if seq.SuppressWarn {
		t.Error("expected SuppressWarn=false")
	}
}

func TestParseSuppressWarnResponse(t *testing.T) {
	src := `package service

// @get Course course = Course.FindByID({ID: request.ID})
// @response! {
//   course: course,
// }
func GetCourse() {}
`
	sfs := parseTestFile(t, src)
	var resp *Sequence
	for i := range sfs[0].Sequences {
		if sfs[0].Sequences[i].Type == SeqResponse {
			resp = &sfs[0].Sequences[i]
			break
		}
	}
	if resp == nil {
		t.Fatal("expected response sequence")
	}
	if !resp.SuppressWarn {
		t.Error("expected SuppressWarn=true for @response!")
	}
}

func TestParseInputsNoColon(t *testing.T) {
	src := `package service

// @get []Gig gigs = Gig.List({query})
func ListGigs(c *gin.Context) {}
`
	dir := t.TempDir()
	path := filepath.Join(dir, "test.go")
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := ParseFile(path)
	if err == nil {
		t.Fatal("expected error for input without colon")
	}
	if !strings.Contains(err.Error(), "유효하지 않은 입력 형식") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestParsePageType(t *testing.T) {
	src := `package service

// @get Page[Gig] gigPage = Gig.List({Query: query})
func ListGigs(c *gin.Context) {}
`
	sfs := parseTestFile(t, src)
	seq := sfs[0].Sequences[0]
	if seq.Result == nil {
		t.Fatal("expected result")
	}
	assertEqual(t, "Result.Wrapper", seq.Result.Wrapper, "Page")
	assertEqual(t, "Result.Type", seq.Result.Type, "Gig")
	assertEqual(t, "Result.Var", seq.Result.Var, "gigPage")
}

func TestParseCursorType(t *testing.T) {
	src := `package service

// @get Cursor[Gig] gigCursor = Gig.List({Query: query})
func ListGigs(c *gin.Context) {}
`
	sfs := parseTestFile(t, src)
	seq := sfs[0].Sequences[0]
	if seq.Result == nil {
		t.Fatal("expected result")
	}
	assertEqual(t, "Result.Wrapper", seq.Result.Wrapper, "Cursor")
	assertEqual(t, "Result.Type", seq.Result.Type, "Gig")
}

func TestParseResponseDirect(t *testing.T) {
	src := `package service

// @get Page[Gig] gigPage = Gig.List({Query: query})
// @response gigPage
func ListGigs(c *gin.Context) {}
`
	sfs := parseTestFile(t, src)
	var resp *Sequence
	for i := range sfs[0].Sequences {
		if sfs[0].Sequences[i].Type == SeqResponse {
			resp = &sfs[0].Sequences[i]
			break
		}
	}
	if resp == nil {
		t.Fatal("expected response sequence")
	}
	assertEqual(t, "Target", resp.Target, "gigPage")
	if len(resp.Fields) != 0 {
		t.Errorf("expected empty Fields for direct response, got %v", resp.Fields)
	}
}

func TestParseResponseSingleLine(t *testing.T) {
	src := `package service

// @get User user = User.FindByID({ID: request.ID})
// @response { user: user }
func GetUser(c *gin.Context) {}
`
	sfs := parseTestFile(t, src)
	var resp *Sequence
	for i := range sfs[0].Sequences {
		if sfs[0].Sequences[i].Type == SeqResponse {
			resp = &sfs[0].Sequences[i]
			break
		}
	}
	if resp == nil {
		t.Fatal("expected response sequence")
	}
	if resp.Target != "" {
		t.Errorf("expected empty Target for single-line struct, got %q", resp.Target)
	}
	if len(resp.Fields) != 1 {
		t.Fatalf("expected 1 field, got %d", len(resp.Fields))
	}
	assertEqual(t, "Fields.user", resp.Fields["user"], "user")
}

func TestParseResponseSingleLineMultiFields(t *testing.T) {
	src := `package service

// @get User user = User.FindByID({ID: request.ID})
// @response { user: user, name: user.Name }
func GetUser(c *gin.Context) {}
`
	sfs := parseTestFile(t, src)
	var resp *Sequence
	for i := range sfs[0].Sequences {
		if sfs[0].Sequences[i].Type == SeqResponse {
			resp = &sfs[0].Sequences[i]
			break
		}
	}
	if resp == nil {
		t.Fatal("expected response sequence")
	}
	if len(resp.Fields) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(resp.Fields))
	}
	assertEqual(t, "Fields.user", resp.Fields["user"], "user")
	assertEqual(t, "Fields.name", resp.Fields["name"], "user.Name")
}

// --- @publish / @subscribe ---

func TestParsePublish(t *testing.T) {
	src := `package service

// @get Order order = Order.FindByID({ID: request.OrderID})
// @publish "order.completed" {OrderID: order.ID, Email: order.Email}
// @response { order: order }
func CompleteOrder() {}
`
	sfs := parseTestFile(t, src)
	if len(sfs[0].Sequences) != 3 {
		t.Fatalf("expected 3 sequences, got %d", len(sfs[0].Sequences))
	}
	seq := sfs[0].Sequences[1]
	assertEqual(t, "Type", seq.Type, SeqPublish)
	assertEqual(t, "Topic", seq.Topic, "order.completed")
	if len(seq.Inputs) != 2 {
		t.Fatalf("expected 2 inputs, got %d", len(seq.Inputs))
	}
	assertEqual(t, "Inputs.OrderID", seq.Inputs["OrderID"], "order.ID")
	assertEqual(t, "Inputs.Email", seq.Inputs["Email"], "order.Email")
	if seq.Options != nil {
		t.Errorf("expected nil options, got %v", seq.Options)
	}
}

func TestParsePublishWithOptions(t *testing.T) {
	src := `package service

// @publish "cart.abandoned" {CartID: cart.ID, UserID: currentUser.ID} {delay: 1800}
func AbandonCart() {}
`
	sfs := parseTestFile(t, src)
	seq := sfs[0].Sequences[0]
	assertEqual(t, "Type", seq.Type, SeqPublish)
	assertEqual(t, "Topic", seq.Topic, "cart.abandoned")
	assertEqual(t, "Inputs.CartID", seq.Inputs["CartID"], "cart.ID")
	if seq.Options == nil {
		t.Fatal("expected options")
	}
	assertEqual(t, "Options.delay", seq.Options["delay"], "1800")
}

func TestParseSubscribe(t *testing.T) {
	src := `package service

type OnOrderCompletedMessage struct {
	OrderID int64
}

// @subscribe "order.completed"
// @get Order order = Order.FindByID({ID: message.OrderID})
func OnOrderCompleted(message OnOrderCompletedMessage) {}
`
	sfs := parseTestFile(t, src)
	sf := sfs[0]
	if sf.Subscribe == nil {
		t.Fatal("expected Subscribe to be set")
	}
	assertEqual(t, "Subscribe.Topic", sf.Subscribe.Topic, "order.completed")
	assertEqual(t, "Subscribe.MessageType", sf.Subscribe.MessageType, "OnOrderCompletedMessage")
	// @subscribe는 시퀀스에 포함되지 않아야 함
	if len(sf.Sequences) != 1 {
		t.Fatalf("expected 1 sequence (subscribe filtered), got %d", len(sf.Sequences))
	}
	assertEqual(t, "seq0.Type", sf.Sequences[0].Type, SeqGet)
}

func TestParseSubscribeWithSequences(t *testing.T) {
	src := `package service

type OnOrderCompletedMessage struct {
	OrderID int64
	Email   string
}

// @subscribe "order.completed"
// @get Order order = Order.FindByID({ID: message.OrderID})
// @call mail.SendEmail({To: message.Email, Subject: "done"})
// @put Order.UpdateNotified({ID: order.ID, Notified: "true"})
func OnOrderCompleted(message OnOrderCompletedMessage) {}
`
	sfs := parseTestFile(t, src)
	sf := sfs[0]
	if sf.Subscribe == nil {
		t.Fatal("expected Subscribe to be set")
	}
	assertEqual(t, "Subscribe.Topic", sf.Subscribe.Topic, "order.completed")
	assertEqual(t, "Subscribe.MessageType", sf.Subscribe.MessageType, "OnOrderCompletedMessage")
	if len(sf.Sequences) != 3 {
		t.Fatalf("expected 3 sequences, got %d", len(sf.Sequences))
	}
	assertEqual(t, "seq0.Type", sf.Sequences[0].Type, SeqGet)
	assertEqual(t, "seq1.Type", sf.Sequences[1].Type, SeqCall)
	assertEqual(t, "seq2.Type", sf.Sequences[2].Type, SeqPut)
}

func TestParseSubscribeParam(t *testing.T) {
	src := `package service

type MyMsg struct {
	ID int64
}

// @subscribe "test.topic"
// @get Order order = Order.FindByID({ID: message.ID})
func OnTest(message MyMsg) {}
`
	sfs := parseTestFile(t, src)
	sf := sfs[0]
	if sf.Param == nil {
		t.Fatal("expected Param to be set")
	}
	assertEqual(t, "Param.TypeName", sf.Param.TypeName, "MyMsg")
	assertEqual(t, "Param.VarName", sf.Param.VarName, "message")
}

func TestParseStructs(t *testing.T) {
	src := `package service

type OnOrderCompletedMessage struct {
	OrderID int64
	Email   string
	Amount  int64
}

// @subscribe "order.completed"
// @get Order order = Order.FindByID({ID: message.OrderID})
func OnOrderCompleted(message OnOrderCompletedMessage) {}
`
	sfs := parseTestFile(t, src)
	sf := sfs[0]
	if len(sf.Structs) != 1 {
		t.Fatalf("expected 1 struct, got %d", len(sf.Structs))
	}
	si := sf.Structs[0]
	assertEqual(t, "Struct.Name", si.Name, "OnOrderCompletedMessage")
	if len(si.Fields) != 3 {
		t.Fatalf("expected 3 fields, got %d", len(si.Fields))
	}
	assertEqual(t, "Field0.Name", si.Fields[0].Name, "OrderID")
	assertEqual(t, "Field0.Type", si.Fields[0].Type, "int64")
	assertEqual(t, "Field1.Name", si.Fields[1].Name, "Email")
	assertEqual(t, "Field1.Type", si.Fields[1].Type, "string")
}

// --- 패키지 접두사 모델 ---

func TestParsePackagePrefixModel(t *testing.T) {
	src := `package service

// @get Session session = session.Session.Get({token: request.Token})
func GetSession(c *gin.Context) {}
`
	sfs := parseTestFile(t, src)
	seq := sfs[0].Sequences[0]
	assertEqual(t, "Package", seq.Package, "session")
	assertEqual(t, "Model", seq.Model, "Session.Get")
	if seq.Result == nil {
		t.Fatal("expected result")
	}
	assertEqual(t, "Result.Type", seq.Result.Type, "Session")
}

func TestParseNoPackagePrefix(t *testing.T) {
	src := `package service

// @get User user = User.FindByID({ID: request.ID})
func GetUser(c *gin.Context) {}
`
	sfs := parseTestFile(t, src)
	seq := sfs[0].Sequences[0]
	assertEqual(t, "Package", seq.Package, "")
	assertEqual(t, "Model", seq.Model, "User.FindByID")
}

func TestParsePackagePrefixPut(t *testing.T) {
	src := `package service

// @put cache.Cache.Set({key: request.Key, value: request.Value})
func SetCache(c *gin.Context) {}
`
	sfs := parseTestFile(t, src)
	seq := sfs[0].Sequences[0]
	assertEqual(t, "Package", seq.Package, "cache")
	assertEqual(t, "Model", seq.Model, "Cache.Set")
}

// --- helpers ---

func parseTestFile(t *testing.T, src string) []ServiceFunc {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.go")
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}
	sfs, err := ParseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(sfs) == 0 {
		t.Fatal("expected at least 1 ServiceFunc")
	}
	return sfs
}

func assertEqual(t *testing.T, name, got, want string) {
	t.Helper()
	if got != want {
		t.Errorf("%s: got %q, want %q", name, got, want)
	}
}
