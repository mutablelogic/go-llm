package manager

import (
	"context"
	"errors"
	"maps"
	"slices"
	"strings"
	"sync"

	// Packages
	auth "github.com/djthorpe/go-auth/schema/auth"
	uuid "github.com/google/uuid"
	otel "github.com/mutablelogic/go-client/pkg/otel"
	llm "github.com/mutablelogic/go-llm"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	pg "github.com/mutablelogic/go-pg"
	types "github.com/mutablelogic/go-server/pkg/types"
	attribute "go.opentelemetry.io/otel/attribute"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type conversationTurn struct {
	Reply      *schema.Message
	Messages   schema.Conversation
	Overhead   uint
	Usage      *schema.UsageMeta
	UsageEntry *schema.UsageInsert
}

type namedTool struct {
	llm.Tool
	name string
}

type toolMap map[string]llm.Tool

const toolSelectionPageSize uint64 = 100

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

	// Load the current session state.
	session, err := m.GetSession(ctx, req.Session, user)
	if err != nil {
		return nil, err
	}

	// Load the persisted conversation history in chronological order.
	conversation, err := m.conversationForSession(ctx, req.Session, user)
	if err != nil {
		return nil, err
	}

	// Fold the per-request system prompt into the session prompt.
	if prompt := strings.TrimSpace(req.SystemPrompt); prompt != "" {
		if system_prompt := types.Value(session.GeneratorMeta.SystemPrompt); system_prompt != "" {
			session.GeneratorMeta.SystemPrompt = types.Ptr(system_prompt + "\n\n" + prompt)
		} else {
			session.GeneratorMeta.SystemPrompt = types.Ptr(prompt)
		}
	}

	// Resolve the model, generator, and provider options for this turn.
	provider, model, generator, opts, err := m.generatorFromMeta(ctx, session.GeneratorMeta, user, generationContextChat)
	if err != nil {
		return nil, err
	}

	// Enable streaming when a callback is provided.
	if fn != nil {
		opts = append(opts, opt.WithStream(fn))
	}

	// Determine the tools we are going to use in this conversation loop and add them to the options.
	tools, err := m.toolsForUser(ctx, user, req.Tools)
	if err != nil {
		return nil, err
	}
	if len(tools) > 0 {
		opts = append(opts, tools.Opts()...)
	}

	// Build the next user turn.
	// TODO: Append the attachments.
	message, err := schema.NewMessage(schema.RoleUser, req.Text)
	if err != nil {
		return nil, err
	}

	// Set up the variables we use to track the conversation loop state
	maxIterations := conversationLoopMaxIterations(req.MaxIterations)
	conversationStart := conversation.Len()
	usageEntries := make([]schema.UsageInsert, 0, maxIterations)
	overhead := uint(0)
	var loopErr error

	// Conversation/agent loop begins here.
	var turn *conversationTurn
	for iteration := range maxIterations {
		loopCtx, endLoopSpan := otel.StartSpan(m.tracer, ctx, "Chat.Iteration",
			attribute.String("session", req.Session.String()),
			attribute.Int("iteration", int(iteration)),
			attribute.Int("max_iterations", int(maxIterations)),
		)
		endLoop := false
		nextMessage := message
		if err := func() (err error) {
			defer func() { endLoopSpan(err) }()

			turn, err = m.executeConversationTurn(loopCtx, req.Session, user, provider, model, generator, types.Value(session.GeneratorMeta.SystemPrompt), &conversation, message, opts...)
			if err != nil {
				return err
			}
			overhead += turn.Overhead
			if turn.UsageEntry != nil {
				usageEntries = append(usageEntries, *turn.UsageEntry)
			}
			if shouldEndConversationLoop(turn.Reply, iteration, maxIterations) {
				endLoop = true
				return nil
			}

			var ok bool
			nextMessage, ok, err = m.nextConversationIteration(loopCtx, turn, tools, fn)
			if err != nil {
				return err
			}
			if !ok {
				endLoop = true
			}
			return nil
		}(); err != nil {
			loopErr = err
			break
		}
		if endLoop {
			break
		}
		message = nextMessage
	}

	if err := m.persistChatLoop(ctx, req.Session, chatMessagesToPersist(conversation, conversationStart, loopErr == nil), usageEntries, overhead); err != nil {
		if loopErr != nil {
			return nil, errors.Join(loopErr, err)
		}
		return nil, err
	}
	if loopErr != nil {
		return nil, loopErr
	}

	// Build the outward response from the final reply.
	response := types.Ptr(schema.ChatResponse{
		CompletionResponse: schema.CompletionResponse{
			Role:    turn.Reply.Role,
			Content: turn.Reply.Content,
			Result:  turn.Reply.Result,
		},
		Usage: turn.Usage,
	})

	// Return the response
	return response, nil
}

