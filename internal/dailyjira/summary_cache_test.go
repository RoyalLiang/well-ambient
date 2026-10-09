package dailyjira

import (
	"fmt"
	"testing"
)

func TestSummaryCacheBoundsOldScopesAndGenerations(t *testing.T) {
	var cache summaryCache
	key := newSummaryCacheKey(Scope{ProjectKeys: []string{" p1 ", "P1"}}, 1, "2026-10-04|UTC")
	cache.put(summaryCacheEntry{key: key, summary: Summary{Total: 17}})
	for i := 0; i < summaryCacheCapacity*3; i++ {
		other := newSummaryCacheKey(Scope{ProjectKeys: []string{fmt.Sprintf("P%d", i)}}, int64(i+2), "2026-10-04|UTC")
		cache.put(summaryCacheEntry{key: other})
		// Actively reused scopes should survive the eviction of obsolete keys.
		if got, ok := cache.get(key); !ok || got.Total != 17 {
			t.Fatal("hot scope evicted or corrupted")
		}
	}
	if len(cache.entries) != summaryCacheCapacity || cache.lru.Len() != summaryCacheCapacity {
		t.Fatal("summary cache is unbounded")
	}
	for _, basis := range []string{"2026-10-05|UTC", "2026-10-04|Asia/Shanghai"} {
		if _, ok := cache.get(newSummaryCacheKey(Scope{ProjectKeys: []string{"P1"}}, 1, basis)); ok {
			t.Fatalf("time basis %s reused stale counts", basis)
		}
	}
}
