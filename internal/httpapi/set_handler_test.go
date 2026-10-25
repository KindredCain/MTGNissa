package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"mtgnissa/internal/health"
	"mtgnissa/internal/setlist"
)

type setListerStub struct {
	page   setlist.Page
	params setlist.Params
	err    error
}

func (s setListerStub) List(_ context.Context, params setlist.Params) (setlist.Page, error) {
	s.params = params
	return s.page, s.err
}

func TestGetSetsReturnsContractDTO(t *testing.T) {
	defaultLanguage := "en"
	releasedAt := time.Date(2021, time.February, 5, 0, 0, 0, 0, time.UTC)
	parentCode := "KHM"
	nextCursor := "next-page"
	service := &capturingSetLister{page: setlist.Page{
		Items: []setlist.Set{{
			Code:            "KHM",
			Name:            "Kaldheim",
			DisplayName:     "凯德海姆",
			SetType:         "expansion",
			CardCount:       405,
			ReleasedAt:      &releasedAt,
			Languages:       []setlist.Language{{Code: "en", DisplayName: "英文", SortOrder: 1, Available: true}, {Code: "ja", DisplayName: "日文", SortOrder: 2, Available: false}},
			DefaultLanguage: &defaultLanguage,
		}, {Code: "TKHM", Name: "Kaldheim Tokens", SetType: "token", ParentCode: &parentCode, Depth: 1}},
		Total:      1,
		NextCursor: &nextCursor,
	}}
	router := newTestRouter(service)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/sets?q=Kaldheim&language=ja&language=en&set_type=expansion&sort=name_asc", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if service.params.Query != "Kaldheim" || service.params.Sort != setlist.SortNameAsc {
		t.Fatalf("params = %#v", service.params)
	}
	if strings.Join(service.params.Languages, ",") != "en,ja" {
		t.Fatalf("languages = %#v", service.params.Languages)
	}
	var envelope struct {
		Status int `json:"status"`
		Data   struct {
			Items []struct {
				DisplayName     string  `json:"display_name"`
				ReleasedAt      *string `json:"released_at"`
				DefaultLanguage *string `json:"default_language"`
				ParentCode      *string `json:"parent_code"`
				Depth           int     `json:"depth"`
				Languages       []any   `json:"languages"`
			} `json:"items"`
			Total      int64   `json:"total"`
			NextCursor *string `json:"next_cursor"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	item := envelope.Data.Items[0]
	if item.DisplayName != "凯德海姆" || item.ReleasedAt == nil || *item.ReleasedAt != "2021-02-05" || item.DefaultLanguage == nil || *item.DefaultLanguage != "en" || item.ParentCode != nil || item.Depth != 0 {
		t.Fatalf("item = %#v", item)
	}
	child := envelope.Data.Items[1]
	if child.ParentCode == nil || *child.ParentCode != parentCode || child.Depth != 1 || child.ReleasedAt != nil || child.DefaultLanguage != nil || child.Languages == nil || len(child.Languages) != 0 {
		t.Fatalf("token child = %#v", child)
	}
	if strings.Contains(response.Body.String(), `"set_id"`) || strings.Contains(response.Body.String(), `"parent_set_id"`) {
		t.Fatalf("response contains obsolete identifiers: %s", response.Body.String())
	}
	if envelope.Data.Total != 1 || envelope.Data.NextCursor == nil || *envelope.Data.NextCursor != nextCursor {
		t.Fatalf("pagination = %#v", envelope.Data)
	}
}

func TestGetSetsRejectsInvalidInputs(t *testing.T) {
	tests := []struct {
		name       string
		target     string
		wantStatus int
		wantCode   string
	}{
		{name: "unknown sort", target: "/api/v1/sets?sort=newest", wantStatus: http.StatusBadRequest, wantCode: "invalid_request"},
		{name: "repeated scalar", target: "/api/v1/sets?q=a&q=b", wantStatus: http.StatusBadRequest, wantCode: "invalid_request"},
		{name: "long query", target: "/api/v1/sets?q=" + strings.Repeat("界", 513), wantStatus: http.StatusUnprocessableEntity, wantCode: "validation_failed"},
		{name: "unknown parameter", target: "/api/v1/sets?limit=1", wantStatus: http.StatusBadRequest, wantCode: "invalid_request"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			router := newTestRouter(&capturingSetLister{})
			request := httptest.NewRequest(http.MethodGet, test.target, nil)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != test.wantStatus || !strings.Contains(response.Body.String(), `"code":"`+test.wantCode+`"`) {
				t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
			}
		})
	}
}

type capturingSetLister struct {
	page   setlist.Page
	params setlist.Params
	err    error
}

func (s *capturingSetLister) List(_ context.Context, params setlist.Params) (setlist.Page, error) {
	s.params = params
	return s.page, s.err
}

func newTestRouter(sets setLister) http.Handler {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	return New(log, health.Handler{}, loaderStub{}, true, sets)
}
