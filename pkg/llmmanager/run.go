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
	"log/slog"
	"sync"
	"time"

	// Packages
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	pg "github.com/mutablelogic/go-pg"
	broadcaster "github.com/mutablelogic/go-pg/pkg/broadcaster"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Run periodically prunes stale sessions until the context is cancelled.
func (m *Manager) Run(ctx context.Context, logger *slog.Logger) error {
	var sync sync.Once
	ticker := time.NewTimer(time.Second)
	defer ticker.Stop()

	// Subscribe to database notifications, if configured
	// We provide a small buffered channel to avoid blocking the database listener
	providerChange := make(chan broadcaster.ChangeNotification, 16)
	defer close(providerChange)
	if m.Broadcaster != nil {
		if err := m.Broadcaster.Subscribe(ctx, func(change broadcaster.ChangeNotification) {
			logger.DebugContext(ctx, "Changes", "schema", change.Schema, "table", change.Table, "action", change.Action)
			if changeMatches(change, m.opt.llmschema, "provider", "") {
				providerChange <- change
			}
		}); err != nil {
			return err
		}
	}

	// Run loop
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-providerChange:
			updates, deletes, err := m.syncProviders(ctx)
			if err != nil {
				logger.ErrorContext(ctx, "failed to sync providers after change notification", "error", err.Error())
			}
			if len(updates) > 0 {
				logger.InfoContext(ctx, "updated providers", "providers", updates)
			}
			if len(deletes) > 0 {
				logger.InfoContext(ctx, "deleted providers", "providers", deletes)
			}
		case <-ticker.C:
			// Sync providers on first ticker event
			sync.Do(func() {
				providerChange <- broadcaster.ChangeNotification{}
			})

			// Ping the registry to determine status of providers
			if err := m.Registry.Ping(ctx); err != nil {
				logger.ErrorContext(ctx, "failed to ping providers", "error", err.Error())
			}

			// Reset ticker for next event
			ticker.Reset(time.Minute)
		}
	}
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func changeMatches(change broadcaster.ChangeNotification, schema, table, op string) bool {
	// Check schema
	if schema != "" && change.Schema != schema {
		return false
	}

	// Check table
	if table != "" && change.Table != table {
		return false
	}

	// Check operation
	if op != "" && change.Action != op {
		return false
	}

	return true
}

func (m *Manager) syncProviders(ctx context.Context) ([]string, []string, error) {
	var providersWithCredentials []providerWithCredentials
	var providers []*schema.Provider

	// Iterate over all providers with credentials
	var offset uint64
	for {
		result, err := m.listProvidersWithCredentials(ctx, schema.ProviderListRequest{
			OffsetLimit: pg.OffsetLimit{
				Offset: offset,
			},
		})
		if err != nil {
			return nil, nil, err
		}
		if len(result.Body) == 0 {
			break
		} else {
			providersWithCredentials = append(providersWithCredentials, result.Body...)
			providers = append(providers, result.Providers()...)
			offset += uint64(len(result.Body))
		}
	}

	// Sync the registry with the list of providers and a decrypter function to obtain credentials
	return m.Registry.Sync(providers, func(i int) (schema.ProviderCredentials, error) {
		var credentials schema.ProviderCredentials
		if len(providersWithCredentials[i].Credentials) == 0 {
			return schema.ProviderCredentials{}, nil
		}
		if err := m.decryptCredentials(providersWithCredentials[i].Credentials, providersWithCredentials[i].PV, &credentials); err != nil {
			return schema.ProviderCredentials{}, err
		} else {
			return credentials, nil
		}
	})
}
