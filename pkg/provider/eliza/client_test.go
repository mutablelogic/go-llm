package eliza_test

import (
	"context"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	eliza "github.com/mutablelogic/go-llm/pkg/provider/eliza"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
)

func TestNew(t *testing.T) {
	client, err := eliza.New()
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	if client == nil {
		t.Fatal("client is nil")
	}

	// Check provider name
	if name := client.Name(); name != "eliza" {
		t.Errorf("expected provider name 'eliza', got %q", name)
	}
}

func TestNewWithSeed(t *testing.T) {
	// Create two clients with the same seed
	client1, err := eliza.New(eliza.WithSeed(42))
	if err != nil {
		t.Fatalf("failed to create client1: %v", err)
	}

	client2, err := eliza.New(eliza.WithSeed(42))
	if err != nil {
		t.Fatalf("failed to create client2: %v", err)
	}

	// They should produce the same response for the same input
	ctx := context.Background()
	model, err := client1.GetModel(ctx, "eliza-1966-en")
	if err != nil {
		t.Fatalf("failed to get model: %v", err)
	}

	msg := &schema.Message{
		Role:    schema.RoleUser,
		Content: []schema.ContentBlock{{Text: ptr("Hello")}},
	}

	resp1, _, _ := client1.WithoutSession(ctx, *model, msg)
	resp2, _, _ := client2.WithoutSession(ctx, *model, msg)

	if resp1.Text() != resp2.Text() {
		t.Errorf("expected same response with same seed, got %q and %q", resp1.Text(), resp2.Text())
	}
}

func TestListModels(t *testing.T) {
	client, err := eliza.New()
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	ctx := context.Background()
	models, err := client.ListModels(ctx)
	if err != nil {
		t.Fatalf("failed to list models: %v", err)
	}

	if len(models) == 0 {
		t.Fatal("expected at least one model")
	}

	// Check that all models are owned by eliza
	found := make(map[string]bool)
	for _, m := range models {
		if m.OwnedBy != "eliza" {
			t.Errorf("expected owned_by 'eliza', got %q for model %q", m.OwnedBy, m.Name)
		}
		found[m.Name] = true
	}

	// Check that English is present
	if !found["eliza-1966-en"] {
		t.Error("expected model 'eliza-1966-en' to be present")
	}

	t.Logf("Listed %d models: %v", len(models), found)
}

func TestGetModel(t *testing.T) {
	client, err := eliza.New()
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	ctx := context.Background()

	// Test valid model name
	model, err := client.GetModel(ctx, "eliza-1966-en")
	if err != nil {
		t.Fatalf("failed to get model: %v", err)
	}
	if model == nil {
		t.Fatal("model is nil")
	}
	if model.Name != "eliza-1966-en" {
		t.Errorf("expected model name 'eliza-1966-en', got %q", model.Name)
	}

	// Test provider name as alias
	model, err = client.GetModel(ctx, "eliza")
	if err != nil {
		t.Fatalf("failed to get model by provider name: %v", err)
	}
	if model == nil {
		t.Fatal("model is nil")
	}

	// Test invalid model name
	_, err = client.GetModel(ctx, "gpt-4")
	if err == nil {
		t.Error("expected error for invalid model name")
	}
}

func TestWithoutSession(t *testing.T) {
	client, err := eliza.New(eliza.WithSeed(42))
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	ctx := context.Background()
	model, err := client.GetModel(ctx, "eliza-1966-en")
	if err != nil {
		t.Fatalf("failed to get model: %v", err)
	}

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "greeting",
			input:   "Hello",
			wantErr: false,
		},
		{
			name:    "feeling",
			input:   "I feel sad",
			wantErr: false,
		},
		{
			name:    "family",
			input:   "My mother is annoying",
			wantErr: false,
		},
		{
			name:    "question",
			input:   "Are you a computer?",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := &schema.Message{
				Role:    schema.RoleUser,
				Content: []schema.ContentBlock{{Text: ptr(tt.input)}},
			}

			resp, usage, err := client.WithoutSession(ctx, *model, msg)
			if (err != nil) != tt.wantErr {
				t.Errorf("WithoutSession() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if resp == nil {
				t.Fatal("response is nil")
			}

			if resp.Role != schema.RoleAssistant {
				t.Errorf("expected role 'assistant', got %q", resp.Role)
			}

			if resp.Text() == "" {
				t.Error("response text is empty")
			}

			if resp.Result != schema.ResultStop {
				t.Errorf("expected result 'stop', got %q", resp.Result)
			}

			if usage == nil {
				t.Fatal("usage is nil")
			}

			if usage.InputTokens == 0 {
				t.Error("input tokens should not be zero")
			}

			if usage.OutputTokens == 0 {
				t.Error("output tokens should not be zero")
			}

			t.Logf("Input: %q", tt.input)
			t.Logf("Response: %q", resp.Text())
			t.Logf("Usage: input=%d, output=%d", usage.InputTokens, usage.OutputTokens)
		})
	}
}

