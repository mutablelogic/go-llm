package registry

import (
	// Packages
	credentials "github.com/mutablelogic/go-llm/credentials"
)

///////////////////////////////////////////////////////////////////////////////
// INTERFACES

// CredentialsProvider expands credentials, such as a provider's API key, whose
// placeholders, such as ${OPENAI_KEY}, are replaced with the credentials
// they name. It's the credentials package's Provider, so one provider is
// shared by the registry, the toolkit and the connectors.
type CredentialsProvider = credentials.Provider
