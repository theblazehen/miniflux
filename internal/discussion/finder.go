// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package discussion // import "miniflux.app/v2/internal/discussion"

import (
	"context"
	"log/slog"
	"net/url"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"miniflux.app/v2/internal/http/client"
	"miniflux.app/v2/internal/version"
)

const (
	httpClientTimeout  = 5 * time.Second
	perProviderTimeout = 3 * time.Second
)

// Finder looks up discussion threads for a given URL across multiple platforms.
type Finder struct {
	cache     *cache
	providers []provider

	// inflight provides simple singleflight deduplication so that concurrent
	// requests for the same URL only trigger one external lookup.
	inflightMu sync.Mutex
	inflight   map[string]*inflightEntry
}

type inflightEntry struct {
	done   chan struct{}
	result []DiscussionLink
}

// NewFinder creates a Finder with all built-in providers and an internal cache.
func NewFinder() *Finder {
	httpClient := client.NewClientWithOptions(client.Options{
		Timeout:              httpClientTimeout,
		BlockPrivateNetworks: true,
	})

	userAgent := "Miniflux/" + version.Version

	return &Finder{
		cache: newCache(),
		providers: []provider{
			&hackerNewsProvider{client: httpClient},
			&redditProvider{client: httpClient, userAgent: userAgent},
			// NOTE: Lobsters is not registered because its search API requires authentication.
		},
		inflight: make(map[string]*inflightEntry),
	}
}

// Find returns discussion links for the given entry URL.
// Results are cached and concurrent requests for the same URL are deduplicated.
// Provider errors are logged but never returned — partial results are fine.
func (f *Finder) Find(ctx context.Context, entryURL string) DiscussionResponse {
	if entryURL == "" {
		return DiscussionResponse{Discussions: []DiscussionLink{}}
	}

	normalized := normalizeURL(entryURL)

	// Check cache first.
	if cached, ok := f.cache.get(normalized); ok {
		return DiscussionResponse{Discussions: cached}
	}

	// Singleflight: if another goroutine is already fetching this URL, wait for it.
	f.inflightMu.Lock()
	if entry, ok := f.inflight[normalized]; ok {
		f.inflightMu.Unlock()
		select {
		case <-entry.done:
			return DiscussionResponse{Discussions: entry.result}
		case <-ctx.Done():
			return DiscussionResponse{Discussions: []DiscussionLink{}}
		}
	}

	entry := &inflightEntry{done: make(chan struct{})}
	f.inflight[normalized] = entry
	f.inflightMu.Unlock()

	// Perform the actual lookup.
	result, allFailed := f.fetchAll(ctx, entryURL)

	// Cache the result with an appropriate TTL:
	// - resultTTL (30m) if we got results
	// - errorTTL (5m) if all providers failed (allows faster retry)
	// - emptyTTL (15m) if no results but at least one provider succeeded
	ttl := resultTTL
	if len(result) == 0 {
		if allFailed {
			ttl = errorTTL
		} else {
			ttl = emptyTTL
		}
	}
	f.cache.set(normalized, result, ttl)

	entry.result = result
	close(entry.done)

	f.inflightMu.Lock()
	delete(f.inflight, normalized)
	f.inflightMu.Unlock()

	return DiscussionResponse{Discussions: result}
}

// fetchAll fans out to all applicable providers concurrently and merges results.
// It returns the merged results and whether ALL queried providers failed.
func (f *Finder) fetchAll(ctx context.Context, entryURL string) ([]DiscussionLink, bool) {
	entryHost := extractHost(entryURL)

	var (
		mu           sync.Mutex
		results      []DiscussionLink
		wg           sync.WaitGroup
		queried      int32 // number of providers actually queried (not skipped)
		failureCount int32 // number of providers that errored or timed out
	)

	for _, p := range f.providers {
		if shouldSkipProvider(entryHost, p.hostnames()) {
			slog.Debug("Skipping discussion provider for self-referencing URL",
				slog.String("provider", p.name()),
				slog.String("entry_url", entryURL),
			)
			continue
		}

		atomic.AddInt32(&queried, 1)
		wg.Add(1)

		go func(p provider) {
			defer wg.Done()

			providerCtx, cancel := context.WithTimeout(ctx, perProviderTimeout)
			defer cancel()

			links, err := p.search(providerCtx, entryURL)
			if err != nil {
				atomic.AddInt32(&failureCount, 1)
				slog.Warn("Unable to fetch discussions",
					slog.String("provider", p.name()),
					slog.String("entry_url", entryURL),
					slog.Any("error", err),
				)
				return
			}

			mu.Lock()
			results = append(results, links...)
			mu.Unlock()
		}(p)
	}

	wg.Wait()

	// Sort all results by comment count descending for a consistent display order.
	sort.Slice(results, func(i, j int) bool {
		return results[i].Comments > results[j].Comments
	})

	// All providers failed if we queried at least one but every one errored.
	totalQueried := atomic.LoadInt32(&queried)
	totalFailed := atomic.LoadInt32(&failureCount)
	allFailed := totalQueried > 0 && totalFailed == totalQueried

	return results, allFailed
}

// shouldSkipProvider returns true if the entry URL's host matches one of the
// provider's own hostnames (e.g. don't search HN for a news.ycombinator.com link).
func shouldSkipProvider(entryHost string, providerHosts []string) bool {
	for _, h := range providerHosts {
		if entryHost == h || strings.HasSuffix(entryHost, "."+h) {
			return true
		}
	}
	return false
}

// extractHost returns the lowercased hostname from a URL, or empty string on error.
func extractHost(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return strings.ToLower(u.Hostname())
}
