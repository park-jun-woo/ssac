package service

import (
	"authz"
	"github.com/geul-org/fullend/pkg/billing"
	"github.com/gin-gonic/gin"
	"model"
	"net/http"
	"states/reservationstate"
	"strconv"
)

func CancelReservation(c *gin.Context) {
	reservationIDStr := c.Param("ReservationID")
	reservationID, err := strconv.ParseInt(reservationIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid path parameter"})
		return
	}

	currentUser := c.MustGet("currentUser").(*model.CurrentUser)

	if err := authz.Check(currentUser, "cancel", "reservation", authz.Input{ID: reservationID}); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "권한이 없습니다"})
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

	if !reservationstate.CanTransition(reservationstate.Input{Status: reservation.Status}, "cancel") {
		c.JSON(http.StatusConflict, gin.H{"error": "취소할 수 없는 상태입니다"})
		return
	}

	refund, err := billing.CalculateRefund(billing.CalculateRefundRequest{ID: reservation.ID, StartAt: reservation.StartAt, EndAt: reservation.EndAt})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "호출 실패"})
		return
	}

	err = reservationModel.UpdateStatus(reservationID, "cancelled")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Reservation 수정 실패"})
		return
	}

	reservation, err = reservationModel.FindByID(reservationID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Reservation 조회 실패"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"refund":      refund,
		"reservation": reservation,
	})

}
