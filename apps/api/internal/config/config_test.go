package config

import (
	"slices"
	"testing"
)

func TestWithWriteScopes(t *testing.T) {
	tests := []struct {
		name    string
		scopes  string
		enabled bool
		want    string
	}{
		{"disabled keeps reads", DefaultOAuthScopes, false, DefaultOAuthScopes},
		{"enabled adds the write set", "read:accounts", true, "read:accounts " + DefaultOAuthWriteScopes},
		{"disabled strips write scopes from an override", "read:accounts write:transactions", false, "read:accounts"},
		{"enabled does not duplicate an override's write scope", "read:accounts write:transactions", true, "read:accounts " + DefaultOAuthWriteScopes},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := withWriteScopes(tc.scopes, tc.enabled); got != tc.want {
				t.Errorf("withWriteScopes(%q, %v) = %q, want %q", tc.scopes, tc.enabled, got, tc.want)
			}
		})
	}
}

func TestSplitList(t *testing.T) {
	if got := splitList(" 172.16.0.0/12, ,10.0.0.0/8 "); !slices.Equal(got, []string{"172.16.0.0/12", "10.0.0.0/8"}) {
		t.Errorf("splitList = %v", got)
	}
	if got := splitList(""); got != nil {
		t.Errorf("splitList(\"\") = %v, want nil", got)
	}
}
