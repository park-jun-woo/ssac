package service

import (
	"authz"
	"github.com/gin-gonic/gin"
	"model"
	"net/http"
	"time"
)

func CreateReservation(c *gin.Context) {
	currentUser := c.MustGet("currentUser").(*model.CurrentUser)

	var req struct {
		RoomID  int64     `json:"RoomID"`
		StartAt time.Time `json:"StartAt"`
		EndAt   time.Time `json:"EndAt"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	roomID := req.RoomID
	startAt := req.StartAt
	endAt := req.EndAt

	if err := authz.Check(currentUser, "create", "reservation", authz.Input{ID: roomID}); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "권한이 없습니다"})
		return
	}

	room, err := roomModel.FindByID(roomID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Room 조회 실패"})
		return
	}

	if room == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "스터디룸이 존재하지 않습니다"})
		return
	}

	conflict, err := reservationModel.FindConflict(roomID, startAt, endAt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Reservation 조회 실패"})
		return
	}

	if conflict != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "해당 시간에 이미 예약이 있습니다"})
		return
	}

	reservation, err := reservationModel.Create(currentUser.ID, roomID, startAt, endAt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Reservation 생성 실패"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"reservation": reservation,
	})

}
