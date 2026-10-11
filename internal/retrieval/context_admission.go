package retrieval

// ContextItemAdmission costs the final representation after service-owned
// ordering and eligibility. It cannot fetch candidates or change ranking.
// Calls without this optional boundary retain native assembly defaults.
type ContextItemAdmission interface {
	AdmitContextItem(section string, hit SearchHit) bool
	NoteContextItemLimit(hit SearchHit)
}

func selectContextSection(hits []SearchHit, section string, quota int, remaining *int, admission ContextItemAdmission) []SearchHit {
	selected := make([]SearchHit, 0)
	for _, hit := range hits {
		if len(selected) >= quota {
			break
		}
		if *remaining <= 0 {
			if admission == nil {
				break
			}
			admission.NoteContextItemLimit(hit)
			continue
		}
		if admission != nil && !admission.AdmitContextItem(section, hit) {
			continue
		}
		selected = append(selected, hit)
		*remaining--
	}
	return selected
}
