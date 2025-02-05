package openai

import "strings"

///////////////////////////////////////////////////////////////////////////////
// TYPES

type ToolChoice struct {
	Type     string `json:"type"`
	Function struct {
		Name string `json:"name"`
	} `json:"function"`
}

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func NewToolChoice(function string) *ToolChoice {
	choice := new(ToolChoice)
	choice.Type = "function"
	choice.Function.Name = strings.TrimSpace(strings.ToLower(function))
	return choice
}
