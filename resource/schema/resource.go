// Package schema has the resources' schema: a resource's content, which is
// kept once, by its hash, and what it is, such as its name and type, which a
// session can have, or which is on its own
package schema

import (
	"encoding/hex"
	"net/url"
	"strconv"
	"strings"
	"time"
	"uuid"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	tui "github.com/mutablelogic/go-server/pkg/tui"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// ResourceMeta is what a resource is: its name, such as a file's, its
// description, its type, and the session it's in, or none, for a resource
// on its own
type ResourceMeta struct {
	Name        string     `json:"name,omitempty" help:"Name of the resource, such as its file's name" example:"report.pdf" optional:""`
	Description string     `json:"description,omitempty" help:"Description of the resource" example:"The quarterly report" optional:""`
	Type        string     `json:"type,omitempty" help:"MIME type of the resource, which is detected from its content, and its name, when it's omitted" example:"application/pdf" optional:""`
	Session     *uuid.UUID `json:"session,omitempty" help:"Session the resource is in, which it's deleted with, or none, for a resource on its own, which is deleted once it hasn't been used for a while" optional:""`
}

// Resource is a resource: its content's hash, which, with its session,
// identifies it, the size of its content, when it was created, and last
// used, and what it is. Resources with the same content, in different
// sessions, have the one content.
type Resource struct {
	Hash    string    `json:"hash" readonly:"" help:"SHA-256 of the resource's content, in hexadecimal, which, with its session, identifies it" example:"9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"`
	Size    int64     `json:"size" readonly:"" help:"Size of the resource's content, in bytes" example:"1024"`
	Created time.Time `json:"created,omitzero" readonly:"" help:"When the resource was created"`
	Used    time.Time `json:"used,omitzero" readonly:"" help:"When the resource was last created, or its content read"`
	ResourceMeta
}

// ResourceKey identifies a resource: its content's hash, and its session,
// or none, for a resource on its own
type ResourceKey struct {
	Hash    string
	Session *uuid.UUID
}

// ResourceRequest is the session of a resource, which is a query parameter
// of its requests, or none, for a resource on its own
type ResourceRequest struct {
	Session string `json:"session,omitempty" format:"uuid" help:"Session the resource is in, or none, for a resource on its own" optional:""`
}

// ResourceContentRequest is the resource whose content is read, and whether
// it's downloaded, as an attachment, which the browser saves, rather than
// shown in it
type ResourceContentRequest struct {
	ResourceRequest
	Download bool `json:"download,omitempty" help:"Whether the content is downloaded, as an attachment, which the browser saves, rather than shown in it" optional:""`
}

// ResourceCreateRequest is what a resource which is created is, as the
// query parameters of its upload, or its form's values
type ResourceCreateRequest struct {
	Session     string `json:"session,omitempty" format:"uuid" help:"Session the resource is in, which it's deleted with, or none, for a resource on its own" optional:""`
	Name        string `json:"name,omitempty" help:"Name of the resource, which is its file's name, when it's omitted" example:"report.pdf" optional:""`
	Description string `json:"description,omitempty" help:"Description of the resource" optional:""`
	Type        string `json:"type,omitempty" help:"MIME type of the resource, which is its file's, or detected from its content, and its name, when it's omitted" example:"application/pdf" optional:""`
}

// ResourceListRequest is a request to list the resources of a session, or
// those on their own, from the offset, and at most the limit of them
type ResourceListRequest struct {
	Session string `json:"session,omitempty" format:"uuid" help:"Returns only the resources in this session, or every resource, in a session, or on its own, when it's omitted" optional:""`
	Offset  uint   `json:"offset,omitempty" help:"Resources to skip" example:"0" optional:""`
	Limit   *uint  `json:"limit,omitempty" help:"Most resources to return" example:"10" optional:""`
}

// ResourceList is the resources which are listed, the most recently used
// first, and how many there are
type ResourceList struct {
	ResourceListRequest
	Count uint       `json:"count" help:"How many resources there are"`
	Body  []Resource `json:"body,omitempty" help:"The resources"`
}

// ResourceEvent is the data of an event of a resource, which is created,
// or deleted: the session it's in, or none
type ResourceEvent struct {
	Session string `json:"session,omitempty"`
}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// Scheme is the scheme of a resource's URI, which is resource://,
	// followed by its content's hash
	Scheme = "resource"

	// Name is the schema of the resources' events
	Name = "resource"

	// ResourceTable is the table of the events of the resources, whose rows
	// are their hashes
	ResourceTable = "resource"

	// DefaultRetention is how long a resource on its own is kept, after it
	// was last used, before it's deleted
	DefaultRetention = 24 * time.Hour

	// CleanInterval is how often the resources which aren't needed, and the
	// content no resource refers to, are deleted
	CleanInterval = time.Hour

	// DefaultMaxSize is the largest content of a resource, in bytes
	DefaultMaxSize = 256 << 20

	// DefaultListLimit is the most resources which are listed, unless the
	// request says otherwise, and MaxListLimit the most which can be
	DefaultListLimit = 100
	MaxListLimit     = 1000
)

///////////////////////////////////////////////////////////////////////////////
// STRINGIFY

func (r Resource) String() string {
	return types.Stringify(r)
}

