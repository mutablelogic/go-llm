package schema_test

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	// Packages
	uuid "github.com/google/uuid"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	pg "github.com/mutablelogic/go-pg"
	assert "github.com/stretchr/testify/assert"
)

type providerMockRow struct {
	values []any
}

func (r providerMockRow) Scan(dest ...any) error {
	if len(dest) != len(r.values) {
		return errors.New("unexpected scan arity")
	}
	for i := range dest {
		switch target := dest[i].(type) {
		case *string:
			*target = r.values[i].(string)
		case *bool:
			*target = r.values[i].(bool)
		case *[]string:
			*target = append((*target)[:0], r.values[i].([]string)...)
		case *time.Time:
			*target = r.values[i].(time.Time)
		case **time.Time:
			*target = r.values[i].(*time.Time)
		case *schema.ProviderMetaMap:
			*target = r.values[i].(schema.ProviderMetaMap)
		case *uint64:
			*target = r.values[i].(uint64)
		default:
			return errors.New("unsupported scan type")
		}
	}
	return nil
}

func TestProviderMetaUpdateNoFields(t *testing.T) {
	assert := assert.New(t)
	b := pg.NewBind()
	err := (schema.ProviderMeta{}).Update(b)
	assert.Error(err)
	assert.True(errors.Is(err, schema.ErrBadParameter))
}

func TestProviderMetaUpdatePatch(t *testing.T) {
	assert := assert.New(t)
	enabled := false
	urlValue := " https://api.example.com "
	b := pg.NewBind()
	err := (schema.ProviderMeta{
		URL:     &urlValue,
		Enabled: &enabled,
		Include: []string{"gpt-.*"},
		Exclude: []string{"gpt-legacy"},
		Meta: schema.ProviderMetaMap{
			"remove": nil,
			"status": "active",
		},
	}).Update(b)
	if !assert.NoError(err) {
		return
	}

	patch, _ := b.Get("patch").(string)
	assert.Contains(patch, "url = @url")
	assert.Contains(patch, "enabled = @enabled")
	assert.Contains(patch, `"include" = @include`)
	assert.Contains(patch, `"exclude" = @exclude`)
	assert.Contains(patch, "meta = jsonb_set(")
	assert.Contains(patch, `COALESCE(meta, '{}'::jsonb)`)
	assert.Contains(patch, " - @meta_delete_0")
	assert.Contains(patch, "@meta_value_1::jsonb")
	assert.Equal("https://api.example.com", b.Get("url"))
	assert.Equal(false, b.Get("enabled"))
	assert.Equal([]string{"gpt-.*"}, b.Get("include"))
	assert.Equal([]string{"gpt-legacy"}, b.Get("exclude"))
	assert.Equal("remove", b.Get("meta_delete_0"))
	assert.Equal("status", b.Get("meta_key_1"))
	assert.Equal(`"active"`, b.Get("meta_value_1"))
}

func TestProviderMetaMarshalOmitsUnsetEnabled(t *testing.T) {
	assert := assert.New(t)

	data, err := json.Marshal(schema.ProviderMeta{})
	if !assert.NoError(err) {
		return
	}

	assert.JSONEq(`{}`, string(data))

	enabled := false
	data, err = json.Marshal(schema.ProviderMeta{Enabled: &enabled})
	if !assert.NoError(err) {
		return
	}

	assert.JSONEq(`{"enabled":false}`, string(data))
}

func TestProviderMetaMapUnmarshalText(t *testing.T) {
	assert := assert.New(t)

	var meta schema.ProviderMetaMap
	for _, value := range []string{"x=y", "enabled=true", `obj={"a":1}`, "remove=null", "empty="} {
		if !assert.NoError((&meta).UnmarshalText([]byte(value))) {
			return
		}
	}

	assert.Equal("y", meta["x"])
	assert.Equal(true, meta["enabled"])
	if obj, ok := meta["obj"].(map[string]any); assert.True(ok) {
		assert.Equal(float64(1), obj["a"])
	}
	assert.Nil(meta["remove"])
	assert.Equal("", meta["empty"])
}

