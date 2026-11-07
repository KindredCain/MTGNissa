package cards

import (
	"encoding/hex"
	"sort"
	"strings"
)

func groupFaces(rows []faceRow) [][]faceRow {
	groups := make(map[[2]string][]faceRow)
	for _, row := range rows {
		key := [2]string{row.SourceID, row.LanguageCode}
		groups[key] = append(groups[key], row)
	}
	result := make([][]faceRow, 0, len(groups))
	for _, group := range groups {
		result = append(result, group)
	}
	return result
}

func summarize(rows []faceRow) (Summary, []Face) {
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Index != rows[j].Index {
			return rows[i].Index < rows[j].Index
		}
		return rows[i].UUID < rows[j].UUID
	})
	r := rows[0]
	displayName := r.DisplayName
	if len(rows) > 1 {
		names := make([]string, 0, len(rows))
		for _, row := range rows {
			names = append(names, row.DisplayName)
		}
		displayName = strings.Join(names, " // ")
	}
	faces := make([]Face, 0, len(rows))
	images := Images{FrontURL: picture(r.ImageID, "front")}
	if distinctBack(r) {
		images.BackURL = picture(r.ImageID, "back")
	}
	for _, row := range rows {
		image := picture(row.ImageID, "front")
		if row.Index > 0 {
			image = nil
			if distinctBack(row) {
				image = picture(row.ImageID, "back")
			}
		}
		faces = append(faces, Face{FaceIndex: row.Index, Name: row.FaceName, DisplayName: row.DisplayName,
			ManaCost: row.ManaCost, TypeLine: row.TypeLine, DisplayTypeLine: row.DisplayTypeLine,
			OracleText: row.Text, DisplayOracleText: row.DisplayText, FlavorText: row.Flavor,
			DisplayFlavorText: row.DisplayFlavor, Power: row.Power, Toughness: row.Toughness, Loyalty: row.Loyalty, ImageURL: image})
	}
	colors := make([]string, 0)
	for i, code := range []string{"W", "U", "B", "R", "G"} {
		if r.Colors&(1<<i) != 0 {
			colors = append(colors, code)
		}
	}
	return Summary{PrintingUUID: r.UUID, OracleID: r.OracleID, Name: r.Name, DisplayName: displayName,
		Set:             Set{Code: r.SetCode, Name: r.SetName, DisplayName: r.SetDisplayName, ReleasedAt: r.SetReleasedAt},
		CollectorNumber: r.Number, Language: Language{Code: r.LanguageCode, DisplayName: r.LanguageName, SortOrder: r.LanguageOrder},
		ReleasedAt: r.ReleasedAt, CMC: r.CMC, ManaCost: r.ManaCost, Colors: colors, Rarity: r.Rarity, Layout: r.Layout, Images: images}, faces
}

func distinctBack(row faceRow) bool {
	return row.FaceCategory == "double_sided" || row.FaceCategory == "reversible" || row.Layout == "battle"
}

func picture(id, side string) *string {
	id = strings.ToLower(id)
	if !ValidUUID(id) {
		return nil
	}
	url := "https://cards.scryfall.io/normal/" + side + "/" + id[:1] + "/" + id[1:2] + "/" + id + ".jpg"
	return &url
}

// ValidUUID checks canonical UUID text without accepting alternate encodings.
func ValidUUID(value string) bool {
	if len(value) != 36 || value[8] != '-' || value[13] != '-' || value[18] != '-' || value[23] != '-' {
		return false
	}
	_, err := hex.DecodeString(strings.ReplaceAll(value, "-", ""))
	return err == nil
}

func sortSummaries(items []Summary) {
	sort.Slice(items, func(i, j int) bool {
		if order := CompareCollectorNumbers(items[i].CollectorNumber, items[j].CollectorNumber); order != 0 {
			return order < 0
		}
		return items[i].PrintingUUID < items[j].PrintingUUID
	})
}

// CompareCollectorNumbers compares numeric and textual collector-number segments.
func CompareCollectorNumbers(a, b string) int {
	for len(a) > 0 && len(b) > 0 {
		if a[0] >= '0' && a[0] <= '9' && b[0] >= '0' && b[0] <= '9' {
			i, j := 0, 0
			for i < len(a) && a[i] >= '0' && a[i] <= '9' {
				i++
			}
			for j < len(b) && b[j] >= '0' && b[j] <= '9' {
				j++
			}
			x, y := strings.TrimLeft(a[:i], "0"), strings.TrimLeft(b[:j], "0")
			if len(x) < len(y) {
				return -1
			}
			if len(x) > len(y) {
				return 1
			}
			if order := strings.Compare(x, y); order != 0 {
				return order
			}
			a, b = a[i:], b[j:]
			continue
		}
		if a[0] != b[0] {
			return strings.Compare(a[:1], b[:1])
		}
		a, b = a[1:], b[1:]
	}
	return strings.Compare(a, b)
}