func (r ResourceList) String() string {
	return types.Stringify(r)
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Key returns the resource's key: its hash, and its session
func (r Resource) Key() ResourceKey {
	return ResourceKey{Hash: r.Hash, Session: r.Session}
}

// URI returns the resource's URI, which is resource://, followed by its
// content's hash
func (r Resource) URI() string {
	return URI(r.Hash)
}

// String returns the key, as its hash, and its session, when it has one
func (k ResourceKey) String() string {
	if k.Session == nil {
		return k.Hash
	}
	return k.Hash + " in session " + k.Session.String()
}

// Key returns the key of the resource with the hash, in the request's
// session, or on its own. Returns ErrBadParameter when the hash isn't a
// SHA-256 in hexadecimal, or the session isn't a UUID.
func (r ResourceRequest) Key(hash string) (ResourceKey, error) {
	hash, err := ParseHash(hash)
	if err != nil {
		return ResourceKey{}, err
	}
	session, err := ParseSession(r.Session)
	if err != nil {
		return ResourceKey{}, err
	}
	return ResourceKey{Hash: hash, Session: session}, nil
}

// Meta returns what the resource which is created is. Returns
// ErrBadParameter when the session isn't a UUID.
func (r ResourceCreateRequest) Meta() (ResourceMeta, error) {
	session, err := ParseSession(r.Session)
	if err != nil {
		return ResourceMeta{}, err
	}
	return ResourceMeta{Name: strings.TrimSpace(r.Name), Description: strings.TrimSpace(r.Description), Type: strings.TrimSpace(r.Type), Session: session}, nil
}

// Query returns the request as the parameters of a URL's query
func (r ResourceRequest) Query() url.Values {
	values := url.Values{}
	if r.Session != "" {
		values.Set("session", r.Session)
	}
	return values
}

// Query returns the request as the parameters of a URL's query
func (r ResourceCreateRequest) Query() url.Values {
	values := url.Values{}
	for key, value := range map[string]string{"session": r.Session, "name": r.Name, "description": r.Description, "type": r.Type} {
		if value != "" {
			values.Set(key, value)
		}
	}
	return values
}

// Query returns the request as the parameters of a URL's query
func (r ResourceListRequest) Query() url.Values {
	values := url.Values{}
	if r.Session != "" {
		values.Set("session", r.Session)
	}
	if r.Offset > 0 {
		values.Set("offset", strconv.FormatUint(uint64(r.Offset), 10))
	}
	if r.Limit != nil {
		values.Set("limit", strconv.FormatUint(uint64(*r.Limit), 10))
	}
	return values
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC FUNCTIONS

// URI returns the URI of the resource with the hash, which is
// resource:///<hash>, whose hash is its path, as it has no host
func URI(hash string) string {
	return Scheme + ":///" + hash
}

// ParseURI returns the hash of the resource with the URI, which is
// resource:///<hash>, or resource://<hash>, as it was before, whose hash was
// its host. Returns ErrBadParameter when it isn't.
func ParseURI(uri string) (string, error) {
	hash, ok := strings.CutPrefix(strings.TrimSpace(uri), Scheme+"://")
	if !ok {
		return "", llm.ErrBadParameter.Withf("%q isn't a resource's URI, which is %s:///<hash>", uri, Scheme)
	}
	return ParseHash(strings.TrimPrefix(hash, "/"))
}

// ParseHash returns the hash, in lower case, or ErrBadParameter when it
// isn't a SHA-256 in hexadecimal
func ParseHash(hash string) (string, error) {
	hash = strings.ToLower(strings.TrimSpace(hash))
	if len(hash) != 64 {
		return "", llm.ErrBadParameter.Withf("invalid resource hash %q", hash)
	} else if _, err := hex.DecodeString(hash); err != nil {
		return "", llm.ErrBadParameter.Withf("invalid resource hash %q", hash)
	}
	return hash, nil
}

// ParseSession returns the session, or nil, when it's empty, or
// ErrBadParameter when it isn't a UUID
func ParseSession(session string) (*uuid.UUID, error) {
	if session = strings.TrimSpace(session); session == "" {
		return nil, nil
	}
	id, err := uuid.Parse(session)
	if err != nil || id == uuid.Nil() {
		return nil, llm.ErrBadParameter.Withf("invalid session %q", session)
	}
	return &id, nil
}

///////////////////////////////////////////////////////////////////////////////
// TABLE

// Header returns the columns of a resource, as it's listed
func (Resource) Header() []string {
	return []string{"Hash", "Name", "Type", "Size", "Session", "Used"}
}

func (Resource) Width(int) int {
	return 0
}

// Cell returns the resource's hash, shortened, its name, type, size,
// session, and when it was last used
func (r Resource) Cell(col int) tui.CellText {
	switch col {
	case 0:
		return r.Hash[:min(12, len(r.Hash))]
	case 1:
		return r.Name
	case 2:
		return r.Type
	case 3:
		return strconv.FormatInt(r.Size, 10)
	case 4:
		if r.Session != nil {
			return r.Session.String()
		}
		return ""
	case 5:
		if !r.Used.IsZero() {
			return r.Used.Local().Format(time.DateTime)
		}
		return ""
	default:
		return ""
	}
}
