// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

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
	hnSearchURL  = "https://hn.algolia.com/api/v1/search"
	hnItemURL    = "https://news.ycombinator.com/item?id=%s"
	hnMaxResults = 2
)

type hackerNewsProvider struct {
	client *http.Client
}

type hnSearchResponse struct {
	Hits []hnHit `json:"hits"`
}

type hnHit struct {
	ObjectID    string `json:"objectID"`
	Title       string `json:"title"`
	URL         string `json:"url"`
	NumComments int    `json:"num_comments"`
	Points      int    `json:"points"`
}

func (p *hackerNewsProvider) name() string {
	return "hackernews"
}

func (p *hackerNewsProvider) hostnames() []string {
	return []string{"news.ycombinator.com", "hacker-news.firebaseio.com"}
}

func (p *hackerNewsProvider) search(ctx context.Context, entryURL string) ([]DiscussionLink, error) {
	reqURL := hnSearchURL + "?" + url.Values{
		"query":                        {entryURL},
		"restrictSearchableAttributes": {"url"},
		"tags":                         {"story"},
	}.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("hackernews: unable to create request: %w", err)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("hackernews: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		io.Copy(io.Discard, resp.Body)
		return nil, fmt.Errorf("hackernews: unexpected status %d", resp.StatusCode)
	}

	var result hnSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("hackernews: unable to decode response: %w", err)
	}

	// Filter hits to only those whose URL matches the entry URL after normalization.
	normalizedEntry := normalizeURL(entryURL)
	var matched []hnHit
	for _, hit := range result.Hits {
		if normalizeURL(hit.URL) == normalizedEntry {
			matched = append(matched, hit)
		}
	}

	// Sort by points descending.
	sort.Slice(matched, func(i, j int) bool {
		return matched[i].Points > matched[j].Points
	})

	limit := hnMaxResults
	if len(matched) < limit {
		limit = len(matched)
	}

	links := make([]DiscussionLink, 0, limit)
	for _, hit := range matched[:limit] {
		links = append(links, DiscussionLink{
			Source:   "hackernews",
			Title:    hit.Title,
			URL:      fmt.Sprintf(hnItemURL, hit.ObjectID),
			Comments: hit.NumComments,
			Score:    hit.Points,
		})
	}

	return links, nil
}
