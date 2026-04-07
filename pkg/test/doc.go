// Package test provides shared integration-test helpers for go-llm packages.
//
// The harness in this package creates a disposable PostgreSQL container,
// bootstraps the minimal auth schema required by llmmanager, resolves
// provider-specific environment variables, exposes a go-pg/pkg/test-style
// Main/Conn lifecycle, and provides common helpers for runtime loops,
// bounded contexts, synthetic auth users, and provider/model setup.
package test
