// Copyright 2026 David Thorpe
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package manager

import (
	"testing"
	"time"

	// Packages
	uuid "github.com/google/uuid"
	schema "github.com/mutablelogic/go-llm/memory/schema"
)

func TestDynamicMemoryEntriesUTC(t *testing.T) {
	session := uuid.New()
	now := time.Date(2026, time.April, 11, 14, 30, 45, 0, time.FixedZone("CEST", 2*60*60))

	entries := dynamicMemoryEntries(session, now)
	if len(entries) != 4 {
		t.Fatalf("expected 4 dynamic entries, got %d", len(entries))
	}

	values := make(map[string]string, len(entries))
	for _, entry := range entries {
		if entry == nil || entry.Value == nil {
			continue
		}
		values[entry.Key] = *entry.Value
		if entry.Session != session {
			t.Fatalf("expected session %q, got %q", session, entry.Session)
		}
	}

	if values["date"] != "2026-04-11" {
		t.Fatalf("unexpected date value: %q", values["date"])
	}
	if values["time"] != "12:30:45 UTC" {
		t.Fatalf("unexpected time value: %q", values["time"])
	}
	if values["datetime"] != "2026-04-11T12:30:45Z" {
		t.Fatalf("unexpected datetime value: %q", values["datetime"])
	}
	if values["timezone"] != "UTC" {
		t.Fatalf("unexpected timezone value: %q", values["timezone"])
	}
}

func TestMergeDynamicMemoryReplacesStoredReservedKeys(t *testing.T) {
	session := uuid.New()
	now := time.Date(2026, time.April, 11, 14, 30, 45, 0, time.UTC)
	storedDate := "stale-date"
	custom := "remembered"

	list := &schema.MemoryList{
		Body: []*schema.Memory{
			{MemoryInsert: schema.MemoryInsert{Session: session, Key: "date", MemoryMeta: schema.MemoryMeta{Value: &storedDate}}},
			{MemoryInsert: schema.MemoryInsert{Session: session, Key: "topic", MemoryMeta: schema.MemoryMeta{Value: &custom}}},
		},
		Count: 2,
	}

	merged := mergeDynamicMemory(list, session, "date", now)
	if merged == nil {
		t.Fatal("expected merged list")
	}
	if merged.Count != 2 {
		t.Fatalf("expected count 2, got %d", merged.Count)
	}
	if len(merged.Body) != 2 {
		t.Fatalf("expected body length 2, got %d", len(merged.Body))
	}
	if merged.Body[0].Key != "date" {
		t.Fatalf("expected date entry, got %q", merged.Body[0].Key)
	}
	if merged.Body[0].Value == nil || *merged.Body[0].Value != "2026-04-11" {
		t.Fatalf("unexpected merged date value: %v", merged.Body[0].Value)
	}
	if merged.Body[1].Key != "topic" {
		t.Fatalf("expected topic entry, got %q", merged.Body[1].Key)
	}
}

func TestFilterDynamicMemoryMatchesDateAndTimeQuery(t *testing.T) {
	session := uuid.New()
	now := time.Date(2026, time.April, 11, 14, 30, 45, 0, time.UTC)

	filtered := filterDynamicMemory(dynamicMemoryEntries(session, now), "current date and time")
	if len(filtered) != 2 {
		t.Fatalf("expected 2 filtered entries, got %d", len(filtered))
	}
	if filtered[0].Key != "date" || filtered[1].Key != "time" {
		t.Fatalf("unexpected filtered keys: %q, %q", filtered[0].Key, filtered[1].Key)
	}
}
