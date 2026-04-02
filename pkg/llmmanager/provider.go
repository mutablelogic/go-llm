package manager

import (
	"context"
	"encoding/json"

	// Packages
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	pg "github.com/mutablelogic/go-pg"
	httpresponse "github.com/mutablelogic/go-server/pkg/httpresponse"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// CreateProvider validates a provider insert request and returns the persisted
// provider shape. Database persistence is not wired up yet.
func (m *Manager) CreateProvider(ctx context.Context, req schema.ProviderInsert) (*schema.Provider, error) {
	if req.Provider == "" {
		req.Provider = req.Name
	}

	pv, credentials, err := m.encryptCredentials(req)
	if err != nil {
		return nil, err
	}

	var result schema.Provider
	if err := m.PoolConn.With("credentials", credentials, "pv", pv).Insert(ctx, &result, req); err != nil {
		return nil, pg.NormalizeError(err)
	}

	// Return success
	return types.Ptr(result), nil
}

// ListProviders returns a list of providers matching the given request parameters.
func (m *Manager) ListProviders(ctx context.Context, req schema.ProviderListRequest) (*schema.ProviderList, error) {
	result := schema.ProviderList{ProviderListRequest: req}
	if err := m.PoolConn.List(ctx, &result, req); err != nil {
		return nil, pg.NormalizeError(err)
	}

	// Set the offset and limit in the result to reflect the actual count of items returned
	// which may be less than the requested limit if there are not enough items in the database.
	result.OffsetLimit = req.OffsetLimit
	result.OffsetLimit.Clamp(result.Count)

	// Return success
	return types.Ptr(result), nil
}

// GetProvider returns a single provider by name.
func (m *Manager) GetProvider(ctx context.Context, name string) (*schema.Provider, error) {
	var result schema.Provider
	if err := m.PoolConn.Get(ctx, &result, schema.ProviderNameSelector(name)); err != nil {
		return nil, pg.NormalizeError(err)
	}

	// Return success
	return types.Ptr(result), nil
}

// UpdateProvider updates the writable metadata for a provider by name and returns the updated provider.
func (m *Manager) UpdateProvider(ctx context.Context, name string, meta schema.ProviderMeta) (*schema.Provider, error) {
	var result schema.Provider
	if err := m.PoolConn.Update(ctx, &result, schema.ProviderNameSelector(name), meta); err != nil {
		return nil, pg.NormalizeError(err)
	}

	// Return success
	return types.Ptr(result), nil
}

// DeleteProvider deletes a single provider by name and returns the deleted provider.
func (m *Manager) DeleteProvider(ctx context.Context, name string) (*schema.Provider, error) {
	var result schema.Provider
	if err := m.PoolConn.Delete(ctx, &result, schema.ProviderNameSelector(name)); err != nil {
		return nil, pg.NormalizeError(err)
	}

	// Return success
	return types.Ptr(result), nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func (m *Manager) encryptCredentials(req schema.ProviderInsert) (uint64, []byte, error) {
	// Check for at least one passphrase configured
	if len(m.passphrases.Keys()) == 0 {
		return 0, nil, httpresponse.ErrServiceUnavailable.Withf("no encryption passphrase configured for provider credentials")
	}

	// Turn the credentials into JSON. If the credentials are empty this will
	// return an empty JSON object, which we can treat as an empty byte array.
	data, err := json.Marshal(req.ProviderCredentials)
	if err != nil {
		return 0, nil, httpresponse.ErrBadRequest.With(err)
	} else if string(data) == "{}" {
		return 0, []byte{}, nil
	}

	// Get the encryption passphrase for the current passphrase version. If there is no
	// passphrase configured for the current version, return an error
	if pv, crypted, err := m.passphrases.Encrypt(0, data); err != nil {
		return 0, nil, httpresponse.ErrBadRequest.With(err)
	} else {
		return pv, []byte(crypted), nil
	}
}

func (m *Manager) decryptCredentials(encrypted []byte, pv uint64, decrypted any) error {
	// Check for at least one passphrase configured
	if len(m.passphrases.Keys()) == 0 {
		return httpresponse.ErrServiceUnavailable.Withf("no encryption passphrase configured for provider credentials")
	}

	if data, err := m.passphrases.Decrypt(pv, string(encrypted)); err != nil {
		return httpresponse.ErrBadRequest.With(err)
	} else if err := json.Unmarshal([]byte(data), decrypted); err != nil {
		return httpresponse.ErrBadRequest.With(err)
	} else {
		return nil
	}
}

func (m *Manager) listProvidersWithCredentials(ctx context.Context, req schema.ProviderListRequest) (*providerWithCredentialsList, error) {
	result := providerWithCredentialsList{ProviderListRequest: req}
	if err := m.PoolConn.List(ctx, &result, result); err != nil {
		return nil, pg.NormalizeError(err)
	}

	result.OffsetLimit = req.OffsetLimit
	result.OffsetLimit.Clamp(result.Count)

	return types.Ptr(result), nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS - PROVIDER WITH CREDENTIALS

type providerWithCredentials struct {
	Provider    schema.Provider
	PV          uint64
	Credentials []byte
}

type providerWithCredentialsList struct {
	schema.ProviderListRequest
	Count uint64
	Body  []providerWithCredentials
}

func (p *providerWithCredentials) Scan(row pg.Row) error {
	var enabled bool
	var url string
	if err := row.Scan(
		&p.Provider.Name,
		&p.Provider.Provider,
		&url,
		&enabled,
		&p.Provider.CreatedAt,
		&p.Provider.ModifiedAt,
		&p.Provider.Meta,
		&p.PV,
		&p.Credentials,
	); err != nil {
		return err
	}
	p.Provider.Enabled = types.Ptr(enabled)
	if url != "" {
		p.Provider.URL = types.Ptr(url)
	} else {
		p.Provider.URL = nil
	}
	if p.Provider.Meta == nil {
		p.Provider.Meta = make(schema.ProviderMetaMap)
	}
	return nil
}

func (p providerWithCredentialsList) Providers() []*schema.Provider {
	result := make([]*schema.Provider, 0, len(p.Body))
	for i := range p.Body {
		result = append(result, &p.Body[i].Provider)
	}
	return result
}

func (list *providerWithCredentialsList) Scan(row pg.Row) error {
	var provider providerWithCredentials
	if err := provider.Scan(row); err != nil {
		return err
	}
	list.Body = append(list.Body, provider)
	return nil
}

func (list *providerWithCredentialsList) ScanCount(row pg.Row) error {
	if err := row.Scan(&list.Count); err != nil {
		return err
	}
	return nil
}

func (list providerWithCredentialsList) Select(bind *pg.Bind, op pg.Op) (string, error) {
	if _, err := list.ProviderListRequest.Select(bind, op); err != nil {
		return "", err
	}

	switch op {
	case pg.List:
		return bind.Query("provider.list_with_credentials"), nil
	default:
		return "", schema.ErrNotImplemented.Withf("unsupported providerWithCredentialsList operation %q", op)
	}
}
