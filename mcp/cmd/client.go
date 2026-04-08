package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	// Packages
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	llm "github.com/mutablelogic/go-llm"
	mcpclient "github.com/mutablelogic/go-llm/mcp/client"
	tui "github.com/mutablelogic/go-llm/pkg/tui"
	server "github.com/mutablelogic/go-server"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type URLFlag struct {
	URL string `name:"url" help:"MCP server endpoint URL" required:""`
}

type tableRow struct {
	headers []string
	widths  []int
	cells   []string
}

var preferredColumns = []string{"entity_id", "name", "state", "class", "unit", "id", "title", "type", "description", "uri"}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC FUNCTIONS

func WithClient(ctx server.Cmd, url string, fn func(context.Context, *mcpclient.Client) error) error {
	_, clientOpts, err := ctx.ClientEndpoint()
	if err != nil {
		return err
	}

	client, err := mcpclient.New(url, ctx.Name(), ctx.Version(), mcpclient.WithClientOpt(clientOpts...))
	if err != nil {
		return err
	}

	parent, cancel := context.WithCancel(ctx.Context())
	defer cancel()

	if err := client.Connect(parent); err != nil {
		return err
	}
	defer client.Close() //nolint:errcheck

	return fn(parent, client)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

func stdinHasData(stdin *os.File) bool {
	if stdin == nil {
		return false
	}
	info, err := stdin.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice == 0
}

func (r tableRow) Header() []string { return r.headers }

func (r tableRow) Cell(index int) string { return r.cells[index] }

func (r tableRow) Width(index int) int {
	if index >= 0 && index < len(r.widths) {
		return r.widths[index]
	}
	return 0
}

func writeTable(w io.Writer, headers []string, widths []int, rows [][]string, options ...tui.Opt) error {
	if len(rows) == 0 {
		_, err := fmt.Fprintln(w, "No results")
		return err
	}

	tableRows := make([]tableRow, 0, len(rows))
	for _, row := range rows {
		tableRows = append(tableRows, tableRow{headers: headers, widths: widths, cells: row})
	}

	if _, err := tui.TableFor[tableRow](options...).Write(w, tableRows...); err != nil {
		return err
	}
	_, err := fmt.Fprintf(w, "\n%d item(s)\n", len(rows))
	return err
}

func writeValue(w io.Writer, value any, options ...tui.Opt) error {
	switch v := value.(type) {
	case nil:
		return nil
	case llm.Resource:
		return writeResource(context.Background(), w, v, options...)
	case json.RawMessage:
		return writeJSON(w, v, options...)
	case []byte:
		_, err := w.Write(v)
		return err
	case string:
		_, err := fmt.Fprintln(w, v)
		return err
	default:
		if data, err := json.Marshal(v); err == nil {
			return writeJSON(w, data, options...)
		}
		_, err := fmt.Fprintln(w, v)
		return err
	}
}

func writeJSON(w io.Writer, data json.RawMessage, options ...tui.Opt) error {
	if !json.Valid(data) {
		_, err := w.Write(data)
		return err
	}
	if rendered, err := writeJSONObjectTable(w, data, options...); rendered || err != nil {
		return err
	}
	pretty, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		_, err = w.Write(data)
		return err
	}
	_, err = fmt.Fprintln(w, string(pretty))
	return err
}

func writeResource(ctx context.Context, w io.Writer, resource llm.Resource, options ...tui.Opt) error {
	if resource == nil {
		return nil
	}
	data, err := resource.Read(ctx)
	if err != nil {
		return err
	}

	switch {
	case resource.Type() == types.ContentTypeJSON:
		return writeJSON(w, data, options...)
	case strings.HasPrefix(resource.Type(), "text/"):
		if _, err := w.Write(data); err != nil {
			return err
		}
		if len(data) == 0 || data[len(data)-1] != '\n' {
			_, err = w.Write([]byte("\n"))
			return err
		}
		return nil
	default:
		_, err := w.Write(data)
		return err
	}
}

