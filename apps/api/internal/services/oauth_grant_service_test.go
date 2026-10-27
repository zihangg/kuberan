package services

import (
	"slices"
	"testing"
	"time"

	"kuberan/internal/models"
	"kuberan/internal/testutil"
)

func TestOAuthGrantService(t *testing.T) {
	setup := func(t *testing.T) (*oauthGrantService, *models.User) {
		t.Helper()
		db := testutil.SetupTestDB(t)
		t.Cleanup(func() { testutil.TeardownTestDB(t, db) })
		return NewOAuthGrantService(db).(*oauthGrantService), testutil.CreateTestUser(t, db)
	}
	record := func(t *testing.T, svc OAuthGrantServicer, userID string, approved ...string) *models.OAuthClientGrant {
		t.Helper()
		g, err := svc.RecordConsent(RecordConsentInput{
			UserID: userID, ClientID: "cid", ClientName: "Claude",
			Approved: approved, Reviewed: []string{"read:accounts", "write:transactions"}, AutoAccept: true,
		})
		testutil.AssertNoError(t, err)
		return g
	}

	t.Run("get_unknown_returns_not_found", func(t *testing.T) {
		svc, user := setup(t)
		_, err := svc.Get(user.ID, "cid")
		testutil.AssertAppError(t, err, "OAUTH_CLIENT_NOT_FOUND")
	})

	t.Run("record_consent_upserts_and_clears_revocation", func(t *testing.T) {
		svc, user := setup(t)
		first := record(t, svc, user.ID, "read:accounts")
		testutil.AssertNoError(t, svc.Revoke(user.ID, "cid"))

		second := record(t, svc, user.ID, "read:accounts", "write:transactions")
		if second.ID != first.ID {
			t.Errorf("expected upsert of the same row, got %s vs %s", first.ID, second.ID)
		}
		got, err := svc.Get(user.ID, "cid")
		testutil.AssertNoError(t, err)
		if got.IsRevoked() {
			t.Error("re-consent must clear the revocation")
		}
		if !slices.Equal(got.Approved(), []string{"read:accounts", "write:transactions"}) {
			t.Errorf("unexpected approved scopes: %v", got.Approved())
		}
		if !slices.Equal(got.Reviewed(), []string{"read:accounts", "write:transactions"}) {
			t.Errorf("unexpected reviewed scopes: %v", got.Reviewed())
		}
	})

	t.Run("grants_are_per_user", func(t *testing.T) {
		svc, user := setup(t)
		other := testutil.CreateTestUser(t, svc.db)
		record(t, svc, user.ID, "read:accounts")

		_, err := svc.Get(other.ID, "cid")
		testutil.AssertAppError(t, err, "OAUTH_CLIENT_NOT_FOUND")
		grants, err := svc.List(other.ID)
		testutil.AssertNoError(t, err)
		if len(grants) != 0 {
			t.Errorf("expected no grants for another user, got %d", len(grants))
		}
	})

	t.Run("revoke_hides_from_list_and_stops_auto_accept", func(t *testing.T) {
		svc, user := setup(t)
		record(t, svc, user.ID, "read:accounts")

		testutil.AssertNoError(t, svc.Revoke(user.ID, "cid"))
		grants, err := svc.List(user.ID)
		testutil.AssertNoError(t, err)
		if len(grants) != 0 {
			t.Errorf("expected revoked grant to be hidden, got %d", len(grants))
		}
		got, err := svc.Get(user.ID, "cid")
		testutil.AssertNoError(t, err)
		if !got.IsRevoked() || got.AutoAccept {
			t.Errorf("expected revoked, non-auto-accept grant, got %+v", got)
		}
		testutil.AssertAppError(t, svc.Revoke(user.ID, "cid"), "OAUTH_CLIENT_NOT_FOUND")
	})

	t.Run("authorize", func(t *testing.T) {
		svc, user := setup(t)
		record(t, svc, user.ID, "read:accounts", "write:transactions")

		testutil.AssertNoError(t, svc.Authorize(user.ID, "cid", "write:transactions"))
		testutil.AssertAppError(t, svc.Authorize(user.ID, "cid", "write:budgets"), "OAUTH_SCOPE_NOT_GRANTED")
		testutil.AssertAppError(t, svc.Authorize(user.ID, "other", "write:transactions"), "OAUTH_SCOPE_NOT_GRANTED")
		testutil.AssertAppError(t, svc.Authorize(user.ID, "", "write:transactions"), "OAUTH_SCOPE_NOT_GRANTED")
	})

	t.Run("authorize_records_last_use_throttled", func(t *testing.T) {
		svc, user := setup(t)
		record(t, svc, user.ID, "write:transactions")
		base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
		lastUsed := func() time.Time {
			g, err := svc.Get(user.ID, "cid")
			testutil.AssertNoError(t, err)
			if g.LastUsedAt == nil {
				t.Fatal("expected last_used_at to be set")
			}
			return g.LastUsedAt.UTC()
		}

		svc.now = func() time.Time { return base }
		testutil.AssertNoError(t, svc.Authorize(user.ID, "cid", "write:transactions"))
		svc.now = func() time.Time { return base.Add(30 * time.Second) }
		testutil.AssertNoError(t, svc.Authorize(user.ID, "cid", "write:transactions"))
		if !lastUsed().Equal(base) {
			t.Errorf("expected throttled last_used_at %v, got %v", base, lastUsed())
		}
		svc.now = func() time.Time { return base.Add(2 * time.Minute) }
		testutil.AssertNoError(t, svc.Authorize(user.ID, "cid", "write:transactions"))
		if !lastUsed().Equal(base.Add(2 * time.Minute)) {
			t.Errorf("expected last_used_at to advance, got %v", lastUsed())
		}
	})

	t.Run("authorize_rejects_revoked_grant", func(t *testing.T) {
		svc, user := setup(t)
		record(t, svc, user.ID, "write:transactions")
		testutil.AssertNoError(t, svc.Revoke(user.ID, "cid"))
		testutil.AssertAppError(t, svc.Authorize(user.ID, "cid", "write:transactions"), "OAUTH_SCOPE_NOT_GRANTED")
	})

	t.Run("authorize_rejects_inactive_user", func(t *testing.T) {
		svc, user := setup(t)
		record(t, svc, user.ID, "write:transactions")
		if err := svc.db.Model(user).Update("is_active", false).Error; err != nil {
			t.Fatal(err)
		}
		testutil.AssertAppError(t, svc.Authorize(user.ID, "cid", "write:transactions"), "OAUTH_SCOPE_NOT_GRANTED")
	})
}
