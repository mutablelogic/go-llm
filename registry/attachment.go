package registry

import (
	"context"
	"errors"
	"io"
	"slices"
	"uuid"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	providerschema "github.com/mutablelogic/go-llm/provider/schema"
	resourceschema "github.com/mutablelogic/go-llm/resource/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Resources are content, such as images, documents and audio, which is kept
// once, and referred to by its URI, which is resource://<hash>, which is in
// a session, or on its own, such as the resource package's manager
type Resources interface {
	// Resource returns the resource with the URI, in the session, or on its
	// own, when it's nil, whose content is read from where it's kept.
	// Returns ErrNotFound when there's none.
	Resource(ctx context.Context, uri string, session *uuid.UUID) (llm.Resource, error)

	// Link links the resource with the URI, which is on its own, to the
	// session, so it's kept with it, and returns it, as the session's.
	// Returns ErrNotFound when there's no resource on its own with it, or
	// the session doesn't exist.
	Link(ctx context.Context, uri string, session uuid.UUID) (resourceschema.Resource, error)
}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// MaxAttachmentsSize is the most content of the resources which a request's
// attachments refer to, together, which is read into memory, as it's sent
// to a model, which is more than any provider accepts
var MaxAttachmentsSize int64 = 64 << 20

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// attach returns the message, whose attachments which refer to resources,
// by their URIs, are resources in the session, or, without one, on their
// own, with their types, when they have none, and without their data, which
// is read as they're sent to a model. A resource on its own is linked to
// the session, so it's kept with it. Returns ErrNotImplemented when the
// registry has no resources, and ErrNotFound when there's no resource with a
// URI.
func (r *Registry) attach(ctx context.Context, session uuid.UUID, message providerschema.Message) (providerschema.Message, error) {
	if !refersToResources(message) {
		return message, nil
	} else if r.resources == nil {
		return providerschema.Message{}, llm.ErrNotImplemented.With("attachments can't refer to resources, as the registry has none")
	}
	message.Content = slices.Clone(message.Content)
	for i, block := range message.Content {
		if !isResourceURL(block.Resource) {
			continue
		}
		uri := block.Resource.URL.String()
		resource, err := r.resource(ctx, uri, session)
		if errors.Is(err, llm.ErrNotFound) && session != uuid.Nil() {
			// A resource on its own, which is linked to the session
			if _, err = r.resources.Link(ctx, uri, session); err == nil {
				resource, err = r.resource(ctx, uri, session)
			}
		}
		if err != nil {
			return providerschema.Message{}, err
		}
		attachment := *block.Resource
		attachment.Data = nil
		if attachment.ContentType == "" {
			attachment.ContentType = resource.Type()
		}
		message.Content[i].Resource = &attachment
	}
	return message, nil
}

// withContent returns the messages, as they're sent to a model, with the
// content of the attachments which refer to resources, in the session, or,
// without one, on their own, read from where they're kept, once for each
// URI, and at most MaxAttachmentsSize of them, together. In a session's
// history, an attachment whose resource isn't found, such as one which was
// deleted, is text which says so, and otherwise it's an error, so it isn't
// embedded, or asked about, as if it were there. The messages aren't
// changed.
func (r *Registry) withContent(ctx context.Context, session uuid.UUID, messages []llm.Message, history bool) ([]llm.Message, error) {
	var result []llm.Message
	reader := r.newContentReader(session)
	for i, m := range messages {
		message, ok := schemaMessage(m)
		if !ok || !refersToResources(*message) {
			if result != nil {
				result = append(result, m)
			}
			continue
		}
		if result == nil {
			result = slices.Clone(messages[:i])
		}
		read, err := reader.attachments(ctx, *message, history)
		if err != nil {
			return nil, err
		}
		result = append(result, &read)
	}
	if result == nil {
		return messages, nil
	}
	return result, nil
}

// withResource returns the resource, as it is, unless it refers to a
// resource, by its URI, without its data, when it's that resource, on its
// own, whose content is read from where it's kept, up to
// MaxAttachmentsSize, as the audio, documents and images which are
// transcribed, recognized, or embedded, are. Returns ErrNotImplemented when
// the registry has no resources, ErrNotFound when there's none with the
// URI, and ErrBadParameter when it's larger than MaxAttachmentsSize.
func (r *Registry) withResource(ctx context.Context, resource llm.Resource) (llm.Resource, error) {
	attachment, ok := resource.(*providerschema.Resource)
	if !ok || !isResourceURL(attachment) || len(attachment.Data) > 0 {
		return resource, nil
	}
	data, contentType, err := r.newContentReader(uuid.Nil()).read(ctx, attachment.URL.String())
	if err != nil {
		return nil, err
	}
	read := *attachment
	read.Data = data
	if read.ContentType == "" {
		read.ContentType = contentType
	}
	return &read, nil
}

// resource returns the resource with the URI, in the session, or on its
// own, without one
func (r *Registry) resource(ctx context.Context, uri string, session uuid.UUID) (llm.Resource, error) {
	if r.resources == nil {
		return nil, llm.ErrNotImplemented.With("attachments can't refer to resources, as the registry has none")
	}
	var scope *uuid.UUID
	if session != uuid.Nil() {
		scope = &session
	}
	resource, err := r.resources.Resource(ctx, uri, scope)
	if errors.Is(err, llm.ErrNotFound) {
		return nil, llm.ErrNotFound.Withf("resource %s", uri)
	}
	return resource, err
}

///////////////////////////////////////////////////////////////////////////////
// CONTENT READER

// contentReader reads the content of the resources which a request's
// attachments refer to, in a session, or on their own, once for each URI,
// and at most MaxAttachmentsSize of them, together, so a request which
// refers to large resources, or to one many times, doesn't hold more than
// that
type contentReader struct {
	registry *Registry
	session  uuid.UUID
	cache    map[string]readContent
	left     int64
}

// readContent is the content of a resource, and its type
type readContent struct {
	data        []byte
	contentType string
}

// newContentReader returns a reader of the content of resources, in the
// session, or on their own, without one
func (r *Registry) newContentReader(session uuid.UUID) *contentReader {
	return &contentReader{registry: r, session: session, cache: make(map[string]readContent), left: MaxAttachmentsSize}
}

// attachments returns the message, with the content of its attachments
// which refer to resources. In a session's history, one whose resource
// isn't found is text which says so, and otherwise it's an error.
func (c *contentReader) attachments(ctx context.Context, message providerschema.Message, history bool) (providerschema.Message, error) {
	message.Content = slices.Clone(message.Content)
	for i, block := range message.Content {
		if !isResourceURL(block.Resource) || len(block.Resource.Data) > 0 {
			continue
		}
		uri := block.Resource.URL.String()
		data, contentType, err := c.read(ctx, uri)
		if history && (errors.Is(err, llm.ErrNotFound) || errors.Is(err, llm.ErrNotImplemented)) {
			message.Content[i] = providerschema.ContentBlock{Text: types.Ptr("(The attachment " + uri + " isn't available.)")}
			continue
		} else if err != nil {
			return providerschema.Message{}, err
		}
		attachment := *block.Resource
		attachment.Data = data
		if attachment.ContentType == "" {
			attachment.ContentType = contentType
		}
		message.Content[i].Resource = &attachment
	}
	return message, nil
}

// read returns the content, and the type, of the resource with the URI,
// which is read once, and shared by each attachment which refers to it.
// Returns ErrBadParameter when it, with the content which was read before,
// is larger than MaxAttachmentsSize.
func (c *contentReader) read(ctx context.Context, uri string) ([]byte, string, error) {
	if content, ok := c.cache[uri]; ok {
		return content.data, content.contentType, nil
	}
	resource, err := c.registry.resource(ctx, uri, c.session)
	if err != nil {
		return nil, "", err
	}
	read, err := resource.Read(ctx)
	if err != nil {
		return nil, "", err
	}
	var data []byte
	if err := read(ctx, func(r io.Reader) error {
		data, err = io.ReadAll(io.LimitReader(r, c.left+1))
		return err
	}); err != nil {
		return nil, "", err
	} else if int64(len(data)) > c.left {
		return nil, "", llm.ErrBadParameter.Withf("the attachments are larger than %d bytes, together", MaxAttachmentsSize)
	}
	c.left -= int64(len(data))
	c.cache[uri] = readContent{data: data, contentType: resource.Type()}
	return data, resource.Type(), nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// refersToResources returns whether any of the message's attachments refer
// to resources, by their URIs
func refersToResources(message providerschema.Message) bool {
	return slices.ContainsFunc(message.Content, func(block providerschema.ContentBlock) bool {
		return isResourceURL(block.Resource)
	})
}

// isResourceURL returns whether the attachment refers to a resource, by its
// URI, which is resource://<hash>
func isResourceURL(attachment *providerschema.Resource) bool {
	return attachment != nil && attachment.URL != nil && attachment.URL.Scheme == resourceschema.Scheme
}
