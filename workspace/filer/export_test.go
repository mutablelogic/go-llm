package filer

import (
	filerschema "github.com/mutablelogic/go-filer/filer/schema"
)

// DeletedURI returns the URI the deletion of the path is said with
func DeletedURI(w *Workspace, path string) string {
	change, _ := w.changeOf(nil, filerschema.Event{EventType: filerschema.EventTypeDeleted, ObjectKey: filerschema.ObjectKey{Path: path}})
	return change.URI
}