func TestWithSession(t *testing.T) {
	client, err := eliza.New(eliza.WithSeed(42))
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	ctx := context.Background()
	model, err := client.GetModel(ctx, "eliza-1966-en")
	if err != nil {
		t.Fatalf("failed to get model: %v", err)
	}

	// Create a conversation
	session := &schema.Conversation{}

	messages := []string{
		"Hello",
		"I feel anxious about my work",
		"My boss is always criticizing me",
		"I want to be more confident",
	}

	for _, input := range messages {
		msg := &schema.Message{
			Role:    schema.RoleUser,
			Content: []schema.ContentBlock{{Text: ptr(input)}},
		}

		resp, usage, err := client.WithSession(ctx, *model, session, msg)
		if err != nil {
			t.Fatalf("WithSession() error = %v", err)
		}

		if resp.Text() == "" {
			t.Error("response text is empty")
		}

		t.Logf("User: %q", input)
		t.Logf("ELIZA: %q", resp.Text())
		t.Logf("Usage: input=%d, output=%d", usage.InputTokens, usage.OutputTokens)
	}

	// Check that the session has accumulated messages
	expectedLen := len(messages) * 2 // user + assistant pairs
	if len(*session) != expectedLen {
		t.Errorf("expected session length %d, got %d", expectedLen, len(*session))
	}

	// Check total tokens
	totalTokens := session.Tokens()
	if totalTokens == 0 {
		t.Error("total tokens should not be zero")
	}
	t.Logf("Total session tokens: %d", totalTokens)
}

func TestWithSessionGoodbye(t *testing.T) {
	client, err := eliza.New(eliza.WithSeed(42))
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	ctx := context.Background()
	model, err := client.GetModel(ctx, "eliza-1966-en")
	if err != nil {
		t.Fatalf("failed to get model: %v", err)
	}
	session := &schema.Conversation{}

	msg := &schema.Message{
		Role:    schema.RoleUser,
		Content: []schema.ContentBlock{{Text: ptr("goodbye")}},
	}

	resp, _, err := client.WithSession(ctx, *model, session, msg)
	if err != nil {
		t.Fatalf("WithSession() error = %v", err)
	}

	t.Logf("Response to 'goodbye': %q", resp.Text())

	// Response should contain goodbye-like content
	text := resp.Text()
	if text == "" {
		t.Error("expected non-empty goodbye response")
	}
}

func TestStreaming(t *testing.T) {
	client, err := eliza.New(eliza.WithSeed(42))
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	ctx := context.Background()
	model, err := client.GetModel(ctx, "eliza-1966-en")
	if err != nil {
		t.Fatalf("failed to get model: %v", err)
	}

	var streamedRole string
	var chunks []string
	streamFn := func(role, text string) {
		streamedRole = role
		chunks = append(chunks, text)
	}

	msg := &schema.Message{
		Role:    schema.RoleUser,
		Content: []schema.ContentBlock{{Text: ptr("I need help")}},
	}

	resp, _, err := client.WithoutSession(ctx, *model, msg, opt.WithStream(streamFn))
	if err != nil {
		t.Fatalf("WithoutSession() error = %v", err)
	}

	if streamedRole != schema.RoleAssistant {
		t.Errorf("expected streamed role 'assistant', got %q", streamedRole)
	}

	if len(chunks) == 0 {
		t.Error("expected at least one streamed chunk")
	}

	// Reassemble chunks and compare to full response
	reassembled := ""
	for _, chunk := range chunks {
		reassembled += chunk
	}
	if reassembled != resp.Text() {
		t.Errorf("reassembled streamed text %q doesn't match response text %q", reassembled, resp.Text())
	}

	t.Logf("Streamed %d chunks: %v", len(chunks), chunks)
}

