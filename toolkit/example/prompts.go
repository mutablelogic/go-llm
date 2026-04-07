package main

import (
	"bytes"
	"io/fs"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	agents "github.com/mutablelogic/go-llm/etc/agent"
	prompt "github.com/mutablelogic/go-llm/toolkit/prompt"
)

// namedReader wraps a bytes.Reader and exposes a Name() method so that
// prompt.Read can derive the prompt name from the filename.
type namedReader struct {
	*bytes.Reader
	name string
}

func (r *namedReader) Name() string { return r.name }

// CreatePrompts walks the agent embedded filesystem and returns one
// llm.Prompt per markdown file found.
func CreatePrompts() ([]llm.Prompt, error) {
	var prompts []llm.Prompt
	err := fs.WalkDir(agents.FS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		data, err := fs.ReadFile(agents.FS, path)
		if err != nil {
			return err
		}
		p, err := prompt.Read(&namedReader{bytes.NewReader(data), path})
		if err != nil {
			return err
		}
		prompts = append(prompts, p)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return prompts, nil
}