func TestProviderMetaMapUnmarshalTextRequiresKeyValue(t *testing.T) {
	assert := assert.New(t)

	var meta schema.ProviderMetaMap
	err := (&meta).UnmarshalText([]byte("broken"))
	if assert.Error(err) {
		assert.ErrorIs(err, schema.ErrBadParameter)
	}
}

func TestProviderMetaMapUnmarshalJSON(t *testing.T) {
	assert := assert.New(t)

	var meta schema.ProviderMetaMap
	err := json.Unmarshal([]byte(`{"x":"y","enabled":true,"obj":{"a":1},"remove":null}`), &meta)
	if !assert.NoError(err) {
		return
	}

	assert.Equal("y", meta["x"])
	assert.Equal(true, meta["enabled"])
	if obj, ok := meta["obj"].(map[string]any); assert.True(ok) {
		assert.Equal(float64(1), obj["a"])
	}
	assert.Nil(meta["remove"])
}

func TestNormalizeProviderGroupAllowsSpecialAuthGroup(t *testing.T) {
	assert := assert.New(t)

	ref := schema.ProviderGroupRef{Provider: "primary", Group: "$admin$"}
	b := pg.NewBind("schema", "llm", "provider_group.insert", "INSERT")
	_, err := ref.Insert(b)
	if !assert.NoError(err) {
		return
	}

	assert.Equal("primary", b.Get("provider"))
	assert.Equal("$admin$", b.Get("group"))
}

func TestProviderNameSelectorUpdate(t *testing.T) {
	assert := assert.New(t)
	b := pg.NewBind("schema", "llm")
	_, err := schema.ProviderNameSelector("ollama").Select(b, pg.Update)
	if !assert.NoError(err) {
		return
	}

	assert.Equal("ollama", b.Get("name"))
}

func TestProviderNameSelectorGet(t *testing.T) {
	assert := assert.New(t)
	b := pg.NewBind("schema", "llm")
	_, err := schema.ProviderNameSelector("ollama").Select(b, pg.Get)
	if !assert.NoError(err) {
		return
	}

	assert.Equal("ollama", b.Get("name"))
}

func TestProviderNameSelectorDelete(t *testing.T) {
	assert := assert.New(t)
	b := pg.NewBind("schema", "llm")
	_, err := schema.ProviderNameSelector("ollama").Select(b, pg.Delete)
	if !assert.NoError(err) {
		return
	}

	assert.Equal("ollama", b.Get("name"))
}

func TestProviderNameSelectorInvalid(t *testing.T) {
	assert := assert.New(t)
	b := pg.NewBind("schema", "llm")
	_, err := schema.ProviderNameSelector("  not valid  ").Select(b, pg.Update)
	if assert.Error(err) {
		assert.True(errors.Is(err, schema.ErrBadParameter))
	}
}

func TestProviderListRequestSelect(t *testing.T) {
	assert := assert.New(t)
	limit := uint64(999)
	b := pg.NewBind("schema", "llm")
	_, err := (schema.ProviderListRequest{OffsetLimit: pg.OffsetLimit{Offset: 10, Limit: &limit}}).Select(b, pg.List)
	if !assert.NoError(err) {
		return
	}

	assert.Equal("", b.Get("where"))
	assert.Equal(`ORDER BY provider."name" ASC`, b.Get("orderby"))
	assert.Equal("LIMIT 100 OFFSET 10", b.Get("offsetlimit"))
}

func TestProviderListRequestSelectWithFilters(t *testing.T) {
	assert := assert.New(t)
	enabled := true
	limit := uint64(10)
	b := pg.NewBind("schema", "llm")
	_, err := (schema.ProviderListRequest{
		OffsetLimit: pg.OffsetLimit{Limit: &limit},
		Name:        " local-ollama ",
		Provider:    "ollama",
		Enabled:     &enabled,
	}).Select(b, pg.List)
	if !assert.NoError(err) {
		return
	}

	assert.Equal("WHERE provider.\"name\" = @name AND provider.provider = @provider AND provider.enabled = @enabled", b.Get("where"))
	assert.Equal("local-ollama", b.Get("name"))
	assert.Equal("ollama", b.Get("provider"))
	assert.Equal(true, b.Get("enabled"))
	assert.Equal("LIMIT 10", b.Get("offsetlimit"))
}

