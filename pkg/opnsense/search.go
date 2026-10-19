// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package opnsense

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// SearchParams configures a search query for the Search[K] function.
type SearchParams struct {
	// RowCount is the number of rows per page. Default: 500.
	RowCount int
	// SearchQuery is an optional filter string passed as "searchPhrase".
	SearchQuery string
}

// searchRequest is the JSON body sent to OPNsense search endpoints.
type searchRequest struct {
	Current      int    `json:"current"`
	RowCount     int    `json:"rowCount"`
	SearchPhrase string `json:"searchPhrase"`
}

// searchResponse is the JSON envelope returned by OPNsense search endpoints.
type searchResponse[K any] struct {
	Rows     []K `json:"rows"`
	RowCount int `json:"rowCount"`
	Total    int `json:"total"`
	Current  int `json:"current"`
}

// strictSearchResponse preserves presence information for authoritative
// pagination metadata. OPNsense reports rowCount as the number of rows in the
// current response page, not the requested page size.
type strictSearchResponse struct {
	Rows     json.RawMessage `json:"rows"`
	RowCount *int            `json:"rowCount"`
	Total    *int            `json:"total"`
	Current  *int            `json:"current"`
}

// Search queries an OPNsense search endpoint and transparently iterates all
// pages to return the complete result set. Search is a read-only operation
// that acquires the read semaphore per page request.
func Search[K any](ctx context.Context, c *Client, opts ReqOpts, params SearchParams) ([]K, error) {
	rowCount := params.RowCount
	if rowCount == 0 {
		rowCount = 500
	}

	var allResults []K
	page := 1
	expectedTotal := -1

	for {
		// Check context cancellation between pages.
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("search %s: %w", opts.SearchEndpoint, err)
		}

		pageResults, total, err := searchPage[K](ctx, c, opts, rowCount, params.SearchQuery, page)
		if err != nil {
			return nil, err
		}
		if total < 0 {
			return nil, fmt.Errorf("search %s: page %d reported negative total %d", opts.SearchEndpoint, page, total)
		}
		if expectedTotal == -1 {
			expectedTotal = total
		} else if total != expectedTotal {
			return nil, fmt.Errorf("search %s: page %d changed total from %d to %d", opts.SearchEndpoint, page, expectedTotal, total)
		}
		if len(allResults)+len(pageResults) > expectedTotal {
			return nil, fmt.Errorf("search %s: page %d would exceed declared total %d", opts.SearchEndpoint, page, expectedTotal)
		}

		allResults = append(allResults, pageResults...)

		if len(pageResults) == 0 && len(allResults) < expectedTotal {
			return nil, fmt.Errorf("search %s: page %d was empty after collecting %d of %d declared results", opts.SearchEndpoint, page, len(allResults), expectedTotal)
		}

		// Stop only after the declared total has been collected.
		if len(allResults) == expectedTotal {
			break
		}

		page++
	}

	// Return empty slice, not nil, for zero results.
	if allResults == nil {
		return []K{}, nil
	}

	return allResults, nil
}

// searchPage fetches a single page from the search endpoint.
// Acquires/releases the read semaphore for this individual request.
func searchPage[K any](ctx context.Context, c *Client, opts ReqOpts, rowCount int, query string, page int) ([]K, int, error) {
	if err := c.AcquireRead(ctx); err != nil {
		return nil, 0, fmt.Errorf("search %s: %w", opts.SearchEndpoint, err)
	}
	defer c.ReleaseRead()

	reqBody := searchRequest{
		Current:      page,
		RowCount:     rowCount,
		SearchPhrase: query,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, 0, fmt.Errorf("search %s: %w", opts.SearchEndpoint, err)
	}

	url := c.BaseURL() + opts.SearchEndpoint
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, 0, fmt.Errorf("search %s: %w", opts.SearchEndpoint, err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient().Do(req) //nolint:gosec // URL from provider-configured ReqOpts
	if err != nil {
		return nil, 0, NewServerError(opts.SearchEndpoint, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if httpErr := CheckHTTPError(resp.StatusCode, opts.SearchEndpoint); httpErr != nil {
		return nil, 0, httpErr
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, fmt.Errorf("search %s: failed to read response: %w", opts.SearchEndpoint, err)
	}

	var result strictSearchResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, 0, fmt.Errorf("search %s: failed to parse response: %w", opts.SearchEndpoint, err)
	}
	if len(result.Rows) == 0 || bytes.Equal(bytes.TrimSpace(result.Rows), []byte("null")) {
		return nil, 0, fmt.Errorf("search %s: page %d response is missing rows", opts.SearchEndpoint, page)
	}
	if result.RowCount == nil || result.Total == nil || result.Current == nil {
		return nil, 0, fmt.Errorf("search %s: page %d response is missing pagination metadata", opts.SearchEndpoint, page)
	}
	var rows []K
	if err := json.Unmarshal(result.Rows, &rows); err != nil {
		return nil, 0, fmt.Errorf("search %s: page %d rows are invalid: %w", opts.SearchEndpoint, page, err)
	}
	if rows == nil {
		return nil, 0, fmt.Errorf("search %s: page %d rows must be an array", opts.SearchEndpoint, page)
	}
	if *result.Current != page {
		return nil, 0, fmt.Errorf("search %s: requested page %d but response reported page %d", opts.SearchEndpoint, page, *result.Current)
	}
	if *result.RowCount < 0 || *result.RowCount != len(rows) {
		return nil, 0, fmt.Errorf("search %s: page %d rowCount %d does not match %d rows", opts.SearchEndpoint, page, *result.RowCount, len(rows))
	}

	return rows, *result.Total, nil
}
