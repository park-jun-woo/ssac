package generator

import (
	"testing"

	"github.com/park-jun-woo/ssac/parser"
	"github.com/park-jun-woo/ssac/validator"
)

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
			{Type: parser.SeqGet, Model: "Course.FindByID", Inputs: map[string]string{"CourseID": "request.CourseID"}, Result: &parser.Result{Type: "Course", Var: "course"}},
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
	assertContains(t, data, "WithTx(tx *sql.Tx) CourseModel")
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
			{Type: parser.SeqGet, Model: "User.FindByID", Inputs: map[string]string{"ID": "currentUser.ID"}, Result: &parser.Result{Type: "User", Var: "user"}},
			{Type: parser.SeqGet, Model: "Reservation.ListByUserID", Inputs: map[string]string{"UserID": "currentUser.ID", "Opts": "query"}, Result: &parser.Result{Type: "[]Reservation", Var: "reservations"}},
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
	assertContains(t, data, "ListByUserID(userID int64, opts QueryOpts)")
	// query arg가 없는 메서드에는 opts 없음
	assertNotContains(t, data, "FindByID(id int64, opts QueryOpts)")
	assertContains(t, data, "FindByID(id int64)")
}

func TestGeneratePageReturnType(t *testing.T) {
	st := &validator.SymbolTable{
		Models: map[string]validator.ModelSymbol{
			"Gig": {Methods: map[string]validator.MethodInfo{
				"List": {Cardinality: "many"},
			}},
		},
		DDLTables:  map[string]validator.DDLTable{},
		Operations: map[string]validator.OperationSymbol{},
	}
	funcs := []parser.ServiceFunc{{
		Name: "ListGigs", FileName: "list_gigs.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqGet, Model: "Gig.List", Inputs: map[string]string{"Query": "query"}, Result: &parser.Result{Type: "Gig", Var: "gigPage", Wrapper: "Page"}},
			{Type: parser.SeqResponse, Target: "gigPage"},
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
	assertContains(t, data, "(*pagination.Page[Gig], error)")
	assertContains(t, data, "pagination")
}

func TestGeneratePackageModelSkipInterface(t *testing.T) {
	st := &validator.SymbolTable{
		Models: map[string]validator.ModelSymbol{
			"Gig": {Methods: map[string]validator.MethodInfo{
				"FindByID": {Cardinality: "one"},
			}},
			"session.Session": {Methods: map[string]validator.MethodInfo{
				"Get": {},
			}},
		},
		DDLTables: map[string]validator.DDLTable{
			"gigs": {Columns: map[string]string{"id": "int64"}},
		},
		Operations: map[string]validator.OperationSymbol{},
	}
	funcs := []parser.ServiceFunc{{
		Name: "GetGig", FileName: "get_gig.go",
		Sequences: []parser.Sequence{
			{Type: parser.SeqGet, Model: "Gig.FindByID", Inputs: map[string]string{"ID": "request.ID"}, Result: &parser.Result{Type: "Gig", Var: "gig"}},
			{Type: parser.SeqGet, Package: "session", Model: "Session.Get", Inputs: map[string]string{"token": "request.Token"}, Result: &parser.Result{Type: "Session", Var: "session"}},
			{Type: parser.SeqResponse, Fields: map[string]string{"gig": "gig"}},
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
	// Gig 모델은 포함, session.Session 패키지 모델은 제외
	assertContains(t, data, "type GigModel interface")
	assertNotContains(t, data, "SessionModel")
}
