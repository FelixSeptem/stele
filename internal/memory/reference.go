package memory

const DurableReferenceVersion = "durable-reference-v1"

// ReferenceEventTypes is a closed admission list. Other event types keep their
// existing archival or extraction semantics.
func ReferenceEventTypes() []string {
 return []string{"provider.reference.task", "provider.reference.run", "provider.reference.handoff", "provider.reference.compilation"}
}

func IsReferenceEventType(kind string) bool {
 for _, supported := range ReferenceEventTypes() { if kind == supported { return true } }
 return false
}
