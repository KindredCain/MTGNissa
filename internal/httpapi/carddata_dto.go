package httpapi

import (
	"time"

	"mtgnissa/internal/carddata"
)

type startLoadRequest struct {
	Confirmation string `json:"confirmation"`
}

type loadTaskResponse struct {
	ID         string                 `json:"id"`
	Stage      carddata.Stage         `json:"stage"`
	StartedAt  time.Time              `json:"started_at"`
	FinishedAt *time.Time             `json:"finished_at"`
	Error      *loadTaskErrorResponse `json:"error"`
}

type loadTaskErrorResponse struct {
	Stage   carddata.Stage `json:"stage"`
	File    string         `json:"file,omitempty"`
	Line    int64          `json:"line,omitempty"`
	Message string         `json:"message"`
}

func newLoadTaskResponse(task carddata.Task) loadTaskResponse {
	response := loadTaskResponse{
		ID:         task.ID,
		Stage:      task.Stage,
		StartedAt:  task.StartedAt,
		FinishedAt: task.FinishedAt,
	}
	if task.Error != nil {
		response.Error = &loadTaskErrorResponse{
			Stage:   task.Error.Stage,
			File:    task.Error.File,
			Line:    task.Error.Line,
			Message: "card data loading failed",
		}
	}
	return response
}