func (m *Manager) executeConversationTurn(ctx context.Context, session uuid.UUID, user *auth.User, provider *schema.Provider, model *schema.Model, generator llm.Generator, systemPrompt string, conversation *schema.Conversation, message *schema.Message, opts ...opt.Opt) (*conversationTurn, error) {
	startLen := conversation.Len()
	reply, usage, err := generator.WithSession(ctx, types.Value(model), conversation, message, opts...)
	if err != nil {
		return nil, err
	}
	if conversation.Len() < startLen+2 {
		return nil, schema.ErrInternalServerError.With("generator did not append a user message and reply to the conversation")
	}

	turn := &conversationTurn{
		Reply:    reply,
		Messages: (*conversation)[startLen:],
		Overhead: conversationTurnOverhead(*conversation, reply, usage, systemPrompt),
		Usage:    mergeUsageMeta(ctx, usage, provider.Meta, reply),
	}
	if turn.Usage != nil {
		turn.UsageEntry = &schema.UsageInsert{
			Type:      schema.UsageTypeChat,
			User:      user.UUID(),
			Session:   session,
			Model:     model.Name,
			Provider:  types.Ptr(model.OwnedBy),
			UsageMeta: types.Value(turn.Usage),
		}
	}

	return turn, nil
}

func conversationTurnOverhead(conversation schema.Conversation, reply *schema.Message, usage *schema.UsageMeta, systemPrompt string) uint {
	if usage == nil || usage.InputTokens == 0 {
		return 0
	}

	messageTokens := conversation.Tokens()
	messageTokens += estimateSystemPromptTokens(systemPrompt)
	if reply != nil {
		if reply.Tokens >= messageTokens {
			messageTokens = 0
		} else {
			messageTokens -= reply.Tokens
		}
	}
	if usage.InputTokens <= messageTokens {
		return 0
	}

	return usage.InputTokens - messageTokens
}

func estimateSystemPromptTokens(systemPrompt string) uint {
	if strings.TrimSpace(systemPrompt) == "" {
		return 0
	}

	return schema.Message{
		Role: schema.RoleSystem,
		Content: []schema.ContentBlock{{
			Text: types.Ptr(systemPrompt),
		}},
	}.EstimateTokens()
}

func normalizeToolMapKey(name string) string {
	return strings.ReplaceAll(name, ".", "__")
}

func withToolName(tool llm.Tool, name string) llm.Tool {
	if tool == nil || name == "" {
		return tool
	}

	return &namedTool{Tool: tool, name: name}
}

func (t *namedTool) Name() string {
	if t == nil {
		return ""
	}

	return t.name
}

func conversationLoopMaxIterations(maxIterations uint) uint {
	if maxIterations == 0 {
		return schema.DefaultMaxIterations
	}

	return maxIterations
}

func shouldEndConversationLoop(reply *schema.Message, iteration, maxIterations uint) bool {
	if reply == nil || reply.Result != schema.ResultToolCall {
		return true
	}
	if iteration+1 >= maxIterations {
		reply.Result = schema.ResultMaxIterations
		return true
	}

	return false
}

func (m *Manager) nextConversationIteration(ctx context.Context, turn *conversationTurn, tools toolMap, fn opt.StreamFn) (*schema.Message, bool, error) {
	if turn == nil || turn.Reply == nil {
		return nil, false, schema.ErrInternalServerError.With("missing conversation reply for tool execution")
	}

	calls := turn.Reply.ToolCalls()
	if len(calls) == 0 {
		return nil, false, schema.ErrInternalServerError.With("assistant requested tool calls but did not include any tool call blocks")
	}
	// TODO: Handle the special structured-output tool here before executing normal tool calls.

	content := make([]schema.ContentBlock, len(calls))
	var wg sync.WaitGroup
	for i, call := range calls {
		if fn != nil {
			fn(schema.RoleTool, toolFeedback(tools[call.Name], call))
		}
		wg.Add(1)
		go func(i int, call schema.ToolCall) {
			defer wg.Done()
			content[i] = m.runToolCall(ctx, tools, call, i)
		}(i, call)
	}
	wg.Wait()

	return &schema.Message{
		Role:    schema.RoleUser,
		Content: content,
	}, true, nil
}

