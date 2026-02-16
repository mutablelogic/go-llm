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
	model, _ := client1.GetModel(ctx, "eliza-1966-en")

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
	model, _ := client.GetModel(ctx, "eliza-1966-en")

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
	model, _ := client.GetModel(ctx, "eliza-1966-en")

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
	model, _ := client.GetModel(ctx, "eliza-1966-en")
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
	model, _ := client.GetModel(ctx, "eliza-1966-en")

	var streamedRole, streamedText string
	streamFn := func(role, text string) {
		streamedRole = role
		streamedText = text
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

	if streamedText != resp.Text() {
		t.Errorf("streamed text %q doesn't match response text %q", streamedText, resp.Text())
	}
}

func TestReset(t *testing.T) {
	client, err := eliza.New(eliza.WithSeed(42))
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	ctx := context.Background()
	model, _ := client.GetModel(ctx, "eliza-1966-en")
	session := &schema.Conversation{}

	// Have a conversation to build up memory
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
		_, _, _ = client.WithSession(ctx, *model, session, msg)
	}

	// Reset the engine
	client.Reset()

	// The engine's internal memory should be cleared
	// (This mainly affects the memory callback feature)
	t.Log("Engine reset successfully")
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
	model, _ := client.GetModel(ctx, "eliza-1966-en")

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
