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
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	trace "go.opentelemetry.io/otel/trace"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Opt configures a Manager during construction.
type Opt func(*opt) error

// opt combines all configuration options for Manager.
type opt struct {
	llmschema   string
	authschema  string
	channel     string
	tracer      trace.Tracer
	passphrases *crypto.Passphrases
	clientopts  []client.ClientOpt
}

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func (o *opt) apply(opts ...Opt) error {
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

func (o *opt) defaults() {
	o.llmschema = schema.DefaultSchema
	o.authschema = schema.DefaultAuthSchema
	o.channel = schema.DefaultNotifyChannel
	o.passphrases = crypto.NewPassphrases()
	o.clientopts = []client.ClientOpt{}
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// WithSchemas sets the database schema names to use for all queries. If not set the default schemas are used.
func WithSchemas(llm, auth string) Opt {
	return func(o *opt) error {
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
	return func(o *opt) error {
		if tracer == nil {
			return fmt.Errorf("tracer is required")
		}
		o.tracer = tracer
		return nil
	}
}

// WithPassphrase registers an in-memory storage passphrase for a certificate
// passphrase version. Versions are uint64 and passphrases must be non-empty.
func WithPassphrase(version uint64, passphrase string) Opt {
	return func(o *opt) error {
		return o.passphrases.Set(version, passphrase)
	}
}

// WithNotificationChannel sets the PostgreSQL LISTEN/NOTIFY channel used by
// the provider table change trigger created during bootstrap.
func WithNotificationChannel(name string) Opt {
	return func(o *opt) error {
		if name == "" {
			return fmt.Errorf("notification channel cannot be empty")
		}
		o.channel = name
		return nil
	}
}

// WithClientOpts provides unified client options for the LLM model
// providers
func WithClientOpts(opts ...client.ClientOpt) Opt {
	return func(o *opt) error {
		o.clientopts = append(o.clientopts, opts...)
		return nil
	}
}
