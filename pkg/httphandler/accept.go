package httphandler

import (
	"net/http"
	"strings"
)

// acceptKind classifies the negotiated response format.
type acceptKind int

const (
	acceptJSON        acceptKind = iota // application/json (or no Accept header)
	acceptStream                        // text/event-stream
	acceptUnsupported                   // unsupported media type
)

// acceptType inspects the Accept header and returns the negotiated format.
// When no Accept header is present, defaults to JSON.
func acceptType(r *http.Request) acceptKind {
	header := r.Header.Get("Accept")
	if header == "" {
		return acceptJSON
	}
	for part := range strings.SplitSeq(header, ",") {
		mt := strings.TrimSpace(part)
		if idx := strings.IndexByte(mt, ';'); idx >= 0 {
			mt = strings.TrimSpace(mt[:idx])
		}
		switch mt {
		case "text/event-stream":
			return acceptStream
		case "application/json", "*/*":
			return acceptJSON
		}
	}
	return acceptUnsupported
}
