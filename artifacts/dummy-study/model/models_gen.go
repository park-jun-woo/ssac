package model

import "time"

type ReservationModel interface {
	CountByRoomID(roomID int64) (*int, error)
	Create(id int64, roomID int64, startAt time.Time, endAt time.Time) (*Reservation, error)
	FindByID(reservationID int64) (*Reservation, error)
	FindConflict(roomID int64, startAt time.Time, endAt time.Time) (*Reservation, error)
	ListByUserID(id int64, opts QueryOpts) ([]Reservation, int, error)
	UpdateStatus(reservationID int64, cancelled string) error
}

type RoomModel interface {
	Delete(roomID int64) error
	FindByID(roomID int64) (*Room, error)
	Update(roomID int64, name string, capacity int64, location string) error
}

type SessionModel interface {
	Create(userID int64) (*Token, error)
}

type UserModel interface {
	FindByEmail(email string) (*User, error)
}

type QueryOpts struct {
	Limit   int
	Offset  int
	Cursor  string
	SortCol string
	SortDir string
	Filters map[string]string
}