func TestProviderListRequestSelectWithGroupFilter(t *testing.T) {
	assert := assert.New(t)
	b := pg.NewBind("schema", "llm")
	_, err := (schema.ProviderListRequest{Groups: []string{"admins", "dev"}}).Select(b, pg.List)
	if !assert.NoError(err) {
		return
	}

	where, _ := b.Get("where").(string)
	assert.Contains(where, `NOT EXISTS (`)
	assert.Contains(where, `FROM "llm".provider_group AS provider_group`)
	assert.Contains(where, `provider_group."group" = ANY(@groups)`)
	assert.Equal([]string{"admins", "dev"}, b.Get("groups"))
}

func TestProviderListRequestSelectForUser(t *testing.T) {
	assert := assert.New(t)
	b := pg.NewBind("schema", "llm", "auth", "auth", "provider.list", "LIST_ALL", "provider.list_for_user", "LIST_USER")
	b.Set("user", uuid.New())

	query, err := (schema.ProviderListRequest{}).Select(b, pg.List)
	if !assert.NoError(err) {
		return
	}

	assert.Equal("LIST_USER", query)
	assert.Equal("", b.Get("where"))
}

func TestProviderListRequestSelectInvalidProviderFilter(t *testing.T) {
	assert := assert.New(t)
	b := pg.NewBind("schema", "llm")
	_, err := (schema.ProviderListRequest{Provider: "not valid"}).Select(b, pg.List)
	if assert.Error(err) {
		assert.ErrorIs(err, schema.ErrBadParameter)
	}
}

func TestProviderListRequestSelectUnknownProviderFilter(t *testing.T) {
	assert := assert.New(t)
	b := pg.NewBind("schema", "llm")
	_, err := (schema.ProviderListRequest{Provider: "openai"}).Select(b, pg.List)
	if assert.Error(err) {
		assert.ErrorIs(err, schema.ErrNotFound)
	}
}

func TestProviderListRequestSelectUnsupported(t *testing.T) {
	assert := assert.New(t)
	b := pg.NewBind("schema", "llm")
	_, err := (schema.ProviderListRequest{}).Select(b, pg.Get)
	if assert.Error(err) {
		assert.ErrorIs(err, schema.ErrNotImplemented)
	}
}

func TestProviderListRequestQueryEmpty(t *testing.T) {
	assert := assert.New(t)

	values := (schema.ProviderListRequest{}).Query()
	assert.Empty(values)
}

func TestProviderListRequestQueryPaginationAndFilters(t *testing.T) {
	assert := assert.New(t)
	enabled := true
	limit := uint64(25)

	values := (schema.ProviderListRequest{
		OffsetLimit: pg.OffsetLimit{Offset: 10, Limit: &limit},
		Name:        " local-ollama ",
		Provider:    " ollama ",
		Enabled:     &enabled,
	}).Query()

	assert.Equal("10", values.Get("offset"))
	assert.Equal("25", values.Get("limit"))
	assert.Equal("local-ollama", values.Get("name"))
	assert.Equal("ollama", values.Get("provider"))
	assert.Equal("true", values.Get("enabled"))
	assert.Len(values, 5)
}

func TestProviderListRequestSelectInvalidNameFilter(t *testing.T) {
	assert := assert.New(t)
	b := pg.NewBind("schema", "llm")
	_, err := (schema.ProviderListRequest{Name: "not valid"}).Select(b, pg.List)
	if assert.Error(err) {
		assert.ErrorIs(err, schema.ErrBadParameter)
	}
}

