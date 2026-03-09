package service

import "net/http"

// @sequence authorize
// @action create
// @resource reservation
// @id RoomID

// @sequence get
// @model Room.FindByID
// @param RoomID request
// @result room Room

// @sequence guard nil room
// @message "스터디룸이 존재하지 않습니다"

// @sequence get
// @model Reservation.FindConflict
// @param RoomID request
// @param StartAt request
// @param EndAt request
// @result conflict Reservation

// @sequence guard exists conflict
// @message "해당 시간에 이미 예약이 있습니다"

// @sequence post
// @model Reservation.Create
// @param UserID currentUser
// @param RoomID request
// @param StartAt request
// @param EndAt request
// @result reservation Reservation

// @sequence response json
// @var reservation
func CreateReservation(w http.ResponseWriter, r *http.Request) {}
