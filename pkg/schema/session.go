package schema

////////////////////////////////////////////////////////////////////////////////
// TYPES

// Session is a sequence of messages exchanged with an LLM
type Session []*Message

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Append adds a message to the session
func (s *Session) Append(message Message) {
	*s = append(*s, &message)
}

// AppendWithOutput adds a message to the session, re-calculating token usage
// for the session
func (s *Session) AppendWithOuput(message Message, input, output uint) {
	// Calculate the input tokens and adjust the last message to account for the tokens
	tokens := uint(0)
	for _, msg := range *s {
		tokens += msg.Tokens
	}
	if input > tokens {
		(*s)[len(*s)-1].Tokens = input - tokens
	}

	// Set the output tokens
	message.Tokens = output

	// Append the message
	*s = append(*s, &message)
}

// Return the total number of tokens in the session
func (s Session) Tokens() uint {
	total := uint(0)
	for _, msg := range s {
		total += msg.Tokens
	}
	return total
}

////////////////////////////////////////////////////////////////////////////////
// STRINGIFY

func (s Session) String() string {
	return stringify(s)
}
