package impl

import (
	// Packages
	"sync"

	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type ModelCache struct {
	sync.RWMutex
	cache map[string]llm.Model
}

type ModelLoadFunc func() ([]llm.Model, error)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func NewModelCache() *ModelCache {
	cache := new(ModelCache)
	cache.cache = make(map[string]llm.Model, 20)
	return cache
}

///////////////////////////////////////////////////////////////////////////////
// METHODS

// Load models and return them
func (c *ModelCache) Load(fn ModelLoadFunc) ([]llm.Model, error) {
	c.Lock()
	defer c.Unlock()

	// Load models
	if len(c.cache) == 0 {
		if models, err := fn(); err != nil {
			return nil, err
		} else {
			for _, m := range models {
				c.cache[m.Name()] = m
			}
		}
	}

	// Return models
	result := make([]llm.Model, 0, len(c.cache))
	for _, model := range c.cache {
		result = append(result, model)
	}
	return result, nil
}

// Return a model by name
func (c *ModelCache) Get(fn ModelLoadFunc, name string) (llm.Model, error) {
	if len(c.cache) == 0 {
		if _, err := c.Load(fn); err != nil {
			return nil, err
		}
	}
	c.RLock()
	defer c.RUnlock()
	return c.cache[name], nil
}
