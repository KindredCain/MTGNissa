package httpapi

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"mtgnissa/internal/apperror"
	"mtgnissa/internal/carddata"
	"mtgnissa/internal/httpresponse"
)

type cardDataLoader interface {
	Start() (carddata.Task, error)
	Get(taskID string) (carddata.Task, bool)
}

func startLoad(manager cardDataLoader, enabled bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !enabled {
			httpresponse.WriteAppError(c, apperror.New(apperror.CodeLoadDisabled, "card data loading is disabled"))
			return
		}
		var request startLoadRequest
		decoder := jsonDecoder(c.Request.Body)
		if err := decoder.Decode(&request); err != nil {
			httpresponse.WriteAppError(c, requestDecodeError(err, "body must be a JSON object containing confirmation"))
			return
		}
		if err := ensureEOF(decoder); err != nil {
			httpresponse.WriteAppError(c, requestDecodeError(err, "body must contain one JSON object"))
			return
		}
		if request.Confirmation != carddata.Confirmation {
			httpresponse.WriteAppError(c, apperror.New(apperror.CodeConfirmationRequired, "confirmation must equal "+carddata.Confirmation))
			return
		}
		task, err := manager.Start()
		if errors.Is(err, carddata.ErrRunning) {
			httpresponse.WriteAppError(c, apperror.Wrap(apperror.CodeLoadInProgress, "a card data load task is already running", err))
			return
		}
		if err != nil {
			httpresponse.WriteAppError(c, apperror.Wrap(apperror.CodeInternalError, "could not start card data loading", err))
			return
		}
		c.Header("Location", "/api/v1/card_data/load/"+task.ID)
		httpresponse.WriteJSON(c, http.StatusAccepted, newLoadTaskResponse(task))
	}
}

func getLoad(manager cardDataLoader, enabled bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !enabled {
			httpresponse.WriteAppError(c, apperror.New(apperror.CodeLoadDisabled, "card data loading is disabled"))
			return
		}
		task, ok := manager.Get(c.Param("task_id"))
		if !ok {
			httpresponse.WriteAppError(c, apperror.New(apperror.CodeTaskNotFound, "card data load task not found"))
			return
		}
		httpresponse.WriteJSON(c, http.StatusOK, newLoadTaskResponse(task))
	}
}
