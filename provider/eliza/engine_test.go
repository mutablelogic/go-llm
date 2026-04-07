package eliza

import (
	"strings"
	"testing"
)

// testEngine creates an engine using the embedded English language for testing
func testEngine(t *testing.T, seed int64) *Engine {
	t.Helper()
	languages, err := LoadLanguages()
	if err != nil {
		t.Fatalf("failed to load languages: %v", err)
	}
	lang, ok := languages["eliza-1966-en"]
	if !ok {
		t.Fatal("english language not found")
	}
	engine, err := NewEngine(lang, seed)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}
	return engine
}

func TestEngineBasic(t *testing.T) {
	engine := testEngine(t, 42)

	tests := []struct {
		name  string
		input string
	}{
		{"greeting hello", "Hello"},
		{"greeting hi", "Hi there!"},
		{"feeling", "I feel sad"},
		{"need", "I need help"},
		{"want", "I want to be happy"},
		{"family mother", "My mother is annoying"},
		{"family father", "My father doesn't understand me"},
		{"question what", "What should I do?"},
		{"question how", "How can I fix this?"},
		{"computer", "Are you a computer?"},
		{"dream", "I had a strange dream last night"},
		{"goodbye", "goodbye"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := engine.Response(tt.input)
			if response == "" {
				t.Errorf("empty response for input %q", tt.input)
			}
			t.Logf("Input: %q -> Response: %q", tt.input, response)
		})
	}
}

func TestEngineReflection(t *testing.T) {
	engine := testEngine(t, 42)

	// Test that reflections work
	tests := []struct {
		input    string
		contains string
	}{
		{"I am happy", "you"},       // "I am" should reflect to "you are"
		{"I feel tired", "you"},     // "I" should reflect to "you"
		{"I need my space", "your"}, // "my" should reflect to "your"
	}

	for _, tt := range tests {
		response := engine.Response(tt.input)
		if !strings.Contains(strings.ToLower(response), tt.contains) {
			t.Errorf("Response for %q = %q, expected it to contain %q", tt.input, response, tt.contains)
		}
		t.Logf("Input: %q -> Response: %q", tt.input, response)
	}
}

func TestEngineGreetings(t *testing.T) {
	engine := testEngine(t, 42)

	greetings := []string{"hello", "hi", "hey", "good morning", "good afternoon"}
	for _, greeting := range greetings {
		response := engine.Response(greeting)
		if response == "" {
			t.Errorf("empty response for greeting %q", greeting)
		}
		t.Logf("Greeting: %q -> Response: %q", greeting, response)
	}
}

func TestEngineGoodbye(t *testing.T) {
	engine := testEngine(t, 42)

	goodbyes := []string{"goodbye", "bye", "farewell", "see you"}
	for _, goodbye := range goodbyes {
		response := engine.Response(goodbye)
		if response == "" {
			t.Errorf("empty response for goodbye %q", goodbye)
		}
		// Response should be a goodbye message
		lower := strings.ToLower(response)
		if !strings.Contains(lower, "goodbye") && !strings.Contains(lower, "thank") && !strings.Contains(lower, "take care") && !strings.Contains(lower, "nice") {
			t.Logf("Goodbye response may not be a goodbye: %q", response)
		}
		t.Logf("Goodbye: %q -> Response: %q", goodbye, response)
	}
}

func TestEngineMemory(t *testing.T) {
	engine := testEngine(t, 42)

	// Have a conversation that should build up memory
	inputs := []string{
		"I want to be successful",
		"I need more confidence",
		"I feel anxious about work",
		"tell me something",
		"tell me something",
		"tell me something",
		"tell me something",
		"tell me something",
	}

	for _, input := range inputs {
		response := engine.Response(input)
		t.Logf("Input: %q -> Response: %q", input, response)
	}
}

func TestEngineReset(t *testing.T) {
	engine := testEngine(t, 42)

	// Build up some memory
	engine.Response("I want to be rich")
	engine.Response("I need a vacation")

	// Reset
	engine.Reset()

	// Memory should be cleared (internal state)
	// We can't easily test this directly, but we can verify no panic
	response := engine.Response("Hello again")
	if response == "" {
		t.Error("empty response after reset")
	}
}

