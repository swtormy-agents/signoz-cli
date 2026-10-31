package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/url"
	"strings"
	"text/tabwriter"

	"github.com/itchyny/gojq"
)

func urlValues(m map[string]string) url.Values {
	if len(m) == 0 {
		return nil
	}
	q := make(url.Values, len(m))
	for k, v := range m {
		if v != "" {
			q.Set(k, v)
		}
	}
	return q
}

func (a *app) emit(raw json.RawMessage) error {
	if a.jq == "" {
		return a.writeOut(raw)
	}
	query, err := gojq.Parse(a.jq)
	if err != nil {
		return usageErr("--jq: %v", err)
	}
	code, err := gojq.Compile(query)
	if err != nil {
		return usageErr("--jq: %v", err)
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var input any
	if err := dec.Decode(&input); err != nil {
		return runtimeErr(err)
	}
	iter := code.Run(normalize(input))
	for {
		v, ok := iter.Next()
		if !ok {
			return nil
		}
		if e, isErr := v.(error); isErr {
			return runtimeErr(fmt.Errorf("--jq: %v", e))
		}
		if s, isStr := v.(string); isStr {
			fmt.Fprintln(a.out, s)
			continue
		}
		b, err := marshal(v)
		if err != nil {
			return runtimeErr(err)
		}
		if err := a.writeOut(b); err != nil {
			return err
		}
	}
}

func marshal(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(buf.Bytes(), []byte("\n")), nil
}

func (a *app) emitValue(v any) error {
	b, err := marshal(v)
	if err != nil {
		return runtimeErr(err)
	}
	return a.emit(b)
}

func (a *app) writeOut(raw []byte) error {
	if a.output == "table" {
		return a.writeTable(raw)
	}
	if a.output != "json" {
		return usageErr("--output must be json or table")
	}
	var buf bytes.Buffer
	var err error
	if a.pretty {
		err = json.Indent(&buf, raw, "", "  ")
	} else {
		err = json.Compact(&buf, raw)
	}
	if err != nil {
		return runtimeErr(err)
	}
	buf.WriteByte('\n')
	_, err = io.Copy(a.out, &buf)
	return err
}

func (a *app) writeTable(raw []byte) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return runtimeErr(err)
	}
	v = normalize(v)
	rows := tableRows(v)
	if len(rows) == 0 {
		b, _ := json.MarshalIndent(v, "", "  ")
		fmt.Fprintln(a.out, string(b))
		return nil
	}
	cols := tableColumns(rows)
	w := tabwriter.NewWriter(a.out, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, strings.Join(cols, "\t"))
	for _, row := range rows {
		cells := make([]string, len(cols))
		for i, c := range cols {
			cells[i] = fmt.Sprint(row[c])
		}
		fmt.Fprintln(w, strings.Join(cells, "\t"))
	}
	return w.Flush()
}

func tableRows(v any) []map[string]any {
	switch t := v.(type) {
	case []any:
		out := make([]map[string]any, 0, len(t))
		for _, item := range t {
			if m, ok := item.(map[string]any); ok {
				out = append(out, m)
			}
		}
		return out
	case map[string]any:
		if data, ok := t["data"].(map[string]any); ok {
			if rules, ok := data["rules"].([]any); ok {
				return mapsFromSlice(rules)
			}
		}
		if data, ok := t["data"].([]any); ok {
			return mapsFromSlice(data)
		}
	}
	return nil
}

func mapsFromSlice(items []any) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if m, ok := item.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

func tableColumns(rows []map[string]any) []string {
	priority := []string{"id", "name", "serviceName", "state", "severity", "status"}
	seen := map[string]bool{}
	var cols []string
	for _, p := range priority {
		for _, row := range rows {
			if _, ok := row[p]; ok && !seen[p] {
				cols = append(cols, p)
				seen[p] = true
				break
			}
		}
	}
	for _, row := range rows {
		for k := range row {
			if !seen[k] && len(cols) < 8 {
				cols = append(cols, k)
				seen[k] = true
			}
		}
	}
	return cols
}

func normalize(v any) any {
	switch t := v.(type) {
	case json.Number:
		if i, err := t.Int64(); err == nil && i >= math.MinInt && i <= math.MaxInt {
			return int(i)
		}
		f, _ := t.Float64()
		return f
	case []any:
		for i := range t {
			t[i] = normalize(t[i])
		}
	case map[string]any:
		for k := range t {
			t[k] = normalize(t[k])
		}
	}
	return v
}
