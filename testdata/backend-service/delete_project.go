package service

import "net/http"

// @sequence authorize
// @action delete
// @resource project
// @id ProjectID

// @sequence get
// @model Project.FindByID
// @param ProjectID request
// @result project Project

// @sequence guard nil project
// @message "프로젝트가 존재하지 않습니다"

// @sequence get
// @model Session.CountByProjectID
// @param ProjectID request
// @result sessionCount int

// @sequence guard exists sessionCount
// @message "하위 세션이 존재하여 삭제할 수 없습니다"

// @sequence call
// @component notification
// @param project.OwnerEmail
// @param "프로젝트가 삭제됩니다"

// @sequence call
// @func cleanupProjectResources
// @param project
// @result cleaned bool

// @sequence delete
// @model Project.Delete
// @param ProjectID request

// @sequence response json
func DeleteProject(w http.ResponseWriter, r *http.Request) {}
