package runtimeknowledge

import "encoding/json"

// Reuse is stateless: Context still authorizes and revalidates every source before
// comparing the caller's receipt. No source text or credentials enter a cache.
func finishContext(out ContextResponse, req ContextRequest) ContextResponse {
	for _, ref := range out.Evidence {
		out.AuditRefs = append(out.AuditRefs, ref.SourceRef)
	}
	receipt, _ := json.Marshal(struct {
		Snapshot  uint
		CheckedAt any
		Compact   string
		Evidence  []EvidenceRef
		Gaps      []string
	}{out.SnapshotID, out.CheckedAt, out.Compact, out.Evidence, out.Gaps})
	out.MemoryHash = hash(string(receipt))
	if req.KnownMemoryHash != "" && req.KnownMemoryHash == out.MemoryHash {
		out.Unchanged = true
		out.ReusedBytes = len(out.Compact)
		out.Compact = ""
		out.Evidence = []EvidenceRef{}
	}
	return out
}
