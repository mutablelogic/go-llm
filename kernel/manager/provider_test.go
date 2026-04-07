// Copyright 2026 David Thorpe
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package manager

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	// Packages
	uuid "github.com/google/uuid"
	schema "github.com/mutablelogic/go-llm/kernel/schema"
	llmtest "github.com/mutablelogic/go-llm/pkg/test"
	pg "github.com/mutablelogic/go-pg"
	assert "github.com/stretchr/testify/assert"
)

type providerWithCredentialsMockRow struct {
	values []any
}

func (r providerWithCredentialsMockRow) Scan(dest ...any) error {
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
		case *[]byte:
			*target = append((*target)[:0], r.values[i].([]byte)...)
		default:
			return fmt.Errorf("unsupported scan type %T", dest[i])
		}
	}
	return nil
}

func Test_providerWithCredentialsListSelect(t *testing.T) {
	assert := assert.New(t)
	enabled := true
	limit := uint64(10)
	b := pg.NewBind("schema", "llm")
	_, err := (providerWithCredentialsList{
		ProviderListRequest: schema.ProviderListRequest{
			OffsetLimit: pg.OffsetLimit{Limit: &limit},
			Provider:    "ollama",
			Enabled:     &enabled,
		},
	}).Select(b, pg.List)
	if !assert.NoError(err) {
		return
	}

	assert.Equal("WHERE provider.provider = @provider AND provider.enabled = @enabled", b.Get("where"))
	assert.Equal("ollama", b.Get("provider"))
	assert.Equal(true, b.Get("enabled"))
	assert.Equal(`ORDER BY provider."name" ASC`, b.Get("orderby"))
	assert.Equal("LIMIT 10", b.Get("offsetlimit"))
}

func Test_providerWithCredentialsListSelectForUser(t *testing.T) {
	assert := assert.New(t)
	b := pg.NewBind("schema", "llm", "auth", "auth", "provider.list", "LIST_ALL", "provider.list_for_user", "LIST_USER", "provider.list_with_credentials", "LIST_ALL_CREDENTIALS", "provider.list_with_credentials_for_user", "LIST_USER_CREDENTIALS")
	b.Set("user", uuid.New())

	query, err := (providerWithCredentialsList{}).Select(b, pg.List)
	if !assert.NoError(err) {
		return
	}

	assert.Equal("LIST_USER_CREDENTIALS", query)
}

func Test_providerWithCredentialsListSelectUnsupported(t *testing.T) {
	assert := assert.New(t)
	b := pg.NewBind("schema", "llm")
	_, err := (providerWithCredentialsList{}).Select(b, pg.Get)
	if assert.Error(err) {
		assert.ErrorIs(err, schema.ErrNotImplemented)
	}
}

func Test_providerWithCredentialsScan(t *testing.T) {
	assert := assert.New(t)
	createdAt := time.Unix(100, 0).UTC()
	modifiedAt := time.Unix(200, 0).UTC()
	meta := schema.ProviderMetaMap{"status": "active"}
	credentials := []byte("secret")
	var provider providerWithCredentials

	err := provider.Scan(providerWithCredentialsMockRow{values: []any{
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
		uint64(7),
		credentials,
	}})
	if !assert.NoError(err) {
		return
	}

	assert.Equal("ollama", provider.Provider.Name)
	assert.Equal("ollama", provider.Provider.Provider)
	if assert.NotNil(provider.Provider.URL) {
		assert.Equal("http://localhost:11434", *provider.Provider.URL)
	}
	assert.NotNil(provider.Provider.Enabled)
	assert.True(*provider.Provider.Enabled)
	assert.Equal([]string{"llama3.*"}, provider.Provider.Include)
	assert.Equal([]string{"legacy"}, provider.Provider.Exclude)
	assert.Equal(createdAt, provider.Provider.CreatedAt)
	assert.Equal(&modifiedAt, provider.Provider.ModifiedAt)
	assert.Equal(meta, provider.Provider.Meta)
	assert.Equal([]string{"admins"}, provider.Provider.Groups)
	assert.Equal(uint64(7), provider.PV)
	assert.Equal(credentials, provider.Credentials)
}

func Test_providerWithCredentialsListScanAndCount(t *testing.T) {
	assert := assert.New(t)
	createdAt := time.Unix(100, 0).UTC()
	meta := schema.ProviderMetaMap{"status": "active"}
	credentials := []byte("secret")
	var list providerWithCredentialsList

	err := list.Scan(providerWithCredentialsMockRow{values: []any{
		"ollama",
		"ollama",
		"http://localhost:11434",
		true,
		[]string{"llama3.*"},
		[]string{"legacy"},
		createdAt,
		(*time.Time)(nil),
		meta,
		[]string{"admins"},
		uint64(7),
		credentials,
	}})
	if !assert.NoError(err) {
		return
	}
	if !assert.NoError(list.ScanCount(providerWithCredentialsMockRow{values: []any{uint64(1)}})) {
		return
	}

	if assert.Len(list.Body, 1) {
		assert.Equal(uint64(7), list.Body[0].PV)
		assert.Equal(credentials, list.Body[0].Credentials)
	}
	assert.Equal(uint64(1), list.Count)
}

