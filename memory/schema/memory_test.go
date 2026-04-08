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
	bind := pg.NewBind("memory.list", "SELECT")

	query, err := (MemoryListRequest{Session: &session, Q: `"david" OR berlin`}).Select(bind, pg.List)
	if err != nil {
		t.Fatal(err)
	}
	if query != "SELECT" {
		t.Fatalf("unexpected query: %q", query)
	}
	where, _ := bind.Get("where").(string)
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
	bind := pg.NewBind("memory.list", "SELECT")

	_, err := (MemoryListRequest{Session: &session, Q: "*"}).Select(bind, pg.List)
	if err != nil {
		t.Fatal(err)
	}
	where, _ := bind.Get("where").(string)
	if strings.Contains(where, "tsquery") {
		t.Fatalf("did not expect text search filter in where clause, got %q", where)
	}
	if !strings.Contains(where, `memory."session" = `) {
		t.Fatalf("expected session filter in where clause, got %q", where)
	}
	if bind.Get("q") != nil {
		t.Fatalf("did not expect q binding for wildcard search, got %v", bind.Get("q"))
	}
}
