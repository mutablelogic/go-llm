package schema

import (
	"strings"
	"testing"

	// Packages
	uuid "github.com/google/uuid"
	pg "github.com/mutablelogic/go-pg"
)

func TestMemoryListRequestSelectUsesWebsearchSyntax(t *testing.T) {
	session := uuid.New()
	bind := pg.NewBind("memory.list_recursive", "SELECT")

	query, err := (MemoryListRequest{Session: &session, Q: `"david" OR berlin`}).Select(bind, pg.List)
	if err != nil {
		t.Fatal(err)
	}
	if query != "SELECT" {
		t.Fatalf("unexpected query: %q", query)
	}
	where, _ := bind.Get("where").(string)
	if !strings.HasPrefix(where, "AND ") {
		t.Fatalf("expected recursive query filters to be AND-prefixed, got %q", where)
	}
	if !strings.Contains(where, "websearch_to_tsquery('simple'") {
		t.Fatalf("expected websearch_to_tsquery in where clause, got %q", where)
	}
	if strings.Contains(where, "plainto_tsquery") {
		t.Fatalf("did not expect plainto_tsquery in where clause, got %q", where)
	}
	if q, _ := bind.Get("q").(string); q != `"david" OR berlin` {
		t.Fatalf("unexpected q binding: %q", q)
	}
	if got := bind.Get("session"); got != session {
		t.Fatalf("unexpected session binding: %v", got)
	}
}

func TestMemoryListRequestSelectWildcardOmitsSearchFilter(t *testing.T) {
	session := uuid.New()
	bind := pg.NewBind("memory.list_recursive", "SELECT")

	_, err := (MemoryListRequest{Session: &session, Q: "*"}).Select(bind, pg.List)
	if err != nil {
		t.Fatal(err)
	}
	where, _ := bind.Get("where").(string)
	if strings.Contains(where, "tsquery") {
		t.Fatalf("did not expect text search filter in where clause, got %q", where)
	}
	if where != "" {
		t.Fatalf("expected no extra where clause for wildcard search, got %q", where)
	}
	if bind.Get("q") != nil {
		t.Fatalf("did not expect q binding for wildcard search, got %v", bind.Get("q"))
	}
}

func TestMemoryListRequestSelectUsesRecursiveSessionQuery(t *testing.T) {
	session := uuid.New()
	bind := pg.NewBind("memory.list_recursive", "SELECT")

	query, err := (MemoryListRequest{Session: &session}).Select(bind, pg.List)
	if err != nil {
		t.Fatal(err)
	}
	if query != "SELECT" {
		t.Fatalf("unexpected query: %q", query)
	}
	if got := bind.Get("session"); got != session {
		t.Fatalf("unexpected session binding: %v", got)
	}
}

func TestMemoryListRequestSelectWithoutSessionUsesPlainListQuery(t *testing.T) {
	bind := pg.NewBind("memory.list", "SELECT")

	query, err := (MemoryListRequest{}).Select(bind, pg.List)
	if err != nil {
		t.Fatal(err)
	}
	if query != "SELECT" {
		t.Fatalf("unexpected query: %q", query)
	}
	if got := bind.Get("session"); got != nil {
		t.Fatalf("did not expect session binding, got %v", got)
	}
}

func TestMemoryRecursiveQueryPrefersChildSessionValues(t *testing.T) {
	if !strings.Contains(Queries, `SELECT DISTINCT ON (memory."key")`) {
		t.Fatalf("expected recursive memory query to de-duplicate by key")
	}
	if !strings.Contains(Queries, `session_tree.depth ASC`) {
		t.Fatalf("expected recursive memory query to prioritize nearest child session")
	}
}

func TestMemorySelectorSelectUsesRecursiveSessionLookup(t *testing.T) {
	session := uuid.New()
	bind := pg.NewBind("memory.select", "SELECT")

	query, err := (MemorySelector{Session: session, Key: "topic"}).Select(bind, pg.Get)
	if err != nil {
		t.Fatal(err)
	}
	if query != "SELECT" {
		t.Fatalf("unexpected query: %q", query)
	}
	if got := bind.Get("session"); got != session {
		t.Fatalf("unexpected session binding: %v", got)
	}
	if got := bind.Get("key"); got != "topic" {
		t.Fatalf("unexpected key binding: %v", got)
	}
	if !strings.Contains(Queries, `ORDER BY
	session_tree.depth ASC`) {
		t.Fatalf("expected memory.select to prefer nearest child session")
	}
	if !strings.Contains(Queries, `LIMIT 1;`) {
		t.Fatalf("expected memory.select to return a single effective value")
	}
}
