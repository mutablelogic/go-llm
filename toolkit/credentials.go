package toolkit

import (
	// Packages
	credential "github.com/mutablelogic/go-auth/credential"
)

///////////////////////////////////////////////////////////////////////////////
// INTERFACES

// CredentialsProvider expands credentials, such as a tool's API key, whose
// placeholders, such as ${OPENAI_KEY}, are replaced with the credentials
// they name. It's go-auth's CredentialProvider, so one provider is
// shared by the registry, the toolkit and the connectors.
type CredentialsProvider = credential.CredentialProvider