func Test_encryptCredentialsEmptyReturnsNoPayload(t *testing.T) {
	assert := assert.New(t)

	var manager Manager
	manager.defaults("test", "0.0.0")
	if err := manager.passphrases.Set(1, "test1234"); !assert.NoError(err) {
		return
	}

	pv, credentials, err := manager.encryptCredentials(schema.ProviderCredentials{})
	if !assert.NoError(err) {
		return
	}

	assert.Equal(uint64(0), pv)
	assert.Empty(credentials)
}

func Test_encryptCredentialsDecryptRoundTrip(t *testing.T) {
	assert := assert.New(t)

	var manager Manager
	manager.defaults("test", "0.0.0")
	if err := manager.passphrases.Set(1, "test1234"); !assert.NoError(err) {
		return
	}

	pv, encrypted, err := manager.encryptCredentials(schema.ProviderInsert{
		ProviderCredentials: schema.ProviderCredentials{APIKey: "secret-key"},
	})
	if !assert.NoError(err) {
		return
	}
	assert.NotZero(pv)
	assert.NotEmpty(encrypted)

	var credentials schema.ProviderCredentials
	if !assert.NoError(manager.decryptCredentials(encrypted, pv, &credentials)) {
		return
	}
	assert.Equal("secret-key", credentials.APIKey)
}

func TestProviderCRUDIntegration(t *testing.T) {
	conn, m := newIntegrationManager(t)
	ctx := context.Background()

	created := llmtest.CreateProvider(t, conn.ProviderInsert(), m.CreateProvider, m.SyncProviders)
	assert := assert.New(t)
	assert.Equal(conn.Config.Name, created.Name)
	assert.Equal(conn.Config.Provider, created.Provider)
	if assert.NotNil(created.URL) {
		assert.Equal(conn.Config.URL, *created.URL)
	}
	assert.Equal(conn.Config.Groups, created.Groups)

	listed, err := m.ListProviders(ctx, schema.ProviderListRequest{Name: created.Name})
	if !assert.NoError(err) {
		return
	}
	if assert.Len(listed.Body, 1) {
		assert.Equal(created.Name, listed.Body[0].Name)
	}

	got, err := m.GetProvider(ctx, created.Name)
	if !assert.NoError(err) {
		return
	}
	assert.Equal(created.Name, got.Name)
	assert.Equal(created.Groups, got.Groups)

	updatedURL := conn.Config.URL
	if updatedURL == "" {
		updatedURL = "http://localhost:11434/api"
	}
	updatedMeta := schema.ProviderMetaMap{"scope": "integration"}
	updated, err := m.UpdateProvider(ctx, created.Name, schema.ProviderMeta{
		URL:     &updatedURL,
		Include: []string{".*"},
		Meta:    updatedMeta,
		Groups:  conn.Config.Groups,
	})
	if !assert.NoError(err) {
		return
	}
	if assert.NotNil(updated.URL) {
		assert.Equal(updatedURL, *updated.URL)
	}
	assert.Equal([]string{".*"}, updated.Include)
	assert.Equal(updatedMeta, updated.Meta)
	assert.NotNil(updated.ModifiedAt)

	deleted, err := m.DeleteProvider(ctx, created.Name)
	if !assert.NoError(err) {
		return
	}
	assert.Equal(created.Name, deleted.Name)

	_, err = m.GetProvider(ctx, created.Name)
	if assert.Error(err) {
		assert.ErrorIs(err, schema.ErrNotFound)
	}
}

func TestListProvidersIntegrationWithGroupFilter(t *testing.T) {
	conn, m := newIntegrationManager(t)
	ctx := context.Background()
	assert := assert.New(t)

	restricted := conn.ProviderInsert()
	restricted.Name = "restricted-ollama"
	llmtest.CreateProvider(t, restricted, m.CreateProvider, m.SyncProviders)

	public := conn.ProviderInsert()
	public.Name = "public-ollama"
	public.Groups = nil
	publicProvider := llmtest.CreateProvider(t, public, m.CreateProvider, m.SyncProviders)

	admins, err := m.ListProviders(ctx, schema.ProviderListRequest{Groups: []string{"admins"}})
	if !assert.NoError(err) {
		return
	}
	assert.Len(admins.Body, 2)

	nonMatching, err := m.ListProviders(ctx, schema.ProviderListRequest{Groups: []string{"dev"}})
	if !assert.NoError(err) {
		return
	}
	if assert.Len(nonMatching.Body, 1) {
		assert.Equal(publicProvider.Name, nonMatching.Body[0].Name)
	}
}

func TestProvidersForUserIntegration(t *testing.T) {
	conn, m := newIntegrationManager(t)
	ctx := context.Background()
	assert := assert.New(t)

	restricted := conn.ProviderInsert()
	restricted.Name = "restricted-ollama"
	llmtest.CreateProvider(t, restricted, m.CreateProvider, m.SyncProviders)

	public := conn.ProviderInsert()
	public.Name = "public-ollama"
	public.Groups = nil
	publicProvider := llmtest.CreateProvider(t, public, m.CreateProvider, m.SyncProviders)

	all, err := m.providersForUser(ctx, "", nil)
	if !assert.NoError(err) {
		return
	}
	assert.Len(all, 2)

	admins, err := m.providersForUser(ctx, "", llmtest.AdminUser(conn))
	if !assert.NoError(err) {
		return
	}
	assert.Len(admins, 2)

	ungrouped, err := m.providersForUser(ctx, "", llmtest.User(conn))
	if !assert.NoError(err) {
		return
	}
	if assert.Len(ungrouped, 1) {
		assert.Equal(publicProvider.Name, ungrouped[0].Name)
	}
}
