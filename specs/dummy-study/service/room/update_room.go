package service

// @auth "update" "room" {id: request.RoomID} "권한이 없습니다"
// @get Room room = Room.FindByID({RoomID: request.RoomID})
// @empty room "스터디룸이 존재하지 않습니다"
// @put Room.Update({RoomID: request.RoomID, Name: request.Name, Capacity: request.Capacity, Location: request.Location})
// @get Room room = Room.FindByID({RoomID: request.RoomID})
// @response {
//   room: room
// }
func UpdateRoom() {}
