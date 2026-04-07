package schema_test

import (
	"errors"
	"testing"
	"time"

	// Packages
	uuid "github.com/google/uuid"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	pg "github.com/mutablelogic/go-pg"
	assert "github.com/stretchr/testify/assert"
)

type connectorMockRow struct {
	values []any
}

func (r connectorMockRow) Scan(dest ...any) error {
	if len(dest) != len(r.values) {
		return errors.New("unexpected scan arity")
	}
	for i := range dest {
		switch target := dest[i].(type) {
		case *string:
			*target = r.values[i].(string)
		case **string:
			switch value := r.values[i].(type) {
			case *string:
				*target = value
			case string:
				stringValue := value
				*target = &stringValue
			case nil:
				*target = nil
			default:
				return errors.New("unsupported string scan source")
			}
		case *bool:
			*target = r.values[i].(bool)
		case **bool:
			switch value := r.values[i].(type) {
			case *bool:
				*target = value
			case bool:
				boolValue := value
				*target = &boolValue
			case nil:
				*target = nil
			default:
				return errors.New("unsupported bool scan source")
			}
		case *time.Time:
			*target = r.values[i].(time.Time)
		case **time.Time:
			switch value := r.values[i].(type) {
			case *time.Time:
				*target = value
			case time.Time:
				timeValue := value
				*target = &timeValue
			case nil:
				*target = nil
			default:
				return errors.New("unsupported time scan source")
			}
		case *schema.ProviderMetaMap:
			*target = r.values[i].(schema.ProviderMetaMap)
		case *[]string:
			*target = append((*target)[:0], r.values[i].([]string)...)
		case *uint:
			*target = r.values[i].(uint)
		default:
			return errors.New("unsupported scan type")
		}
	}
	return nil
}

func TestConnectorScan(t *testing.T) {
	assert := assert.New(t)
	createdAt := time.Unix(100, 0).UTC()
	modifiedAt := time.Unix(101, 0).UTC()
	connectedAt := time.Unix(102, 0).UTC()
	meta := schema.ProviderMetaMap{"env": "dev"}

	var connector schema.Connector
	err := connector.Scan(connectorMockRow{values: []any{
		"https://example.com/sse",
		"mcp",
		true,
		"server-name",
		"Server Title",
		"Server Description",
		meta,
		[]string{"admins"},
		createdAt,
		&modifiedAt,
		&connectedAt,
	}})
	if !assert.NoError(err) {
		return
	}

	assert.Equal("https://example.com/sse", connector.URL)
	assert.Equal("mcp", *connector.Namespace)
	assert.True(*connector.Enabled)
	assert.Equal("server-name", *connector.Name)
	assert.Equal("Server Title", *connector.Title)
	assert.Equal("Server Description", *connector.Description)
	assert.Equal(meta, connector.Meta)
	assert.Equal([]string{"admins"}, connector.Groups)
	assert.Equal(createdAt, connector.CreatedAt)
	if assert.NotNil(connector.ModifiedAt) {
		assert.Equal(modifiedAt, *connector.ModifiedAt)
	}
	if assert.NotNil(connector.ConnectedAt) {
		assert.Equal(connectedAt, *connector.ConnectedAt)
	}
}

func TestConnectorInsert(t *testing.T) {
	assert := assert.New(t)
	b := pg.NewBind("schema", "llm", "connector.insert", "INSERT")
	enabled := false
	namespace := "mcp"

	query, err := (schema.ConnectorInsert{
		URL: "HTTPS://Example.COM/sse?token=abc#frag",
		ConnectorMeta: schema.ConnectorMeta{
			Enabled:   &enabled,
			Namespace: &namespace,
			Meta:      schema.ProviderMetaMap{"env": "dev"},
		},
	}).Insert(b)
	if !assert.NoError(err) {
		return
	}

	assert.Equal("INSERT", query)
	assert.Equal("https://example.com/sse", b.Get("url"))
	assert.Equal("mcp", b.Get("namespace"))
	assert.Equal(false, b.Get("enabled"))
	assert.Equal(schema.ProviderMetaMap{"env": "dev"}, b.Get("meta"))
}

func TestConnectorURLSelectorGet(t *testing.T) {
	assert := assert.New(t)
	b := pg.NewBind("schema", "llm", "connector.select", "SELECT_ALL", "connector.select_for_user", "SELECT_USER")

	query, err := schema.ConnectorURLSelector("HTTPS://Example.COM/sse?token=abc#frag").Select(b, pg.Get)
	if !assert.NoError(err) {
		return
	}

	assert.Equal("SELECT_ALL", query)
	assert.Equal("https://example.com/sse", b.Get("url"))
}

