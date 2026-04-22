//ff:func feature=pkg-auth type=test control=sequence topic=auth-refresh
//ff:what RefreshStore Create/Consume/RevokeAll/재사용 감지 동작을 sqlmock 으로 검증한다
package auth

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

func TestRefreshStore_CreateConsume(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	store := &RefreshStore{DB: db}
	ctx := context.Background()
	token := "plaintext.refresh.jwt"
	claimSet := sampleClaim{UserID: 1, Email: "a@b.c", Role: "admin", OrgID: 42}
	expiresAt := time.Now().Add(time.Hour)

	// Create — expect an INSERT with the hashed token.
	mock.ExpectExec(`INSERT INTO refresh_tokens`).
		WithArgs(hashRefreshToken(token), sqlmock.AnyArg(), expiresAt).
		WillReturnResult(sqlmock.NewResult(1, 1))
	if err := store.Create(ctx, token, claimSet, expiresAt); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Consume — SELECT, UPDATE revoked_at, COMMIT.
	rawClaims, _ := json.Marshal(claimSet)
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT claims, expires_at, revoked_at FROM refresh_tokens`).
		WithArgs(hashRefreshToken(token)).
		WillReturnRows(sqlmock.NewRows([]string{"claims", "expires_at", "revoked_at"}).
			AddRow(rawClaims, expiresAt, nil))
	mock.ExpectExec(`UPDATE refresh_tokens SET revoked_at`).
		WithArgs(hashRefreshToken(token)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	got, err := store.Consume(ctx, token)
	if err != nil {
		t.Fatalf("Consume: %v", err)
	}
	var back sampleClaim
	if err := json.Unmarshal(got, &back); err != nil {
		t.Fatalf("unmarshal claims: %v", err)
	}
	if back != claimSet {
		t.Fatalf("claims mismatch\nwant: %+v\ngot:  %+v", claimSet, back)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestRefreshStore_ConsumeReuseDetected(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	store := &RefreshStore{DB: db, DetectReuseLogoutAll: true}
	ctx := context.Background()
	token := "reused.refresh.jwt"
	claimSet := sampleClaim{UserID: 1, Email: "a@b.c", Role: "admin", OrgID: 42}
	raw, _ := json.Marshal(claimSet)
	revokedAt := time.Now().Add(-time.Minute)
	expiresAt := time.Now().Add(time.Hour)

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT claims, expires_at, revoked_at FROM refresh_tokens`).
		WithArgs(hashRefreshToken(token)).
		WillReturnRows(sqlmock.NewRows([]string{"claims", "expires_at", "revoked_at"}).
			AddRow(raw, expiresAt, revokedAt))
	// Rollback is implicit via defer — sqlmock accepts it silently.
	mock.ExpectRollback()

	claimsBack, err := store.Consume(ctx, token)
	if !errors.Is(err, ErrRefreshTokenReused) {
		t.Fatalf("expected ErrRefreshTokenReused, got %v", err)
	}
	if len(claimsBack) == 0 {
		t.Fatal("expected revoked-row claims to be returned on reuse")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestRefreshStore_ConsumeMissing(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	store := &RefreshStore{DB: db}
	ctx := context.Background()
	token := "missing.jwt"

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT claims, expires_at, revoked_at FROM refresh_tokens`).
		WithArgs(hashRefreshToken(token)).
		WillReturnError(sqlErrNoRows())
	mock.ExpectRollback()

	if _, err := store.Consume(ctx, token); !errors.Is(err, ErrRefreshTokenNotFound) {
		t.Fatalf("expected ErrRefreshTokenNotFound, got %v", err)
	}
}

func TestRefreshStore_RevokeAllRejectsEmptyMatcher(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	store := &RefreshStore{DB: db}
	if err := store.RevokeAll(context.Background(), ClaimMatcher{}); err == nil {
		t.Fatal("expected empty matcher to be rejected")
	}
}

func TestRefreshStore_RevokeAllWithMatcher(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	store := &RefreshStore{DB: db}
	mock.ExpectExec(`UPDATE refresh_tokens SET revoked_at`).
		WithArgs(sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 3))

	if err := store.RevokeAll(context.Background(), ClaimMatcher{"user_id": int64(1)}); err != nil {
		t.Fatalf("RevokeAll: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func sqlErrNoRows() error {
	// Imported indirectly by sqlmock; expose via helper so tests don't
	// import database/sql just for the sentinel.
	return errSQLNoRows
}
