package registry

import (
	"context"
	"errors"
	"io"
	"net/url"
	"slices"
	"uuid"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	providerschema "github.com/mutablelogic/go-llm/provider/schema"
	resourceschema "github.com/mutablelogic/go-llm/resource/schema"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	toolschema "github.com/mutablelogic/go-llm/toolkit/schema"
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

	// Create keeps the content, as it's read, as a resource, which is what
	// the meta says, in its session, or on its own, and returns it, and
	// whether it was created, rather than there being one with its content
	// already
	Create(ctx context.Context, meta resourceschema.ResourceMeta, content io.Reader) (resourceschema.Resource, bool, error)
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
// URI, and at most MaxAttachmentsSize of them, together. A resource which is
// text, such as code or subtitles, is sent as text, with its name, as most
// models don't accept text as an attachment. In a session's
// history, an attachment whose resource isn't found, such as one which was
// deleted, is text which says so, and otherwise it's an error, so it isn't
// embedded, or asked about, as if it were there. The messages aren't
// changed. In a session's history, an attachment of a type which the model,
// when there is one, doesn't accept, such as audio, isn't read, or sent, but
// is text which says its URI, so the model can give it to a tool, such as
// one which transcribes it, rather than its provider rejecting it.
func (r *Registry) withContent(ctx context.Context, session uuid.UUID, messages []llm.Message, history bool, model llm.Model) ([]llm.Message, error) {
	var result []llm.Message
	reader := r.newContentReader(session)
	if history {
		reader.accepts = accepts(model)
	}
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
	data, contentType, _, err := r.newContentReader(uuid.Nil()).read(ctx, attachment.URL.String())
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

// toolStore returns the function which keeps the resources which the tools
// run in the session return, such as an image, or audio, as resources of
// the session, or, without one, on their own, so the tools' results have
// links to them, rather than their content, or nil, when the registry has
// no resources, when their results have their content
func (r *Registry) toolStore(session uuid.UUID) toolkit.StoreFunc {
	if r.resources == nil {
		return nil
	}
	return func(ctx context.Context, resource llm.Resource) (toolschema.ResourceLink, error) {
		meta := resourceschema.ResourceMeta{Name: resource.Name(), Description: resource.Description(), Type: resource.Type()}
		if session != uuid.Nil() {
			meta.Session = &session
		}
		read, err := resource.Read(ctx)
		if err != nil {
			return toolschema.ResourceLink{}, err
		}
		var created resourceschema.Resource
		if err := read(ctx, func(content io.Reader) error {
			created, _, err = r.resources.Create(ctx, meta, content)
			return err
		}); err != nil {
			return toolschema.ResourceLink{}, err
		}
		return toolschema.ResourceLink{URI: created.URI(), Name: created.Name, Title: created.Description, Type: created.Type, Size: created.Size}, nil
	}
}

// toolResolver returns the function which returns the resources which the
// tools run in the session are given, by their URIs: the session's, or,
// otherwise, or without one, those on their own, whose content is read, up
// to MaxAttachmentsSize, as an attachment's is, so a tool, such as one which
// recognizes text, doesn't read a large resource without bound
func (r *Registry) toolResolver(session uuid.UUID) toolkit.ResolveFunc {
	return func(ctx context.Context, uri string) (llm.Resource, error) {
		u, err := url.Parse(uri)
		if err != nil || u.Scheme != resourceschema.Scheme {
			return nil, llm.ErrBadParameter.Withf("not a resource's URI: %q", uri)
		}
		data, contentType, name, err := r.newContentReader(session).read(ctx, uri)
		if errors.Is(err, llm.ErrNotFound) && session != uuid.Nil() {
			data, contentType, name, err = r.newContentReader(uuid.Nil()).read(ctx, uri)
		}
		if err != nil {
			return nil, err
		}
		return namedResource{Resource: &providerschema.Resource{ResourceMeta: providerschema.ResourceMeta{ContentType: contentType, Data: data, URL: u}}, name: name}, nil
	}
}

// namedResource is a resource, whose content was read, with the name it's
// kept with, such as scan.pdf, which a tool says it's reading, and names
// what it keeps after
type namedResource struct {
	*providerschema.Resource
	name string
}

// Name returns the resource's name, which is its file's name, or none
func (r namedResource) Name() string {
	return r.name
}

// attachmentText returns the function which returns the text an attachment,
// which refers to a resource in the session, or on its own, is sent to a
// model as, when its resource is text, so the conversation is measured as
// it's sent, rather than with an estimate of an attachment, such as an
// image. A resource which isn't text, or can't be read, isn't.
func (r *Registry) attachmentText(session uuid.UUID) textFunc {
	reader := r.newContentReader(session)
	return func(ctx context.Context, resource providerschema.Resource) (string, bool) {
		if !isResourceURL(&resource) || len(resource.Data) > 0 || !providerschema.IsText(nil, resource.ContentType) {
			return "", false
		}
		uri := resource.URL.String()
		data, contentType, name, err := reader.read(ctx, uri)
		if resource.ContentType == "" {
			resource.ContentType = contentType
		}
		if err != nil || !providerschema.IsText(data, resource.ContentType) {
			return "", false
		}
		if name == "" {
			name = uri
		}
		return types.Value(providerschema.TextBlock(name, data).Text), true
	}
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

	// accepts returns whether the model accepts an attachment of the type,
	// or is nil, when every attachment is sent
	accepts func(string) bool
}

// readContent is the content of a resource, its type, and its name
type readContent struct {
	data        []byte
	contentType string
	name        string
}

// newContentReader returns a reader of the content of resources, in the
// session, or on their own, without one
func (r *Registry) newContentReader(session uuid.UUID) *contentReader {
	return &contentReader{registry: r, session: session, cache: make(map[string]readContent), left: MaxAttachmentsSize}
}

// attachments returns the message, with the content of its attachments
// which refer to resources, of which those which are text are text, with
// their names, or else their URIs. In a session's history, each is followed by
// text which says its URI, so the model can give it to a tool, such as one
// which recognizes its text, and one whose resource isn't found is text
// which says so, and otherwise it's an error.
func (c *contentReader) attachments(ctx context.Context, message providerschema.Message, history bool) (providerschema.Message, error) {
	content := make([]providerschema.ContentBlock, 0, len(message.Content))
	for _, block := range message.Content {
		if !isResourceURL(block.Resource) || len(block.Resource.Data) > 0 {
			content = append(content, block)
			continue
		}
		uri := block.Resource.URL.String()
		// An attachment the model doesn't accept isn't read, unless it may be
		// text, which is sent as text
		if contentType := block.Resource.ContentType; !c.accept(contentType) && !providerschema.IsText(nil, contentType) {
			content = append(content, unreadable(uri, contentType))
			continue
		}
		data, contentType, name, err := c.read(ctx, uri)
		if history && (errors.Is(err, llm.ErrNotFound) || errors.Is(err, llm.ErrNotImplemented)) {
			content = append(content, providerschema.ContentBlock{Text: types.Ptr("(The attachment " + uri + " isn't available.)")})
			continue
		} else if err != nil {
			return providerschema.Message{}, err
		}
		attachment := *block.Resource
		attachment.Data = data
		if attachment.ContentType == "" {
			attachment.ContentType = contentType
		}
		if providerschema.IsText(data, attachment.ContentType) {
			if name == "" {
				name = uri
			}
			content = append(content, providerschema.TextBlock(name, data))
		} else if !c.accept(attachment.ContentType) {
			content = append(content, unreadable(uri, attachment.ContentType))
			continue
		} else {
			block.Resource = &attachment
			content = append(content, block)
		}
		if history {
			content = append(content, providerschema.ContentBlock{Text: types.Ptr("(The attachment above is " + uri + ", which is its URI for tools.)")})
		}
	}
	message.Content = content
	return message, nil
}

// read returns the content, the type, and the name, of the resource with
// the URI, which is read once, and shared by each attachment which refers to it.
// Returns ErrBadParameter when it, with the content which was read before,
// is larger than MaxAttachmentsSize.
func (c *contentReader) read(ctx context.Context, uri string) ([]byte, string, string, error) {
	if content, ok := c.cache[uri]; ok {
		return content.data, content.contentType, content.name, nil
	}
	resource, err := c.registry.resource(ctx, uri, c.session)
	if err != nil {
		return nil, "", "", err
	}
	read, err := resource.Read(ctx)
	if err != nil {
		return nil, "", "", err
	}
	var data []byte
	if err := read(ctx, func(r io.Reader) error {
		data, err = io.ReadAll(io.LimitReader(r, c.left+1))
		return err
	}); err != nil {
		return nil, "", "", err
	} else if int64(len(data)) > c.left {
		return nil, "", "", llm.ErrBadParameter.Withf("the attachments are larger than %d bytes, together", MaxAttachmentsSize)
	}
	c.left -= int64(len(data))
	c.cache[uri] = readContent{data: data, contentType: resource.Type(), name: resource.Name()}
	return data, resource.Type(), resource.Name(), nil
}

// accept returns whether an attachment of the type is sent to the model,
// which it is, unless its type is known, and the model doesn't accept it
func (c *contentReader) accept(contentType string) bool {
	return c.accepts == nil || contentType == "" || c.accepts(contentType)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// accepts returns the function which says whether the model accepts an
// attachment of a type, when its provider says which it does, or nil
func accepts(model llm.Model) func(string) bool {
	if model == nil || model.Provider() == nil {
		return nil
	}
	accepter, ok := model.Provider().Self().(llm.Accepter)
	if !ok {
		return nil
	}
	return func(contentType string) bool {
		return accepter.Accepts(model, contentType)
	}
}

// unreadable returns the text which is sent in place of an attachment which
// the model doesn't accept, which says its URI, so it can give it to a tool
func unreadable(uri, contentType string) providerschema.ContentBlock {
	return providerschema.ContentBlock{Text: types.Ptr("(The attachment " + uri + " is " + contentType + ", which this model can't read, so it isn't sent. Its URI is for tools, which can read it.)")}
}

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
