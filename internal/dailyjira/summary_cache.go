package dailyjira

import (
	"container/list"
	"crypto/sha256"
	"encoding/json"
	"sync"
)

const summaryCacheCapacity = 128

type summaryCacheKey struct {
	scope      [sha256.Size]byte
	generation int64
	timeBasis  string
}

type summaryCacheEntry struct {
	key     summaryCacheKey
	summary Summary
}

// The cache belongs to one Reader/database. Values are immutable scoped counts,
// reused only after checking the database watermark in a repeatable-read
// snapshot. It is bounded across both scope changes and old generations.
type summaryCache struct {
	mu      sync.Mutex
	entries map[summaryCacheKey]*list.Element
	lru     list.List
}

func newSummaryCacheKey(scope Scope, generation int64, timeBasis string) summaryCacheKey {
	scope.ProjectKeys = normalizeProjects(scope.ProjectKeys)
	if scope.FilterAssignees {
		scope.Assignees = normalizeAssignees(scope.Assignees)
	} else {
		scope.Assignees = nil
	}
	payload, _ := json.Marshal(scope)
	return summaryCacheKey{scope: sha256.Sum256(payload), generation: generation, timeBasis: timeBasis}
}

func (c *summaryCache) get(key summaryCacheKey) (Summary, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if element, ok := c.entries[key]; ok {
		c.lru.MoveToFront(element)
		return element.Value.(summaryCacheEntry).summary, true
	}
	return Summary{}, false
}

func (c *summaryCache) put(entry summaryCacheEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.entries == nil {
		c.entries = make(map[summaryCacheKey]*list.Element)
	}
	if element, ok := c.entries[entry.key]; ok {
		element.Value = entry
		c.lru.MoveToFront(element)
		return
	}
	c.entries[entry.key] = c.lru.PushFront(entry)
	if c.lru.Len() > summaryCacheCapacity {
		oldest := c.lru.Back()
		delete(c.entries, oldest.Value.(summaryCacheEntry).key)
		c.lru.Remove(oldest)
	}
}
