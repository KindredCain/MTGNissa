package httpapi

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"mtgnissa/internal/carddata"
	"mtgnissa/internal/health"
)

type loaderStub struct{}

func (loaderStub) Start() (carddata.Task, error) {
	return carddata.Task{ID: "task-1", Stage: carddata.StageValidating}, nil
}

func (loaderStub) Get(id string) (carddata.Task, bool) {
	if id != "task-1" {
		return carddata.Task{}, false
	}
	return carddata.Task{ID: id, Stage: carddata.StageValidating}, true
}

func TestLoadCardDataRoutes(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	router := New(log, health.Handler{}, loaderStub{}, true)

	request := httptest.NewRequest(http.MethodPost, "/api/v1/card-data/load", bytes.NewBufferString(`{"confirmation":"LOAD_CARD_DATA"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusAccepted {
		t.Fatalf("POST load status = %d, body = %s", response.Code, response.Body.String())
	}
	if got := response.Header().Get("Location"); got != "/api/v1/card-data/load/task-1" {
		t.Fatalf("Location = %q", got)
	}
}
