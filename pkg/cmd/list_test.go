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

package cmd

import "testing"

func TestListSummaryNoResults(t *testing.T) {
	if got := listSummary(0, 0, 0); got != "No results" {
		t.Fatalf("unexpected summary: %q", got)
	}
}

func TestListSummaryRange(t *testing.T) {
	if got := listSummary(10, 5, 42); got != "Showing 11-15 of 42 items" {
		t.Fatalf("unexpected summary: %q", got)
	}
}

func TestListSummaryClampsToTotal(t *testing.T) {
	if got := listSummary(40, 10, 42); got != "Showing 41-42 of 42 items" {
		t.Fatalf("unexpected summary: %q", got)
	}
}
