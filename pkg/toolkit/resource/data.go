package resource

import (
	"bytes"
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
	"golang.org/x/text/encoding/htmlindex"
	"golang.org/x/text/transform"
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
// is resolved by content sniffing first; if the sniffed type is
// application/octet-stream and name has an extension, the extension is used as
// a fallback. If name contains path separators or an extension, the bare base
// name (without extension) is used as the resource identifier.
func Data(name string, data []byte) (llm.Resource, error) {
	// Derive identifier from filename: strip directory and extension
	id := strings.TrimSuffix(filepath.Base(name), filepath.Ext(name))
	if !types.IsIdentifier(id) {
		return nil, llm.ErrBadParameter.Withf("name: must be a non-empty identifier, got %q", id)
	}

	// Resolve MIME type: content sniffing wins, extension as fallback
	mimetype := http.DetectContentType(data)
	if mimetype == types.ContentTypeBinary && strings.Contains(name, ".") {
		mimetype = mime.TypeByExtension(filepath.Ext(name))
	}

	// If no MIME type could be resolved, default to application/octet-stream
	if mimetype == "" {
		mimetype = types.ContentTypeBinary
	}

	// For text/* types, transcode to UTF-8 and return a textResource.
	mediaType, params, _ := mime.ParseMediaType(mimetype)
	if strings.HasPrefix(mediaType, "text/") {
		content := dataToUTF8(data, params["charset"])
		return &textResource{name: id, content: content}, nil
	}

	return &dataResource{name: id, mimetype: mimetype, content: data}, nil
}

// dataToUTF8 converts data from the named charset to a UTF-8 string.
// On error or empty/utf-8/us-ascii charset the raw bytes are used as-is.
func dataToUTF8(data []byte, charsetName string) string {
	lower := strings.ToLower(charsetName)
	if lower == "" || lower == "utf-8" || lower == "us-ascii" {
		return string(data)
	}
	enc, err := htmlindex.Get(charsetName)
	if err != nil {
		return string(data)
	}
	reader := transform.NewReader(bytes.NewReader(data), enc.NewDecoder())
	decoded, err := io.ReadAll(reader)
	if err != nil {
		return string(data)
	}
	return string(decoded)
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
		Blob []byte `json:"blob,omitempty"`
	}{d.URI(), d.name, d.Type(), d.content})
}

func (d *dataResource) String() string { return types.Stringify(d) }
