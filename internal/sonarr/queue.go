package sonarr

import (
	"context"
	"fmt"
	"strconv"
	"time"
)

const (
	queuePageSize    = 100
	maxQueuePages    = 100
	queueReadTimeout = 30 * time.Second
)

// Queue returns a synthetic single page containing the complete queue. It
// rejects incomplete or inconsistent pagination instead of returning partial
// data that a caller might mistake for proof of an item's absence.
func (c *HTTPClient) Queue(ctx context.Context) (*QueuePage, error) {
	ctx, cancel := context.WithTimeout(ctx, queueReadTimeout)
	defer cancel()
	first, pages, err := c.queueSnapshot(ctx)
	if err != nil {
		return nil, err
	}
	if pages > 1 {
		// Sonarr provides no snapshot token. Require matching membership in
		// two bounded scans to detect churn that total counts alone miss.
		second, _, err := c.queueSnapshot(ctx)
		if err != nil {
			return nil, fmt.Errorf("confirm complete queue: %w", err)
		}
		if !sameQueueIDs(first.Records, second.Records) {
			return nil, fmt.Errorf("queue membership changed between complete scans")
		}
		return second, nil
	}
	return first, nil
}

func sameQueueIDs(left, right []QueueItem) bool {
	if len(left) != len(right) {
		return false
	}
	ids := make(map[int]bool, len(left))
	for _, item := range left {
		ids[item.ID] = true
	}
	for _, item := range right {
		if !ids[item.ID] {
			return false
		}
	}
	return true
}

func (c *HTTPClient) queueSnapshot(ctx context.Context) (*QueuePage, int, error) {
	var records []QueueItem
	seen := make(map[int]bool)
	total, pageSize := 0, 0
	for page := 1; page <= maxQueuePages; page++ {
		u := c.url("/api/v3/queue")
		q := u.Query()
		q.Set("page", strconv.Itoa(page))
		q.Set("pageSize", strconv.Itoa(queuePageSize))
		q.Set("sortKey", "title")
		q.Set("sortDirection", "ascending")
		u.RawQuery = q.Encode()

		// Missing fields must not decode to a plausible empty queue. In
		// particular, omitted totalRecords/records is not proof of absence.
		var response struct {
			Page         *int `json:"page"`
			PageSize     *int `json:"pageSize"`
			TotalRecords *int `json:"totalRecords"`
			Records      []struct {
				QueueItem
				ID *int `json:"id"`
			} `json:"records"`
		}
		if err := c.get(ctx, u.String(), &response); err != nil {
			return nil, 0, fmt.Errorf("get queue page %d: %w", page, err)
		}
		if response.Page == nil || response.PageSize == nil || response.TotalRecords == nil || response.Records == nil {
			return nil, 0, fmt.Errorf("missing required fields on queue page %d", page)
		}
		result := QueuePage{Page: *response.Page, PageSize: *response.PageSize, TotalRecords: *response.TotalRecords}
		for _, record := range response.Records {
			if record.ID == nil {
				return nil, 0, fmt.Errorf("missing queue entry identity on page %d", page)
			}
			item := record.QueueItem
			item.ID = *record.ID
			result.Records = append(result.Records, item)
		}
		if result.Page != page || result.PageSize <= 0 || result.PageSize > queuePageSize || result.TotalRecords < 0 {
			return nil, 0, fmt.Errorf("invalid pagination metadata on queue page %d", page)
		}
		if page == 1 {
			total, pageSize = result.TotalRecords, result.PageSize
			if total > maxQueuePages*pageSize {
				return nil, 0, fmt.Errorf("queue exceeds %d-page safety limit", maxQueuePages)
			}
		} else if result.TotalRecords != total || result.PageSize != pageSize {
			return nil, 0, fmt.Errorf("queue pagination changed on page %d", page)
		}
		if len(result.Records) != min(pageSize, total-len(records)) {
			return nil, 0, fmt.Errorf("incomplete or oversized queue page %d", page)
		}
		for _, item := range result.Records {
			if seen[item.ID] {
				return nil, 0, fmt.Errorf("duplicate queue entry on page %d", page)
			}
			seen[item.ID] = true
			records = append(records, item)
		}
		if len(records) == total {
			return &QueuePage{Page: 1, PageSize: len(records), TotalRecords: total, Records: records}, page, nil
		}
	}
	return nil, 0, fmt.Errorf("queue exceeds %d-page safety limit", maxQueuePages)
}