func TestConnectorURLSelectorGetForUser(t *testing.T) {
	assert := assert.New(t)
	b := pg.NewBind("schema", "llm", "auth", "auth", "connector.select", "SELECT_ALL", "connector.select_for_user", "SELECT_USER")
	b.Set("user", uuid.New())

	query, err := schema.ConnectorURLSelector("https://example.com/sse").Select(b, pg.Get)
	if !assert.NoError(err) {
		return
	}

	assert.Equal("SELECT_USER", query)
	assert.Equal("https://example.com/sse", b.Get("url"))
}

func TestConnectorGroupRefInsert(t *testing.T) {
	assert := assert.New(t)
	b := pg.NewBind("schema", "llm", "connector_group.insert", "INSERT")

	query, err := (schema.ConnectorGroupRef{Connector: "HTTPS://Example.COM/sse?token=abc#frag", Group: "$admin$"}).Insert(b)
	if !assert.NoError(err) {
		return
	}

	assert.Equal("INSERT", query)
	assert.Equal("https://example.com/sse", b.Get("connector"))
	assert.Equal("$admin$", b.Get("group"))
}

func TestConnectorListRequestQuery(t *testing.T) {
	assert := assert.New(t)
	limit := uint64(10)
	enabled := true

	values := (schema.ConnectorListRequest{
		OffsetLimit: pg.OffsetLimit{Offset: 5, Limit: &limit},
		Namespace:   "mcp",
		Enabled:     &enabled,
	}).Query()

	assert.Equal("5", values.Get("offset"))
	assert.Equal("10", values.Get("limit"))
	assert.Equal("mcp", values.Get("namespace"))
	assert.Equal("true", values.Get("enabled"))
}

func TestConnectorListRequestSelect(t *testing.T) {
	assert := assert.New(t)
	b := pg.NewBind("schema", "llm", "connector.list", "LIST_ALL", "connector.list_for_user", "LIST_USER")
	limit := uint64(10)
	enabled := true

	query, err := (schema.ConnectorListRequest{
		OffsetLimit: pg.OffsetLimit{Offset: 10, Limit: &limit},
		Namespace:   "mcp",
		Enabled:     &enabled,
	}).Select(b, pg.List)
	if !assert.NoError(err) {
		return
	}

	assert.Equal("LIST_ALL", query)
	assert.Equal("WHERE connector.namespace = @namespace AND connector.enabled = @enabled", b.Get("where"))
	assert.Equal("mcp", b.Get("namespace"))
	assert.Equal(true, b.Get("enabled"))
	assert.Equal("LIMIT 10 OFFSET 10", b.Get("offsetlimit"))
	assert.Equal(`ORDER BY connector.created_at DESC, connector.url ASC`, b.Get("orderby"))
}

func TestConnectorListRequestSelectInvalidNamespace(t *testing.T) {
	assert := assert.New(t)
	b := pg.NewBind("schema", "llm")

	_, err := (schema.ConnectorListRequest{Namespace: "bad namespace"}).Select(b, pg.List)
	if assert.Error(err) {
		assert.ErrorIs(err, schema.ErrBadParameter)
	}
}

func TestConnectorListRequestSelectForUser(t *testing.T) {
	assert := assert.New(t)
	b := pg.NewBind("schema", "llm", "auth", "auth", "connector.list", "LIST_ALL", "connector.list_for_user", "LIST_USER")
	b.Set("user", uuid.New())

	query, err := (schema.ConnectorListRequest{}).Select(b, pg.List)
	if !assert.NoError(err) {
		return
	}

	assert.Equal("LIST_USER", query)
	assert.Equal("", b.Get("where"))
}

func TestConnectorListRequestSelectIgnoresNilUser(t *testing.T) {
	assert := assert.New(t)
	b := pg.NewBind("schema", "llm", "auth", "auth", "connector.list", "LIST_ALL", "connector.list_for_user", "LIST_USER")
	b.Set("user", uuid.Nil)

	query, err := (schema.ConnectorListRequest{}).Select(b, pg.List)
	if !assert.NoError(err) {
		return
	}

	assert.Equal("LIST_USER", query)
}

func TestConnectorListScan(t *testing.T) {
	assert := assert.New(t)
	createdAt := time.Unix(100, 0).UTC()

	var list schema.ConnectorList
	err := list.Scan(connectorMockRow{values: []any{
		"https://example.com/sse",
		"mcp",
		true,
		"server-name",
		"Server Title",
		"Server Description",
		schema.ProviderMetaMap{"env": "dev"},
		[]string{"admins"},
		createdAt,
		nil,
		nil,
	}})
	if !assert.NoError(err) {
		return
	}

	if assert.Len(list.Body, 1) {
		assert.Equal("https://example.com/sse", list.Body[0].URL)
		assert.Equal([]string{"admins"}, list.Body[0].Groups)
	}
}

