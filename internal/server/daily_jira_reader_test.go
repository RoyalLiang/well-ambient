package server

import (
	"sync"
	"testing"

	"well-ambient/internal/dailyjira"
	"well-ambient/internal/db"

	"gorm.io/gorm"
)

func TestDailyJiraReaderLifecycleIsBoundToServerAndDatabase(t *testing.T) {
	previous := db.DB
	t.Cleanup(func() { db.DB = previous })
	// No query is issued here: this checks ownership before the module's
	// PostgreSQL tests exercise actual cache invalidation and scope boundaries.
	db.DB = &gorm.DB{}
	server := &Server{}
	first := server.dailyJiraReadModel()
	readers := make(chan *dailyjira.Reader, 32)
	var wg sync.WaitGroup
	for i := 0; i < cap(readers); i++ {
		wg.Add(1)
		go func() { defer wg.Done(); readers <- server.dailyJiraReadModel() }()
	}
	wg.Wait()
	close(readers)
	for reader := range readers {
		if reader != first {
			t.Fatal("requests created separate Readers")
		}
	}
	otherServer := &Server{}
	if otherServer.dailyJiraReadModel() == first {
		t.Fatal("cache escaped server ownership")
	}
	db.DB = &gorm.DB{}
	if server.dailyJiraReadModel() == first {
		t.Fatal("database replacement reused previous cache")
	}
}
