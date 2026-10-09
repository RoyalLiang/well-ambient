package codereview

import (
	"context"
	"sort"
	"strings"
	"well-ambient/internal/db"
)

type rankedSystemFact struct {
	fact  db.ContextFact
	score int
}

// Read bounded batches and retain only the best 200 eligible facts. Applying
// this cap after relevance ranking prevents unrelated repository facts from
// hiding relevant global or cross-repository evidence before it is scored.
func (s *Service) systemCorpusCandidates(ctx context.Context, terms []string) ([]db.ContextFact, error) {
	const batchSize = 256
	const candidateLimit = 200
	var afterID uint
	best := make([]rankedSystemFact, 0, candidateLimit+batchSize)
	for {
		var batch []db.ContextFact
		if err := s.DB.WithContext(ctx).Where("status = ? AND id > ?", "active", afterID).
			Where("length(COALESCE(summary, '')) + length(COALESCE(content, '')) BETWEEN 1 AND 24000").
			Order("id ASC").Limit(batchSize).Find(&batch).Error; err != nil {
			return nil, err
		}
		if len(batch) == 0 {
			break
		}
		for _, fact := range batch {
			afterID = fact.ID
			n := len(fact.Summary) + len(fact.Content)
			if n == 0 || n > 24000 {
				continue
			}
			content := strings.ToLower(fact.Summary + " " + fact.Content)
			score := 0
			for _, term := range terms {
				if strings.Contains(content, term) {
					score++
				}
			}
			best = append(best, rankedSystemFact{fact: fact, score: score})
		}
		sort.Slice(best, func(i, j int) bool {
			if best[i].score != best[j].score {
				return best[i].score > best[j].score
			}
			if !best[i].fact.UpdatedAt.Equal(best[j].fact.UpdatedAt) {
				return best[i].fact.UpdatedAt.After(best[j].fact.UpdatedAt)
			}
			return best[i].fact.ID > best[j].fact.ID
		})
		if len(best) > candidateLimit {
			best = best[:candidateLimit]
		}
		if len(batch) < batchSize {
			break
		}
	}
	facts := make([]db.ContextFact, len(best))
	for i, candidate := range best {
		facts[i] = candidate.fact
	}
	return facts, nil
}
