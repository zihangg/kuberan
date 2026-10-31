package services

import (
	"strings"
	"testing"

	"kuberan/internal/models"
	"kuberan/internal/testutil"
	"kuberan/internal/uuid"
)

func TestAuditServiceLog(t *testing.T) {
	validUser := uuid.New()
	validResource := uuid.New()

	tests := []struct {
		name         string
		userID       string
		resourceID   string
		changes      map[string]any
		wantUser     *string
		wantResource *string
		wantInJSON   []string
	}{
		{
			name: "valid_ids_are_stored", userID: validUser, resourceID: validResource,
			wantUser: &validUser, wantResource: &validResource,
		},
		{
			// LOGIN_FAILED, OAUTH_LOGIN_DENIED and friends have no user. On
			// Postgres an empty string is not a valid uuid, so these rows were
			// dropped before user_id became nullable.
			name: "empty_ids_are_null", userID: "", resourceID: "",
		},
		{
			name: "uppercase_uuid_is_canonicalized", userID: strings.ToUpper(validUser), resourceID: "",
			wantUser: &validUser,
		},
		{
			name: "non_uuid_resource_is_kept_in_changes", userID: validUser, resourceID: "claude-1",
			changes:  map[string]any{"client_name": "Claude"},
			wantUser: &validUser, wantInJSON: []string{`"resource_ref":"claude-1"`, `"client_name":"Claude"`},
		},
		{
			name: "non_uuid_user_is_kept_in_changes", userID: "not-a-uuid", resourceID: validResource,
			wantResource: &validResource, wantInJSON: []string{`"user_ref":"not-a-uuid"`},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			db := testutil.SetupTestDB(t)
			defer testutil.TeardownTestDB(t, db)

			NewAuditService(db).Log(tc.userID, "TEST_ACTION", "thing", tc.resourceID, "198.51.100.1", tc.changes)

			var entry models.AuditLog
			if err := db.Where("action = ?", "TEST_ACTION").First(&entry).Error; err != nil {
				t.Fatalf("audit row was not written: %v", err)
			}
			if !equalPtr(entry.UserID, tc.wantUser) {
				t.Errorf("user_id = %v, want %v", deref(entry.UserID), deref(tc.wantUser))
			}
			if !equalPtr(entry.ResourceID, tc.wantResource) {
				t.Errorf("resource_id = %v, want %v", deref(entry.ResourceID), deref(tc.wantResource))
			}
			for _, want := range tc.wantInJSON {
				if !strings.Contains(entry.Changes, want) {
					t.Errorf("changes %s missing %s", entry.Changes, want)
				}
			}
		})
	}

	t.Run("does_not_mutate_the_callers_changes", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		defer testutil.TeardownTestDB(t, db)
		changes := map[string]any{"a": 1}

		NewAuditService(db).Log(validUser, "TEST_ACTION", "thing", "claude-1", "", changes)

		if len(changes) != 1 {
			t.Errorf("caller's map was modified: %v", changes)
		}
	})
}

func equalPtr(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func deref(p *string) string {
	if p == nil {
		return "<nil>"
	}
	return *p
}
