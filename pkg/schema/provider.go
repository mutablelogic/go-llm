package schema

import (
	"encoding/json"
	"fmt"
	"net/url"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	// Packages
	pg "github.com/mutablelogic/go-pg"
	types "github.com/mutablelogic/go-server/pkg/types"
)

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

// Provider name constants
const (
	Gemini    = "gemini"
	Anthropic = "anthropic"
	Mistral   = "mistral"
	Eliza     = "eliza"
	Ollama    = "ollama"
)

var (
	allProviders = []string{Gemini, Anthropic, Mistral, Eliza, Ollama}
)

const (
	ProviderListMax uint64 = 100
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// ProviderMeta contains the user-writable fields for an existing provider row.
// The provider kind itself is immutable after creation and is stored on Provider.
type ProviderMeta struct {
	URL     *string         `json:"url,omitempty" name:"url" help:"Provider endpoint URL" optional:""`
	Enabled *bool           `json:"enabled,omitempty" name:"enabled" help:"Enable the provider" negatable:""`
	Meta    ProviderMetaMap `json:"meta,omitempty" name:"meta" help:"Provider metadata as a JSON object" optional:""`
}

type ProviderMetaMap map[string]any

// ProviderCredentials contains the secret material used to access a provider.
type ProviderCredentials struct {
	APIKey string `json:"api_key,omitempty" name:"api-key" help:"Provider API key" optional:"" env:"LLM_PROVIDER_API_KEY"`
}

// ProviderInsert contains the fields required to insert a new provider row.
type ProviderInsert struct {
	Name     string `json:"name" arg:"" name:"name" help:"Unique provider name"`
	Provider string `json:"provider" name:"provider" help:"Provider identifier"`
	ProviderMeta
	ProviderCredentials
}

// ProviderListRequest contains pagination for listing providers.
type ProviderListRequest struct {
	pg.OffsetLimit
	Provider string `json:"provider,omitempty" name:"provider" help:"Filter by provider identifier" optional:""`
	Enabled  *bool  `json:"enabled,omitempty" name:"enabled" help:"Filter by enabled state" negatable:""`
}

// ProviderList represents a paginated list of providers.
type ProviderList struct {
	ProviderListRequest
	Count uint64     `json:"count" readonly:""`
	Body  []Provider `json:"body,omitempty"`
}

// ProviderNameSelector selects a provider by name for get, update, and delete operations.
type ProviderNameSelector string

// Provider is the persisted representation of a provider row.
type Provider struct {
	Name       string     `json:"name" help:"Unique provider name"`
	Provider   string     `json:"provider" help:"Provider identifier"`
	CreatedAt  time.Time  `json:"created_at" help:"Creation timestamp"`
	ModifiedAt *time.Time `json:"modified_at,omitempty" help:"Last modification timestamp" optional:""`
	ProviderMeta
}

////////////////////////////////////////////////////////////////////////////////
// STRINGIFY

func (p ProviderMeta) String() string {
	return types.Stringify(p)
}

func (p ProviderCredentials) String() string {
	return types.Stringify(p)
}

func (p ProviderInsert) String() string {
	return types.Stringify(p)
}

func (p ProviderListRequest) String() string {
	return types.Stringify(p)
}

func (p ProviderList) String() string {
	return types.Stringify(p)
}

func (p Provider) String() string {
	return types.Stringify(p)
}

////////////////////////////////////////////////////////////////////////////////
// QUERY

func (req ProviderListRequest) Query() url.Values {
	values := url.Values{}
	if req.Offset > 0 {
		values.Set("offset", strconv.FormatUint(req.Offset, 10))
	}
	if req.Limit != nil {
		values.Set("limit", strconv.FormatUint(types.Value(req.Limit), 10))
	}
	if provider := strings.TrimSpace(req.Provider); provider != "" {
		values.Set("provider", provider)
	}
	if req.Enabled != nil {
		values.Set("enabled", strconv.FormatBool(types.Value(req.Enabled)))
	}
	return values
}

////////////////////////////////////////////////////////////////////////////////
// SELECTORS

func (p ProviderNameSelector) Select(bind *pg.Bind, op pg.Op) (string, error) {
	name := strings.TrimSpace(string(p))
	if !types.IsIdentifier(name) {
		return "", ErrBadParameter.Withf("provider name: must be a valid identifier, got %q", string(p))
	}
	bind.Set("name", name)

	switch op {
	case pg.Get:
		return bind.Query("provider.select"), nil
	case pg.Update:
		return bind.Query("provider.update"), nil
	case pg.Delete:
		return bind.Query("provider.delete"), nil
	default:
		return "", ErrInternalServerError.Withf("unsupported ProviderNameSelector operation %q", op)
	}
}

func (req ProviderListRequest) Select(bind *pg.Bind, op pg.Op) (string, error) {
	bind.Del("where")

	if provider := strings.TrimSpace(req.Provider); provider != "" {
		if !types.IsIdentifier(provider) {
			return "", ErrBadParameter.Withf("provider: must be a valid identifier, got %q", req.Provider)
		} else if !slices.Contains(allProviders, provider) {
			return "", ErrNotFound.Withf("provider: must be one of %q, got %q", allProviders, provider)
		} else {
			bind.Append("where", `provider.provider = `+bind.Set("provider", provider))
		}
	}
	if req.Enabled != nil {
		bind.Append("where", `provider.enabled = `+bind.Set("enabled", types.Value(req.Enabled)))
	}
	if where := bind.Join("where", " AND "); where == "" {
		bind.Set("where", "")
	} else {
		bind.Set("where", "WHERE "+where)
	}
	bind.Set("orderby", `ORDER BY provider."name" ASC`)
	req.OffsetLimit.Bind(bind, ProviderListMax)

	switch op {
	case pg.List:
		return bind.Query("provider.list"), nil
	default:
		return "", ErrNotImplemented.Withf("unsupported ProviderListRequest operation %q", op)
	}
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - READER

// Scan reads a provider row into the receiver.
// Expected column order: name, provider, url, enabled, created_at, modified_at, meta.
func (p *Provider) Scan(row pg.Row) error {
	var enabled bool
	var url string
	if err := row.Scan(
		&p.Name,
		&p.Provider,
		&url,
		&enabled,
		&p.CreatedAt,
		&p.ModifiedAt,
		&p.Meta,
	); err != nil {
		return err
	}
	p.Enabled = types.Ptr(enabled)
	if url = strings.TrimSpace(url); url != "" {
		p.URL = types.Ptr(url)
	} else {
		p.URL = nil
	}
	if p.Meta == nil {
		p.Meta = make(ProviderMetaMap)
	}
	return nil
}

func (list *ProviderList) Scan(row pg.Row) error {
	var provider Provider
	if err := provider.Scan(row); err != nil {
		return err
	}
	list.Body = append(list.Body, provider)
	return nil
}

func (list *ProviderList) ScanCount(row pg.Row) error {
	if err := row.Scan(&list.Count); err != nil {
		return err
	}
	return nil
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - WRITER

// Insert binds all required provider fields for an INSERT and returns the named query.
func (p ProviderInsert) Insert(bind *pg.Bind) (string, error) {
	if name := strings.TrimSpace(p.Name); !types.IsIdentifier(name) {
		return "", fmt.Errorf("provider name: must be a valid identifier, got %q", p.Name)
	} else {
		bind.Set("name", name)
	}

	if provider := strings.TrimSpace(p.Provider); !types.IsIdentifier(provider) {
		return "", fmt.Errorf("provider: must be a valid identifier, got %q", p.Provider)
	} else if !slices.Contains(allProviders, provider) {
		return "", ErrNotFound.Withf("provider: must be one of %q, got %q", allProviders, provider)
	} else {
		bind.Set("provider", provider)
	}

	if p.URL == nil {
		bind.Set("url", "")
	} else {
		bind.Set("url", strings.TrimSpace(*p.URL))
	}

	enabled := true
	if p.Enabled != nil {
		enabled = *p.Enabled
	}
	bind.Set("enabled", enabled)

	if !bind.Has("credentials") {
		return "", ErrInternalServerError.With("provider insert requires encrypted credentials binding")
	}
	if !bind.Has("pv") {
		return "", ErrInternalServerError.With("provider insert requires passphrase version binding")
	}

	meta := p.Meta
	if meta == nil {
		meta = make(ProviderMetaMap)
	}
	bind.Set("meta", meta)

	// Return the named query for inserting a provider
	return bind.Query("provider.insert"), nil
}

// Update is not supported for ProviderInsert.
func (p ProviderInsert) Update(_ *pg.Bind) error {
	return fmt.Errorf("ProviderInsert: update: not supported")
}

// Update binds mutable provider fields for an UPDATE.
func (p ProviderMeta) Update(bind *pg.Bind) error {
	bind.Del("patch")

	if p.URL != nil {
		bind.Append("patch", `url = `+bind.Set("url", strings.TrimSpace(*p.URL)))
	}
	if p.Enabled != nil {
		bind.Append("patch", `enabled = `+bind.Set("enabled", types.Value(p.Enabled)))
	}
	if p.Meta != nil {
		if expr, err := providerMetaPatch(bind, p.Meta); err != nil {
			return err
		} else if expr != "" {
			bind.Append("patch", `meta = `+expr)
		}
	}

	// Set the patch
	if bind.Join("patch", ", ") == "" {
		return ErrBadParameter.With("no fields to update")
	} else {
		bind.Set("patch", bind.Join("patch", ", "))
	}

	// Return success
	return nil
}

// Insert is not supported for ProviderMeta.
func (p ProviderMeta) Insert(_ *pg.Bind) (string, error) {
	return "", fmt.Errorf("ProviderMeta: insert: not supported")
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func (m *ProviderMetaMap) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*m = nil
		return nil
	}

	var meta map[string]any
	if err := json.Unmarshal(data, &meta); err != nil {
		return err
	}

	if meta == nil {
		*m = nil
	} else {
		*m = ProviderMetaMap(meta)
	}

	return nil
}

func (m *ProviderMetaMap) UnmarshalText(text []byte) error {
	key, raw, ok := strings.Cut(string(text), "=")
	if !ok {
		return ErrBadParameter.Withf("meta: expected key=value, got %q", string(text))
	}

	key = strings.TrimSpace(key)
	if key == "" {
		return ErrBadParameter.Withf("meta: key cannot be empty in %q", string(text))
	}

	decoded, err := parseProviderMetaValue(strings.TrimSpace(raw))
	if err != nil {
		return ErrBadParameter.Withf("meta[%q]: %v", key, err)
	}

	if *m == nil {
		*m = make(ProviderMetaMap)
	}
	(*m)[key] = decoded

	return nil
}

func parseProviderMetaValue(value string) (any, error) {
	if value == "null" {
		return nil, nil
	}

	if value == "" {
		return "", nil
	}

	var decoded any
	if err := json.Unmarshal([]byte(value), &decoded); err == nil {
		return decoded, nil
	}

	return value, nil
}

func providerMetaPatch(bind *pg.Bind, meta ProviderMetaMap) (string, error) {
	keys := make([]string, 0, len(meta))
	for key := range meta {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	expr := `COALESCE(meta, '{}'::jsonb)`
	changed := false
	for index, key := range keys {
		value := meta[key]
		if isNilValue(value) {
			expr = `(` + expr + ` - ` + bind.Set(fmt.Sprintf("meta_delete_%d", index), key) + `)`
			changed = true
			continue
		}

		data, err := json.Marshal(value)
		if err != nil {
			return "", ErrBadParameter.Withf("meta[%q]: %v", key, err)
		}
		expr = `jsonb_set(` + expr + `, ARRAY[` + bind.Set(fmt.Sprintf("meta_key_%d", index), key) + `]::text[], ` + bind.Set(fmt.Sprintf("meta_value_%d", index), string(data)) + `::jsonb, true)`
		changed = true
	}

	if !changed {
		return "", nil
	}

	return expr, nil
}

func isNilValue(value any) bool {
	if value == nil {
		return true
	}
	rv := reflect.ValueOf(value)
	switch rv.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return rv.IsNil()
	default:
		return false
	}
}
