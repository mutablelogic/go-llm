package main

import (
	"io/fs"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	testdata "github.com/mutablelogic/go-llm/etc/testdata"
	resource "github.com/mutablelogic/go-llm/pkg/toolkit/resource"
)

// CreateResources walks the testdata embedded filesystem and returns one
// llm.Resource per file found. Text files are automatically returned as
// text resources (with UTF-8 transcoding); binary files as data resources.
func CreateResources() ([]llm.Resource, error) {
	var resources []llm.Resource
	err := fs.WalkDir(testdata.FS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		data, err := fs.ReadFile(testdata.FS, path)
		if err != nil {
			return err
		}
		r, err := resource.Data(path, data)
		if err != nil {
			return err
		}
		resources = append(resources, resource.WithURI("file:"+path, r))
		return nil
	})
	if err != nil {
		return nil, err
	}
	return resources, nil
}
