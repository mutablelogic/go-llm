package registry

import (
	"context"
	"errors"
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
// without one, on their own, read from where they're kept. An attachment
// whose resource isn't found, such as one which was deleted, is text which
// says so. The messages aren't changed.
func (r *Registry) withContent(ctx context.Context, session uuid.UUID, messages []llm.Message) ([]llm.Message, error) {
	var result []llm.Message
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
		read, err := r.readAttachments(ctx, session, *message)
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

// readAttachments returns the message, with the content of its attachments
// which refer to resources, read from where they're kept
func (r *Registry) readAttachments(ctx context.Context, session uuid.UUID, message providerschema.Message) (providerschema.Message, error) {
	message.Content = slices.Clone(message.Content)
	for i, block := range message.Content {
		if !isResourceURL(block.Resource) || len(block.Resource.Data) > 0 {
			continue
		}
		data, contentType, err := r.readResource(ctx, block.Resource.URL.String(), session)
		if errors.Is(err, llm.ErrNotFound) || errors.Is(err, llm.ErrNotImplemented) {
			message.Content[i] = providerschema.ContentBlock{Text: types.Ptr("(The attachment " + block.Resource.URL.String() + " isn't available.)")}
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

// withResource returns the resource, as it is, unless it refers to a
// resource, by its URI, without its data, when it's that resource, on its
// own, whose content is read from where it's kept, as the audio, documents
// and images which are transcribed, recognized, or embedded, are. Returns
// ErrNotImplemented when the registry has no resources, and ErrNotFound when
// there's none with the URI.
func (r *Registry) withResource(ctx context.Context, resource llm.Resource) (llm.Resource, error) {
	attachment, ok := resource.(*providerschema.Resource)
	if !ok || !isResourceURL(attachment) || len(attachment.Data) > 0 {
		return resource, nil
	}
	data, contentType, err := r.readResource(ctx, attachment.URL.String(), uuid.Nil())
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

// readResource returns the content, and the type, of the resource with the
// URI, in the session, or on its own, without one
func (r *Registry) readResource(ctx context.Context, uri string, session uuid.UUID) ([]byte, string, error) {
	resource, err := r.resource(ctx, uri, session)
	if err != nil {
		return nil, "", err
	}
	data, err := llm.ReadAll(ctx, resource)
	if err != nil {
		return nil, "", err
	}
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
