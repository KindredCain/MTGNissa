package httpapi

import "mtgnissa/internal/setlist"

type setListResponse struct {
	Items      []setListItemResponse `json:"items"`
	Total      int64                 `json:"total"`
	NextCursor *string               `json:"next_cursor"`
}

type setListItemResponse struct {
	Code            string                `json:"code"`
	Name            string                `json:"name"`
	DisplayName     string                `json:"display_name"`
	SetType         string                `json:"set_type"`
	CardCount       uint64                `json:"card_count"`
	ReleasedAt      *string               `json:"released_at"`
	Languages       []setLanguageResponse `json:"languages"`
	DefaultLanguage *string               `json:"default_language"`
	ParentCode      *string               `json:"parent_code"`
	Depth           int                   `json:"depth"`
}

type setLanguageResponse struct {
	Code        string `json:"code"`
	DisplayName string `json:"display_name"`
	SortOrder   int    `json:"sort_order"`
	Available   bool   `json:"available"`
}

func newSetListResponse(page setlist.Page) setListResponse {
	response := setListResponse{
		Items:      make([]setListItemResponse, 0, len(page.Items)),
		Total:      page.Total,
		NextCursor: page.NextCursor,
	}
	for _, item := range page.Items {
		var releasedAt *string
		if item.ReleasedAt != nil {
			value := item.ReleasedAt.Format("2006-01-02")
			releasedAt = &value
		}
		languages := make([]setLanguageResponse, 0, len(item.Languages))
		for _, language := range item.Languages {
			languages = append(languages, setLanguageResponse{
				Code:        language.Code,
				DisplayName: language.DisplayName,
				SortOrder:   language.SortOrder,
				Available:   language.Available,
			})
		}
		response.Items = append(response.Items, setListItemResponse{
			Code:            item.Code,
			Name:            item.Name,
			DisplayName:     item.DisplayName,
			SetType:         item.SetType,
			CardCount:       item.CardCount,
			ReleasedAt:      releasedAt,
			Languages:       languages,
			DefaultLanguage: item.DefaultLanguage,
			ParentCode:      item.ParentCode,
			Depth:           item.Depth,
		})
	}
	return response
}
