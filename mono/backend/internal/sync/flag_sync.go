package sync

import "rmsmail/internal/models"

const inboundFlagSyncLimit = 2000

func mergeFlagSyncCandidates(primary, extra []models.Email) []models.Email {
	if len(extra) == 0 {
		return primary
	}
	seen := make(map[string]struct{}, len(primary)+len(extra))
	out := make([]models.Email, 0, len(primary)+len(extra))
	for _, e := range primary {
		if _, ok := seen[e.ID]; ok {
			continue
		}
		seen[e.ID] = struct{}{}
		out = append(out, e)
	}
	for _, e := range extra {
		if _, ok := seen[e.ID]; ok {
			continue
		}
		seen[e.ID] = struct{}{}
		out = append(out, e)
	}
	return out
}
