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
	"fmt"

	// Packages
	crypto "github.com/djthorpe/go-auth/pkg/crypto"
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/kernel/schema"
	metric "go.opentelemetry.io/otel/metric"
	trace "go.opentelemetry.io/otel/trace"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Opt configures a Manager during construction.
type Opt func(*manageropt) error

// manageropt combines all configuration options for Manager.
type manageropt struct {
	name        string
	version     string
	llmschema   string
	authschema  string
	channel     string
	tracer      trace.Tracer
	metrics     metric.Meter
	passphrases *crypto.Passphrases
	clientopts  []client.ClientOpt
	tools       []llm.Tool
	prompts     []llm.Prompt
	resources   []llm.Resource
	connectors  map[string]llm.Connector
}

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func (o *manageropt) apply(opts ...Opt) error {
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(o); err != nil {
			return err
		}
	}
	return nil
}

func (o *manageropt) defaults(name, version string) {
	o.name = name
	o.version = version
	o.llmschema = schema.DefaultSchema
	o.authschema = schema.DefaultAuthSchema
	o.channel = schema.DefaultNotifyChannel
	o.passphrases = crypto.NewPassphrases()
	o.clientopts = []client.ClientOpt{}
	o.connectors = make(map[string]llm.Connector)
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// WithSchemas sets the database schema names to use for all queries. If not set the default schemas are used.
func WithSchemas(llm, auth string) Opt {
	return func(o *manageropt) error {
		if llm != "" {
			o.llmschema = llm
		}
		if auth != "" {
			o.authschema = auth
		}
		return nil
	}
}

// WithTracer sets the OpenTelemetry tracer used for manager spans.
func WithTracer(tracer trace.Tracer) Opt {
	return func(o *manageropt) error {
		o.tracer = tracer
		return nil
	}
}

// WithMeter sets the OpenTelemetry meter used for manager metrics.
func WithMeter(meter metric.Meter) Opt {
	return func(o *manageropt) error {
		o.metrics = meter
		return nil
	}
}

// WithPassphrase registers an in-memory storage passphrase for a certificate
// passphrase version. Versions are uint64 and passphrases must be non-empty.
func WithPassphrase(version uint64, passphrase string) Opt {
	return func(o *manageropt) error {
		return o.passphrases.Set(version, passphrase)
	}
}

// WithNotificationChannel sets the PostgreSQL LISTEN/NOTIFY channel used by
// the provider table change trigger created during bootstrap.
func WithNotificationChannel(name string) Opt {
	return func(o *manageropt) error {
		if name == "" {
			return fmt.Errorf("notification channel cannot be empty")
		}
		o.channel = name
		return nil
	}
}

// WithClientOpts provides unified client options for the LLM model
// providers and connectors
func WithClientOpts(opts ...client.ClientOpt) Opt {
	return func(o *manageropt) error {
		o.clientopts = append(o.clientopts, opts...)
		return nil
	}
}

// WithTools provides unified tool options for the LLM model
// providers
func WithTools(opts ...llm.Tool) Opt {
	return func(o *manageropt) error {
		o.tools = append(o.tools, opts...)
		return nil
	}
}

// WithPrompts provides unified prompt options for the LLM model
// providers
func WithPrompts(opts ...llm.Prompt) Opt {
	return func(o *manageropt) error {
		o.prompts = append(o.prompts, opts...)
		return nil
	}
}

// WithResources provides unified resource options for the LLM model
// providers
func WithResources(opts ...llm.Resource) Opt {
	return func(o *manageropt) error {
		o.resources = append(o.resources, opts...)
		return nil
	}
}

// WithConnector adds a local connector to the manager, by URL
func WithConnector(url string, connector llm.Connector) Opt {
	return func(o *manageropt) error {
		if url, err := schema.CanonicalURL(url); err != nil {
			return fmt.Errorf("invalid connector url %q: %w", url, err)
		} else if _, exists := o.connectors[url]; exists {
			return fmt.Errorf("connector url %q already exists", url)
		} else {
			o.connectors[url] = connector
		}
		return nil
	}
}
