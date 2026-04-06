package httphandler

import (
	_ "embed"
	"errors"

	// Packages
	authmanager "github.com/djthorpe/go-auth/pkg/authmanager"
	llmmanager "github.com/mutablelogic/go-llm/pkg/manager"
	httprouter "github.com/mutablelogic/go-server/pkg/httprouter"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// RegisterManagerHandlers registers manager resource handlers with the provided router.
func RegisterHandlers(router *httprouter.Router, manager *llmmanager.Manager, authmanager *authmanager.Manager, auth bool) error {
	// Add tag groups and tags
	router.Spec().AddTagGroup("LLM Management", "Providers", "Models", "Connectors", "Tools & Agents", "Responses", "Sessions")

	// TODO: Register the security scheme

	// Register the security schemes, then the paths
	return errors.Join(
		router.RegisterPath(AgentHandler(manager)),
		router.RegisterPath(AgentResourceHandler(manager)),
		router.RegisterPath(CredentialHandler(manager)),
		router.RegisterPath(ConnectorHandler(manager)),
		router.RegisterPath(ConnectorResourceHandler(manager)),
		router.RegisterPath(ModelHandler(manager)),
		router.RegisterPath(ModelResourceHandler(manager)),
		router.RegisterPath(ModelProviderResourceHandler(manager)),
		router.RegisterPath(ProviderHandler(manager)),
		router.RegisterPath(ProviderResourceHandler(manager)),
		router.RegisterPath(ToolHandler(manager)),
		router.RegisterPath(ToolResourceHandler(manager)),
		router.RegisterPath(EmbeddingHandler(manager)),
		router.RegisterPath(AskHandler(manager)),
		router.RegisterPath(ChatHandler(manager)),
		router.RegisterPath(SessionHandler(manager)),
		router.RegisterPath(SessionResourceHandler(manager)),
	)
}
