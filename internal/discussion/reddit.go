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
	redditSearchURL  = "https://www.reddit.com/search.json"
	redditBaseURL    = "https://www.reddit.com"
	redditMaxResults = 3
)

type redditProvider struct {
	client    *http.Client
	userAgent string
}

type redditSearchResponse struct {
	Data struct {
		Children []redditChild `json:"children"`
	} `json:"data"`
}

type redditChild struct {
	Data redditPost `json:"data"`
}

type redditPost struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	Permalink   string `json:"permalink"`
	NumComments int    `json:"num_comments"`
	Score       int    `json:"score"`
	Subreddit   string `json:"subreddit"`
	Selftext    string `json:"selftext"`
}

func (p *redditProvider) name() string {
	return "reddit"
}

func (p *redditProvider) hostnames() []string {
	return []string{"reddit.com", "www.reddit.com", "old.reddit.com", "redd.it"}
}

func (p *redditProvider) search(ctx context.Context, entryURL string) ([]DiscussionLink, error) {
	reqURL := redditSearchURL + "?" + url.Values{
		"q":     {"url:" + entryURL},
		"sort":  {"relevance"},
		"limit": {"5"},
	}.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("reddit: unable to create request: %w", err)
	}

	// Reddit requires a descriptive User-Agent for unauthenticated API access.
	req.Header.Set("User-Agent", p.userAgent)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("reddit: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		io.Copy(io.Discard, resp.Body)
		return nil, fmt.Errorf("reddit: unexpected status %d", resp.StatusCode)
	}

	var result redditSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("reddit: unable to decode response: %w", err)
	}

	// Filter posts to only those whose URL matches the entry URL after normalization.
	normalizedEntry := normalizeURL(entryURL)
	var matched []redditPost
	for _, child := range result.Data.Children {
		if normalizeURL(child.Data.URL) == normalizedEntry {
			matched = append(matched, child.Data)
		}
	}

	// Sort by comment count descending.
	sort.Slice(matched, func(i, j int) bool {
		return matched[i].NumComments > matched[j].NumComments
	})

	limit := redditMaxResults
	if len(matched) < limit {
		limit = len(matched)
	}

	links := make([]DiscussionLink, 0, limit)
	for _, post := range matched[:limit] {
		links = append(links, DiscussionLink{
			Source:    "reddit",
			Title:     post.Title,
			URL:       redditBaseURL + post.Permalink,
			Comments:  post.NumComments,
			Score:     post.Score,
			Community: post.Subreddit,
		})
	}

	return links, nil
}
