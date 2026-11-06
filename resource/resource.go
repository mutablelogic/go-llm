// Package resource has resources: content, such as images, documents and
// audio, which is uploaded, or which tools return, which is kept once, by
// its hash, and referred to as resource://<hash>. A resource is in a
// session, which it's deleted with, or on its own, when it's deleted once
// it hasn't been used for a while.
package resource

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"path"
	"slices"
	"strings"
	"sync"
	"time"
	"uuid"

	// Packages
	otel "github.com/mutablelogic/go-client/pkg/otel"
	llm "github.com/mutablelogic/go-llm"
	broadcaster "github.com/mutablelogic/go-llm/broadcaster"
	eventschema "github.com/mutablelogic/go-llm/broadcaster/schema"
	schema "github.com/mutablelogic/go-llm/resource/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
	attribute "go.opentelemetry.io/otel/attribute"
	trace "go.opentelemetry.io/otel/trace"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Manager manages the resources, which are kept in a store. It's safe to
// use concurrently.
type Manager struct {
	store       Store
	broadcaster *broadcaster.Broadcaster
	tracer      trace.Tracer
	session     SessionFunc
	retention   time.Duration
	maxSize     int64
	now         func() time.Time

	// A resource of a session is created, or linked, with the session's
	// existence checked, with this held, as the session's resources are
	// deleted, so one which is created as its session is deleted, such as by
	// a slow upload, is deleted with the others
	sessions sync.Mutex

	// Content which is written, and the resource which refers to it, are
	// written with the lock read, so the cleanup, which removes content no
	// resource refers to, with the lock held, doesn't remove it before the
	// resource which refers to it is created
	contents sync.RWMutex
}

// resource is a resource, as an llm.Resource, whose content is read from
// the store
type resource struct {
	manager *Manager
	schema.Resource
}

