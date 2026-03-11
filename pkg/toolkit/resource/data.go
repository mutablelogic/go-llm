package resource

import (
	"context"
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// dataResource is an in-memory raw-bytes resource with an auto-detected MIME type.
type dataResource struct {
	name     string
	mimetype string
	content  []byte
}

var _ llm.Resource = (*dataResource)(nil)

///////////////////////////////////////////////////////////////////////////////
// CONSTRUCTOR

// Data returns a new in-memory resource for the given raw bytes. The MIME type
// is resolved by first checking the file extension of name, then falling back
// to content sniffing. If name contains path separators or an extension, the
// bare base name (without extension) is used as the resource identifier.
func Data(name string, data []byte) (llm.Resource, error) {
	// Derive identifier from filename: strip directory and extension
	id := strings.TrimSuffix(filepath.Base(name), filepath.Ext(name))
	if !types.IsIdentifier(id) {
		return nil, llm.ErrBadParameter.Withf("name: must be a non-empty identifier, got %q", id)
	}

	// Resolve MIME type: extension wins, content sniffing as fallback
	mimetype := mime.TypeByExtension(filepath.Ext(name))
	if mimetype == "" {
		mimetype = http.DetectContentType(data)
	}

	return &dataResource{name: id, mimetype: mimetype, content: data}, nil
}

// Read reads all bytes from r and returns a resource. If r is an *os.File its
// path is used to derive the name and MIME type, exactly as Data() does.
// For other readers r must be an io.Reader with discoverable content only;
// the name defaults to "data" and the MIME type is content-sniffed.
func Read(r io.Reader) (llm.Resource, error) {
	name := "data"
	if f, ok := r.(*os.File); ok {
		name = f.Name()
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return Data(name, data)
}

///////////////////////////////////////////////////////////////////////////////

func (d *dataResource) URI() string         { return "data:" + d.name }
func (d *dataResource) Name() string        { return d.name }
func (d *dataResource) Description() string { return "" }
func (d *dataResource) Type() string        { return d.mimetype }

func (d *dataResource) Read(_ context.Context) ([]byte, error) {
	return d.content, nil
}

func (d *dataResource) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		URI  string `json:"uri"`
		Name string `json:"name"`
		Type string `json:"type"`
		Data []byte `json:"data"`
	}{d.URI(), d.name, d.Type(), d.content})
}
