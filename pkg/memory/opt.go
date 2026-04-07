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

package memory

import (
	// Packages
	schema "github.com/mutablelogic/go-llm/pkg/schema/memory"
	trace "go.opentelemetry.io/otel/trace"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Opt configures a Manager during construction.
type Opt func(*opt) error

// opt combines all configuration options for the memory manager.
type opt struct {
	llm_schema    string
	auth_schema   string
	memory_schema string
	tracer        trace.Tracer
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
	o.llm_schema = schema.DefaultLLMSchema
	o.auth_schema = schema.DefaultAuthSchema
	o.memory_schema = schema.DefaultMemorySchema
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// WithSchemas sets the database schema names to use for all queries. If not set the default schemas are used.
func WithSchemas(memory, llm, auth string) Opt {
	return func(o *opt) error {
		if memory != "" {
			o.memory_schema = memory
		}
		if llm != "" {
			o.llm_schema = llm
		}
		if auth != "" {
			o.auth_schema = auth
		}
		return nil
	}
}

// WithTracer sets the OpenTelemetry tracer used for manager spans.
func WithTracer(tracer trace.Tracer) Opt {
	return func(o *opt) error {
		o.tracer = tracer
		return nil
	}
}
