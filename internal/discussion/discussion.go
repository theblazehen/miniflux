// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package discussion // import "miniflux.app/v2/internal/discussion"

import "context"

// DiscussionLink represents a single discussion thread on an external platform.
type DiscussionLink struct {
	Source    string `json:"source"` // "hackernews", "reddit"
	Title     string `json:"title"`
	URL       string `json:"url"`
	Comments  int    `json:"comments"`
	Score     int    `json:"score"`
	Community string `json:"community,omitempty"` // subreddit for Reddit
}

// DiscussionResponse is the JSON envelope returned by the discussions endpoint.
type DiscussionResponse struct {
	Discussions []DiscussionLink `json:"discussions"`
}

// provider is the internal interface each discussion source implements.
type provider interface {
	// name returns the provider identifier (e.g. "hackernews").
	name() string

	// hostnames returns domains that, if they match the entry URL, mean this
	// provider should be skipped (the entry itself is from this platform).
	hostnames() []string

	// search queries the external API and returns discussion links.
	// Implementations must respect the context for cancellation and timeouts.
	search(ctx context.Context, entryURL string) ([]DiscussionLink, error)
}
