package service

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
)

func GetReservation(c *gin.Context) {
	reservationIDStr := c.Param("ReservationID")
	reservationID, err := strconv.ParseInt(reservationIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid path parameter"})
		return
	}

	reservation, err := reservationModel.FindByID(reservationID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Reservation 조회 실패"})
		return
	}

	if reservation == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "예약을 찾을 수 없습니다"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"reservation": reservation,
	})

}
