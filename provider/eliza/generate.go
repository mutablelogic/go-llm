package eliza

import (
	"context"
	"errors"
	"fmt"
	"strings"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/provider/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// WithoutSession returns eliza's reply to the last of the messages, which
// is the user's, and must be for a model of this client, and the tokens
// used, which are estimated, as eliza has no tokens. Eliza remembers what
// the user said in the earlier messages, and with WithThinking, the reply's
// thinking is what it remembers. It can't call tools, so there can't be
// any. When the stream function isn't nil, it's called with the reply's
// thinking, and then its text a word at a time, as a model would generate
// it. The reply is the same for the same messages, with the client's seed.
func (c *Client) WithoutSession(ctx context.Context, model llm.Model, messages []llm.Message, tools []llm.Tool, stream llm.GeneratorStreamFn, opts ...llm.Opt) (llm.Message, llm.Usage, error) {
	m, ok := model.(*Model)
	if !ok || m.provider != c {
		return nil, llm.Usage{}, llm.ErrBadParameter.With("not a model of this provider")
	} else if len(tools) > 0 {
		return nil, llm.Usage{}, llm.ErrNotImplemented.With("eliza can't call tools")
	} else if len(messages) == 0 {
		return nil, llm.Usage{}, llm.ErrBadParameter.With("missing messages")
	}

	// The last message is the user's, which is replied to
	last := messages[len(messages)-1]
	if last == nil || last.Role() != "user" {
		return nil, llm.Usage{}, llm.ErrBadParameter.With("the last message must be the user's")
	}
	input := last.Text()
	if strings.TrimSpace(input) == "" {
		return nil, llm.Usage{}, llm.ErrBadParameter.With("the last message has no text")
	}

	// Whether to think
	options, err := llm.ApplyOpts(opts...)
	if err != nil {
		return nil, llm.Usage{}, err
	}
	thinking, err := llm.GetOpt[bool](options, llm.ThinkingKey)
	if err != nil && !errors.Is(err, llm.ErrNotFound) {
		return nil, llm.Usage{}, err
	}

	// Create an engine for the request, which remembers the user's earlier
	// messages, with a seed for the conversation, so the replies to its
	// messages vary
	engine, err := NewEngine(m.lang, c.seed+int64(len(messages)))
	if err != nil {
		return nil, llm.Usage{}, err
	}
	var earlier []string
	inputTokens := estimateTokens(input)
	for _, message := range messages[:len(messages)-1] {
		if message == nil {
			continue
		}
		inputTokens += estimateTokens(message.Text())
		if message.Role() == "user" {
			earlier = append(earlier, message.Text())
		}
	}
	engine.InferMemory(earlier)

	// Think about what it remembers, and reply
	reply := &schema.Message{
		MessageMeta: schema.MessageMeta{Sender: "assistant"},
		Result:      schema.ResultStop,
	}
	if thinking {
		if text := formatMemory(engine.Memory()); text != "" {
			reply.Content = append(reply.Content, schema.ContentBlock{Thinking: types.Ptr(text)})
		}
	}
	text := engine.Response(input)
	reply.Content = append(reply.Content, schema.ContentBlock{Text: types.Ptr(text)})
	reply.Tokens = uint64(estimateTokens(text))

	// Stream the reply
	if stream != nil {
		if err := streamReply(ctx, stream, reply.Thinking(), text); err != nil {
			return nil, llm.Usage{}, err
		}
	}

	// Return the reply, and the tokens used
	return reply, llm.Usage{InputTokens: inputTokens, OutputTokens: uint(reply.Tokens)}, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// streamReply calls the function with the thinking, when there is any, and
// then the text a word at a time, as a model would generate it, until the
// context is cancelled
func streamReply(ctx context.Context, fn llm.GeneratorStreamFn, thinking, text string) error {
	if thinking != "" {
		if err := ctx.Err(); err != nil {
			return err
		}
		fn(llm.StreamThinking, thinking)
	}
	for i, word := range strings.Fields(text) {
		if err := ctx.Err(); err != nil {
			return err
		}
		if i > 0 {
			word = " " + word
		}
		fn(llm.StreamText, word)
	}
	return nil
}

// estimateTokens returns about how many tokens the text would be, which is
// about four characters a token
func estimateTokens(text string) uint {
	return uint(len(text)+3) / 4
}

// formatMemory returns what the engine remembers of the conversation, as
// its thinking, or an empty string when it remembers nothing
func formatMemory(memory []string) string {
	if len(memory) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("Recalled memories from conversation:\n")
	for i, m := range memory {
		fmt.Fprintf(&b, "  %d. %s\n", i+1, m)
	}
	return b.String()
}
