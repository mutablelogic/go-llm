package ollama

import (
	"encoding/json"
	"os"
	"testing"

	// Packages
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	assert "github.com/stretchr/testify/assert"
)

///////////////////////////////////////////////////////////////////////////////
// HELPERS

func loadGeneratePair(t *testing.T, name string) (json.RawMessage, json.RawMessage) {
	t.Helper()
	data, err := os.ReadFile(testdataPath(name))
	if err != nil {
		t.Fatalf("failed to read %s: %v", name, err)
	}
	var pair struct {
		Name     string          `json:"name"`
		Generate json.RawMessage `json:"generate"`
		Schema   json.RawMessage `json:"schema"`
	}
	if err := json.Unmarshal(data, &pair); err != nil {
		t.Fatalf("failed to unmarshal %s: %v", name, err)
	}
	return pair.Generate, pair.Schema
}

func decodeGenerateResponse(t *testing.T, data json.RawMessage) *generateResponse {
	t.Helper()
	var resp generateResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatalf("failed to unmarshal generate response: %v", err)
	}
	return &resp
}

///////////////////////////////////////////////////////////////////////////////
// generatePromptFromMessage TESTS

func Test_marshal_generate_prompt_from_text(t *testing.T) {
	_, schemaJSON := loadTestPair(t, "message_text_user.json")
	a := assert.New(t)
	msg := decodeSchemaMessage(t, schemaJSON)
	prompt := generatePromptFromMessage(msg)
	a.Equal("What is the best French cheese?", prompt)
}

func Test_marshal_generate_prompt_from_multipart(t *testing.T) {
	_, schemaJSON := loadTestPair(t, "message_text_multipart.json")
	a := assert.New(t)
	msg := decodeSchemaMessage(t, schemaJSON)
	prompt := generatePromptFromMessage(msg)
	a.Equal("First paragraph.\nSecond paragraph.", prompt)
}

///////////////////////////////////////////////////////////////////////////////
// generateImagesFromMessage TESTS

func Test_marshal_generate_images_from_message(t *testing.T) {
	_, schemaJSON := loadTestPair(t, "message_image_base64.json")
	a := assert.New(t)
	msg := decodeSchemaMessage(t, schemaJSON)
	images, err := generateImagesFromMessage(msg)
	a.NoError(err)
	a.Len(images, 1)
	a.NotEmpty(images[0])
}

func Test_marshal_generate_images_from_text_message(t *testing.T) {
	_, schemaJSON := loadTestPair(t, "message_text_user.json")
	a := assert.New(t)
	msg := decodeSchemaMessage(t, schemaJSON)
	images, err := generateImagesFromMessage(msg)
	a.NoError(err)
	a.Empty(images)
}

///////////////////////////////////////////////////////////////////////////////
// messageFromGenerateResponse TESTS

func Test_marshal_generate_response_text(t *testing.T) {
	generateJSON, schemaJSON := loadGeneratePair(t, "generate_text.json")
	a := assert.New(t)
	resp := decodeGenerateResponse(t, generateJSON)
	msg, err := messageFromGenerateResponse(resp)
	a.NoError(err)
	a.NotNil(msg)
	assertSchemaMessageEquals(t, schemaJSON, msg)
	a.Equal(schema.RoleAssistant, msg.Role)
	a.Len(msg.Content, 1)
	a.NotNil(msg.Content[0].Text)
	a.Equal("Hello! How can I help you today?", *msg.Content[0].Text)
}

func Test_marshal_generate_response_image(t *testing.T) {
	generateJSON, schemaJSON := loadGeneratePair(t, "generate_image.json")
	a := assert.New(t)
	resp := decodeGenerateResponse(t, generateJSON)
	msg, err := messageFromGenerateResponse(resp)
	a.NoError(err)
	a.NotNil(msg)
	assertSchemaMessageEquals(t, schemaJSON, msg)
	a.Equal(schema.RoleAssistant, msg.Role)
	a.Len(msg.Content, 1)
	a.NotNil(msg.Content[0].Attachment)
	a.Equal("image/png", msg.Content[0].Attachment.ContentType)
	a.NotEmpty(msg.Content[0].Attachment.Data)
}
