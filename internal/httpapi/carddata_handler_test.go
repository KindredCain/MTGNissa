package httpapi

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
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
	router := New(log, health.Handler{}, loaderStub{}, true, setListerStub{})

	request := httptest.NewRequest(http.MethodPost, "/api/v1/card_data/load", bytes.NewBufferString(`{"confirmation":"LOAD_CARD_DATA"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusAccepted {
		t.Fatalf("POST load status = %d, body = %s", response.Code, response.Body.String())
	}
	if got := response.Header().Get("Location"); got != "/api/v1/card_data/load/task-1" {
		t.Fatalf("Location = %q", got)
	}
	body := response.Body.String()
	for _, field := range []string{`"finished_at":null`, `"error":null`} {
		if !strings.Contains(body, field) {
			t.Errorf("POST load body does not contain %s: %s", field, body)
		}
	}

	request = httptest.NewRequest(http.MethodGet, "/api/v1/card_data/load/task-1", nil)
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("GET load task status = %d, body = %s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `"id":"task-1"`) {
		t.Fatalf("GET load task body does not contain task id: %s", response.Body.String())
	}
}

func TestLoadTaskResponseHidesInternalFailure(t *testing.T) {
	response := newLoadTaskResponse(carddata.Task{
		ID:    "task-1",
		Stage: carddata.StageFailed,
		Error: &carddata.TaskError{
			Stage:   carddata.StageImporting,
			File:    "scryfall_card.json",
			Line:    12,
			Message: `insert into secret_table failed at C:\\private\\card-data`,
		},
	})
	if response.Error == nil {
		t.Fatal("task response error is nil")
	}
	if response.Error.Message != "card data loading failed" {
		t.Fatalf("task response message = %q", response.Error.Message)
	}
	if strings.Contains(response.Error.Message, "secret_table") || strings.Contains(response.Error.Message, `C:\\private`) {
		t.Fatalf("task response exposes internal failure: %q", response.Error.Message)
	}
	if response.Error.File != "scryfall_card.json" || response.Error.Line != 12 {
		t.Fatalf("task response source location = %#v", response.Error)
	}
}