func TestConnectorListScanCount(t *testing.T) {
	assert := assert.New(t)
	var list schema.ConnectorList

	err := list.ScanCount(connectorMockRow{values: []any{uint(3)}})
	if !assert.NoError(err) {
		return
	}

	assert.Equal(uint(3), list.Count)
}

func TestConnectorInsertRequiresNamespace(t *testing.T) {
	assert := assert.New(t)
	b := pg.NewBind("schema", "llm", "connector.insert", "INSERT")

	_, err := (schema.ConnectorInsert{URL: "https://example.com/sse"}).Insert(b)
	if assert.Error(err) {
		assert.ErrorIs(err, schema.ErrBadParameter)
	}
}

func TestConnectorInsertRejectsInvalidNamespace(t *testing.T) {
	assert := assert.New(t)
	b := pg.NewBind("schema", "llm", "connector.insert", "INSERT")
	namespace := "bad namespace"

	_, err := (schema.ConnectorInsert{
		URL: "https://example.com/sse",
		ConnectorMeta: schema.ConnectorMeta{
			Namespace: &namespace,
		},
	}).Insert(b)
	if assert.Error(err) {
		assert.ErrorIs(err, schema.ErrBadParameter)
	}
}

func TestCanonicalURLRejectsRelativeURL(t *testing.T) {
	assert := assert.New(t)

	_, err := schema.CanonicalURL("sss")
	if assert.Error(err) {
		assert.ErrorIs(err, schema.ErrBadParameter)
	}
}

func TestConnectorMetaUpdatePatch(t *testing.T) {
	assert := assert.New(t)
	b := pg.NewBind()
	enabled := false
	namespace := "renamed"

	err := (schema.ConnectorMeta{
		Enabled:   &enabled,
		Namespace: &namespace,
		Meta: schema.ProviderMetaMap{
			"remove": nil,
			"status": "active",
		},
	}).Update(b)
	if !assert.NoError(err) {
		return
	}

	patch, _ := b.Get("patch").(string)
	assert.Contains(patch, "enabled = @enabled")
	assert.Contains(patch, "namespace = @namespace")
	assert.Contains(patch, "meta = jsonb_set(")
	assert.Equal(false, b.Get("enabled"))
	assert.Equal("renamed", b.Get("namespace"))
	assert.Equal("remove", b.Get("meta_delete_0"))
	assert.Equal("status", b.Get("meta_key_1"))
	assert.Equal(`"active"`, b.Get("meta_value_1"))
}

func TestConnectorMetaUpdateNoFields(t *testing.T) {
	assert := assert.New(t)
	b := pg.NewBind()
	err := (schema.ConnectorMeta{}).Update(b)
	if assert.Error(err) {
		assert.ErrorIs(err, schema.ErrBadParameter)
	}
}

func TestConnectorMetaUpdateRejectsEmptyNamespace(t *testing.T) {
	assert := assert.New(t)
	b := pg.NewBind()
	empty := ""

	err := (schema.ConnectorMeta{Namespace: &empty}).Update(b)
	if assert.Error(err) {
		assert.ErrorIs(err, schema.ErrBadParameter)
	}
}

func TestConnectorStateUpdatePatch(t *testing.T) {
	assert := assert.New(t)
	b := pg.NewBind()
	connectedAt := time.Unix(200, 0).UTC()
	name := "server-name"
	title := "Server Title"
	description := "Server Description"

	err := (schema.ConnectorState{
		ConnectedAt: &connectedAt,
		Name:        &name,
		Title:       &title,
		Description: &description,
	}).Update(b)
	if !assert.NoError(err) {
		return
	}

	patch, _ := b.Get("patch").(string)
	assert.Contains(patch, "connected_at = @connected_at")
	assert.Contains(patch, "name = @name")
	assert.Contains(patch, "title = @title")
	assert.Contains(patch, "description = @description")
	assert.Equal(connectedAt, b.Get("connected_at"))
	assert.Equal(name, b.Get("name"))
	assert.Equal(title, b.Get("title"))
	assert.Equal(description, b.Get("description"))
}

func TestConnectorStateUpdateNoSupportedFields(t *testing.T) {
	assert := assert.New(t)
	b := pg.NewBind()
	version := "1.0.0"

	err := (schema.ConnectorState{Version: &version}).Update(b)
	if assert.Error(err) {
		assert.ErrorIs(err, schema.ErrBadParameter)
	}
}
