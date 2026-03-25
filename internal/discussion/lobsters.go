// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

// NOTE: The Lobsters search API requires authentication for URL searches,
// so this provider is not currently registered in the Finder.
// It is kept here for future use if a public API becomes available.

package discussion // import "miniflux.app/v2/internal/discussion"

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
)

const (
	lobstersSearchURL  = "https://lobste.rs/search"
	lobstersMaxResults = 2
)

type lobstersProvider struct {
	client *http.Client
}

type lobstersStory struct {
	ShortID      string `json:"short_id"`
	Title        string `json:"title"`
	CommentCount int    `json:"comment_count"`
	CommentsURL  string `json:"comments_url"`
	Score        int    `json:"score"`
}

func (p *lobstersProvider) name() string {
	return "lobsters"
}

func (p *lobstersProvider) hostnames() []string {
	return []string{"lobste.rs"}
}

func (p *lobstersProvider) search(ctx context.Context, entryURL string) ([]DiscussionLink, error) {
	reqURL := lobstersSearchURL + "?" + url.Values{
		"q":      {entryURL},
		"what":   {"stories"},
		"format": {"json"},
	}.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("lobsters: unable to create request: %w", err)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("lobsters: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		io.Copy(io.Discard, resp.Body)
		return nil, fmt.Errorf("lobsters: unexpected status %d", resp.StatusCode)
	}

	var stories []lobstersStory
	if err := json.NewDecoder(resp.Body).Decode(&stories); err != nil {
		return nil, fmt.Errorf("lobsters: unable to decode response: %w", err)
	}

	// Sort by comment count descending.
	sort.Slice(stories, func(i, j int) bool {
		return stories[i].CommentCount > stories[j].CommentCount
	})

	limit := lobstersMaxResults
	if len(stories) < limit {
		limit = len(stories)
	}

	links := make([]DiscussionLink, 0, limit)
	for _, story := range stories[:limit] {
		links = append(links, DiscussionLink{
			Source:   "lobsters",
			Title:    story.Title,
			URL:      story.CommentsURL,
			Comments: story.CommentCount,
			Score:    story.Score,
		})
	}

	return links, nil
}
