package sonarr

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

func page(number, size, total int, ids ...int) QueuePage {
	result := QueuePage{Page: number, PageSize: size, TotalRecords: total, Records: []QueueItem{}}
	for _, id := range ids {
		result.Records = append(result.Records, QueueItem{ID: id})
	}
	return result
}

func TestQueueReadsAndConfirmsAllPages(t *testing.T) {
	responses := []QueuePage{page(1, 2, 3, 11, 12), page(2, 2, 3, 13), page(1, 2, 3, 11, 12), page(2, 2, 3, 13)}
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls >= len(responses) {
			t.Error("unexpected extra page request")
			w.WriteHeader(500)
			return
		}
		expected := responses[calls]
		calls++
		q := r.URL.Query()
		if r.URL.Path != "/api/v3/queue" || q.Get("page") != strconv.Itoa(expected.Page) || q.Get("pageSize") != "100" || q.Get("sortKey") != "title" || q.Get("sortDirection") != "ascending" {
			t.Errorf("unexpected page request: %s", r.URL.String())
		}
		_ = json.NewEncoder(w).Encode(expected)
	}))
	defer srv.Close()
	result, err := NewClient(srv.URL, "test-key").Queue(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if calls != 4 || result.TotalRecords != 3 || result.Page != 1 || result.PageSize != 3 || len(result.Records) != 3 || result.Records[2].ID != 13 {
		t.Fatalf("incomplete result after %d calls: %+v", calls, result)
	}
}

func TestQueueRejectsIncompleteOrChangingPages(t *testing.T) {
	tests := []struct {
		name  string
		pages []QueuePage
	}{
		{"missing_metadata", []QueuePage{{}}},
		{"negative_total", []QueuePage{page(1, 2, -1)}},
		{"invalid_page_size", []QueuePage{page(1, 0, 0)}},
		{"oversized_page_size", []QueuePage{page(1, 101, 0)}},
		{"short_first_page", []QueuePage{page(1, 2, 3, 1)}},
		{"extra_records", []QueuePage{page(1, 2, 1, 1, 2)}},
		{"missing_later_page", []QueuePage{page(1, 2, 3, 1, 2), page(2, 2, 3)}},
		{"repeated_page", []QueuePage{page(1, 2, 3, 1, 2), page(1, 2, 3, 1, 2)}},
		{"duplicate_id", []QueuePage{page(1, 2, 3, 1, 2), page(2, 2, 3, 2)}},
		{"duplicate_within_page", []QueuePage{page(1, 2, 2, 1, 1)}},
		{"growing_total", []QueuePage{page(1, 2, 3, 1, 2), page(2, 2, 4, 3, 4)}},
		{"shrinking_total", []QueuePage{page(1, 2, 3, 1, 2), page(2, 2, 2)}},
		{"changing_page_size", []QueuePage{page(1, 2, 3, 1, 2), page(2, 1, 3, 3)}},
		{"excessive_pages", []QueuePage{page(1, 1, maxQueuePages+1, 1)}},
		{"changed_membership_same_total", []QueuePage{page(1, 1, 2, 1), page(2, 1, 2, 2), page(1, 1, 2, 1), page(2, 1, 2, 3)}},
		{"shrinks_to_empty_between_scans", []QueuePage{page(1, 1, 2, 1), page(2, 1, 2, 2), page(1, 1, 0)}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if calls >= len(tc.pages) {
					t.Error("unexpected extra request")
					w.WriteHeader(500)
					return
				}
				response := tc.pages[calls]
				calls++
				_ = json.NewEncoder(w).Encode(response)
			}))
			defer srv.Close()
			result, err := NewClient(srv.URL, "test-key").Queue(context.Background())
			if err == nil || result != nil {
				t.Fatalf("returned partial/inconsistent queue: %+v, error: %v", result, err)
			}
			if calls != len(tc.pages) {
				t.Fatalf("made %d requests, want %d", calls, len(tc.pages))
			}
		})
	}
}

func TestQueuePageFailureReturnsNoPartialResult(t *testing.T) {
	for _, failAt := range []int{2, 4} {
		t.Run(strconv.Itoa(failAt), func(t *testing.T) {
			calls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if calls == failAt {
					http.Error(w, "unavailable", http.StatusServiceUnavailable)
					return
				}
				n, _ := strconv.Atoi(r.URL.Query().Get("page"))
				_ = json.NewEncoder(w).Encode(page(n, 1, 2, n))
			}))
			defer srv.Close()
			result, err := NewClient(srv.URL, "test-key").Queue(context.Background())
			if err == nil || result != nil || calls != failAt {
				t.Fatalf("result=%+v err=%v calls=%d", result, err, calls)
			}
		})
	}
}

func TestQueueStopsWhenContextIsCanceledBetweenPages(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(page(1, 1, 2, 1))
		cancel()
	}))
	defer srv.Close()
	result, err := NewClient(srv.URL, "test-key").Queue(ctx)
	if result != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestQueueRequiresExplicitEmptyQueue(t *testing.T) {
	for _, body := range []string{
		`null`,
		`{"page":1,"pageSize":100,"records":[]}`,
		`{"page":1,"pageSize":100,"totalRecords":0}`,
		`{"page":1,"pageSize":100,"totalRecords":0,"records":null}`,
		`{"page":1,"pageSize":100,"totalRecords":1,"records":[{}]}`,
		`{"page":1,"pageSize":100,"totalRecords":1,"records":[null]}`,
	} {
		t.Run(body, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(body)) }))
			defer srv.Close()
			result, err := NewClient(srv.URL, "test-key").Queue(context.Background())
			if result != nil || err == nil {
				t.Fatalf("accepted missing data as empty queue: %+v, %v", result, err)
			}
		})
	}
}
