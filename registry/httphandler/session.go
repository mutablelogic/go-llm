package httphandler

import (
	"errors"
	"net/http"
	"uuid"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	providerschema "github.com/mutablelogic/go-llm/provider/schema"
	registry "github.com/mutablelogic/go-llm/registry"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	httprequest "github.com/mutablelogic/go-server/pkg/httprequest"
	httpresponse "github.com/mutablelogic/go-server/pkg/httpresponse"
	httprouter "github.com/mutablelogic/go-server/pkg/httprouter"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
	openapi "github.com/mutablelogic/go-server/pkg/openapi"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// sessionID is the path parameter of a session's identifier
type sessionID struct {
	ID uuid.UUID `json:"id" help:"Session identifier"`
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func sessionHandlers(r *registry.Registry, router *httprouter.Router, documentation *openapi.MarkdownDoc) error {
	router.Spec().AddTag("Sessions", documentation.Section(2, "Session Operations").Body)
	return errors.Join(
		sessionCreateListHandlers(r, router, documentation),
		sessionGetUpdateDeleteHandlers(r, router, documentation),
		sessionMessagesHandlers(r, router, documentation),
		sessionCompactHandlers(r, router, documentation),
		sessionStatusHandlers(r, router, documentation),
	)
}

func sessionCreateListHandlers(reg *registry.Registry, router *httprouter.Router, documentation *openapi.MarkdownDoc) error {
	return router.Register("llm/session", nil, func(path httprequest.PathItem) {
		path.Tag("Sessions")

		// GET
		path.Get(func(w http.ResponseWriter, r *http.Request) {
			var req schema.SessionListRequest
			if err := httprequest.Query(r.URL.Query(), &req); err != nil {
				httpresponse.Error(w, httpresponse.ErrBadRequest.With(err.Error()))
			} else if list, err := reg.ListSessions(r.Context(), req); err != nil {
				httpresponse.Error(w, llm.HTTPErr(err), req.String())
			} else {
				httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), list)
			}
		}, func(op httprequest.PathOperation) {
			op.Summary("List Sessions")
			op.Description(documentation.Section(3, "GET /llm/session").Body)
			op.Query(jsonschema.MustFor[schema.SessionListRequest]())
			op.JSONResponse(http.StatusOK, jsonschema.MustFor[schema.SessionList](), "List of sessions, without their messages")
			op.ErrorResponse(http.StatusBadRequest, "Invalid request")
			op.ErrorResponse(http.StatusServiceUnavailable, "The server has no session store")
		})

		// POST
		path.Post(func(w http.ResponseWriter, r *http.Request) {
			var meta schema.SessionMeta
			if err := httprequest.Read(r, &meta); err != nil {
				httpresponse.Error(w, httpresponse.ErrBadRequest.Withf("%w", err))
			} else if session, err := reg.CreateSession(r.Context(), meta); err != nil {
				httpresponse.Error(w, llm.HTTPErr(err))
			} else {
				httpresponse.JSON(w, http.StatusCreated, httprequest.Indent(r), session)
			}
		}, func(op httprequest.PathOperation) {
			op.Summary("Create Session")
			op.Description(documentation.Section(3, "POST /llm/session").Body)
			op.RequestBody(jsonschema.MustFor[schema.SessionMeta]())
			op.JSONResponse(http.StatusCreated, jsonschema.MustFor[schema.Session](), "Created session")
			op.ErrorResponse(http.StatusBadRequest, "Invalid request, an invalid schema, or an option which is a field")
			op.ErrorResponse(http.StatusNotFound, "Model or tool not found")
			op.ErrorResponse(http.StatusConflict, "The name of the model, or a tool, is ambiguous")
			op.ErrorResponse(http.StatusServiceUnavailable, "The server has no session store")
		})
	})
}

