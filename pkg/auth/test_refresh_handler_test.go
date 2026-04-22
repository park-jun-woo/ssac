//ff:func feature=pkg-auth type=test control=sequence topic=auth-refresh
//ff:what RefreshHandler end-to-end — mock JWT + mock store 로 rotation 흐름을 검증한다
package auth

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
)

func TestRefreshHandler_HappyPath(t *testing.T) {
	gin.SetMode(gin.TestMode)
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
	r := gin.New()
	r.POST("/auth/refresh", RefreshHandler(store))

	body, _ := json.Marshal(map[string]string{"refresh_token": issued.RefreshToken})
	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200. body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal resp: %v", err)
	}
	if resp.AccessToken == "" || resp.RefreshToken == "" {
		t.Fatalf("missing tokens in response: %+v", resp)
	}
	// The new access token must carry the same claim set.
	verified, err := VerifyToken(VerifyTokenRequest{Token: resp.AccessToken})
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

func TestRefreshHandler_InvalidJWT(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("JWT_SECRET", "test-secret-test-secret-test-secret-xx")
	Configure(Config{SecretEnv: "JWT_SECRET", AccessTTL: time.Minute, RefreshTTL: time.Hour})

	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	r := gin.New()
	r.POST("/auth/refresh", RefreshHandler(&RefreshStore{DB: db}))

	body, _ := json.Marshal(map[string]string{"refresh_token": "garbage.not.jwt"})
	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status: got %d, want 401", rec.Code)
	}
}