var _ llm.Resource = (*resource)(nil)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// sniffSize is how much of the content its type is detected from
	sniffSize = 512
)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// New returns the resources in the store, with the options, such as the
// broadcaster they publish their changes to. Returns ErrBadParameter when
// there's no store, or an option isn't valid.
func New(store Store, opts ...Opt) (*Manager, error) {
	var o opt
	if store == nil {
		return nil, llm.ErrBadParameter.With("missing resource store")
	} else if err := o.apply(opts); err != nil {
		return nil, err
	}
	if o.retention == 0 {
		o.retention = schema.DefaultRetention
	}
	if o.maxSize == 0 {
		o.maxSize = schema.DefaultMaxSize
	}
	return &Manager{store: store, broadcaster: o.broadcaster, tracer: o.tracer, session: o.session, retention: o.retention, maxSize: o.maxSize, now: time.Now}, nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Create keeps the content, as it's read, as a resource, and returns it,
// and whether it was created, rather than there being one with its content
// in its session already, which then has its name, description and type,
// when they're given, and is used again. Its type, when it isn't given, or
// is application/octet-stream, is its name's, or is detected from its
// content. Returns ErrNotFound when its session doesn't exist,
// *http.MaxBytesError when its content is larger than the most which is
// kept, ErrBadParameter when it isn't valid, and the store's errors.
func (r *Manager) Create(ctx context.Context, meta schema.ResourceMeta, content io.Reader) (_ schema.Resource, _ bool, err error) {
	ctx, endSpan := otel.StartSpan(r.tracer, ctx, "resource.Create",
		attribute.String("arg.meta", types.Stringify(meta)),
	)
	defer func() { endSpan(err) }()

	if content == nil {
		return schema.Resource{}, false, llm.ErrBadParameter.With("missing content")
	} else if err := r.sessionExists(ctx, meta.Session); err != nil {
		return schema.Resource{}, false, err
	}
	meta.Name = path.Base(strings.TrimSpace(meta.Name))
	if meta.Name == "." || meta.Name == "/" {
		meta.Name = ""
	}

	// Its type is detected from its name, or the start of its content,
	// unless it's given, which it isn't when it's application/octet-stream
	buffered := bufio.NewReaderSize(content, sniffSize)
	given := givenType(meta.Type)
	if meta.Type, err = contentType(meta.Type, meta.Name, buffered); err != nil {
		return schema.Resource{}, false, err
	}

	// Its content, and it, are kept, with the lock read, so the content
	// isn't removed, until it's kept
	r.contents.RLock()
	defer r.contents.RUnlock()
	hash, size, err := r.store.Write(ctx, &limitReader{r: buffered, limit: r.maxSize})
	if err != nil {
		return schema.Resource{}, false, err
	}

	// Its session is checked again, as it may have been deleted as the
	// content was written, with the session's resources held, so it's
	// created before they're deleted, or not at all
	r.sessions.Lock()
	defer r.sessions.Unlock()
	if err := r.sessionExists(ctx, meta.Session); err != nil {
		return schema.Resource{}, false, err
	}
	now := r.now().UTC()
	created := schema.Resource{Hash: hash, Size: size, Created: now, Used: now, ResourceMeta: meta}
	if err := r.store.Create(ctx, created); err == nil {
		r.publish(eventschema.EventCreate, created)
		return created, true, nil
	} else if !errors.Is(err, llm.ErrConflict) {
		return schema.Resource{}, false, err
	}

	// There's one with its content in its session, which is used again
	updated, err := r.store.Update(ctx, created.Key(), func(existing *schema.Resource) error {
		if meta.Name != "" {
			existing.Name = meta.Name
		}
		if meta.Description != "" {
			existing.Description = meta.Description
		}
		if given {
			existing.Type = meta.Type
		}
		existing.Used = now
		return nil
	})
	if err != nil {
		return schema.Resource{}, false, err
	}
	r.publish(eventschema.EventUpdate, updated)
	return updated, false, nil
}

// Get returns the resource with the key, or ErrNotFound when there's none
func (r *Manager) Get(ctx context.Context, key schema.ResourceKey) (_ schema.Resource, err error) {
	ctx, endSpan := otel.StartSpan(r.tracer, ctx, "resource.Get",
		attribute.String("arg.key", key.String()),
	)
	defer func() { endSpan(err) }()
	return r.store.Get(ctx, key)
}

// List returns the resources of the request's session, or every resource,
// in a session, or on its own, without one, the most recently used first, from its offset, and at most its
// limit of them, which is schema.DefaultListLimit, unless it says otherwise, up to
// schema.MaxListLimit, and how many there are. Returns ErrBadParameter when the
// session isn't a UUID.
func (r *Manager) List(ctx context.Context, req schema.ResourceListRequest) (_ schema.ResourceList, err error) {
	ctx, endSpan := otel.StartSpan(r.tracer, ctx, "resource.List",
		attribute.String("arg.req", types.Stringify(req)),
	)
	defer func() { endSpan(err) }()

	session, err := schema.ParseSession(req.Session)
	if err != nil {
		return schema.ResourceList{}, err
	}
	all, err := r.store.List(ctx)
	if err != nil {
		return schema.ResourceList{}, err
	}
	if session != nil {
		all = slices.DeleteFunc(all, func(resource schema.Resource) bool {
			return !sameSession(resource.Session, session)
		})
	}
	// The most recently used first, and then by hash, and session, as the
	// same content can be in more than one session, used at the same time,
	// so the order, and the pages, are always the same
	slices.SortFunc(all, func(a, b schema.Resource) int {
		if c := b.Used.Compare(a.Used); c != 0 {
			return c
		} else if c := strings.Compare(a.Hash, b.Hash); c != 0 {
			return c
		}
		return strings.Compare(sessionKey(a.Session), sessionKey(b.Session))
	})

	// The page of them
	limit := uint(schema.DefaultListLimit)
	if req.Limit != nil {
		limit = min(*req.Limit, schema.MaxListLimit)
	}
	list := schema.ResourceList{ResourceListRequest: req, Count: uint(len(all))}
	if start := min(req.Offset, uint(len(all))); start < uint(len(all)) {
		list.Body = all[start:min(start+limit, uint(len(all)))]
	}
	return list, nil
}

// Open returns the resource with the key, and a reader of its content,
// which is closed when it's read, and it's used. Returns ErrNotFound when
// there's none.
func (r *Manager) Open(ctx context.Context, key schema.ResourceKey) (_ schema.Resource, _ io.ReadCloser, err error) {
	ctx, endSpan := otel.StartSpan(r.tracer, ctx, "resource.Open",
		attribute.String("arg.key", key.String()),
	)
	defer func() { endSpan(err) }()

	resource, err := r.use(ctx, key)
	if err != nil {
		return schema.Resource{}, nil, err
	}
	content, err := r.store.Open(ctx, resource.Hash)
	if err != nil {
		return schema.Resource{}, nil, err
	}
	return resource, content, nil
}

// Resource returns the resource with the URI, which is resource://<hash>,
// in the session, or on its own, when it's nil, as an llm.Resource, whose
// content is read from the store, so a tool, or a model, can read it, and
// it's used. Returns ErrBadParameter when the URI isn't a resource's, and
// ErrNotFound when there's no resource with it in the session.
func (r *Manager) Resource(ctx context.Context, uri string, session *uuid.UUID) (llm.Resource, error) {
	hash, err := schema.ParseURI(uri)
	if err != nil {
		return nil, err
	}
	stored, err := r.use(ctx, schema.ResourceKey{Hash: hash, Session: session})
	if err != nil {
		return nil, err
	}
	return &resource{manager: r, Resource: stored}, nil
}

// Link links the resource with the URI, which is resource://<hash>, which is
// on its own, to the session, as a resource of the session, with the one
// content, and what it is, so it's kept with the session, and returns it,
// or the session's, when it's linked to it already. Returns ErrBadParameter
// when the URI isn't a resource's, ErrNotFound when there's no resource on
// its own with it, or the session doesn't exist, and the store's errors.
func (r *Manager) Link(ctx context.Context, uri string, session uuid.UUID) (_ schema.Resource, err error) {
	ctx, endSpan := otel.StartSpan(r.tracer, ctx, "resource.Link",
		attribute.String("arg.uri", uri),
		attribute.String("arg.session", session.String()),
	)
	defer func() { endSpan(err) }()

	hash, err := schema.ParseURI(uri)
	if err != nil {
		return schema.Resource{}, err
	}

	// With the session's resources held, so it isn't linked to a session as
	// it's deleted, unless it's deleted with the others
	r.sessions.Lock()
	defer r.sessions.Unlock()
	if err := r.sessionExists(ctx, &session); err != nil {
		return schema.Resource{}, err
	}
	key := schema.ResourceKey{Hash: hash, Session: &session}
	if linked, err := r.use(ctx, key); err == nil {
		return linked, nil
	} else if !errors.Is(err, llm.ErrNotFound) {
		return schema.Resource{}, err
	}

	// A resource of the session, which is the one on its own, with the lock
	// read, so its content isn't removed, as it's linked
	r.contents.RLock()
	defer r.contents.RUnlock()
	alone, err := r.use(ctx, schema.ResourceKey{Hash: hash})
	if err != nil {
		return schema.Resource{}, err
	}
	now := r.now().UTC()
	linked := alone
	linked.Session, linked.Created, linked.Used = &session, now, now
	if err := r.store.Create(ctx, linked); errors.Is(err, llm.ErrConflict) {
		return r.store.Get(ctx, key)
	} else if err != nil {
		return schema.Resource{}, err
	}
	r.publish(eventschema.EventCreate, linked)
	return linked, nil
}

// Delete deletes the resource with the key, publishes that it was, and
// returns it, as it was. Its content is kept, as other resources may refer
// to it, until the resources are cleaned, when it's removed, once none
// does. Returns ErrNotFound when there's none.
func (r *Manager) Delete(ctx context.Context, key schema.ResourceKey) (_ schema.Resource, err error) {
	ctx, endSpan := otel.StartSpan(r.tracer, ctx, "resource.Delete",
		attribute.String("arg.key", key.String()),
	)
	defer func() { endSpan(err) }()

	deleted, err := r.store.Delete(ctx, key, nil)
	if err != nil {
		return schema.Resource{}, err
	}
	r.publish(eventschema.EventDelete, deleted)

	return deleted, nil
}

// DeleteSession deletes the resources in the session, which was deleted,
// and returns how many were deleted. Their content is kept, until the
// resources are cleaned, as Delete keeps it.
func (r *Manager) DeleteSession(ctx context.Context, session uuid.UUID) (_ int, err error) {
	ctx, endSpan := otel.StartSpan(r.tracer, ctx, "resource.DeleteSession",
		attribute.String("arg.session", session.String()),
	)
	defer func() { endSpan(err) }()

	// With the session's resources held, so one which is being created is
	// created first, and deleted with them
	r.sessions.Lock()
	defer r.sessions.Unlock()
	all, err := r.store.List(ctx)
	if err != nil {
		return 0, err
	}
	var errs []error
	deleted := 0
	for _, resource := range all {
		if resource.Session == nil || *resource.Session != session {
			continue
		}
		if _, err := r.Delete(ctx, resource.Key()); err != nil && !errors.Is(err, llm.ErrNotFound) {
			errs = append(errs, err)
		} else if err == nil {
			deleted++
		}
	}
	return deleted, errors.Join(errs...)
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - RESOURCE

// URI returns the resource's URI, which is resource://<hash>
func (r *resource) URI() string {
	return r.Resource.URI()
}

// Name returns the resource's name
func (r *resource) Name() string {
	return r.Resource.Name
}

// Description returns the resource's description
func (r *resource) Description() string {
	return r.Resource.Description
}

// Type returns the resource's type
func (r *resource) Type() string {
	return r.Resource.Type
}

// Read returns a function which reads the resource's content, through the
// manager, each time it's called, so the resource is used, and it's read
// only while it exists
func (r *resource) Read(context.Context) (llm.ReadFunc, error) {
	return func(ctx context.Context, fn func(io.Reader) error) error {
		_, content, err := r.manager.Open(ctx, r.Key())
		if err != nil {
			return err
		}
		defer content.Close()
		return fn(contextReader{ctx, content})
	}, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// use returns the resource with the key, which was used now
func (r *Manager) use(ctx context.Context, key schema.ResourceKey) (schema.Resource, error) {
	now := r.now().UTC()
	return r.store.Update(ctx, key, func(resource *schema.Resource) error {
		resource.Used = now
		return nil
	})
}

// sessionExists returns nil when there's no session, or it exists, or
// there's no way to know, or ErrNotFound when it doesn't
func (r *Manager) sessionExists(ctx context.Context, session *uuid.UUID) error {
	if session == nil || r.session == nil {
		return nil
	}
	return r.session(ctx, *session)
}

// publish publishes the change of the resource, with its session
func (r *Manager) publish(action eventschema.EventAction, resource schema.Resource) {
	if r.broadcaster == nil {
		return
	}
	var event schema.ResourceEvent
	if resource.Session != nil {
		event.Session = resource.Session.String()
	}
	data, _ := json.Marshal(event)
	r.broadcaster.Publish(eventschema.Event{Action: action, Schema: schema.Name, Table: schema.ResourceTable, Row: resource.Hash, Data: data})
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// sameSession returns whether the sessions are the same, or both none
func sameSession(a, b *uuid.UUID) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

// givenType returns whether the type is given, rather than empty, or
// application/octet-stream, which says only that it isn't known
func givenType(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(strings.TrimSpace(contentType))
	return err == nil && mediaType != types.ContentTypeBinary
}

// contentType returns the type, when it's given, and isn't
// application/octet-stream, or the name's, by its extension, or the type
// detected from the start of the content. Returns ErrBadParameter when the
// type which is given isn't a MIME type.
func contentType(given, name string, content *bufio.Reader) (string, error) {
	if given != "" {
		mediaType, params, err := mime.ParseMediaType(given)
		if err != nil || !strings.Contains(mediaType, "/") {
			return "", llm.ErrBadParameter.Withf("invalid type %q", given)
		} else if mediaType != types.ContentTypeBinary {
			return mime.FormatMediaType(mediaType, params), nil
		}
	}
	if ext := path.Ext(name); ext != "" {
		if byName := mime.TypeByExtension(ext); byName != "" {
			return byName, nil
		}
	}
	start, err := content.Peek(sniffSize)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, bufio.ErrBufferFull) {
		return "", err
	}
	return http.DetectContentType(start), nil
}

// limitReader is a reader of at most the limit of bytes, which returns
// *http.MaxBytesError when there are more
type limitReader struct {
	r     io.Reader
	limit int64
	read  int64
}

func (l *limitReader) Read(p []byte) (int, error) {
	n, err := l.r.Read(p)
	l.read += int64(n)
	if l.read > l.limit {
		return n, &http.MaxBytesError{Limit: l.limit}
	}
	return n, err
}

// sessionKey returns the session, as text, which orders a resource on its
// own, which has none, first
func sessionKey(session *uuid.UUID) string {
	if session == nil {
		return ""
	}
	return session.String()
}
