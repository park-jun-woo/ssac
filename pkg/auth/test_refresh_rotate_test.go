//ff:func feature=pkg-auth type=test control=sequence topic=auth-refresh
//ff:what RefreshRotate happy path + reuse 감지 + invalid JWT — sqlmock 기반
package auth

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

func TestRefreshRotate_HappyPath(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret-test-secret-test-secret-xx")
	Configure(Config{SecretEnv: "JWT_SECRET", AccessTTL: time.Minute, RefreshTTL: time.Hour})

	src := sampleClaim{UserID: 1, Email: "a@b.c", Role: "admin", OrgID: 42}
	issued, err := RefreshToken(RefreshTokenRequest{Claims: src})
	if err != nil {
		t.Fatalf("RefreshToken: %v", err)
	}
	rawClaims, _ := json.Marshal(src)

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	// Consume: existing active row → revoke.
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT claims, expires_at, revoked_at FROM refresh_tokens`).
		WithArgs(hashRefreshToken(issued.RefreshToken)).
		WillReturnRows(sqlmock.NewRows([]string{"claims", "expires_at", "revoked_at"}).
			AddRow(rawClaims, issued.ExpiresAt, nil))
	mock.ExpectExec(`UPDATE refresh_tokens SET revoked_at`).
		WithArgs(hashRefreshToken(issued.RefreshToken)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	// Create new row for the rotated refresh.
	mock.ExpectExec(`INSERT INTO refresh_tokens`).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	store := &RefreshStore{DB: db}
	out, err := RefreshRotate(context.Background(), store, issued.RefreshToken)
	if err != nil {
		t.Fatalf("RefreshRotate: %v", err)
	}
	if out.AccessToken == "" || out.RefreshToken == "" {
		t.Fatalf("missing tokens in response: %+v", out)
	}
	verified, err := VerifyToken(VerifyTokenRequest{Token: out.AccessToken})
	if err != nil {
		t.Fatalf("verify new access: %v", err)
	}
	if int64(verified.Claims["user_id"].(float64)) != 1 {
		t.Fatalf("claims not preserved in rotation: %v", verified.Claims)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestRefreshRotate_ReuseDetected(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret-test-secret-test-secret-xx")
	Configure(Config{SecretEnv: "JWT_SECRET", AccessTTL: time.Minute, RefreshTTL: time.Hour})

	src := sampleClaim{UserID: 1, Email: "a@b.c", Role: "admin", OrgID: 42}
	issued, err := RefreshToken(RefreshTokenRequest{Claims: src})
	if err != nil {
		t.Fatalf("RefreshToken: %v", err)
	}
	rawClaims, _ := json.Marshal(src)
	revokedAt := time.Now().Add(-time.Minute)

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	// Consume: row is already revoked — must return ErrRefreshTokenReused.
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT claims, expires_at, revoked_at FROM refresh_tokens`).
		WithArgs(hashRefreshToken(issued.RefreshToken)).
		WillReturnRows(sqlmock.NewRows([]string{"claims", "expires_at", "revoked_at"}).
			AddRow(rawClaims, issued.ExpiresAt, revokedAt))
	mock.ExpectRollback()
	// DetectReuseLogoutAll → bulk revoke is called.
	mock.ExpectExec(`UPDATE refresh_tokens SET revoked_at`).
		WithArgs(sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 2))

	store := &RefreshStore{DB: db, DetectReuseLogoutAll: true}
	_, err = RefreshRotate(context.Background(), store, issued.RefreshToken)
	if !errors.Is(err, ErrRefreshTokenReused) {
		t.Fatalf("expected ErrRefreshTokenReused, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestRefreshRotate_InvalidJWT(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret-test-secret-test-secret-xx")
	Configure(Config{SecretEnv: "JWT_SECRET", AccessTTL: time.Minute, RefreshTTL: time.Hour})

	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	store := &RefreshStore{DB: db}
	_, err = RefreshRotate(context.Background(), store, "garbage.not.jwt")
	if err == nil {
		t.Fatal("expected error for invalid JWT, got nil")
	}
	// Must surface as a verify-prefixed error so the legacy handler's
	// 401 mapping still works.
	if !containsVerifyPrefix(err.Error()) {
		t.Fatalf("expected verify-prefixed error, got %q", err.Error())
	}
}