func TestProviderListScan(t *testing.T) {
	assert := assert.New(t)
	createdAt := time.Unix(100, 0).UTC()
	modifiedAt := time.Unix(200, 0).UTC()
	meta := schema.ProviderMetaMap{"status": "active"}
	var list schema.ProviderList

	err := list.Scan(providerMockRow{values: []any{
		"ollama",
		"ollama",
		"http://localhost:11434",
		true,
		[]string{"llama3.*"},
		[]string{"legacy"},
		createdAt,
		&modifiedAt,
		meta,
		[]string{"admins"},
	}})
	if !assert.NoError(err) {
		return
	}

	if assert.Len(list.Body, 1) {
		assert.Equal("ollama", list.Body[0].Name)
		assert.Equal("ollama", list.Body[0].Provider)
		if assert.NotNil(list.Body[0].URL) {
			assert.Equal("http://localhost:11434", *list.Body[0].URL)
		}
		assert.NotNil(list.Body[0].Enabled)
		assert.True(*list.Body[0].Enabled)
		assert.Equal([]string{"llama3.*"}, list.Body[0].Include)
		assert.Equal([]string{"legacy"}, list.Body[0].Exclude)
		assert.Equal(createdAt, list.Body[0].CreatedAt)
		assert.Equal(&modifiedAt, list.Body[0].ModifiedAt)
		assert.Equal(meta, list.Body[0].Meta)
		assert.Equal([]string{"admins"}, list.Body[0].Groups)
	}
}

func TestProviderListScanCount(t *testing.T) {
	assert := assert.New(t)
	var list schema.ProviderList

	err := list.ScanCount(providerMockRow{values: []any{uint64(3)}})
	if !assert.NoError(err) {
		return
	}

	assert.Equal(uint64(3), list.Count)
}

func TestProviderCellHandlesNilOptionalFields(t *testing.T) {
	assert := assert.New(t)
	createdAt := time.Unix(100, 0).UTC()

	provider := schema.Provider{
		Name:      "local",
		Provider:  "ollama",
		CreatedAt: createdAt,
	}

	assert.Equal("local", provider.Cell(0))
	assert.Equal("ollama", provider.Cell(1))
	assert.Equal("", provider.Cell(2))
	assert.Equal("false", provider.Cell(3))
	assert.Equal("", provider.Cell(4))
	assert.Equal("", provider.Cell(5))
	assert.Equal("1970-01-01 00:01:40", provider.Cell(6))
	assert.Equal("", provider.Cell(7))
	assert.Equal("", provider.Cell(99))
}

func TestProviderInsertRequiresEncryptedCredentialsBinding(t *testing.T) {
	assert := assert.New(t)
	b := pg.NewBind("schema", "llm", "provider.insert", "INSERT")

	_, err := (schema.ProviderInsert{
		Name:     "primary",
		Provider: "ollama",
	}).Insert(b)
	if assert.Error(err) {
		assert.ErrorIs(err, schema.ErrInternalServerError)
	}
}

func TestProviderInsertPreservesPreboundCredentialsAndPV(t *testing.T) {
	assert := assert.New(t)
	b := pg.NewBind(
		"schema", "llm",
		"provider.insert", "INSERT",
		"credentials", []byte("encrypted"),
		"pv", uint64(7),
	)

	_, err := (schema.ProviderInsert{
		Name:     "primary",
		Provider: "ollama",
		ProviderMeta: schema.ProviderMeta{
			URL: func() *string {
				value := "http://localhost:11434"
				return &value
			}(),
		},
		ProviderCredentials: schema.ProviderCredentials{APIKey: "should-not-be-written"},
	}).Insert(b)
	if !assert.NoError(err) {
		return
	}

	assert.Equal([]byte("encrypted"), b.Get("credentials"))
	assert.Equal(uint64(7), b.Get("pv"))
	assert.Equal("primary", b.Get("name"))
	assert.Equal("ollama", b.Get("provider"))
	assert.Equal("http://localhost:11434", b.Get("url"))
	assert.Equal([]string{}, b.Get("include"))
	assert.Equal([]string{}, b.Get("exclude"))
}
