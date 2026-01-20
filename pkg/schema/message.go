package schema

import "encoding/json"

////////////////////////////////////////////////////////////////////////////////
// TYPES

type content struct {
	// If content is a string
	Text *string

	// If content is an array of text
	TextArray []string

	// If content is an array of types
	TypeArray []contentType
}

type contentType struct {
	Type string `json:"type,omitempty"`
	Text string `json:"text,omitempty"`
}

// Represents content to send to or received from an LLM
type Message struct {
	Role    string  `json:"role,omitempty"`    // assistant, user, tool, system
	Content content `json:"content,omitempty"` // string or array of text, reference, image_url
	Tokens  uint    `json:"-"`                 // number of tokens for the message
}

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func StringMessage(role, message string) Message {
	return Message{
		Role:    role,
		Content: content{Text: &message},
	}
}

////////////////////////////////////////////////////////////////////////////////
// STRINGIFY

func (m Message) String() string {
	return stringify(m)
}

////////////////////////////////////////////////////////////////////////////////
// JSON

func (c *content) UnmarshalJSON(data []byte) error {
	// Try string
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		c.Text = &str
		return nil
	}

	// Try array of strings
	var strArray []string
	if err := json.Unmarshal(data, &strArray); err == nil {
		c.TextArray = strArray
		return nil
	}

	// Try array of types
	var typeArray []contentType
	if err := json.Unmarshal(data, &typeArray); err == nil {
		c.TypeArray = typeArray
		return nil
	}

	return nil
}

func (c content) MarshalJSON() ([]byte, error) {
	if c.Text != nil {
		return json.Marshal(c.Text)
	}
	if c.TextArray != nil {
		return json.Marshal(c.TextArray)
	}
	if c.TypeArray != nil {
		return json.Marshal(c.TypeArray)
	}
	return json.Marshal(nil)
}
