package schema

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// A generic option type, which can set options on an agent or session
type Opt func(*Message) error

///////////////////////////////////////////////////////////////////////////////
// HELPERS

// isTextMediaType checks if a media type represents text content
func isTextMediaType(mediaType string) bool {
	return strings.HasPrefix(mediaType, "text/") ||
		mediaType == "application/json" ||
		mediaType == "application/xml" ||
		mediaType == "application/javascript" ||
		strings.HasSuffix(mediaType, "+xml") ||
		strings.HasSuffix(mediaType, "+json")
}

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

// Append additional file content block to the message
// Detects the file type and creates either an ImageSource or DocumentSource
func WithFile(r io.Reader) Opt {
	return func(m *Message) error {
		// Read the file content
		data, err := io.ReadAll(r)
		if err != nil {
			return err
		}

		// Detect the media type from the content
		mediaType := http.DetectContentType(data)

		// Encode the data as base64
		encoded := base64.StdEncoding.EncodeToString(data)

		// Determine the content type and create appropriate block
		if strings.HasPrefix(mediaType, "image/") {
			// Create an image content block
			m.Content = append(m.Content, ContentBlock{
				Type: ContentTypeImage,
				ImageSource: &ImageSource{
					Type:      "base64",
					MediaType: mediaType,
					Data:      &encoded,
				},
			})
		} else if mediaType == "application/pdf" {
			// Create a document content block (only for PDFs)
			m.Content = append(m.Content, ContentBlock{
				Type: ContentTypeDocument,
				DocumentSource: &DocumentSource{
					Type:      "base64",
					MediaType: mediaType,
					Data:      &encoded,
				},
			})
		} else if isTextMediaType(mediaType) {
			// Create a text content block for recognized text types
			text := string(data)
			m.Content = append(m.Content, ContentBlock{
				Type: ContentTypeText,
				Text: &text,
			})
		} else {
			// Return error for unsupported file types
			return fmt.Errorf("unsupported file type: %s (only images, PDFs, and text files are supported)", mediaType)
		}

		return nil
	}
}

// WithFileAndMetadata appends a file content block with additional metadata (title, context)
func WithFileAndMetadata(r io.Reader, title, context string) Opt {
	return func(m *Message) error {
		// Read the file content
		data, err := io.ReadAll(r)
		if err != nil {
			return err
		}

		// Detect the media type from the content
		mediaType := http.DetectContentType(data)

		// Encode the data as base64
		encoded := base64.StdEncoding.EncodeToString(data)

		// Determine if this is an image or document based on media type
		if strings.HasPrefix(mediaType, "image/") {
			// Create an image content block (title and context not applicable)
			m.Content = append(m.Content, ContentBlock{
				Type: ContentTypeImage,
				ImageSource: &ImageSource{
					Type:      "base64",
					MediaType: mediaType,
					Data:      &encoded,
				},
			})
		} else if mediaType == "application/pdf" {
			// Create a document content block with metadata (only for PDFs)
			block := ContentBlock{
				Type: ContentTypeDocument,
				DocumentSource: &DocumentSource{
					Type:      "base64",
					MediaType: mediaType,
					Data:      &encoded,
				},
			}

			// Add title if provided
			if title != "" {
				block.DocumentTitle = &title
			}

			// Add context if provided
			if context != "" {
				block.DocumentContext = &context
			}

			m.Content = append(m.Content, block)
		} else if isTextMediaType(mediaType) {
			// Create a text content block with title and context at the top
			text := string(data)
			var header strings.Builder
			if title != "" {
				header.WriteString("# ")
				header.WriteString(title)
				header.WriteString("\n\n")
			}
			if context != "" {
				header.WriteString("Context: ")
				header.WriteString(context)
				header.WriteString("\n\n")
			}
			if header.Len() > 0 {
				text = header.String() + text
			}
			m.Content = append(m.Content, ContentBlock{
				Type: ContentTypeText,
				Text: &text,
			})
		} else {
			// Return error for unsupported file types
			return fmt.Errorf("unsupported file type: %s (only images, PDFs, and text files are supported)", mediaType)
		}

		return nil
	}
}

// WithImageURL appends an image content block from a URL
func WithImageURL(url string) Opt {
	return func(m *Message) error {
		if url == "" {
			return nil
		}
		m.Content = append(m.Content, ContentBlock{
			Type: ContentTypeImage,
			ImageSource: &ImageSource{
				Type: "url",
				URL:  &url,
			},
		})
		return nil
	}
}

// WithDocumentURL appends a document content block from a URL
func WithDocumentURL(url string, title, context string) Opt {
	return func(m *Message) error {
		if url == "" {
			return nil
		}

		block := ContentBlock{
			Type: ContentTypeDocument,
			DocumentSource: &DocumentSource{
				Type: "url",
				URL:  &url,
			},
		}

		// Add title if provided
		if title != "" {
			block.DocumentTitle = &title
		}

		// Add context if provided
		if context != "" {
			block.DocumentContext = &context
		}

		m.Content = append(m.Content, block)
		return nil
	}
}

// WithImageByPath appends an image content block from a file path
func WithImageByPath(path string) Opt {
	return func(m *Message) error {
		// Detect media type from file extension
		ext := strings.ToLower(filepath.Ext(path))
		var mediaType string
		switch ext {
		case ".jpg", ".jpeg":
			mediaType = "image/jpeg"
		case ".png":
			mediaType = "image/png"
		case ".gif":
			mediaType = "image/gif"
		case ".webp":
			mediaType = "image/webp"
		default:
			mediaType = "image/jpeg" // default
		}

		// For now, we store the path as a file_id reference
		// This would need provider-specific handling
		m.Content = append(m.Content, ContentBlock{
			Type: ContentTypeImage,
			ImageSource: &ImageSource{
				Type:      "file",
				MediaType: mediaType,
				FileID:    &path,
			},
		})
		return nil
	}
}
