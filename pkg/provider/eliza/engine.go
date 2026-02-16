package eliza

import (
	"math/rand"
	"regexp"
	"strings"

	schema "github.com/mutablelogic/go-llm/pkg/schema"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Engine implements the classic ELIZA conversation algorithm
// created by Joseph Weizenbaum at MIT in 1966.
type Engine struct {
	lang   *Language
	rules  []compiledRule
	memory []string // Store previous statements for callbacks
	rng    *rand.Rand
}

type compiledRule struct {
	pattern   *regexp.Regexp
	responses []string
	memorable bool
}

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// NewEngine creates a new ELIZA conversation engine using the given language data
func NewEngine(lang *Language, seed int64) (*Engine, error) {
	e := &Engine{
		lang:   lang,
		memory: make([]string, 0, 10),
		rng:    rand.New(rand.NewSource(seed)),
	}

	// Compile rules
	e.rules = make([]compiledRule, 0, len(lang.Rules))
	for _, r := range lang.Rules {
		re, err := regexp.Compile(r.Pattern)
		if err != nil {
			return nil, err
		}
		e.rules = append(e.rules, compiledRule{
			pattern:   re,
			responses: r.Responses,
			memorable: r.Memorable,
		})
	}

	return e, nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Response generates an ELIZA response to the given input
func (e *Engine) Response(input string) string {
	// Normalize input
	input = strings.TrimSpace(input)
	input = strings.ToLower(input)
	input = strings.TrimRight(input, ".!?")

	// Check for quit
	if e.isQuit(input) {
		return e.randomChoice(e.lang.GoodbyeResponses)
	}

	// Check for greeting
	if e.isGreeting(input) {
		return e.randomChoice(e.lang.GreetingResponses)
	}

	// Try keyword-based responses
	for _, rule := range e.rules {
		if match := rule.pattern.FindStringSubmatch(input); match != nil {
			response := e.randomChoice(rule.responses)
			// Perform reflection on captured groups
			if len(match) > 1 {
				reflected := e.reflect(match[1])
				response = strings.Replace(response, "%s", reflected, 1)
			}
			// Store interesting statements in memory
			if rule.memorable && len(match) > 1 {
				e.remember(match[1])
			}
			return response
		}
	}

	// Maybe recall something from memory
	if len(e.memory) > 0 && e.rng.Float64() < 0.3 {
		idx := e.rng.Intn(len(e.memory))
		mem := e.memory[idx]
		e.memory = append(e.memory[:idx], e.memory[idx+1:]...) // Remove used memory
		return e.randomChoice(e.lang.MemoryResponses) + " " + e.reflect(mem) + "?"
	}

	// Default response
	return e.randomChoice(e.lang.DefaultResponses)
}

// Reset clears the conversation memory
func (e *Engine) Reset() {
	e.memory = e.memory[:0]
}

// Memory returns the current memory contents
func (e *Engine) Memory() []string {
	return e.memory
}

// InferMemory scans conversation messages and rebuilds the memory list
// from user messages that match memorable rules. This allows the engine
// to be created fresh per request while preserving memory state.
func (e *Engine) InferMemory(messages []*schema.Message) {
	e.memory = e.memory[:0]
	for _, msg := range messages {
		if msg.Role != schema.RoleUser {
			continue
		}
		input := strings.ToLower(strings.TrimRight(strings.TrimSpace(msg.Text()), ".!?"))
		for _, rule := range e.rules {
			if !rule.memorable {
				continue
			}
			if match := rule.pattern.FindStringSubmatch(input); match != nil {
				if len(match) > 1 {
					e.remember(match[1])
				}
			}
		}
	}
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func (e *Engine) randomChoice(choices []string) string {
	return choices[e.rng.Intn(len(choices))]
}

func (e *Engine) remember(statement string) {
	if len(e.memory) >= 10 {
		e.memory = e.memory[1:]
	}
	e.memory = append(e.memory, statement)
}

// reflect transforms first-person to second-person and vice versa
func (e *Engine) reflect(text string) string {
	words := strings.Fields(text)
	for i, word := range words {
		if reflected, ok := e.lang.Reflections[word]; ok {
			words[i] = reflected
		}
	}
	return strings.TrimSpace(strings.Join(words, " "))
}

func (e *Engine) isQuit(input string) bool {
	words := strings.Fields(input)
	for _, q := range e.lang.Quits {
		qWords := strings.Fields(q)
		if containsPhrase(words, qWords) {
			return true
		}
	}
	return false
}

// containsPhrase checks whether the phrase (sequence of words) appears in the words slice
func containsPhrase(words, phrase []string) bool {
	if len(phrase) == 0 || len(phrase) > len(words) {
		return false
	}
	for i := 0; i <= len(words)-len(phrase); i++ {
		match := true
		for j, p := range phrase {
			if words[i+j] != p {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

func (e *Engine) isGreeting(input string) bool {
	for _, g := range e.lang.Greetings {
		if strings.HasPrefix(input, g) || input == g {
			return true
		}
	}
	return false
}