func sessionGetUpdateDeleteHandlers(reg *registry.Registry, router *httprouter.Router, documentation *openapi.MarkdownDoc) error {
	return router.Register("llm/session/{id}", jsonschema.MustFor[sessionID](), func(path httprequest.PathItem) {
		// Chat is inference, and the rest of the session's operations aren't,
		// so each has its own tag

		// GET
		path.Get(func(w http.ResponseWriter, r *http.Request) {
			id, err := pathSessionID(r)
			if err != nil {
				httpresponse.Error(w, err)
			} else if session, err := reg.GetSession(r.Context(), id); err != nil {
				httpresponse.Error(w, llm.HTTPErr(err), id.String())
			} else {
				httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), session)
			}
		}, func(op httprequest.PathOperation) {
			op.Tags("Sessions")
			op.Summary("Get Session")
			op.Description(documentation.Section(3, "GET /llm/session/{id}").Body)
			op.JSONResponse(http.StatusOK, jsonschema.MustFor[schema.Session](), "Session, without its messages")
			op.ErrorResponse(http.StatusBadRequest, "The identifier isn't a UUID")
			op.ErrorResponse(http.StatusNotFound, "Session not found")
			op.ErrorResponse(http.StatusServiceUnavailable, "The server has no session store")
		})

		// POST, adding messages to the session, which its model replies to
		path.Post(func(w http.ResponseWriter, r *http.Request) {
			// The body is limited, as messages can have images, which are
			// read into memory
			var req schema.ChatRequest
			id, err := pathSessionID(r)
			if err != nil {
				httpresponse.Error(w, err)
			} else if err := limitBody(w, r, schema.AskBodyLimit); err != nil {
				httpresponse.Error(w, err)
			} else if err := httprequest.Read(r, &req); err != nil {
				httpresponse.Error(w, httpresponse.ErrBadRequest.Withf("%w", err))
			} else {
				// What the system says of a message is the server's, so a
				// client's annotations are taken out
				messages := make([]llm.Message, len(req.Messages))
				for i := range req.Messages {
					messages[i] = &providerschema.Message{MessageMeta: req.Messages[i].WithoutAnnotations()}
				}
				respond(w, r, id.String(), func(write streamWriteFn) (*schema.ChatResponse, error) {
					// What the person sent is passed on, such as to the
					// session's own chats, before its model replies
					if prompts := reg.Prompts(); prompts != nil {
						prompts(r.Context(), id, messages)
					}
					stream := streamWriter(write)
					return reg.Chat(r.Context(), id, messages, stream)
				})
			}
		}, func(op httprequest.PathOperation) {
			op.Tags("Inference")
			op.Summary("Chat")
			op.Description(documentation.Section(3, "POST /llm/session/{id}").Body)
			op.RequestBody(jsonschema.MustFor[schema.ChatRequest]())
			op.JSONResponse(http.StatusOK, jsonschema.MustFor[schema.ChatResponse](), "The messages added to the session, with the model's reply, and the tokens used, or a stream of server-sent events when text/event-stream is accepted")
			op.TextStreamResponse(http.StatusOK)
			op.ErrorResponse(http.StatusBadRequest, "Invalid request body, no messages, an identifier which isn't a UUID, a session without a model, or a message the provider can't send")
			op.ErrorResponse(http.StatusNotFound, "Session, or its model or a tool, not found")
			op.ErrorResponse(http.StatusNotAcceptable, "The response can't be any of the accepted types")
			op.ErrorResponse(http.StatusConflict, "The session was changed as the model replied, so the reply isn't stored")
			op.ErrorResponse(http.StatusRequestEntityTooLarge, "The request body is larger than 32 MiB")
			op.ErrorResponse(http.StatusNotImplemented, "The model's provider can't generate messages, or the model is known not to, or not to call tools, think, or see images, when it's asked to")
			op.ErrorResponse(http.StatusServiceUnavailable, "The server has no session store, or the model couldn't reply")
		})

		// PATCH
		path.Patch(func(w http.ResponseWriter, r *http.Request) {
			var meta schema.SessionMeta
			id, err := pathSessionID(r)
			if err != nil {
				httpresponse.Error(w, err)
			} else if err := httprequest.Read(r, &meta); err != nil {
				httpresponse.Error(w, httpresponse.ErrBadRequest.Withf("%w", err))
			} else if session, err := reg.UpdateSession(r.Context(), id, meta); err != nil {
				httpresponse.Error(w, llm.HTTPErr(err), id.String())
			} else {
				httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), session)
			}
		}, func(op httprequest.PathOperation) {
			op.Tags("Sessions")
			op.Summary("Update Session")
			op.Description(documentation.Section(3, "PATCH /llm/session/{id}").Body)
			op.RequestBody(jsonschema.MustFor[schema.SessionMeta]())
			op.JSONResponse(http.StatusOK, jsonschema.MustFor[schema.Session](), "Updated session")
			op.ErrorResponse(http.StatusBadRequest, "Invalid request, an identifier which isn't a UUID, an update which changes nothing, or settings which, as changed, aren't valid")
			op.ErrorResponse(http.StatusNotFound, "Session, model or tool not found")
			op.ErrorResponse(http.StatusConflict, "The session was changed as it was updated, or the name of the model, or a tool, is ambiguous")
			op.ErrorResponse(http.StatusServiceUnavailable, "The server has no session store")
		})

		// DELETE
		path.Delete(func(w http.ResponseWriter, r *http.Request) {
			id, err := pathSessionID(r)
			if err != nil {
				httpresponse.Error(w, err)
			} else if session, err := reg.DeleteSession(r.Context(), id); err != nil {
				httpresponse.Error(w, llm.HTTPErr(err), id.String())
			} else {
				httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), session)
			}
		}, func(op httprequest.PathOperation) {
			op.Tags("Sessions")
			op.Summary("Delete Session")
			op.Description(documentation.Section(3, "DELETE /llm/session/{id}").Body)
			op.JSONResponse(http.StatusOK, jsonschema.MustFor[schema.Session](), "Deleted session")
			op.ErrorResponse(http.StatusBadRequest, "The identifier isn't a UUID")
			op.ErrorResponse(http.StatusNotFound, "Session not found")
			op.ErrorResponse(http.StatusConflict, "The session was changed as it was deleted")
			op.ErrorResponse(http.StatusServiceUnavailable, "The server has no session store")
		})
	})
}

