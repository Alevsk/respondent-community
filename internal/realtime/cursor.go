package realtime

import (
	"encoding/base64"
	"encoding/json"
)

// paginationCursor is the cursor token for paginated snapshots.
// Base64-encoded JSON, opaque to the frontend.
type paginationCursor struct {
	Offset int `json:"offset"`
}

func encodeCursor(offset int) string {
	data, _ := json.Marshal(paginationCursor{Offset: offset})
	return base64.StdEncoding.EncodeToString(data)
}

func decodeCursor(cursor string) (paginationCursor, error) {
	data, err := base64.StdEncoding.DecodeString(cursor)
	if err != nil {
		return paginationCursor{}, err
	}
	var pc paginationCursor
	if err := json.Unmarshal(data, &pc); err != nil {
		return paginationCursor{}, err
	}
	return pc, nil
}