func writePromptResult(w io.Writer, result *sdkmcp.GetPromptResult) error {
	if result == nil {
		return nil
	}
	for i, message := range result.Messages {
		if i > 0 {
			if _, err := fmt.Fprintln(w); err != nil {
				return err
			}
		}
		switch content := message.Content.(type) {
		case *sdkmcp.TextContent:
			if _, err := fmt.Fprintln(w, content.Text); err != nil {
				return err
			}
		default:
			data, err := json.MarshalIndent(content, "", "  ")
			if err != nil {
				return err
			}
			if _, err := fmt.Fprintln(w, string(data)); err != nil {
				return err
			}
		}
	}
	return nil
}

func writeJSONObjectTable(w io.Writer, data json.RawMessage, options ...tui.Opt) (bool, error) {
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return false, nil
	}

	switch typed := value.(type) {
	case []any:
		headers, widths, rows, ok := tableFromJSONArray(typed)
		if !ok {
			return false, nil
		}
		return true, writeTable(w, headers, widths, rows, options...)
	case map[string]any:
		headers, widths, rows := tableFromJSONObject(typed)
		return true, writeTable(w, headers, widths, rows, options...)
	default:
		return false, nil
	}
}

func tableFromJSONArray(items []any) ([]string, []int, [][]string, bool) {
	if len(items) == 0 {
		return []string{"VALUE"}, []int{0}, nil, true
	}

	objects := make([]map[string]any, 0, len(items))
	for _, item := range items {
		object, ok := item.(map[string]any)
		if !ok {
			return nil, nil, nil, false
		}
		objects = append(objects, object)
	}

	headers := orderedKeys(objects)
	widths := make([]int, len(headers))
	rows := make([][]string, 0, len(objects))
	for _, object := range objects {
		row := make([]string, len(headers))
		for i, header := range headers {
			row[i] = formatCellValue(object[header])
			widths[i] = max(widths[i], suggestedWidth(header, row[i]))
		}
		rows = append(rows, row)
	}
	for i, header := range headers {
		widths[i] = max(widths[i], suggestedWidth(header, header))
	}
	return headers, widths, rows, true
}

func tableFromJSONObject(object map[string]any) ([]string, []int, [][]string) {
	keys := make([]string, 0, len(object))
	for key := range object {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	rows := make([][]string, 0, len(keys))
	for _, key := range keys {
		rows = append(rows, []string{key, formatCellValue(object[key])})
	}
	return []string{"KEY", "VALUE"}, []int{24, 0}, rows
}

func orderedKeys(objects []map[string]any) []string {
	seen := make(map[string]struct{})
	keys := make([]string, 0)
	for _, preferred := range preferredColumns {
		for _, object := range objects {
			if _, ok := object[preferred]; ok {
				if _, exists := seen[preferred]; !exists {
					seen[preferred] = struct{}{}
					keys = append(keys, preferred)
				}
				break
			}
		}
	}
	remaining := make([]string, 0)
	for _, object := range objects {
		for key := range object {
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			remaining = append(remaining, key)
		}
	}
	sort.Strings(remaining)
	return append(keys, remaining...)
}

func formatCellValue(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	case float64:
		return fmt.Sprintf("%v", typed)
	case bool:
		return fmt.Sprintf("%t", typed)
	default:
		data, err := json.Marshal(typed)
		if err != nil {
			return fmt.Sprint(typed)
		}
		return string(data)
	}
}

func suggestedWidth(header, value string) int {
	switch header {
	case "entity_id", "uri":
		return 32
	case "name", "title":
		return 24
	case "state", "class", "unit", "type", "id":
		return 16
	case "description", "value":
		return 0
	default:
		if len(value) <= 16 {
			return 16
		} else if len(value) <= 24 {
			return 24
		}
		return 32
	}
}
