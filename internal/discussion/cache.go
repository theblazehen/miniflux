// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package discussion // import "miniflux.app/v2/internal/discussion"

import (
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	resultTTL = 30 * time.Minute
	emptyTTL  = 15 * time.Minute
	errorTTL  = 5 * time.Minute
	maxSize   = 10000
)

type cacheEntry struct {
	discussions []DiscussionLink
	expiresAt   time.Time
}

type cache struct {
	mu      sync.RWMutex
	entries map[string]cacheEntry
}

func newCache() *cache {
	return &cache{
		entries: make(map[string]cacheEntry),
	}
}

// get returns cached discussion links and true if a non-expired entry exists.
func (c *cache) get(normalizedURL string) ([]DiscussionLink, bool) {
	c.mu.RLock()
	entry, found := c.entries[normalizedURL]
	c.mu.RUnlock()

	if !found {
		return nil, false
	}

	if time.Now().After(entry.expiresAt) {
		c.mu.Lock()
		delete(c.entries, normalizedURL)
		c.mu.Unlock()
		return nil, false
	}

	return entry.discussions, true
}

// set stores discussion links with the given TTL.
// If the cache exceeds maxSize, the oldest entries are evicted.
func (c *cache) set(normalizedURL string, links []DiscussionLink, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Evict expired entries if we're at capacity.
	if len(c.entries) >= maxSize {
		now := time.Now()
		for key, entry := range c.entries {
			if now.After(entry.expiresAt) {
				delete(c.entries, key)
			}
		}
	}

	// If still at capacity after expiry sweep, evict ~10% of entries at random.
	// Map iteration order in Go is randomized, so this is a simple approximation.
	if len(c.entries) >= maxSize {
		toEvict := maxSize / 10
		evicted := 0
		for key := range c.entries {
			delete(c.entries, key)
			evicted++
			if evicted >= toEvict {
				break
			}
		}
	}

	c.entries[normalizedURL] = cacheEntry{
		discussions: links,
		expiresAt:   time.Now().Add(ttl),
	}
}

// normalizeURL produces a canonical cache key from a raw URL.
// It lowercases the scheme and host, strips the fragment, strips trailing
// slashes from the path, and removes common tracking query parameters.
func normalizeURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}

	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)
	u.Fragment = ""
	u.Path = strings.TrimRight(u.Path, "/")

	// Strip UTM tracking parameters only — other params may be legitimate.
	q := u.Query()
	for key := range q {
		if strings.HasPrefix(strings.ToLower(key), "utm_") {
			q.Del(key)
		}
	}
	u.RawQuery = q.Encode()

	return u.String()
}
