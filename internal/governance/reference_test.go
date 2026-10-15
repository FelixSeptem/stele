package governance

import (
 "context"
 "testing"
 "time"

 "github.com/FelixSeptem/stele/internal/memory"
 "github.com/FelixSeptem/stele/internal/policy"
)

func TestReferenceEventsUseGovernedDurableEpisodicCandidates(t *testing.T) {
 for _, kind := range []string{"provider.reference.task", "provider.reference.run", "provider.reference.handoff", "provider.reference.compilation"} {
  t.Run(kind, func(t *testing.T) {
   event := memory.RawEvent{ID:"0192a1a2-01f1-4000-8000-000000000001", EventType:kind, Content:"artifact-1", MemoryPath:"tasks/run-1", Metadata:map[string]any{"reference_contract_version":"durable-reference-v1", "reference_id":"ref-1"}}
   items, err := (RuleBasedExtractor{}).Extract(context.Background(),event)
   if err != nil || len(items)!=1 { t.Fatalf("reference extraction: %d items, %v",len(items),err) }
   item := items[0].ToCandidateMemory("candidate",event,time.Now())
   if item.MemoryPath != event.MemoryPath || item.SourceRawEventID != event.ID || item.Content != event.Content || item.Class != memory.MemoryClassEpisodic || item.RetentionClass != policy.RetentionClassDurable { t.Fatalf("lost reference lineage: %+v",item) }
  })
 }
}

func TestUnsupportedOrUnmarkedReferenceEventsRemainArchival(t *testing.T) {
 for _, event := range []memory.RawEvent{
  {EventType:"task.completed",Content:"value"},
  {EventType:"provider.reference.task",Content:"value"},
  {EventType:"provider.reference.unknown",Content:"value",Metadata:map[string]any{"reference_contract_version":"durable-reference-v1","reference_id":"ref-1"}},
 } {
  items,err := (RuleBasedExtractor{}).Extract(context.Background(),event)
  if err != nil || len(items)!=0 { t.Fatalf("archival event was promoted: %+v, %v",items,err) }
 }
}
