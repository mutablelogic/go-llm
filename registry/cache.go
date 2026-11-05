package registry

import (
	"context"
	"regexp"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/registry/schema"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// cached is a provider in the registry, with the patterns its models are
// filtered by, the result of its last ping, and its models, which are cached
type cached struct {
	provider llm.Provider

	// The request the provider was created with, with its name, and its API
	// key's placeholder, rather than its credentials, or nil when it was
	// added without one, and its serial, which is kept apart from it, so a
	// store can set it, once it has the definition
	definition *schema.Provider
	serial     atomic.Uint64

	// Called when whether the provider can be reached changes, or its
	// models do, without a lock, or nil
	changed func()

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
	// The models are nil until they're cached. The listings are counted, as
	// are the invalidations, so a listing which started before the models
	// were invalidated doesn't make them fresh.
	models     sync.RWMutex
	list       []llm.Model
	byName     map[string]llm.Model
	modelsAt   time.Time
	modelsErr  error
	listings   uint64
	generation uint64
}

// ProviderOpt is an option of a provider which is added to the registry
type ProviderOpt func(*cached) error

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

var (
	// PingInterval is how long the result of a provider's ping is used, and
	// how often Run pings the providers, so whether one can be reached is
	// known within it. A provider decides how often it asks its server, so
	// one of an API, such as OpenAI's, which is rarely unreachable, can keep
	// its ping's result for longer. A provider which downloads models, such
	// as Ollama, has its models listed again as often, as they can be
	// downloaded or deleted without the registry, and listing them is quick.
	PingInterval = 30 * time.Second

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

// withDefinition is the request the provider was created with, and its
// serial
func withDefinition(req schema.Provider) ProviderOpt {
	return func(c *cached) error {
		req.Include, req.Exclude = slices.Clone(req.Include), slices.Clone(req.Exclude)
		c.serial.Store(req.Serial)
		req.Serial = 0
		c.definition = &req
		return nil
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

// setPing sets when the provider was pinged, and the error of the ping, and
// says it changed, when whether it can be reached has, since it was last
// pinged
func (c *cached) setPing(at time.Time, err error) {
	c.result.Lock()
	changed := !c.pingAt.IsZero() && (c.pingErr == nil) != (err == nil)
	c.pingAt, c.pingErr = at, err
	c.result.Unlock()
	if changed {
		c.notify()
	}
}

// notify says the provider changed, when there's anything to say it to
func (c *cached) notify() {
	if c.changed != nil {
		c.changed()
	}
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

// refresh lists the provider's models, when they're stale, and caches
// those which match its patterns, by their names, and their aliases, which
// aren't other models' names. A name which is listed more than once is the
// first model with the name. The models which were cached are kept when
// they can't be listed. Concurrent refreshes wait for the one in progress,
// and use its models, or its error. It says the provider changed, when the
// names of its models have, since they were last cached.
func (c *cached) refresh(ctx context.Context) (err error) {
	listings, _ := c.counts()
	c.refreshing.Lock()
	defer c.refreshing.Unlock()

	// Use the listing which ended while waiting, unless the models are stale
	if !c.stale() {
		return nil
	} else if latest, _ := c.counts(); latest != listings {
		if err := c.lastErr(); err != nil {
			return err
		}
	}

	// Keep the error of the listing, so the models are listed again
	start := time.Now()
	_, generation := c.counts()
	defer func() {
		c.models.Lock()
		defer c.models.Unlock()
		c.listings++
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

	// Cache them, but leave them stale when they were invalidated while
	// they were listed, as they could be from before then
	c.models.Lock()
	changed := c.byName != nil && !sameNames(c.list, list)
	c.list, c.byName = list, byName
	if c.generation == generation {
		c.modelsAt = start
	}
	c.models.Unlock()
	if changed {
		c.notify()
	}
	return nil
}

// counts returns how many times the models have been listed, and
// invalidated
func (c *cached) counts() (listings, generation uint64) {
	c.models.RLock()
	defer c.models.RUnlock()
	return c.listings, c.generation
}

// lastErr returns the error of the last listing, or nil when it succeeded
func (c *cached) lastErr() error {
	c.models.RLock()
	defer c.models.RUnlock()
	return c.modelsErr
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
// a model is downloaded or deleted, including by a listing in progress
func (c *cached) invalidate() {
	c.models.Lock()
	defer c.models.Unlock()
	c.modelsAt = time.Time{}
	c.generation++
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

// sameNames returns true when the models have the same names, in any order
func sameNames(a, b []llm.Model) bool {
	if len(a) != len(b) {
		return false
	}
	names := make(map[string]int, len(a))
	for _, model := range a {
		names[model.Name()]++
	}
	for _, model := range b {
		if names[model.Name()]--; names[model.Name()] < 0 {
			return false
		}
	}
	return true
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