func TestEngineDeterministic(t *testing.T) {
	// Two engines with the same seed should produce the same responses
	engine1 := testEngine(t, 12345)
	engine2 := testEngine(t, 12345)

	inputs := []string{"Hello", "I am sad", "My mother bothers me"}

	for _, input := range inputs {
		resp1 := engine1.Response(input)
		resp2 := engine2.Response(input)
		if resp1 != resp2 {
			t.Errorf("different responses for same seed: %q vs %q", resp1, resp2)
		}
	}
}

func TestEngineKeywords(t *testing.T) {
	engine := testEngine(t, 42)

	// Collect default responses to verify keyword routing produces different output
	defaultEngine := testEngine(t, 42)
	defaultResponses := make(map[string]bool)
	// Generate many default responses from non-keyword input
	for range 50 {
		resp := defaultEngine.Response("xyzzy gibberish")
		defaultResponses[resp] = true
	}

	// Test that specific keywords trigger non-default responses
	tests := []struct {
		input           string
		expectRelatedTo string
	}{
		{"I always fail", "always"},
		{"I never succeed", "never"},
		{"maybe I should try", "uncertain"},
		{"everyone hates me", "everyone"},
		{"my dreams are weird", "dream"},
		{"a robot is scary", "computer"},
		{"I'm sorry for everything", "apologize"},
	}

	for _, tt := range tests {
		response := engine.Response(tt.input)
		if response == "" {
			t.Errorf("Keyword [%s]: empty response for %q", tt.expectRelatedTo, tt.input)
		}
		if defaultResponses[response] {
			t.Errorf("Keyword [%s]: input %q got default response %q, expected keyword-specific response", tt.expectRelatedTo, tt.input, response)
		}
		t.Logf("Keyword test [%s]: %q -> %q", tt.expectRelatedTo, tt.input, response)
	}
}

func TestEngineYesNo(t *testing.T) {
	engine := testEngine(t, 42)

	// Test yes/no responses
	yesInputs := []string{"yes", "yeah", "yep"}
	noInputs := []string{"no", "nope", "nah"}

	for _, input := range yesInputs {
		response := engine.Response(input)
		t.Logf("Yes: %q -> %q", input, response)
	}

	for _, input := range noInputs {
		response := engine.Response(input)
		t.Logf("No: %q -> %q", input, response)
	}
}

func TestEngineReflect(t *testing.T) {
	engine := testEngine(t, 42)

	tests := []struct {
		input    string
		expected string
	}{
		{"i am happy", "you are happy"},
		{"my problem", "your problem"},
		{"me", "you"},
		{"you are nice", "I am nice"}, // reflect maps both "you"->"I" and "are"->"am"
	}

	for _, tt := range tests {
		result := engine.reflect(tt.input)
		if result != tt.expected {
			t.Errorf("reflect(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestIsQuit(t *testing.T) {
	engine := testEngine(t, 42)

	tests := []struct {
		input string
		want  bool
	}{
		{"quit", true},
		{"exit", true},
		{"bye", true},
		{"goodbye", true},
		{"see you later", true},
		{"hello", false},
		{"I feel sad", false},
	}

	for _, tt := range tests {
		got := engine.isQuit(tt.input)
		if got != tt.want {
			t.Errorf("isQuit(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestIsGreeting(t *testing.T) {
	engine := testEngine(t, 42)

	tests := []struct {
		input string
		want  bool
	}{
		{"hello", true},
		{"hi", true},
		{"hey", true},
		{"good morning", true},
		{"good afternoon", true},
		{"goodbye", false},
		{"I feel sad", false},
	}

	for _, tt := range tests {
		got := engine.isGreeting(tt.input)
		if got != tt.want {
			t.Errorf("isGreeting(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func BenchmarkEngineResponse(b *testing.B) {
	languages, err := LoadLanguages()
	if err != nil {
		b.Fatalf("failed to load languages: %v", err)
	}
	engine, err := NewEngine(languages["eliza-1966-en"], 42)
	if err != nil {
		b.Fatalf("failed to create engine: %v", err)
	}
	inputs := []string{
		"Hello",
		"I feel sad",
		"My mother is annoying",
		"I want to be happy",
		"What should I do?",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		input := inputs[i%len(inputs)]
		_ = engine.Response(input)
	}
}
