//go:build !client

package main

import (
	"context"
	"encoding/json"

	// Packages
	memory "github.com/mutablelogic/go-llm/memory"
	schema "github.com/mutablelogic/go-llm/memory/schema"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// memoryStore keeps the memory's details in a file of JSON, under the
// details key, so other things the memory keeps can be beside them
type memoryStore struct {
	file *jsonFile
}

var _ memory.Store = (*memoryStore)(nil)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// memoryFile is the name of the file which the server keeps the
	// memory in, in the directory of the command's defaults
	memoryFile = "memory.json"

	// detailsKey is the key of the details, in the file
	detailsKey = "details"
)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// newMemoryStore returns the store of the memory, in the file
func newMemoryStore(file *jsonFile) *memoryStore {
	return &memoryStore{file: file}
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Details returns the details in the file, which are all not known, when
// there's no file, or no details in it. Returns an error when the file
// can't be read, or its details aren't valid.
func (s *memoryStore) Details(context.Context) (schema.Details, error) {
	var details schema.Details
	value, err := s.file.Lookup(detailsKey)
	if err != nil || value == nil {
		return details, err
	}
	data, err := json.Marshal(value)
	if err != nil {
		return details, err
	} else if err := json.Unmarshal(data, &details); err != nil {
		return details, err
	}
	return details, details.Validate()
}

// PutDetails writes the details to the file, keeping what else is in it
func (s *memoryStore) PutDetails(_ context.Context, details schema.Details) error {
	if err := details.Validate(); err != nil {
		return err
	}
	return s.file.Set(detailsKey, details)
}
