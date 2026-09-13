// Package simarr provides deterministic in-process *arr APIs for integration tests.
package simarr

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"

	"github.com/patflynn/reel-life/internal/sonarr"
)

// Sonarr is a stateful subset of the Sonarr API used by remediation tests.
type Sonarr struct {
	mu            sync.Mutex
	server        *httptest.Server
	queue         []sonarr.QueueItem
	deleteStatus  int
	deleteCount   int
	lastBlocklist bool
	queueStatuses []int
	retainDeleted bool
}

func NewSonarr(queue []sonarr.QueueItem) *Sonarr {
	s := &Sonarr{queue: append([]sonarr.QueueItem(nil), queue...)}
	s.server = httptest.NewServer(http.HandlerFunc(s.serveHTTP))
	return s
}

func (s *Sonarr) URL() string { return s.server.URL }
func (s *Sonarr) Close()      { s.server.Close() }

// FailDeletes injects an HTTP status for delete operations. Zero disables it.
func (s *Sonarr) FailDeletes(status int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deleteStatus = status
}

// QueueStatuses scripts HTTP responses for successive queue reads. Zero means
// a normal response; after the script is exhausted, normal responses resume.
func (s *Sonarr) QueueStatuses(statuses ...int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.queueStatuses = append([]int(nil), statuses...)
}

// RetainDeleted simulates a successful delete response with no state change.
func (s *Sonarr) RetainDeleted() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.retainDeleted = true
}

func (s *Sonarr) DeleteResult() (count int, blocklist bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.deleteCount, s.lastBlocklist
}

func (s *Sonarr) serveHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")

	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/api/v3/queue":
		if len(s.queueStatuses) > 0 {
			status := s.queueStatuses[0]
			s.queueStatuses = s.queueStatuses[1:]
			if status != 0 {
				http.Error(w, "injected queue failure", status)
				return
			}
		}
		_ = json.NewEncoder(w).Encode(sonarr.QueuePage{Page: 1, PageSize: len(s.queue), TotalRecords: len(s.queue), Records: s.queue})
	case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/api/v3/queue/"):
		s.deleteCount++
		s.lastBlocklist = r.URL.Query().Get("blocklist") == "true"
		if s.deleteStatus != 0 {
			http.Error(w, "injected delete failure", s.deleteStatus)
			return
		}
		id, err := strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/api/v3/queue/"))
		if err != nil {
			http.Error(w, "invalid queue id", http.StatusBadRequest)
			return
		}
		for i, item := range s.queue {
			if item.ID == id {
				if !s.retainDeleted {
					s.queue = append(s.queue[:i], s.queue[i+1:]...)
				}
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}
		http.Error(w, "not found", http.StatusNotFound)
	default:
		http.Error(w, "not implemented", http.StatusNotImplemented)
	}
}
