package service

import (
	"authz"
	"github.com/gin-gonic/gin"
	"model"
	"net/http"
	"strconv"
)

func DeleteRoom(c *gin.Context) {
	roomIDStr := c.Param("RoomID")
	roomID, err := strconv.ParseInt(roomIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid path parameter"})
		return
	}

	currentUser := c.MustGet("currentUser").(*model.CurrentUser)

	if err := authz.Check(currentUser, "delete", "room", authz.Input{ID: roomID}); err != nil {
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

	reservationCount, err := reservationModel.CountByRoomID(roomID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Reservation 조회 실패"})
		return
	}

	if reservationCount > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "예약이 존재하여 삭제할 수 없습니다"})
		return
	}

	err = roomModel.Delete(roomID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Room 삭제 실패"})
		return
	}

	c.JSON(http.StatusOK, gin.H{})

}
