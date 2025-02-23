package gemini

/////////////////////////////////////////////////////////////////////////////////
// TYPES

type Content struct {
	Role  string `json:"role"`
	Parts []Part `json:"parts"`
}

type Part struct {
	Text string `json:"text"`
}
