package httpx

import (
	"net/http"

	"github.com/google/uuid"
)

const MaxBulkIDs = 500

// UUIDList checks a bulk request's IDs. It returns the IDs in their canonical form without duplicates,
// or writes a 400 response and returns false.
func UUIDList(w http.ResponseWriter, raw []string) ([]string, bool) {
	if len(raw) == 0 {
		Error(w, http.StatusBadRequest, "ids must be a non-empty list")
		return nil, false
	}
	if len(raw) > MaxBulkIDs {
		Error(w, http.StatusBadRequest, "at most 500 items can be changed at once")
		return nil, false
	}
	seen := make(map[string]bool, len(raw))
	ids := make([]string, 0, len(raw))
	for _, s := range raw {
		id, err := uuid.Parse(s)
		if err != nil {
			Error(w, http.StatusBadRequest, "ids must be item or folder IDs")
			return nil, false
		}
		if !seen[id.String()] {
			seen[id.String()] = true
			ids = append(ids, id.String())
		}
	}
	return ids, true
}
