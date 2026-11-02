package registry

import (
	"context"
	"regexp"
	"slices"
	"sync"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// cached is a provider in the registry, with the patterns its models are
// filtered by, the result of its last ping, and its models, which are cached
type cached struct {
	provider llm.Provider

	// The patterns of the names of the models which are included, or all of
	// them when there are none, and excluded, as they were given, and
	// compiled
	include, exclude     []string
	includeRe, excludeRe []*regexp.Regexp

	// Held while the provider is pinged, so concurrent pings wait for it
	pinging sync.Mutex

	// When the provider was last pinged, and the error of the ping
	result  sync.RWMutex
	pingAt  time.Time
	pingErr error

	// Held while the models are listed and cached, so they're cached once
	// at a time, and a listing which ends later than a newer one doesn't
	// replace it
	refreshing sync.Mutex

	// The models, in the order the provider lists them, and by their names
	// and aliases, when they were cached, and the error of the last listing.
	// The models are nil until they're cached.
	models    sync.RWMutex
	list      []llm.Model
	byName    map[string]llm.Model
	modelsAt  time.Time
	modelsErr error
}

// ProviderOpt is an option of a provider which is added to the registry
type ProviderOpt func(*cached) error

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

var (
	// PingInterval is how long the result of a provider's ping is used, and
	// how often Run pings the providers. A provider which downloads models,
	// such as Ollama, has its models listed again as often, as they can be
	// downloaded or deleted without the registry, and listing them is quick.
	PingInterval = time.Minute

	// ModelCacheInterval is how long a provider's models are cached, unless
	// it downloads models
	ModelCacheInterval = 6 * time.Hour
)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// newCached returns the provider, with the options
func newCached(provider llm.Provider, opts ...ProviderOpt) (*cached, error) {
	c := &cached{provider: provider}
	for _, opt := range opts {
		if err := opt(c); err != nil {
			return nil, err
		}
	}
	return c, nil
}

///////////////////////////////////////////////////////////////////////////////
// OPTIONS

// WithInclude includes only the models whose names match one of the
// patterns, which are regular expressions, such as ^gpt-, or every model
// when there are none
func WithInclude(patterns ...string) ProviderOpt {
	return func(c *cached) (err error) {
		c.includeRe, err = compilePatterns("include", patterns)
		c.include = slices.Clone(patterns)
		return err
	}
}

// WithExclude excludes the models whose names match one of the patterns,
// which are regular expressions, such as -mini$
func WithExclude(patterns ...string) ProviderOpt {
	return func(c *cached) (err error) {
		c.excludeRe, err = compilePatterns("exclude", patterns)
		c.exclude = slices.Clone(patterns)
		return err
	}
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS - PING

// ping returns the error of pinging the provider, or nil when it can be
// reached. It's only pinged when it hasn't been within the age, and
// otherwise the result of its last ping is returned. Concurrent pings wait
// for the one in progress.
func (c *cached) ping(ctx context.Context, maxAge time.Duration) error {
	c.pinging.Lock()
	defer c.pinging.Unlock()

	// Return the last result, when it's recent
	if at, err := c.lastPing(); !at.IsZero() && time.Since(at) < maxAge {
		return err
	}

	// Ping the provider, and keep the result, from when the ping started
	start := time.Now()
	err := c.provider.Ping(ctx)
	c.setPing(start, err)
	return err
}

// lastPing returns when the provider was last pinged, and the error of the
// ping, without waiting for a ping in progress
func (c *cached) lastPing() (time.Time, error) {
	c.result.RLock()
	defer c.result.RUnlock()
	return c.pingAt, c.pingErr
}

// setPing sets when the provider was pinged, and the error of the ping
func (c *cached) setPing(at time.Time, err error) {
	c.result.Lock()
	defer c.result.Unlock()
	c.pingAt, c.pingErr = at, err
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS - MODELS

// listModels returns the provider's models, which match its patterns,
// listing them again first, when they're stale. When they can't be listed,
// the models which were cached are returned, or the error, when none were.
func (c *cached) listModels(ctx context.Context) ([]llm.Model, error) {
	if err := c.cache(ctx); err != nil {
		return nil, err
	}
	c.models.RLock()
	defer c.models.RUnlock()
	return slices.Clone(c.list), nil
}

// getModel returns the provider's model with the name, or an alias, which
// matches its patterns, listing its models again first, when they're
// stale, or ErrNotFound when it has none
func (c *cached) getModel(ctx context.Context, name string) (llm.Model, error) {
	if err := c.cache(ctx); err != nil {
		return nil, err
	}
	c.models.RLock()
	defer c.models.RUnlock()
	if model, exists := c.byName[name]; exists {
		return model, nil
	}
	return nil, llm.ErrNotFound.Withf("model %q", name)
}

// cache lists the models again when they're stale, and returns an error
// when they can't be listed, and none were cached, which is
// ErrServiceUnavailable, unless the provider's error is an llm.Err
func (c *cached) cache(ctx context.Context) error {
	if c.stale() {
		if err := c.refresh(ctx); err != nil && !c.isCached() {
			return unavailable(err)
		}
	}
	return nil
}

// refresh lists the provider's models, and caches those which match its
// patterns, by their names, and their aliases, which aren't other models'
// names. A name which is listed more than once is the first model with the
// name. The models which were cached are kept when they can't be listed.
func (c *cached) refresh(ctx context.Context) (err error) {
	c.refreshing.Lock()
	defer c.refreshing.Unlock()

	// Keep the error of the listing, so the models are listed again
	start := time.Now()
	defer func() {
		c.models.Lock()
		defer c.models.Unlock()
		c.modelsErr = err
	}()

	// List the models, and keep those which match the patterns
	models, err := c.provider.ListModels(ctx)
	if err != nil {
		return err
	}
	byName := make(map[string]llm.Model, len(models))
	list := make([]llm.Model, 0, len(models))
	for _, model := range models {
		if _, exists := byName[model.Name()]; exists || !c.matches(model.Name()) {
			continue
		}
		byName[model.Name()] = model
		list = append(list, model)
	}
	for _, model := range list {
		for _, alias := range model.Aliases() {
			if _, exists := byName[alias]; !exists {
				byName[alias] = model
			}
		}
	}

	// Cache them
	c.models.Lock()
	defer c.models.Unlock()
	c.list, c.byName, c.modelsAt = list, byName, start
	return nil
}

// stale returns true when the models should be listed again, as they
// haven't been, or couldn't be the last time, or were listed longer ago
// than the provider's interval
func (c *cached) stale() bool {
	c.models.RLock()
	defer c.models.RUnlock()
	return c.modelsAt.IsZero() || c.modelsErr != nil || time.Since(c.modelsAt) >= c.interval()
}

// interval returns how long the provider's models are cached
func (c *cached) interval() time.Duration {
	if _, ok := c.provider.Self().(llm.Downloader); ok {
		return PingInterval
	}
	return ModelCacheInterval
}

// isCached returns true when the models have been cached
func (c *cached) isCached() bool {
	c.models.RLock()
	defer c.models.RUnlock()
	return c.byName != nil
}

// invalidate makes the models stale, so they're listed again, such as when
// a model is downloaded or deleted
func (c *cached) invalidate() {
	c.models.Lock()
	defer c.models.Unlock()
	c.modelsAt = time.Time{}
}

// matches returns true when the model's name matches an include pattern,
// or there are none, and matches no exclude pattern
func (c *cached) matches(name string) bool {
	if len(c.includeRe) > 0 && !matchesAny(c.includeRe, name) {
		return false
	}
	return !matchesAny(c.excludeRe, name)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// compilePatterns returns the patterns, which are regular expressions,
// compiled, or ErrBadParameter when one isn't valid
func compilePatterns(field string, patterns []string) ([]*regexp.Regexp, error) {
	result := make([]*regexp.Regexp, 0, len(patterns))
	for _, pattern := range patterns {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return nil, llm.ErrBadParameter.Withf("%s pattern %q: %v", field, pattern, err)
		}
		result = append(result, re)
	}
	return result, nil
}

// matchesAny returns true when the name matches one of the patterns
func matchesAny(patterns []*regexp.Regexp, name string) bool {
	for _, re := range patterns {
		if re.MatchString(name) {
			return true
		}
	}
	return false
}
