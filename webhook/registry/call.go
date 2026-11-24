package registry

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"io"
	"net/http"
	"time"
	"uuid"

	// Packages
	otel "github.com/mutablelogic/go-client/pkg/otel"
	llm "github.com/mutablelogic/go-llm"
	protocol "github.com/mutablelogic/go-llm/webhook/protocol"
	schema "github.com/mutablelogic/go-llm/webhook/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
	attribute "go.opentelemetry.io/otel/attribute"
	trace "go.opentelemetry.io/otel/trace"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Received is what a request of a webhook was: a call, which was kept, a
// handshake, which a service checks the webhook with, as it's registered,
// whose response is returned, and which isn't kept, or a delivery which was
// sent again, which isn't kept again
type Received struct {
	// Call is the call, as it was kept, with its serial number
	Call schema.Call

	// Handshake is the response to a handshake, when it was one
	Handshake []byte

	// IsHandshake is whether it was a handshake
	IsHandshake bool

	// Duplicate is whether it was a delivery which was sent again
	Duplicate bool
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Call receives a request of the webhook with the token, which is in the
// URL its service calls, and returns what it was: a handshake, whose
// response is returned, a delivery which was sent again, or a call, which is
// kept, as it was received, with what its protocol says happened, and
// whether it was verified, with the webhook's secret. A call which is
// refused, such as one which isn't verified, with a webhook's secret, or
// whose body is too large, is kept too, with why, so a secret which is
// wrong can be seen, and it's returned, with its error.
//
// Returns ErrNotFound when no webhook has the token, ErrForbidden when the
// call isn't verified, and ErrBadParameter when it isn't one which the
// service makes, such as without its event, or its body is larger than the
// registry's largest, or isn't what its content type says.
func (r *Registry) Call(ctx context.Context, token string, req *http.Request) (_ Received, err error) {
	ctx, endSpan := otel.StartSpan(r.tracer, ctx, "webhook.registry.Call",
		attribute.String("arg.method", req.Method),
	)
	defer func() { endSpan(err) }()

	// The webhook, of the token
	webhook, err := r.store.GetWebhookByHash(ctx, Hash(token))
	if err != nil {
		return Received{}, err
	}
	return r.receive(ctx, webhook, req, false)
}

// TestCall makes a test call to the webhook with the serial number, which
// its service could make, signed as the signature says, and returns it, as
// it was kept, whether it was accepted, or refused. It's received as any
// call is, apart from that it's marked as a test, which a service's call
// can't be, and that the webhook's URL isn't called.
//
// Returns ErrNotFound when there's no webhook, and ErrBadParameter when the
// signature isn't one, or is good, and the webhook has no secret.
func (r *Registry) TestCall(ctx context.Context, test schema.TestRequest) (_ schema.Call, err error) {
	ctx, endSpan := otel.StartSpan(r.tracer, ctx, "webhook.registry.TestCall",
		attribute.String("arg.test", types.Stringify(test)),
	)
	defer func() { endSpan(err) }()

	// The webhook, and its protocol
	webhook, err := r.store.GetWebhook(ctx, test.Webhook)
	if err != nil {
		return schema.Call{}, err
	}
	p, err := protocol.New(webhook.Type)
	if err != nil {
		return schema.Call{}, err
	}

	// A call which its service could make, which is signed
	header, body := p.Sample(uuid.New().String())
	signature := test.Signature
	if signature == "" && webhook.Secret != "" {
		signature = schema.SignatureGood
	}
	switch signature {
	case "", schema.SignatureNone:
	case schema.SignatureGood:
		if webhook.Secret == "" {
			return schema.Call{}, llm.ErrBadParameter.With("the webhook has no secret, so a test call can't be signed with it")
		}
		p.Sign(header, body, webhook.Secret)
	case schema.SignatureBad:
		p.Sign(header, body, rand.Text())
	default:
		return schema.Call{}, llm.ErrBadParameter.Withf("signature %q isn't %s, %s or %s", test.Signature, schema.SignatureGood, schema.SignatureBad, schema.SignatureNone)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "/", bytes.NewReader(body))
	if err != nil {
		return schema.Call{}, err
	}
	req.Header = header

	// One which was refused, and kept, was still made
	received, err := r.receive(ctx, webhook, req, true)
	if received.Call.ID != 0 {
		return received.Call, nil
	}
	return schema.Call{}, err
}

// GetCall returns the call with the serial number, or ErrNotFound when
// there's none
func (r *Registry) GetCall(ctx context.Context, id uint64) (_ schema.Call, err error) {
	ctx, endSpan := otel.StartSpan(r.tracer, ctx, "webhook.registry.GetCall",
		attribute.Int64("arg.id", int64(id)),
	)
	defer func() { endSpan(err) }()
	return r.store.GetCall(ctx, id)
}

// DeleteCall deletes the call with the serial number, and returns it, as it
// was, or ErrNotFound when there's none. Its delivery is kept again, when
// it's sent again.
func (r *Registry) DeleteCall(ctx context.Context, id uint64) (_ schema.Call, err error) {
	ctx, endSpan := otel.StartSpan(r.tracer, ctx, "webhook.registry.DeleteCall",
		attribute.Int64("arg.id", int64(id)),
	)
	defer func() { endSpan(err) }()
	return r.store.DeleteCall(ctx, id)
}

// ListCalls returns the calls, of the webhook, or the session's webhooks,
// when the request has one, which were accepted, or refused, or both, of
// its status, after the call with its serial number, and received since its
// time, oldest first, so what watches them carries on after the last it had,
// or newest, when the request says so, and how many there are. Returns ErrBadParameter when its status isn't one.
func (r *Registry) ListCalls(ctx context.Context, req schema.CallListRequest) (_ schema.CallList, err error) {
	ctx, endSpan := otel.StartSpan(r.tracer, ctx, "webhook.registry.ListCalls",
		attribute.String("arg.req", types.Stringify(req)),
	)
	defer func() { endSpan(err) }()
	return r.store.ListCalls(ctx, req)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// receive receives a request of the webhook, or a test of it, and returns
// what it was, with its error, when it was refused
func (r *Registry) receive(ctx context.Context, webhook schema.Webhook, req *http.Request, test bool) (Received, error) {
	p, err := protocol.New(webhook.Type)
	if err != nil {
		return Received{}, err
	}
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(attribute.Int64("webhook.id", int64(webhook.ID)), attribute.String("webhook.type", p.Name()))

	// A handshake is answered, and isn't kept
	if response, ok := p.Handshake(req, webhook.Secret); ok {
		return Received{Handshake: response, IsHandshake: true}, nil
	}

	// What it was, as it was received, which is kept, whether it's
	// refused, or not
	call := schema.Call{
		Webhook:  webhook.ID,
		Received: time.Now(),
		Method:   req.Method,
		Query:    req.URL.Query(),
		Headers:  protocol.KeptHeaders(p, req.Header),
		Test:     test,
	}

	// Its body, as it was sent, up to the largest, which is refused, without
	// it, beyond it
	body, err := r.readBody(req)
	if err != nil {
		return r.refuse(ctx, call, err)
	}
	call.Body = body

	// What it is, and what happened, which is known, when it can be decoded,
	// whether it's verified, or not, so a call which is refused says what
	// it was
	values, decodeErr := p.Decode(req.Header, body)
	if decodeErr == nil {
		call.Values = values
		call.Event, call.Delivery = p.Identify(req.Header, values)
	}

	// It's verified, when the webhook has a secret, and refused, when it
	// isn't, or it isn't one which the service makes, or can't be decoded
	if webhook.Secret != "" {
		if err := p.Verify(req.Header, body, webhook.Secret); err != nil {
			return r.refuse(ctx, call, err)
		}
		call.Verified = true
	}
	if err := p.Check(req.Method, req.Header); err != nil {
		return r.refuse(ctx, call, err)
	}
	if decodeErr != nil {
		return r.refuse(ctx, call, decodeErr)
	}
	span.SetAttributes(attribute.String("webhook.event", call.Event), attribute.String("webhook.delivery", call.Delivery), attribute.Bool("webhook.verified", call.Verified))

	// It's kept, unless it's a delivery which was sent again
	kept, err := r.store.CreateCall(ctx, call)
	if errors.Is(err, llm.ErrConflict) && call.Delivery != "" {
		return Received{Duplicate: true}, nil
	} else if err != nil {
		return Received{}, err
	}
	return Received{Call: kept}, nil
}

// readBody returns the request's body, or ErrBadParameter, when it's larger
// than the largest, which isn't read beyond
func (r *Registry) readBody(req *http.Request) ([]byte, error) {
	if req.Body == nil {
		return nil, nil
	}
	defer req.Body.Close()
	if req.ContentLength > r.maxBody {
		return nil, llm.ErrBadParameter.Withf("the call's body is larger than %d bytes", r.maxBody)
	}
	body, err := io.ReadAll(io.LimitReader(req.Body, r.maxBody+1))
	if err != nil {
		return nil, llm.ErrBadParameter.Withf("the call's body: %v", err)
	} else if int64(len(body)) > r.maxBody {
		return nil, llm.ErrBadParameter.Withf("the call's body is larger than %d bytes", r.maxBody)
	}
	return body, nil
}

// refuse keeps the call, which was refused, with why, and whether it was
// verified, as one which was can be refused, such as when it can't be
// decoded. It isn't a delivery which was sent again, when it's kept. It
// returns the call, when it's kept, and why, whether it's kept, or not.
func (r *Registry) refuse(ctx context.Context, call schema.Call, reason error) (Received, error) {
	call.Error = reason.Error()
	trace.SpanFromContext(ctx).SetAttributes(attribute.String("webhook.refused", call.Error))
	kept, err := r.store.CreateCall(ctx, call)
	if err != nil {
		return Received{}, errors.Join(reason, err)
	}
	return Received{Call: kept}, reason
}