func sessionCompactHandlers(reg *registry.Registry, router *httprouter.Router, documentation *openapi.MarkdownDoc) error {
	return router.Register("llm/session/{id}/compact", jsonschema.MustFor[sessionID](), func(path httprequest.PathItem) {
		path.Tag("Sessions")

		// POST, adding a marker, after which its context starts
		path.Post(func(w http.ResponseWriter, r *http.Request) {
			id, err := pathSessionID(r)
			if err != nil {
				httpresponse.Error(w, err)
			} else if session, err := reg.CompactSession(r.Context(), id); err != nil {
				httpresponse.Error(w, llm.HTTPErr(err), id.String())
			} else {
				httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), session)
			}
		}, func(op httprequest.PathOperation) {
			op.Summary("Compact Session")
			op.Description(documentation.Section(3, "POST /llm/session/{id}/compact").Body)
			op.JSONResponse(http.StatusOK, jsonschema.MustFor[schema.Session](), "The session, without its messages")
			op.ErrorResponse(http.StatusBadRequest, "An identifier which isn't a UUID, or no messages since it was last compacted")
			op.ErrorResponse(http.StatusForbidden, "The session is an event's")
			op.ErrorResponse(http.StatusNotFound, "Session not found")
			op.ErrorResponse(http.StatusConflict, "Its last summary is still being written")
			op.ErrorResponse(http.StatusServiceUnavailable, "The server has no session store")
		})
	})
}

