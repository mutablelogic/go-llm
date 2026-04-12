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
	"log/slog"
	"slices"
	"sync"
	"time"

	// Packages
	schema "github.com/mutablelogic/go-llm/kernel/schema"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	pg "github.com/mutablelogic/go-pg"
	broadcaster "github.com/mutablelogic/go-pg/pkg/broadcaster"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Run initializes runtime resources and tears them down when the context ends.
func (m *Manager) Run(ctx context.Context, logger *slog.Logger) error {
	ticker := time.NewTimer(time.Second)
	defer ticker.Stop()

	// Close the broadcaster when the manager is stopped
	defer m.broadcaster.Close()

	// Create the toolkit and add any tools, prompts, and resources that were
	// added in the options
	toolkitOpts := []toolkit.Option{
		toolkit.WithTracer(m.tracer),
		toolkit.WithDelegate(m.delegate),
		toolkit.WithLogger(logger),
	}
	toolkitOpts = append(toolkitOpts, toolkit.WithTool(m.tools...))
	toolkitOpts = append(toolkitOpts, toolkit.WithPrompt(m.prompts...))
	toolkitOpts = append(toolkitOpts, toolkit.WithResource(m.resources...))
	if tookit, err := toolkit.New(toolkitOpts...); err != nil {
		return fmt.Errorf("create toolkit: %w", err)
	} else {
		m.Toolkit = tookit
	}

	// Add runtime-local connectors to the toolkit.
	if len(m.connectors) > 0 {
		names := make([]string, 0, len(m.connectors))
		for name := range m.connectors {
			names = append(names, name)
		}
		slices.Sort(names)

		var result error
		for _, name := range names {
			result = errors.Join(result, m.Toolkit.AddLocalConnector(name))
		}
		if result != nil {
			return fmt.Errorf("add local connectors: %w", result)
		}
	}

	// Sync connectors
	if err := m.syncConnectors(ctx); err != nil {
		return fmt.Errorf("sync connectors: %w", err)
	}

	// Subscribe to database notifications, if configured
	// We provide a small buffered channel to avoid blocking the database listener
	providerChange, connectorChange, messageChange :=
		make(chan broadcaster.ChangeNotification, 16),
		make(chan broadcaster.ChangeNotification, 16),
		make(chan broadcaster.ChangeNotification, 16)
	if m.broadcaster != nil {
		if err := m.broadcaster.Subscribe(ctx, func(change broadcaster.ChangeNotification) {
			switch {
			case change.Matches(m.llmschema, "provider", ""):
				select {
				case providerChange <- change:
				case <-ctx.Done():
				}
			case change.Matches(m.llmschema, "connector", ""):
				select {
				case connectorChange <- change:
				case <-ctx.Done():
				}
			case change.Matches(m.llmschema, "message", "INSERT"):
				select {
				case messageChange <- change:
				case <-ctx.Done():
				}
			default:
				logger.DebugContext(ctx, "Changes", "schema", change.Schema, "table", change.Table, "action", change.Action)
			}
		}); err != nil {
			return err
		}
	}

	// Run the toolkit in the background to process any connections and disconnections
	var wg sync.WaitGroup
	toolkit_ctx, toolkit_cancel := context.WithCancel(context.Background())
	defer func() {
		toolkit_cancel()
		wg.Wait()
	}()
	wg.Go(func() {
		if err := m.Toolkit.Run(toolkit_ctx); err != nil {
			logger.ErrorContext(ctx, "toolkit run error", "error", err.Error())
		}
	})

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
		case <-connectorChange:
			if err := m.syncConnectors(ctx); err != nil {
				logger.ErrorContext(ctx, "failed to sync connectors after change notification", "error", err.Error())
			}
		case <-messageChange:
			if err := m.sessionfeed.update(ctx); err != nil {
				logger.ErrorContext(ctx, "failed to update session feed after message change notification", "error", err.Error())
			}
		case <-ticker.C:
			// Ping the registry to determine status of providers
			if err := m.Registry.Ping(ctx); err != nil {
				logger.ErrorContext(ctx, "failed to ping providers", "error", err.Error())
			}

			// Reset ticker for next event
			ticker.Reset(time.Minute)
		}
	}
}

// SyncProviders refreshes the in-memory provider registry from the database.
func (m *Manager) SyncProviders(ctx context.Context) ([]string, []string, error) {
	return m.syncProviders(ctx)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// Sync providers with the registry and log any changes. This is called in response to database
// notifications and periodically to ensure the registry is up to date with the database.
// Return the names of any providers that were updated or deleted in the registry as a result of the sync.
func (m *Manager) syncProviders(ctx context.Context) ([]string, []string, error) {
	var providersWithCredentials []providerWithCredentials
	var providers []*schema.Provider

	// Iterate over all providers, and also retrieve their encrypted credentials and PV for
	// decryption. We do this in batches.
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

func (m *Manager) syncConnectors(ctx context.Context) error {
	// Iterate over all connectors
	var offset uint64
	var connectors []*schema.Connector
	for {
		result, err := m.ListConnectors(ctx, schema.ConnectorListRequest{
			OffsetLimit: pg.OffsetLimit{
				Offset: offset,
			},
		}, nil)
		if err != nil {
			return err
		}
		// Allocate the slice on the first iteration to avoid unnecessary allocations
		// if there are many connectors
		if connectors == nil {
			connectors = make([]*schema.Connector, 0, result.Count)
		}

		// If no results, we are done, otherwise add to the list and continue
		if len(result.Body) == 0 {
			break
		} else {
			connectors = append(connectors, result.Body...)
			offset += uint64(len(result.Body))
		}
	}

	// Syncronize the registry with the list of connectors
	var result error
	for _, connector := range connectors {
		enabled := types.Value(connector.Enabled)
		exists := m.Toolkit.ExistsConnector(connector.URL)

		switch {
		case enabled && !exists:
			result = errors.Join(result, m.Toolkit.AddConnectorNS(types.Value(connector.Namespace), connector.URL))
		case !enabled && exists:
			result = errors.Join(result, m.Toolkit.RemoveConnector(connector.URL))
		}
	}

	// Return any errors
	return result
}
