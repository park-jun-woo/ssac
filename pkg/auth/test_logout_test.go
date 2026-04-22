//ff:func feature=pkg-auth type=test control=sequence topic=auth-refresh
//ff:what Logout idempotent 검증 — 미존재/이미 revoked token 도 nil error
package auth

import (
	"context"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

func TestLogout_IdempotentRevoke(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	store := &RefreshStore{DB: db}
	token := "plaintext.refresh.jwt"

	// Revoke — UPDATE WHERE revoked_at IS NULL. Affected rows can be 0
	// (already revoked) or 1 (newly revoked); both are no-error.
	mock.ExpectExec(`UPDATE refresh_tokens SET revoked_at`).
		WithArgs(hashRefreshToken(token)).
		WillReturnResult(sqlmock.NewResult(0, 0))

	out, err := Logout(context.Background(), store, token)
	if err != nil {
		t.Fatalf("Logout: %v", err)
	}
	if !out.Success {
		t.Fatalf("expected Success=true, got %+v", out)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestLogout_EmptyTokenSilent(t *testing.T) {
	// No DB calls expected — an empty token is a no-op.
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	out, err := Logout(context.Background(), &RefreshStore{DB: db}, "")
	if err != nil {
		t.Fatalf("Logout empty: %v", err)
	}
	if !out.Success {
		t.Fatalf("empty token should still return Success=true, got %+v", out)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestLogout_NilStore(t *testing.T) {
	_, err := Logout(context.Background(), nil, "some.token")
	if err == nil {
		t.Fatal("expected error for nil store")
	}
}