func sessionMessagesHandlers(reg *registry.Registry, router *httprouter.Router, documentation *openapi.MarkdownDoc) error {
	return router.Register("llm/session/{id}/messages", jsonschema.MustFor[sessionID](), func(path httprequest.PathItem) {
		path.Tag("Sessions")

		// GET
		path.Get(func(w http.ResponseWriter, r *http.Request) {
			var req schema.MessageListRequest
			id, err := pathSessionID(r)
			if err != nil {
				httpresponse.Error(w, err)
			} else if err := httprequest.Query(r.URL.Query(), &req); err != nil {
				httpresponse.Error(w, httpresponse.ErrBadRequest.With(err.Error()))
			} else if list, err := reg.ListSessionMessages(r.Context(), id, req); err != nil {
				httpresponse.Error(w, llm.HTTPErr(err), id.String())
			} else {
				httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), list)
			}
		}, func(op httprequest.PathOperation) {
			op.Summary("List Session Messages")
			op.Description(documentation.Section(3, "GET /llm/session/{id}/messages").Body)
			op.Query(jsonschema.MustFor[schema.MessageListRequest]())
			op.JSONResponse(http.StatusOK, jsonschema.MustFor[schema.MessageList](), "List of the session's messages, oldest first")
			op.ErrorResponse(http.StatusBadRequest, "Invalid request, or an identifier which isn't a UUID")
			op.ErrorResponse(http.StatusNotFound, "Session not found")
			op.ErrorResponse(http.StatusServiceUnavailable, "The server has no session store")
		})

		// DELETE
		path.Delete(func(w http.ResponseWriter, r *http.Request) {
			id, err := pathSessionID(r)
			if err != nil {
				httpresponse.Error(w, err)
			} else if session, err := reg.ForgetSessionMessages(r.Context(), id); err != nil {
				httpresponse.Error(w, llm.HTTPErr(err), id.String())
			} else {
				httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), session)
			}
		}, func(op httprequest.PathOperation) {
			op.Summary("Forget Session Messages")
			op.Description(documentation.Section(3, "DELETE /llm/session/{id}/messages").Body)
			op.JSONResponse(http.StatusOK, jsonschema.MustFor[schema.Session](), "The session, without its messages")
			op.ErrorResponse(http.StatusBadRequest, "An identifier which isn't a UUID")
			op.ErrorResponse(http.StatusNotFound, "Session not found")
			op.ErrorResponse(http.StatusConflict, "The session was changed as its messages were forgotten")
			op.ErrorResponse(http.StatusServiceUnavailable, "The server has no session store")
		})
	})
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// pathSessionID returns the session's identifier in the request's path, or
// ErrBadRequest when it isn't a UUID
func pathSessionID(r *http.Request) (uuid.UUID, error) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		return uuid.UUID{}, httpresponse.ErrBadRequest.Withf("invalid session identifier %q", r.PathValue("id"))
	}
	return id, nil
}

func sessionStatusHandlers(reg *registry.Registry, router *httprouter.Router, documentation *openapi.MarkdownDoc) error {
	return router.Register("llm/session/{id}/status", jsonschema.MustFor[sessionID](), func(path httprequest.PathItem) {
		path.Tag("Sessions")

		// PUT
		path.Put(func(w http.ResponseWriter, r *http.Request) {
			var req schema.SessionStatusRequest
			id, err := pathSessionID(r)
			if err != nil {
				httpresponse.Error(w, err)
			} else if err := httprequest.Read(r, &req); err != nil {
				httpresponse.Error(w, httpresponse.ErrBadRequest.Withf("%w", err))
			} else if session, err := reg.SetSessionStatus(r.Context(), id, req.Status); err != nil {
				httpresponse.Error(w, llm.HTTPErr(err), id.String())
			} else {
				httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), session)
			}
		}, func(op httprequest.PathOperation) {
			op.Summary("Set Session Status")
			op.Description(documentation.Section(3, "PUT /llm/session/{id}/status").Body)
			op.RequestBody(jsonschema.MustFor[schema.SessionStatusRequest]())
			op.JSONResponse(http.StatusOK, jsonschema.MustFor[schema.Session](), "Session, without its messages, with its status")
			op.ErrorResponse(http.StatusBadRequest, "Invalid request, an identifier which isn't a UUID, or a status which isn't typing or idle")
			op.ErrorResponse(http.StatusNotFound, "Session not found")
			op.ErrorResponse(http.StatusServiceUnavailable, "The server has no session store")
		})
	})
}
