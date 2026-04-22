//ff:func feature=pkg-queue type=test control=sequence
//ff:what postgres 백엔드에서 PublishTx가 tx.ExecContext로 fullend_queue INSERT를 수행하는지 stub driver로 검증한다
package queue

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"strings"
	"sync"
	"testing"
)

// stubDriver captures the last Exec call made via a *sql.Tx so the test can
// assert that PublishTx binds its INSERT to the caller's transaction.
type stubDriver struct{ conn *stubConn }

func (d *stubDriver) Open(name string) (driver.Conn, error) { return d.conn, nil }

type stubConn struct {
	mu       sync.Mutex
	lastSQL  string
	lastArgs []driver.Value
	inTx     bool
	txExec   bool // true if last Exec happened via a Tx (not direct)
}

func (c *stubConn) Prepare(query string) (driver.Stmt, error) { return &stubStmt{c: c, q: query}, nil }
func (c *stubConn) Close() error                              { return nil }
func (c *stubConn) Begin() (driver.Tx, error)                 { c.inTx = true; return &stubTx{c: c}, nil }

// Exec implements driver.Execer (legacy path) — used so sql.DB.ExecContext
// can work without relying on the stmt path.
func (c *stubConn) Exec(query string, args []driver.Value) (driver.Result, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastSQL = query
	c.lastArgs = args
	c.txExec = c.inTx
	return driver.RowsAffected(0), nil
}

type stubTx struct{ c *stubConn }

func (t *stubTx) Commit() error   { t.c.inTx = false; return nil }
func (t *stubTx) Rollback() error { t.c.inTx = false; return nil }

type stubStmt struct {
	c *stubConn
	q string
}

func (s *stubStmt) Close() error  { return nil }
func (s *stubStmt) NumInput() int { return -1 }
func (s *stubStmt) Exec(args []driver.Value) (driver.Result, error) {
	s.c.mu.Lock()
	defer s.c.mu.Unlock()
	s.c.lastSQL = s.q
	s.c.lastArgs = args
	s.c.txExec = s.c.inTx
	return driver.RowsAffected(0), nil
}
func (s *stubStmt) Query(args []driver.Value) (driver.Rows, error) { return nil, errors.New("no rows") }

func registerStubOnce(name string, d *stubDriver) {
	for _, n := range sql.Drivers() {
		if n == name {
			return
		}
	}
	sql.Register(name, d)
}

// bypassInit forcibly sets the queue package to the "postgres" backend without
// running initPostgres (which issues DDL). The stub driver does not need the
// table, so we just set internal state directly.
func bypassInit(t *testing.T, dbConn *sql.DB) {
	t.Helper()
	mu.Lock()
	defer mu.Unlock()
	backend = "postgres"
	db = dbConn
	handlers = map[string][]func(ctx context.Context, msg []byte) error{}
	inited = true
}

func TestPublishTxPostgresBindsToTx(t *testing.T) {
	resetQueue()
	ctx := context.Background()

	conn := &stubConn{}
	registerStubOnce("stub-publishtx", &stubDriver{conn: conn})

	dbConn, err := sql.Open("stub-publishtx", "")
	if err != nil {
		t.Fatal(err)
	}
	defer dbConn.Close()

	bypassInit(t, dbConn)
	defer resetQueue()

	tx, err := dbConn.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}

	if err := PublishTx(ctx, tx, "order.created", map[string]int64{"OrderID": 42},
		WithPriority("high")); err != nil {
		t.Fatalf("PublishTx: %v", err)
	}

	// INSERT must have been routed through the Tx (not a fresh conn-less Exec).
	if !conn.txExec {
		t.Errorf("PublishTx Exec did not run inside a Tx")
	}
	if !strings.Contains(conn.lastSQL, "INSERT INTO fullend_queue") {
		t.Errorf("unexpected SQL: %q", conn.lastSQL)
	}
	if len(conn.lastArgs) != 5 {
		t.Fatalf("expected 5 args (topic,payload,priority,deliver_at,traceparent), got %d", len(conn.lastArgs))
	}
	if got := conn.lastArgs[0]; got != "order.created" {
		t.Errorf("topic arg = %v, want order.created", got)
	}
	if got := conn.lastArgs[2]; got != "high" {
		t.Errorf("priority arg = %v, want high", got)
	}
	// traceparent is empty when no active span — column carries "" by contract.
	if got := conn.lastArgs[4]; got != "" {
		t.Errorf("traceparent arg = %v, want empty (no active span)", got)
	}

	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
}

func TestPublishTxPostgresNilTx(t *testing.T) {
	resetQueue()
	ctx := context.Background()

	conn := &stubConn{}
	registerStubOnce("stub-publishtx-niltx", &stubDriver{conn: conn})
	dbConn, err := sql.Open("stub-publishtx-niltx", "")
	if err != nil {
		t.Fatal(err)
	}
	defer dbConn.Close()

	bypassInit(t, dbConn)
	defer resetQueue()

	err = PublishTx(ctx, nil, "x", map[string]any{"a": 1})
	if err == nil {
		t.Fatal("expected error for nil tx, got nil")
	}
	if !strings.Contains(err.Error(), "non-nil") {
		t.Errorf("unexpected error: %v", err)
	}
}
