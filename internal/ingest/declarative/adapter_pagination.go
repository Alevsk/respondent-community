package declarative

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Alevsk/respondent/internal/logging"
)

// fetchPaginated performs a multi-page fetch loop according to the pagination spec.
// It supports page_number, offset, and cursor pagination types.
func (a *DeclarativeAdapter) fetchPaginated(ctx context.Context, cs *CompiledSource) error {
	def := cs.Definition()
	pagination := def.Transport.Pagination
	fetchStart := time.Now()

	method := def.Transport.Method

	var allRecords []map[string]interface{}
	var cursor string // used only for cursor-type pagination

	for page := 0; page < pagination.MaxPages; page++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		pageURL := a.buildPaginatedURL(def.Transport.URL, pagination, page, cursor)

		a.logger.Debug("fetching page",
			logging.String("source_name", a.name),
			logging.Int("page", page+1),
			logging.String("url", pageURL),
		)

		body, statusCode, err := a.fetchWithRetry(ctx, cs, method, pageURL)
		if err != nil {
			a.logger.Warn("paginated fetch failed, stopping pagination",
				logging.String("source_name", a.name),
				logging.Int("page", page+1),
				logging.Err("error", err),
			)
			break
		}

		a.logger.Debug("page fetch completed",
			logging.String("source_name", a.name),
			logging.Int("page", page+1),
			logging.Int("http_status", statusCode),
		)

		// For cursor pagination, extract the next cursor from the raw response
		// before parsing records (which may drill into a sub-path).
		if pagination.Type == "cursor" && pagination.CursorPath != "" {
			cursor = extractCursorFromJSON(body, pagination.CursorPath)
		}

		records, err := a.parseBody(def, body)
		if err != nil {
			a.logger.Warn("page parse failed, stopping pagination",
				logging.String("source_name", a.name),
				logging.Int("page", page+1),
				logging.Err("error", err),
			)
			break
		}

		a.logger.Debug("parsed page records",
			logging.String("source_name", a.name),
			logging.Int("page", page+1),
			logging.Int("records", len(records)),
		)

		// Evaluate stop_when CEL expression if configured.
		if cs.StopWhen() != nil {
			// Convert records to []interface{} for CEL list evaluation.
			recordsList := make([]interface{}, len(records))
			for i, r := range records {
				recordsList[i] = r
			}
			activation := map[string]interface{}{"records": recordsList}
			shouldStop, evalErr := evalBool(cs.StopWhen(), activation)
			if evalErr != nil {
				a.logger.Warn("stop_when eval error, stopping pagination",
					logging.String("source_name", a.name),
					logging.Err("error", evalErr),
				)
				break
			}
			if shouldStop {
				a.logger.Debug("stop_when triggered, ending pagination",
					logging.String("source_name", a.name),
					logging.Int("page", page+1),
				)
				break
			}
		}

		allRecords = append(allRecords, records...)

		// For cursor pagination, stop if no next cursor was found.
		if pagination.Type == "cursor" && cursor == "" && page > 0 {
			a.logger.Debug("no next cursor found, ending pagination",
				logging.String("source_name", a.name),
				logging.Int("page", page+1),
			)
			break
		}
	}

	fetchDuration := time.Since(fetchStart)
	a.logger.Debug("paginated fetch completed",
		logging.String("source_name", a.name),
		logging.Any("fetch_duration_ms", fetchDuration.Milliseconds()),
		logging.Int("total_records", len(allRecords)),
	)

	// Process all accumulated records through CEL filter + mapping
	entities, observations := a.processRecords(cs, allRecords)

	if len(entities) == 0 && len(allRecords) > 0 {
		a.logger.Warn("empty batch: all records failed or were filtered",
			logging.String("source_name", a.name),
			logging.Int("records_received", len(allRecords)),
		)
	}

	a.SetEntities(entities, observations)

	a.logger.Info("processed paginated declarative source",
		logging.String("source_name", a.name),
		logging.Int("records_received", len(allRecords)),
		logging.Int("entities_produced", len(entities)),
	)

	return nil
}

// buildPaginatedURL constructs the URL for a specific page based on the pagination type.
func (a *DeclarativeAdapter) buildPaginatedURL(baseURL string, pagination *PaginationSpec, page int, cursor string) string {
	// Parse existing URL to preserve any existing query parameters.
	sep := "?"
	if strings.Contains(baseURL, "?") {
		sep = "&"
	}

	switch pagination.Type {
	case "page_number":
		// Pages are 1-indexed.
		pageNum := page + 1
		result := fmt.Sprintf("%s%s%s=%d", baseURL, sep, pagination.PageParam, pageNum)
		if pagination.SizeParam != "" {
			result = fmt.Sprintf("%s&%s=%d", result, pagination.SizeParam, pagination.Size)
		}
		return result

	case "offset":
		offset := page * pagination.Size
		result := fmt.Sprintf("%s%s%s=%d", baseURL, sep, pagination.PageParam, offset)
		if pagination.SizeParam != "" {
			result = fmt.Sprintf("%s&%s=%d", result, pagination.SizeParam, pagination.Size)
		}
		return result

	case "cursor":
		if page == 0 || cursor == "" {
			// First page: no cursor param, just size.
			if pagination.SizeParam != "" {
				return fmt.Sprintf("%s%s%s=%d", baseURL, sep, pagination.SizeParam, pagination.Size)
			}
			return baseURL
		}
		result := fmt.Sprintf("%s%s%s=%s", baseURL, sep, pagination.PageParam, cursor)
		if pagination.SizeParam != "" {
			result = fmt.Sprintf("%s&%s=%d", result, pagination.SizeParam, pagination.Size)
		}
		return result

	default:
		return baseURL
	}
}

// extractCursorFromJSON extracts a cursor value from a JSON response body using a dot-separated path.
// Returns empty string if the path is not found or the value is not a string.
func extractCursorFromJSON(body []byte, cursorPath string) string {
	var raw interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return ""
	}

	m, ok := raw.(map[string]interface{})
	if !ok {
		return ""
	}

	segments := strings.Split(cursorPath, ".")
	current := interface{}(m)

	for _, segment := range segments {
		obj, ok := current.(map[string]interface{})
		if !ok {
			return ""
		}
		val, exists := obj[segment]
		if !exists {
			return ""
		}
		current = val
	}

	switch v := current.(type) {
	case string:
		return v
	case float64:
		return formatFloat(v)
	case int64:
		return fmt.Sprintf("%d", v)
	default:
		return fmt.Sprintf("%v", v)
	}
}
