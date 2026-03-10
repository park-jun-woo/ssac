package service

import (
	"github.com/geul-org/fullend/pkg/auth"
	"github.com/gin-gonic/gin"
	"net/http"
)

func Login(c *gin.Context) {
	var req struct {
		Email    string `json:"Email"`
		Password string `json:"Password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	email := req.Email
	password := req.Password

	user, err := userModel.FindByEmail(email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "User 조회 실패"})
		return
	}

	if user == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "사용자를 찾을 수 없습니다"})
		return
	}

	if err = auth.VerifyPassword(auth.VerifyPasswordRequest{PasswordHash: user.PasswordHash, Password: password}); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "호출 실패"})
		return
	}

	token, err := sessionModel.Create(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Session 생성 실패"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token": token,
	})

}
