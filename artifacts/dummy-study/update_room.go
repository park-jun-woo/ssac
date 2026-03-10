package service

import (
	"authz"
	"github.com/gin-gonic/gin"
	"model"
	"net/http"
	"strconv"
)

func UpdateRoom(c *gin.Context) {
	roomIDStr := c.Param("RoomID")
	roomID, err := strconv.ParseInt(roomIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid path parameter"})
		return
	}

	currentUser := c.MustGet("currentUser").(*model.CurrentUser)

	var req struct {
		Name     string `json:"Name"`
		Capacity int64  `json:"Capacity"`
		Location string `json:"Location"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	name := req.Name
	capacity := req.Capacity
	location := req.Location

	if err := authz.Check(currentUser, "update", "room", authz.Input{ID: roomID}); err != nil {
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

	err = roomModel.Update(roomID, name, capacity, location)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Room 수정 실패"})
		return
	}

	room, err = roomModel.FindByID(roomID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Room 조회 실패"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"room": room,
	})

}
