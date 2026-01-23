package schema

import "strings"

///////////////////////////////////////////////////////////////////////////////
// TYPES

// A generic option type, which can set options on an agent or session
type Opt func(*Message) error

///////////////////////////////////////////////////////////////////////////////
// OPTIONS

// Append additional text content block to the message
func WithText(text string) Opt {
	return func(m *Message) error {
		text = strings.TrimSpace(text)
		if text == "" {
			return nil
		}
		m.Content = append(m.Content, ContentBlock{
			Type: ContentTypeText,
			Text: &text,
		})
		return nil
	}
}
