package service

import (
	"github.com/gin-gonic/gin"
	"model"
	"net/http"
	"strconv"
)

func ListMyReservations(c *gin.Context) {
	currentUser := c.MustGet("currentUser").(*model.CurrentUser)

	opts := QueryOpts{}
	if v := c.Query("limit"); v != "" {
		opts.Limit, _ = strconv.Atoi(v)
	}
	if v := c.Query("offset"); v != "" {
		opts.Offset, _ = strconv.Atoi(v)
	}
	if v := c.Query("sort"); v != "" {
		opts.SortCol = v
	}

	reservations, total, err := reservationModel.ListByUserID(currentUser.ID, opts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Reservation 조회 실패"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"reservations": reservations,
		"total":        total,
	})

}
