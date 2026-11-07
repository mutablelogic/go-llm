package schema

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strings"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	types "github.com/mutablelogic/go-server/pkg/types"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// ResourceMeta holds the fields of a resource which can be set: its type and
// its data, a URL, or both
type ResourceMeta struct {
	ContentType string   `json:"type" help:"Attachment MIME type, for example image/png or application/pdf" example:"image/png"`
	Data        []byte   `json:"data,omitempty" help:"Inline attachment payload encoded as a byte string" example:"iVBORw0KGgo="`
	URL         *url.URL `json:"url,omitempty" help:"Attachment URL reference, for example https, gs, or file" example:"https://example.com/image.png"`
}

// Resource represents binary or URI-referenced media (images, documents,
// etc.) in a message
type Resource struct {
	ResourceMeta
}

var _ llm.Resource = (*Resource)(nil)

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// URI returns the URL of the resource, or an empty string when it's inline
// data only
func (r Resource) URI() string {
	if r.URL != nil {
		return r.URL.String()
	}
	return ""
}

// Name returns the last element of the path of the resource's URL, or an
// empty string when it's inline data only
func (r Resource) Name() string {
	if r.URL != nil && r.URL.Path != "" {
		return path.Base(r.URL.Path)
	}
	return ""
}

// Description returns an empty string, as resources in a message don't have
// a description
func (r Resource) Description() string {
	return ""
}

// Type returns the MIME type of the resource
func (r Resource) Type() string {
	return r.ContentType
}

// Read returns a function which reads the inline data of the resource.
// Resources referred to by a URL can't be read yet, as reading files or
// fetching URLs on the server would give clients access to its files and
// network.
func (r Resource) Read(ctx context.Context) (llm.ReadFunc, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if r.URL != nil && len(r.Data) == 0 {
		return nil, llm.ErrNotImplemented.Withf("reading resource url %q", r.URL.Redacted())
	}
	return llm.ReadBytes(r.Data), nil
}

// MarshalJSON returns the resource as JSON, whose URL is a string, such as
// resource:///<hash>, rather than its parts
func (r ResourceMeta) MarshalJSON() ([]byte, error) {
	type meta ResourceMeta
	var u string
	if r.URL != nil {
		u = r.URL.String()
	}
	return json.Marshal(struct {
		meta
		URL string `json:"url,omitempty"`
	}{meta: meta(r), URL: u})
}

// UnmarshalJSON reads the resource from JSON, whose URL is a string, or, as
// it was once written, its parts. Returns an error when the URL isn't one.
func (r *ResourceMeta) UnmarshalJSON(data []byte) error {
	type meta ResourceMeta
	var value struct {
		meta
		URL json.RawMessage `json:"url,omitempty"`
	}
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	*r = ResourceMeta(value.meta)
	r.URL = nil
	switch raw := bytes.TrimSpace(value.URL); {
	case len(raw) == 0 || string(raw) == "null":
	case raw[0] == '"':
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return err
		} else if s = strings.TrimSpace(s); s != "" {
			u, err := url.Parse(s)
			if err != nil {
				return llm.ErrBadParameter.Withf("invalid attachment url %q", s)
			}
			r.URL = u
		}
	default:
		var u url.URL
		if err := json.Unmarshal(raw, &u); err != nil {
			return err
		}
		r.URL = &u
	}
	return nil
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC FUNCTIONS

// ResourceType returns the MIME type of a resource's data, detected from the
// data, such as image/png or audio/wave, or from the extension of its name,
// when the data isn't recognised, or is only text, or is an Ogg container,
// which the extension is more specific about, such as text/markdown or
// audio/ogg. Ogg data without an extension is audio/ogg, as Ogg is almost
// always audio, and MP4 data whose brand is M4A or M4B, or which is named
// .m4a or .m4b, is audio/mp4. Without data, it's the type of the extension. Returns
// application/octet-stream when neither is recognised, and an empty string
// when there's no data, and the extension isn't known.
func ResourceType(data []byte, name string) string {
	byName := ""
	if ext := path.Ext(name); ext != "" {
		byName = mime.TypeByExtension(strings.ToLower(ext))
	}
	if len(data) == 0 {
		return byName
	}
	if isAudioMP4(data) {
		return "audio/mp4"
	}
	detected := http.DetectContentType(data)
	mediaType, _, err := mime.ParseMediaType(detected)
	if err != nil {
		return detected
	}
	switch mediaType {
	case types.ContentTypeBinary, "text/plain":
		if byName != "" {
			return byName
		}
	case "application/ogg":
		if strings.HasPrefix(byName, "audio/") || strings.HasPrefix(byName, "video/") {
			return byName
		}
		return "audio/ogg"
	case "video/mp4":
		// An MP4 container named as audio is audio, as an M4A file can have
		// the brands of video
		if ext := strings.ToLower(path.Ext(name)); ext == ".m4a" || ext == ".m4b" {
			return "audio/mp4"
		}
	}
	return detected
}

// isAudioMP4 returns true when the data is an MP4 container, whose ftyp box
// has the brand of audio, M4A or M4B, such as from iTunes or a voice
// recorder, as its major brand, or one of its compatible brands, which
// http.DetectContentType doesn't recognize, unless it also has a brand of
// video
func isAudioMP4(data []byte) bool {
	if len(data) < 16 || string(data[4:8]) != "ftyp" {
		return false
	}
	size := int(binary.BigEndian.Uint32(data[:4]))
	if size < 16 || size%4 != 0 || size > len(data) {
		return false
	}
	for i := 8; i < size; i += 4 {
		if i == 12 {
			// The minor version, rather than a brand
			continue
		}
		if brand := string(data[i : i+4]); brand == "M4A " || brand == "M4B " {
			return true
		}
	}
	return false
}

// UnknownType returns true when the MIME type is empty, or is
// application/octet-stream, in any case, and with any parameters, so it's
// detected from the data
func UnknownType(contentType string) bool {
	contentType = strings.TrimSpace(contentType)
	if contentType == "" {
		return true
	}
	mediaType, _, err := mime.ParseMediaType(contentType)
	return err == nil && mediaType == types.ContentTypeBinary
}
