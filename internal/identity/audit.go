package identity

import (
	"context"
	"encoding/json"
)

// Log menulis satu baris ke audit_log. Method ini secara struktural
// memenuhi interface AuditLogger yang dideklarasikan di sisi pemakai
// (internal/tenant.AuditLogger) — di-inject dari main.go tanpa tenant perlu
// mengimpor package identity (lihat aturan wiring di CLAUDE.md).
func (s *Service) Log(ctx context.Context, schoolID, userID *int64, action, entity string, entityID *int64, oldValue, newValue any) error {
	oldJSON, err := marshalNullable(oldValue)
	if err != nil {
		return err
	}
	newJSON, err := marshalNullable(newValue)
	if err != nil {
		return err
	}
	return s.repo.InsertAuditLog(ctx, InsertAuditLogInput{
		SchoolID: schoolID,
		UserID:   userID,
		Action:   action,
		Entity:   entity,
		EntityID: entityID,
		OldValue: oldJSON,
		NewValue: newJSON,
	})
}

func marshalNullable(v any) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	return json.Marshal(v)
}
