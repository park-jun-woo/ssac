package service

import "net/http"

// @sequence authorize
// @action cancel
// @resource reservation
// @id ReservationID

// @sequence get
// @model Reservation.FindByID
// @param ReservationID request
// @result reservation Reservation

// @sequence guard nil reservation
// @message "예약을 찾을 수 없습니다"

// @sequence call
// @func billing.calculateRefund
// @param reservation
// @result refund Refund

// @sequence put
// @model Reservation.UpdateStatus
// @param ReservationID request
// @param "cancelled"

// @sequence response json
// @var reservation
// @var refund
func CancelReservation(w http.ResponseWriter, r *http.Request) {}