func toolFeedback(tool llm.Tool, call schema.ToolCall) string {
	if tool != nil && tool.Description() != "" {
		return call.Name + ": " + tool.Description()
	}

	return call.Name
}

func (m *Manager) runToolCall(ctx context.Context, tools toolMap, call schema.ToolCall, index int) (result schema.ContentBlock) {
	var err error
	ctx, endSpan := otel.StartSpan(m.tracer, ctx, "Chat.ToolCall",
		attribute.String("call", types.Stringify(call)),
	)
	defer func() { endSpan(err) }()

	tool, ok := tools[call.Name]
	if !ok || tool == nil {
		err = schema.ErrNotFound.Withf("tool %q", call.Name)
		return schema.NewToolError(call.ID, call.Name, err)
	}

	output, err := tool.Run(ctx, call.Input)
	if err != nil {
		return schema.NewToolError(call.ID, call.Name, err)
	}

	return schema.NewToolResult(call.ID, call.Name, output)
}

func chatMessagesToPersist(conversation schema.Conversation, start int, persist bool) schema.Conversation {
	if !persist || start >= len(conversation) {
		return nil
	}
	return conversation[start:]
}

func (m *Manager) persistChatLoop(ctx context.Context, session uuid.UUID, messages schema.Conversation, usageEntries []schema.UsageInsert, overhead uint) error {
	if len(messages) == 0 && len(usageEntries) == 0 && overhead == 0 {
		return nil
	}

	return m.PoolConn.Tx(ctx, func(conn pg.Conn) error {
		for _, message := range messages {
			if message == nil {
				continue
			}
			if err := conn.Insert(ctx, nil, schema.MessageInsert{Session: session, Message: types.Value(message)}); err != nil {
				return pg.NormalizeError(err)
			}
		}
		for _, usageEntry := range usageEntries {
			if err := conn.Insert(ctx, nil, usageEntry); err != nil {
				return pg.NormalizeError(err)
			}
		}
		if overhead > 0 {
			if err := conn.Update(ctx, nil, schema.SessionOverheadSelector(session), schema.SessionOverheadUpdate{Increment: overhead}); err != nil {
				return pg.NormalizeError(err)
			}
		}

		return nil
	})
}

func (m *Manager) conversationForSession(ctx context.Context, session uuid.UUID, user *auth.User) (schema.Conversation, error) {
	conn := m.PoolConn.With("session", session, "user", user.UUID())
	req := schema.MessageListRequest{}
	conversation := make(schema.Conversation, 0)
	for {
		var page schema.MessageList
		if err := conn.List(ctx, &page, req); err != nil {
			return nil, pg.NormalizeError(err)
		}
		if len(conversation) == 0 && page.Count > 0 {
			conversation = make(schema.Conversation, 0, page.Count)
		}
		for _, message := range page.Body {
			if message == nil {
				continue
			}
			conversation.Append(types.Value(message))
		}
		if len(page.Body) == 0 {
			break
		} else {
			req.Offset += uint64(len(page.Body))
		}
	}
	return conversation, nil
}

func (m *Manager) toolsForUser(ctx context.Context, user *auth.User, tools []string) (toolMap, error) {
	limit := toolSelectionPageSize
	req := schema.ToolListRequest{
		OffsetLimit: pg.OffsetLimit{Limit: &limit},
		Name:        tools,
	}
	result := make(toolMap)
	for {
		page, _, err := m.listTools(ctx, req, user)
		if err != nil {
			return nil, err
		}
		for _, tool := range page {
			// Normalize the tool name and add to the map
			name := tool.Name()
			if name == "" {
				continue
			} else {
				name = normalizeToolMapKey(name)
			}
			if _, exists := result[name]; exists {
				return nil, schema.ErrConflict.Withf("duplicate tool name after normalization: %q", name)
			} else {
				result[name] = withToolName(tool, name)
			}
		}
		if len(page) == 0 {
			break
		} else {
			req.Offset += uint64(len(page))
		}
	}
	return result, nil
}

func (m toolMap) Opts() []opt.Opt {
	tools := slices.Collect(maps.Values(m))
	if len(tools) == 0 {
		return nil
	}
	return []opt.Opt{opt.WithTool(tools...)}
}
