package schema

import (
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"strconv"
	"strings"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	tui "github.com/mutablelogic/go-server/pkg/tui"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// MemoryMeta is what's remembered: its key, which identifies it, a line
// which describes it, and its body
type MemoryMeta struct {
	Key         string `json:"key" help:"Key of the memory, which is an identifier, such as kitchen_lights" example:"kitchen_lights"`
	Description string `json:"description" help:"A line which describes what's remembered" example:"Which lights are in the kitchen"`
	Body        string `json:"body" help:"What's remembered, as Markdown"`
}

// Memory is what's remembered, its version, which is one when it's
// remembered, and one more each time it's changed, and when it was
// remembered, and last changed
type Memory struct {
	MemoryMeta
	Version  string    `json:"version" readonly:"" help:"Version of the memory, which is a hash of its description and body, so it changes when either does, and which it's replaced at" example:"3f9a1c0b7d2e"`
	Created  time.Time `json:"created,omitzero" readonly:"" help:"When it was remembered"`
	Modified time.Time `json:"modified,omitzero" readonly:"" help:"When it was last changed"`
}

// MemoryEvent is the data of an event of a memory, which was created, or
// updated: its version
type MemoryEvent struct {
	Version string `json:"version" help:"Version of the memory, as it is now"`
}

// MemoryRequest is a memory to remember, under the key its path has: its
// description and body, and the version of the memory it replaces, or none,
// to remember a new memory
type MemoryRequest struct {
	Description string `json:"description" help:"A line which describes what's remembered" example:"Which lights are in the kitchen"`
	Body        string `json:"body" help:"What's remembered, as Markdown, which replaces all of what was remembered under the key"`
	Version     string `json:"version,omitempty" help:"Version of the memory which it replaces, or none, to remember a new memory" optional:""`
}

// MemoryForgetRequest is what's forgotten of a memory: a line of it, or all
// of it
type MemoryForgetRequest struct {
	Line string `json:"line,omitempty" help:"A line of the memory's body to forget, rather than the whole memory" optional:""`
}

// MemoryListRequest is which memories are listed: those which match the
// query, when there is one, from the offset, and at most the limit of them
type MemoryListRequest struct {
	Q      string  `json:"q,omitempty" help:"Words which each memory has, in its key, description or body, in any case, or every memory" optional:""`
	Offset uint64  `json:"offset,omitempty" help:"How many memories to skip" optional:""`
	Limit  *uint64 `json:"limit,omitempty" help:"Most memories to return" optional:""`
}

// MemoryList is the memories which are listed, the most recently changed
// first, and how many there are
type MemoryList struct {
	MemoryListRequest
	Count uint     `json:"count" help:"How many memories match"`
	Body  []Memory `json:"body,omitzero" help:"The memories"`
}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// MemoryListMax is the most memories which are listed at once
const MemoryListMax uint64 = 100

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - MEMORY

// Validate returns the memory, with its key, description and body trimmed,
// or ErrBadParameter when its key isn't an identifier, or it has no
// description, or body
func (m MemoryMeta) Validate() (MemoryMeta, error) {
	m.Key = strings.TrimSpace(m.Key)
	m.Description = strings.TrimSpace(m.Description)
	m.Body = strings.TrimSpace(m.Body)
	switch {
	case !types.IsIdentifier(m.Key):
		return MemoryMeta{}, llm.ErrBadParameter.Withf("invalid memory key %q, which must be an identifier, such as kitchen_lights", m.Key)
	case m.Description == "":
		return MemoryMeta{}, llm.ErrBadParameter.Withf("memory %q has no description", m.Key)
	case strings.ContainsAny(m.Description, "\r\n"):
		return MemoryMeta{}, llm.ErrBadParameter.Withf("memory %q has a description of more than one line", m.Key)
	case m.Body == "":
		return MemoryMeta{}, llm.ErrBadParameter.Withf("memory %q has no body", m.Key)
	}
	return m, nil
}

// Hash returns the version of what's remembered, which is a hash of its
// description and body, so it changes when either does, however they're
// changed, such as by hand, and is never the version of a memory with other
// content
func (m MemoryMeta) Hash() string {
	sum := sha256.Sum256([]byte(m.Description + "\n" + m.Body))
	return hex.EncodeToString(sum[:6])
}

// Matches returns whether the memory has each word of the query, in its
// key, description or body, in any case, which every memory does, when
// there are none
func (m Memory) Matches(q string) bool {
	text := strings.ToLower(m.Key + "\n" + m.Description + "\n" + m.Body)
	for _, word := range strings.Fields(strings.ToLower(q)) {
		if !strings.Contains(text, word) {
			return false
		}
	}
	return true
}

// WithoutLine returns the body without the lines which are the line, apart
// from the space around them, and a bullet, such as "- ", and whether there
// were any
func (m Memory) WithoutLine(line string) (string, bool) {
	line = bulletless(line)
	if line == "" {
		return m.Body, false
	}
	var kept []string
	removed := false
	for _, l := range strings.Split(m.Body, "\n") {
		if bulletless(l) == line {
			removed = true
		} else {
			kept = append(kept, l)
		}
	}
	return strings.TrimSpace(strings.Join(kept, "\n")), removed
}

func (m Memory) String() string {
	return types.Stringify(m)
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - LIST

// Query returns the request as URL query values
func (r MemoryListRequest) Query() url.Values {
	values := url.Values{}
	if q := strings.TrimSpace(r.Q); q != "" {
		values.Set("q", q)
	}
	if r.Offset > 0 {
		values.Set("offset", strconv.FormatUint(r.Offset, 10))
	}
	if r.Limit != nil {
		values.Set("limit", strconv.FormatUint(*r.Limit, 10))
	}
	return values
}

// Page returns the memories of the request's page, from its offset, and at
// most its limit, or MemoryListMax, of them, and how many there are
func (r MemoryListRequest) Page(memories []Memory) MemoryList {
	limit := MemoryListMax
	if r.Limit != nil {
		limit = min(*r.Limit, MemoryListMax)
	}
	list := MemoryList{MemoryListRequest: r, Count: uint(len(memories))}
	if r.Offset < uint64(len(memories)) {
		memories = memories[r.Offset:]
		list.Body = memories[:min(uint64(len(memories)), limit)]
	}
	return list
}

func (r MemoryListRequest) String() string {
	return types.Stringify(r)
}

func (l MemoryList) String() string {
	return types.Stringify(l)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// bulletless returns the line without the space around it, and a bullet,
// such as "- ", "* " or "+ "
func bulletless(line string) string {
	line = strings.TrimSpace(line)
	for _, bullet := range []string{"- ", "* ", "+ "} {
		if rest, ok := strings.CutPrefix(line, bullet); ok {
			return strings.TrimSpace(rest)
		}
	}
	return line
}

///////////////////////////////////////////////////////////////////////////////
// TABLE

func (Memory) Header() []string {
	return []string{"Key", "Description", "Updated"}
}

func (Memory) Width(int) int {
	return 0
}

// Cell returns the memory's key, description, and when it was last
// changed, in local time
func (m Memory) Cell(col int) tui.CellText {
	switch col {
	case 0:
		return m.Key
	case 1:
		return m.Description
	case 2:
		when := m.Created
		if m.Modified.After(when) {
			when = m.Modified
		}
		if when.IsZero() {
			return ""
		}
		return when.Local().Format(time.DateTime)
	}
	return ""
}
