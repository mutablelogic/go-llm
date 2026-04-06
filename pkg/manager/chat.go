package manager

import (
	"context"
	"strings"

	// Packages
	auth "github.com/djthorpe/go-auth/schema/auth"
	otel "github.com/mutablelogic/go-client/pkg/otel"
	llm "github.com/mutablelogic/go-llm"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	pg "github.com/mutablelogic/go-pg"
	types "github.com/mutablelogic/go-server/pkg/types"
	attribute "go.opentelemetry.io/otel/attribute"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Chat processes a message within a session context (stateful).
// If fn is non-nil, text chunks are streamed to the callback as they arrive.
func (m *Manager) Chat(ctx context.Context, req schema.ChatRequest, fn opt.StreamFn, user *auth.User, attachments ...llm.Resource) (_ *schema.ChatResponse, err error) {
	// Otel span
	ctx, endSpan := otel.StartSpan(m.tracer, ctx, "Chat",
		attribute.String("req", types.Stringify(req)),
		attribute.String("user", types.Stringify(user)),
	)
	defer func() { endSpan(err) }()

	// Retrieve the session
	session, err := m.GetSession(ctx, req.Session, user)
	if err != nil {
		return nil, err
	}

	// Add system prompt from session meta to the per-request system prompt
	if prompt := strings.TrimSpace(req.SystemPrompt); prompt != "" {
		if system_prompt := types.Value(session.GeneratorMeta.SystemPrompt); system_prompt != "" {
			session.GeneratorMeta.SystemPrompt = types.Ptr(system_prompt + "\n\n" + prompt)
		} else {
			session.GeneratorMeta.SystemPrompt = types.Ptr(prompt)
		}
	}

	// Resolve the model, generator, and options from the session meta
	provider, model, generator, opts, err := m.generatorFromMeta(ctx, session.GeneratorMeta, user, generationContextChat)
	if err != nil {
		return nil, err
	}

	// Enable streaming when a callback is provided
	if fn != nil {
		opts = append(opts, opt.WithStream(fn))
	}

	// The message is the text, plus any attachments as content blocks
	// TODO: Append the attachments
	message, err := schema.NewMessage(schema.RoleUser, req.Text)
	if err != nil {
		return nil, err
	}

	// Run generation against an in-memory conversation so providers can attribute
	// per-message token counts before we persist the resulting messages.
	conversation := new(schema.Conversation)

	// Generate a response from the message
	reply, usage, err := generator.WithSession(ctx, types.Value(model), conversation, message, opts...)
	if err != nil {
		return nil, err
	} else if conversation.Len() < 2 {
		return nil, schema.ErrInternalServerError.With("generator did not return a conversation with at least two messages")
	}

	// Create the response object
	response := types.Ptr(schema.ChatResponse{
		CompletionResponse: schema.CompletionResponse{
			Role:    reply.Role,
			Content: reply.Content,
			Result:  reply.Result,
		},
		Usage: usage,
	})

	// Perform a transaction to insert the new messages and usage together
	if err := m.PoolConn.Tx(ctx, func(conn pg.Conn) error {
		// Insert the user message and the assistant reply as messages in the session conversation.
		if err := conn.Insert(ctx, nil, schema.MessageInsert{Session: req.Session, Message: types.Value(conversation.Last(1))}); err != nil {
			return pg.NormalizeError(err)
		}
		if err := conn.Insert(ctx, nil, schema.MessageInsert{Session: req.Session, Message: types.Value(conversation.Last(0))}); err != nil {
			return pg.NormalizeError(err)
		}

		// Fold provider metadata into the usage metadata and include the
		// current trace_id for downstream observability.
		response.Usage = mergeUsageMeta(ctx, response.Usage, provider.Meta, reply)
		if response.Usage != nil {
			var usage schema.Usage
			if err := conn.Insert(ctx, &usage, schema.UsageInsert{
				Type:      schema.UsageTypeChat,
				User:      user.UUID(),
				Session:   req.Session,
				Model:     model.Name,
				Provider:  types.Ptr(model.OwnedBy),
				UsageMeta: types.Value(response.Usage),
			}); err != nil {
				return pg.NormalizeError(err)
			}
		}

		return nil
	}); err != nil {
		return nil, err
	}

	// Return the response
	return response, nil
}
