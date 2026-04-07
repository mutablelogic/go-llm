package ollama

import (
	"encoding/base64"
	"fmt"
	"mime"
	"net/http"
	"strings"

	// Packages
	schema "github.com/mutablelogic/go-llm/kernel/schema"
)

///////////////////////////////////////////////////////////////////////////////
// SCHEMA MESSAGE → GENERATE REQUEST FIELDS

// generatePromptFromMessage extracts the text content of a schema.Message as a
// single prompt string for use with POST /api/generate.
// Text blocks are joined with newlines; text attachments are appended inline.
func generatePromptFromMessage(msg *schema.Message) string {
	var parts []string
	for _, block := range msg.Content {
		if block.Text != nil {
			parts = append(parts, *block.Text)
			continue
		}
		if block.Attachment != nil && block.Attachment.IsText() && len(block.Attachment.Data) > 0 {
			parts = append(parts, block.Attachment.TextContent())
		}
	}
	return strings.Join(parts, "\n")
}

// generateImagesFromMessage extracts image attachment bytes from a schema.Message
// for use in the generateRequest.Images field.
// Only image/* MIME types are included; URL-only attachments return an error.
func generateImagesFromMessage(msg *schema.Message) ([][]byte, error) {
	var images [][]byte
	for _, block := range msg.Content {
		if block.Attachment == nil {
			continue
		}
		mediaType, _, _ := mime.ParseMediaType(block.Attachment.ContentType)
		if !strings.HasPrefix(mediaType, "image/") {
			continue
		}
		if len(block.Attachment.Data) == 0 {
			return nil, fmt.Errorf("image attachment %q has no data (URL-only images are not supported by /api/generate)", block.Attachment.ContentType)
		}
		images = append(images, block.Attachment.Data)
	}
	return images, nil
}

///////////////////////////////////////////////////////////////////////////////
// GENERATE RESPONSE → SCHEMA MESSAGE

// messageFromGenerateResponse converts a generateResponse to a schema.Message.
// The response text becomes a text block; any returned images become attachment
// blocks with their MIME type detected from the raw bytes.
func messageFromGenerateResponse(resp *generateResponse) (*schema.Message, error) {
	var blocks []schema.ContentBlock

	if resp.Response != "" {
		text := resp.Response
		blocks = append(blocks, schema.ContentBlock{Text: &text})
	}

	// Ollama image-generation models return a single base64 image in "image".
	if resp.Image != "" {
		data, err := base64.StdEncoding.DecodeString(resp.Image)
		if err != nil {
			return nil, fmt.Errorf("decode image: %w", err)
		}
		blocks = append(blocks, schema.ContentBlock{
			Attachment: &schema.Attachment{
				ContentType: http.DetectContentType(data),
				Data:        data,
			},
		})
	}

	for _, img := range resp.Images {
		blocks = append(blocks, schema.ContentBlock{
			Attachment: &schema.Attachment{
				ContentType: http.DetectContentType(img),
				Data:        img,
			},
		})
	}

	return &schema.Message{
		Role:    schema.RoleAssistant,
		Content: blocks,
		Result:  resultFromDoneReason(resp.DoneReason),
	}, nil
}
