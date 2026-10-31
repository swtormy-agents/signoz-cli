package signoz

import "time"

// BuilderRawQuery builds a POST /api/v5/query_range body for raw log or trace rows.
func BuilderRawQuery(signal, filter string, start, end time.Time, limit, offset int) map[string]any {
	if limit <= 0 {
		limit = 50
	}
	spec := map[string]any{
		"name":   "A",
		"signal": signal,
		"limit":  limit,
		"offset": offset,
		"order": []any{
			map[string]any{
				"key":       map[string]any{"fieldContext": rowFieldContext(signal), "name": "timestamp"},
				"direction": "desc",
			},
		},
	}
	if filter != "" {
		spec["filter"] = map[string]any{"expression": filter}
	}
	if signal == "logs" {
		spec["selectFields"] = []any{
			map[string]any{"fieldContext": "log", "name": "body"},
			map[string]any{"fieldContext": "resource", "name": "service.name"},
			map[string]any{"fieldContext": "log", "name": "severity_text"},
		}
	} else if signal == "traces" {
		spec["selectFields"] = []any{
			map[string]any{"fieldContext": "span", "name": "name"},
			map[string]any{"fieldContext": "resource", "name": "service.name"},
			map[string]any{"fieldContext": "span", "name": "duration_nano"},
		}
	}
	return map[string]any{
		"schemaVersion": "v1",
		"start":         start.UnixMilli(),
		"end":           end.UnixMilli(),
		"requestType":   "raw",
		"compositeQuery": map[string]any{
			"queries": []any{
				map[string]any{
					"type": "builder_query",
					"spec": spec,
				},
			},
		},
	}
}

func rowFieldContext(signal string) string {
	if signal == "logs" {
		return "log"
	}
	return "span"
}
