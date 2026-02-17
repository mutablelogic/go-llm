package store_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	assert "github.com/stretchr/testify/assert"
)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

var testMeta = schema.SessionMeta{
	Name:          "test",
	GeneratorMeta: schema.GeneratorMeta{Model: "test-model", Provider: "test-provider"},
}

func textPtr(s string) *string { return &s }

///////////////////////////////////////////////////////////////////////////////
// SHARED SESSION STORE TESTS

type sessionStoreTest struct {
	Name string
	Fn   func(*testing.T, schema.SessionStore)
}

var sessionStoreTests = []sessionStoreTest{
	// Create
	{"CreateSuccess", func(t *testing.T, s schema.SessionStore) {
		assert := assert.New(t)
		session, err := s.CreateSession(context.TODO(), schema.SessionMeta{
			Name:          "my chat",
			GeneratorMeta: schema.GeneratorMeta{Model: "test-model", Provider: "test-provider"},
		})
		assert.NoError(err)
		assert.NotNil(session)
		assert.NotEmpty(session.ID)
		assert.Equal("my chat", session.Name)
		assert.Equal("test-model", session.Model)
		assert.Equal("test-provider", session.Provider)
		assert.Empty(session.Messages)
		assert.False(session.Created.IsZero())
		assert.False(session.Modified.IsZero())
	}},
	{"CreateMissingModel", func(t *testing.T, s schema.SessionStore) {
		assert := assert.New(t)
		_, err := s.CreateSession(context.TODO(), schema.SessionMeta{Name: "test"})
		assert.Error(err)
		assert.ErrorIs(err, llm.ErrBadParameter)
	}},
	{"CreateUniqueIDs", func(t *testing.T, s schema.SessionStore) {
		assert := assert.New(t)
		s1, err := s.CreateSession(context.TODO(), schema.SessionMeta{
			Name:          "first",
			GeneratorMeta: schema.GeneratorMeta{Model: "test-model", Provider: "test-provider"},
		})
		assert.NoError(err)
		s2, err := s.CreateSession(context.TODO(), schema.SessionMeta{
			Name:          "second",
			GeneratorMeta: schema.GeneratorMeta{Model: "test-model", Provider: "test-provider"},
		})
		assert.NoError(err)
		assert.NotEqual(s1.ID, s2.ID)
	}},
	{"CreateEmptyName", func(t *testing.T, s schema.SessionStore) {
		assert := assert.New(t)
		session, err := s.CreateSession(context.TODO(), schema.SessionMeta{
			Name:          "",
			GeneratorMeta: schema.GeneratorMeta{Model: "test-model", Provider: "test-provider"},
		})
		assert.NoError(err)
		assert.Equal("", session.Name)
	}},

	// Get
	{"GetByID", func(t *testing.T, s schema.SessionStore) {
		assert := assert.New(t)
		created, _ := s.CreateSession(context.TODO(), testMeta)
		got, err := s.GetSession(context.TODO(), created.ID)
		assert.NoError(err)
		assert.Equal(created.ID, got.ID)
		assert.Equal(created.Name, got.Name)
	}},
	{"GetNotFound", func(t *testing.T, s schema.SessionStore) {
		assert := assert.New(t)
		_, err := s.GetSession(context.TODO(), "nonexistent")
		assert.Error(err)
	}},

	// Delete
	{"DeleteByID", func(t *testing.T, s schema.SessionStore) {
		assert := assert.New(t)
		session, _ := s.CreateSession(context.TODO(), testMeta)
		err := s.DeleteSession(context.TODO(), session.ID)
		assert.NoError(err)
		_, err = s.GetSession(context.TODO(), session.ID)
		assert.Error(err)
	}},
	{"DeleteNotFound", func(t *testing.T, s schema.SessionStore) {
		assert := assert.New(t)
		err := s.DeleteSession(context.TODO(), "nonexistent")
		assert.Error(err)
	}},

	// List
	{"ListEmpty", func(t *testing.T, s schema.SessionStore) {
		assert := assert.New(t)
		resp, err := s.ListSessions(context.TODO(), schema.ListSessionRequest{})
		assert.NoError(err)
		assert.Empty(resp.Body)
	}},
	{"ListAll", func(t *testing.T, s schema.SessionStore) {
		assert := assert.New(t)
		s.CreateSession(context.TODO(), schema.SessionMeta{Name: "first", GeneratorMeta: schema.GeneratorMeta{Model: "test-model", Provider: "test-provider"}})
		s.CreateSession(context.TODO(), schema.SessionMeta{Name: "second", GeneratorMeta: schema.GeneratorMeta{Model: "test-model", Provider: "test-provider"}})
		s.CreateSession(context.TODO(), schema.SessionMeta{Name: "third", GeneratorMeta: schema.GeneratorMeta{Model: "test-model", Provider: "test-provider"}})
		resp, err := s.ListSessions(context.TODO(), schema.ListSessionRequest{})
		assert.NoError(err)
		assert.Len(resp.Body, 3)
	}},
	{"ListOrderedByModified", func(t *testing.T, s schema.SessionStore) {
		assert := assert.New(t)
		s1, _ := s.CreateSession(context.TODO(), schema.SessionMeta{Name: "oldest", GeneratorMeta: schema.GeneratorMeta{Model: "test-model", Provider: "test-provider"}})
		time.Sleep(10 * time.Millisecond)
		s2, _ := s.CreateSession(context.TODO(), schema.SessionMeta{Name: "middle", GeneratorMeta: schema.GeneratorMeta{Model: "test-model", Provider: "test-provider"}})
		time.Sleep(10 * time.Millisecond)
		s3, _ := s.CreateSession(context.TODO(), schema.SessionMeta{Name: "newest", GeneratorMeta: schema.GeneratorMeta{Model: "test-model", Provider: "test-provider"}})
		resp, err := s.ListSessions(context.TODO(), schema.ListSessionRequest{})
		assert.NoError(err)
		assert.Len(resp.Body, 3)
		assert.Equal(s3.ID, resp.Body[0].ID)
		assert.Equal(s2.ID, resp.Body[1].ID)
		assert.Equal(s1.ID, resp.Body[2].ID)
	}},
	{"ListAfterDelete", func(t *testing.T, s schema.SessionStore) {
		assert := assert.New(t)
		doomed, _ := s.CreateSession(context.TODO(), schema.SessionMeta{Name: "doomed", GeneratorMeta: schema.GeneratorMeta{Model: "test-model", Provider: "test-provider"}})
		s.CreateSession(context.TODO(), schema.SessionMeta{Name: "keeper", GeneratorMeta: schema.GeneratorMeta{Model: "test-model", Provider: "test-provider"}})
		s.DeleteSession(context.TODO(), doomed.ID)
		resp, err := s.ListSessions(context.TODO(), schema.ListSessionRequest{})
		assert.NoError(err)
		assert.Len(resp.Body, 1)
		assert.Equal("keeper", resp.Body[0].Name)
	}},

	// Update
	{"UpdateChangesName", func(t *testing.T, s schema.SessionStore) {
		assert := assert.New(t)
		session, _ := s.CreateSession(context.TODO(), schema.SessionMeta{
			Name:          "original",
			GeneratorMeta: schema.GeneratorMeta{Model: "test-model", Provider: "test-provider"},
		})
		updated, err := s.UpdateSession(context.TODO(), session.ID, schema.SessionMeta{Name: "renamed"})
		assert.NoError(err)
		assert.Equal("renamed", updated.Name)
		assert.Equal("test-model", updated.Model)
		assert.Equal("test-provider", updated.Provider)

		// Verify via Get
		got, err := s.GetSession(context.TODO(), session.ID)
		assert.NoError(err)
		assert.Equal("renamed", got.Name)
	}},
	{"UpdateChangesModelProvider", func(t *testing.T, s schema.SessionStore) {
		assert := assert.New(t)
		session, _ := s.CreateSession(context.TODO(), schema.SessionMeta{
			Name:          "test",
			GeneratorMeta: schema.GeneratorMeta{Model: "model-a", Provider: "provider-a"},
		})
		updated, err := s.UpdateSession(context.TODO(), session.ID, schema.SessionMeta{
			GeneratorMeta: schema.GeneratorMeta{Model: "model-b", Provider: "provider-b"},
		})
		assert.NoError(err)
		assert.Equal("model-b", updated.Model)
		assert.Equal("provider-b", updated.Provider)
		assert.Equal("test", updated.Name)
	}},
	{"UpdateNotFound", func(t *testing.T, s schema.SessionStore) {
		assert := assert.New(t)
		_, err := s.UpdateSession(context.TODO(), "nonexistent", schema.SessionMeta{Name: "x"})
		assert.Error(err)
	}},
	{"UpdateNonZeroFieldsOnly", func(t *testing.T, s schema.SessionStore) {
		assert := assert.New(t)
		session, _ := s.CreateSession(context.TODO(), schema.SessionMeta{
			Name:          "keep",
			GeneratorMeta: schema.GeneratorMeta{Model: "test-model", Provider: "test-provider", SystemPrompt: "original"},
		})
		updated, err := s.UpdateSession(context.TODO(), session.ID, schema.SessionMeta{
			GeneratorMeta: schema.GeneratorMeta{SystemPrompt: "changed"},
		})
		assert.NoError(err)
		assert.Equal("keep", updated.Name)
		assert.Equal("test-model", updated.Model)
		assert.Equal("changed", updated.SystemPrompt)
	}},
	{"UpdateAdvancesModified", func(t *testing.T, s schema.SessionStore) {
		assert := assert.New(t)
		session, _ := s.CreateSession(context.TODO(), testMeta)
		original := session.Modified
		time.Sleep(5 * time.Millisecond)
		updated, err := s.UpdateSession(context.TODO(), session.ID, schema.SessionMeta{Name: "new"})
		assert.NoError(err)
		assert.True(updated.Modified.After(original))
	}},

	// Labels
	{"CreateWithLabels", func(t *testing.T, s schema.SessionStore) {
		assert := assert.New(t)
		labels := map[string]string{"env": "prod", "team": "backend"}
		session, err := s.CreateSession(context.TODO(), schema.SessionMeta{
			Name:          "labeled",
			GeneratorMeta: schema.GeneratorMeta{Model: "test-model", Provider: "test-provider"},
			Labels:        labels,
		})
		assert.NoError(err)
		assert.Equal("prod", session.Labels["env"])
		assert.Equal("backend", session.Labels["team"])

		// Verify labels survive round-trip via Get
		got, err := s.GetSession(context.TODO(), session.ID)
		assert.NoError(err)
		assert.Equal("prod", got.Labels["env"])
		assert.Equal("backend", got.Labels["team"])
	}},
	{"CreateInvalidLabelKey", func(t *testing.T, s schema.SessionStore) {
		assert := assert.New(t)
		_, err := s.CreateSession(context.TODO(), schema.SessionMeta{
			Name:          "bad",
			GeneratorMeta: schema.GeneratorMeta{Model: "test-model", Provider: "test-provider"},
			Labels:        map[string]string{"invalid key!": "value"},
		})
		assert.Error(err)
	}},
	{"ListFiltersByLabels", func(t *testing.T, s schema.SessionStore) {
		assert := assert.New(t)
		s.CreateSession(context.TODO(), schema.SessionMeta{
			Name:          "a",
			GeneratorMeta: schema.GeneratorMeta{Model: "test-model", Provider: "test-provider"},
			Labels:        map[string]string{"env": "prod"},
		})
		s.CreateSession(context.TODO(), schema.SessionMeta{
			Name:          "b",
			GeneratorMeta: schema.GeneratorMeta{Model: "test-model", Provider: "test-provider"},
			Labels:        map[string]string{"env": "dev"},
		})
		s.CreateSession(context.TODO(), schema.SessionMeta{
			Name:          "c",
			GeneratorMeta: schema.GeneratorMeta{Model: "test-model", Provider: "test-provider"},
		})

		// Filter by env:prod
		resp, err := s.ListSessions(context.TODO(), schema.ListSessionRequest{Label: []string{"env:prod"}})
		assert.NoError(err)
		assert.Len(resp.Body, 1)
		assert.Equal("a", resp.Body[0].Name)

		// Filter by env:dev
		resp, err = s.ListSessions(context.TODO(), schema.ListSessionRequest{Label: []string{"env:dev"}})
		assert.NoError(err)
		assert.Len(resp.Body, 1)
		assert.Equal("b", resp.Body[0].Name)

		// No filter returns all
		resp, err = s.ListSessions(context.TODO(), schema.ListSessionRequest{})
		assert.NoError(err)
		assert.Len(resp.Body, 3)
	}},
	{"ListMultipleLabelFilters", func(t *testing.T, s schema.SessionStore) {
		assert := assert.New(t)
		s.CreateSession(context.TODO(), schema.SessionMeta{
			Name:          "match",
			GeneratorMeta: schema.GeneratorMeta{Model: "test-model", Provider: "test-provider"},
			Labels:        map[string]string{"env": "prod", "team": "backend"},
		})
		s.CreateSession(context.TODO(), schema.SessionMeta{
			Name:          "partial",
			GeneratorMeta: schema.GeneratorMeta{Model: "test-model", Provider: "test-provider"},
			Labels:        map[string]string{"env": "prod"},
		})

		resp, err := s.ListSessions(context.TODO(), schema.ListSessionRequest{Label: []string{"env:prod", "team:backend"}})
		assert.NoError(err)
		assert.Len(resp.Body, 1)
		assert.Equal("match", resp.Body[0].Name)
	}},
	{"UpdateMergesLabels", func(t *testing.T, s schema.SessionStore) {
		assert := assert.New(t)
		session, _ := s.CreateSession(context.TODO(), schema.SessionMeta{
			Name:          "test",
			GeneratorMeta: schema.GeneratorMeta{Model: "test-model", Provider: "test-provider"},
			Labels:        map[string]string{"env": "prod", "team": "backend"},
		})
		updated, err := s.UpdateSession(context.TODO(), session.ID, schema.SessionMeta{
			Labels: map[string]string{"team": "frontend", "region": "us"},
		})
		assert.NoError(err)
		assert.Equal("prod", updated.Labels["env"])
		assert.Equal("frontend", updated.Labels["team"])
		assert.Equal("us", updated.Labels["region"])

		// Verify via Get
		got, err := s.GetSession(context.TODO(), session.ID)
		assert.NoError(err)
		assert.Equal("prod", got.Labels["env"])
		assert.Equal("frontend", got.Labels["team"])
		assert.Equal("us", got.Labels["region"])
	}},
	{"UpdateRemovesLabels", func(t *testing.T, s schema.SessionStore) {
		assert := assert.New(t)
		session, _ := s.CreateSession(context.TODO(), schema.SessionMeta{
			Name:          "test",
			GeneratorMeta: schema.GeneratorMeta{Model: "test-model", Provider: "test-provider"},
			Labels:        map[string]string{"env": "prod", "team": "backend"},
		})
		updated, err := s.UpdateSession(context.TODO(), session.ID, schema.SessionMeta{
			Labels: map[string]string{"team": ""},
		})
		assert.NoError(err)
		assert.Equal("prod", updated.Labels["env"])
		_, exists := updated.Labels["team"]
		assert.False(exists)

		// Verify via Get
		got, err := s.GetSession(context.TODO(), session.ID)
		assert.NoError(err)
		assert.Equal("prod", got.Labels["env"])
		_, exists = got.Labels["team"]
		assert.False(exists)
	}},
	{"UpdateInvalidLabelKey", func(t *testing.T, s schema.SessionStore) {
		assert := assert.New(t)
		session, _ := s.CreateSession(context.TODO(), testMeta)
		_, err := s.UpdateSession(context.TODO(), session.ID, schema.SessionMeta{
			Labels: map[string]string{"bad key!": "value"},
		})
		assert.Error(err)
	}},

	// Write
	{"WriteSessionPersistsMessages", func(t *testing.T, s schema.SessionStore) {
		assert := assert.New(t)
		session, _ := s.CreateSession(context.TODO(), testMeta)
		session.Append(schema.Message{Role: schema.RoleUser, Content: []schema.ContentBlock{{Text: textPtr("hello")}}})
		err := s.WriteSession(session)
		assert.NoError(err)
		got, err := s.GetSession(context.TODO(), session.ID)
		assert.NoError(err)
		assert.Len(got.Messages, 1)
		assert.Equal("hello", got.Messages[0].Text())
	}},
	{"WriteSessionRoundTrip", func(t *testing.T, s schema.SessionStore) {
		assert := assert.New(t)
		session, _ := s.CreateSession(context.TODO(), schema.SessionMeta{
			Name:          "round-trip",
			GeneratorMeta: schema.GeneratorMeta{Model: "test-model", Provider: "test-provider"},
		})
		session.Append(schema.Message{Role: schema.RoleUser, Tokens: 5, Content: []schema.ContentBlock{{Text: textPtr("hi")}}})
		session.Append(schema.Message{Role: schema.RoleAssistant, Tokens: 10, Content: []schema.ContentBlock{{Text: textPtr("hello")}}})
		s.WriteSession(session)
		got, err := s.GetSession(context.TODO(), session.ID)
		assert.NoError(err)
		assert.Equal(session.ID, got.ID)
		assert.Equal("round-trip", got.Name)
		assert.Equal("test-model", got.Model)
		assert.Len(got.Messages, 2)
		assert.Equal(uint(15), got.Tokens())
	}},
	{"ListOrderAfterWrite", func(t *testing.T, s schema.SessionStore) {
		assert := assert.New(t)
		s1, _ := s.CreateSession(context.TODO(), schema.SessionMeta{Name: "first", GeneratorMeta: schema.GeneratorMeta{Model: "test-model", Provider: "test-provider"}})
		time.Sleep(10 * time.Millisecond)
		s.CreateSession(context.TODO(), schema.SessionMeta{Name: "second", GeneratorMeta: schema.GeneratorMeta{Model: "test-model", Provider: "test-provider"}})
		time.Sleep(10 * time.Millisecond)
		s1.Append(schema.Message{Role: schema.RoleUser, Content: []schema.ContentBlock{{Text: textPtr("hello")}}})
		s.WriteSession(s1)
		resp, err := s.ListSessions(context.TODO(), schema.ListSessionRequest{})
		assert.NoError(err)
		assert.Equal(s1.ID, resp.Body[0].ID)
	}},

	// Concurrency
	{"ConcurrentCreates", func(t *testing.T, s schema.SessionStore) {
		assert := assert.New(t)
		const n = 20
		var wg sync.WaitGroup
		errs := make([]error, n)
		wg.Add(n)
		for i := range n {
			go func(i int) {
				defer wg.Done()
				_, errs[i] = s.CreateSession(context.TODO(), schema.SessionMeta{
					Name:          fmt.Sprintf("session_%03d", i),
					GeneratorMeta: schema.GeneratorMeta{Model: "m", Provider: "p"},
				})
			}(i)
		}
		wg.Wait()
		for i, err := range errs {
			assert.NoError(err, "session_%03d", i)
		}
		resp, err := s.ListSessions(context.TODO(), schema.ListSessionRequest{})
		assert.NoError(err)
		assert.Equal(uint(n), resp.Count)
	}},
	{"ConcurrentReadsAndWrites", func(t *testing.T, s schema.SessionStore) {
		created, _ := s.CreateSession(context.TODO(), testMeta)

		const n = 20
		var wg sync.WaitGroup
		wg.Add(n * 2)
		for range n {
			go func() {
				defer wg.Done()
				s.GetSession(context.TODO(), created.ID)
			}()
			go func() {
				defer wg.Done()
				s.ListSessions(context.TODO(), schema.ListSessionRequest{})
			}()
		}
		wg.Wait()
	}},
	{"ConcurrentMixedOps", func(t *testing.T, s schema.SessionStore) {
		// Seed the store
		seeds := make([]*schema.Session, 5)
		for i := range seeds {
			seeds[i], _ = s.CreateSession(context.TODO(), schema.SessionMeta{
				Name:          fmt.Sprintf("seed_%d", i),
				GeneratorMeta: schema.GeneratorMeta{Model: "m", Provider: "p"},
			})
		}

		const n = 10
		var wg sync.WaitGroup
		wg.Add(n * 4)
		for i := range n {
			// Create
			go func(i int) {
				defer wg.Done()
				s.CreateSession(context.TODO(), schema.SessionMeta{
					Name:          fmt.Sprintf("mixed_%d", i),
					GeneratorMeta: schema.GeneratorMeta{Model: "m", Provider: "p"},
				})
			}(i)
			// Get
			go func() {
				defer wg.Done()
				s.GetSession(context.TODO(), seeds[0].ID)
			}()
			// List
			go func() {
				defer wg.Done()
				s.ListSessions(context.TODO(), schema.ListSessionRequest{})
			}()
			// Update
			go func(i int) {
				defer wg.Done()
				s.UpdateSession(context.TODO(), seeds[0].ID, schema.SessionMeta{
					Name: fmt.Sprintf("updated_%d", i),
				})
			}(i)
		}
		wg.Wait()
	}},
	{"ConcurrentWriteSession", func(t *testing.T, s schema.SessionStore) {
		assert := assert.New(t)

		const n = 20
		sessions := make([]*schema.Session, n)
		for i := range n {
			sessions[i], _ = s.CreateSession(context.TODO(), schema.SessionMeta{
				Name:          fmt.Sprintf("ws_%03d", i),
				GeneratorMeta: schema.GeneratorMeta{Model: "m", Provider: "p"},
			})
			sessions[i].Append(schema.Message{
				Role:    schema.RoleUser,
				Content: []schema.ContentBlock{{Text: textPtr(fmt.Sprintf("msg_%d", i))}},
			})
		}

		var wg sync.WaitGroup
		wg.Add(n)
		for i := range n {
			go func(i int) {
				defer wg.Done()
				s.WriteSession(sessions[i])
			}(i)
		}
		wg.Wait()

		for i := range n {
			got, err := s.GetSession(context.TODO(), sessions[i].ID)
			assert.NoError(err)
			assert.NotNil(got)
		}
	}},
}

// runSessionStoreTests runs every shared behavioural test against a store
// implementation. The factory is called once per subtest so each gets a
// clean, independent store.
func runSessionStoreTests(t *testing.T, factory func() schema.SessionStore) {
	t.Helper()
	for _, tt := range sessionStoreTests {
		t.Run(tt.Name, func(t *testing.T) {
			tt.Fn(t, factory())
		})
	}
}