func TestMemoryInferredFromConversation(t *testing.T) {
	client, err := eliza.New(eliza.WithSeed(42))
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	ctx := context.Background()
	model, err := client.GetModel(ctx, "eliza-1966-en")
	if err != nil {
		t.Fatalf("failed to get model: %v", err)
	}
	session := &schema.Conversation{}

	// Have a conversation to build up memory via memorable rules
	messages := []string{
		"I want to be happy",
		"I need love",
		"I feel lonely",
	}

	for _, input := range messages {
		msg := &schema.Message{
			Role:    schema.RoleUser,
			Content: []schema.ContentBlock{{Text: ptr(input)}},
		}
		resp, _, err := client.WithSession(ctx, *model, session, msg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		t.Logf("User: %s -> ELIZA: %s", input, resp.Text())
	}

	// Memory is derived from conversation, so starting a fresh session
	// should have no memory from the previous one
	freshSession := &schema.Conversation{}
	msg := &schema.Message{
		Role:    schema.RoleUser,
		Content: []schema.ContentBlock{{Text: ptr("hello")}},
	}
	resp, _, err := client.WithSession(ctx, *model, freshSession, msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	t.Logf("Fresh session: User: hello -> ELIZA: %s", resp.Text())
}

func TestThinkingEmitsMemory(t *testing.T) {
	client, err := eliza.New(eliza.WithSeed(42))
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	ctx := context.Background()
	model, err := client.GetModel(ctx, "eliza-1966-en")
	if err != nil {
		t.Fatalf("failed to get model: %v", err)
	}
	session := &schema.Conversation{}

	// Build up memory with memorable patterns (e.g. "i need ...")
	memorableInputs := []string{
		"I need some help",
		"I need to feel better",
	}
	for _, input := range memorableInputs {
		msg := &schema.Message{
			Role:    schema.RoleUser,
			Content: []schema.ContentBlock{{Text: ptr(input)}},
		}
		_, _, err := client.WithSession(ctx, *model, session, msg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	// Now send another message with thinking enabled
	var thinkingChunks []string
	streamFn := func(role, text string) {
		if role == schema.RoleThinking {
			thinkingChunks = append(thinkingChunks, text)
		}
	}

	msg := &schema.Message{
		Role:    schema.RoleUser,
		Content: []schema.ContentBlock{{Text: ptr("I feel sad")}},
	}
	resp, _, err := client.WithSession(ctx, *model, session, msg,
		opt.SetBool(opt.ThinkingKey, true),
		opt.WithStream(streamFn),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify thinking was streamed
	if len(thinkingChunks) == 0 {
		t.Error("expected thinking chunks to be streamed when thinking is enabled")
	} else {
		t.Logf("Thinking output: %s", thinkingChunks[0])
	}

	// Verify the response message contains a thinking content block
	hasThinking := false
	for _, block := range resp.Content {
		if block.Thinking != nil {
			hasThinking = true
			t.Logf("Thinking block: %s", *block.Thinking)
		}
	}
	if !hasThinking {
		t.Error("expected response to contain a thinking content block")
	}

	// Verify text response is still present
	if resp.Text() == "" {
		t.Error("expected non-empty text response")
	}
}

func TestThinkingDisabledNoMemory(t *testing.T) {
	client, err := eliza.New(eliza.WithSeed(42))
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	ctx := context.Background()
	model, err := client.GetModel(ctx, "eliza-1966-en")
	if err != nil {
		t.Fatalf("failed to get model: %v", err)
	}
	session := &schema.Conversation{}

	// Build up memory
	msg := &schema.Message{
		Role:    schema.RoleUser,
		Content: []schema.ContentBlock{{Text: ptr("I need some help")}},
	}
	_, _, err = client.WithSession(ctx, *model, session, msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Send another message WITHOUT thinking enabled
	msg = &schema.Message{
		Role:    schema.RoleUser,
		Content: []schema.ContentBlock{{Text: ptr("I feel sad")}},
	}
	resp, _, err := client.WithSession(ctx, *model, session, msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify no thinking content block in the response
	for _, block := range resp.Content {
		if block.Thinking != nil {
			t.Error("expected no thinking content block when thinking is disabled")
		}
	}
}

func TestInterfaceCompliance(t *testing.T) {
	client, err := eliza.New()
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	// Verify interface compliance at runtime
	var _ llm.Client = client
	var _ llm.Generator = client
}

func TestErrorCases(t *testing.T) {
	client, err := eliza.New()
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	ctx := context.Background()
	model, err := client.GetModel(ctx, "eliza-1966-en")
	if err != nil {
		t.Fatalf("failed to get model: %v", err)
	}

	t.Run("nil message", func(t *testing.T) {
		_, _, err := client.WithoutSession(ctx, *model, nil)
		if err == nil {
			t.Error("expected error for nil message")
		}
	})

	t.Run("empty message", func(t *testing.T) {
		msg := &schema.Message{
			Role:    schema.RoleUser,
			Content: []schema.ContentBlock{},
		}
		_, _, err := client.WithoutSession(ctx, *model, msg)
		if err == nil {
			t.Error("expected error for empty message")
		}
	})

	t.Run("nil session", func(t *testing.T) {
		msg := &schema.Message{
			Role:    schema.RoleUser,
			Content: []schema.ContentBlock{{Text: ptr("Hello")}},
		}
		_, _, err := client.WithSession(ctx, *model, nil, msg)
		if err == nil {
			t.Error("expected error for nil session")
		}
	})
}

// Helper function
func ptr(s string) *string {
	return &s
}
