//go:build rwa

package e2e_test

import "testing"

// TestRWAPortability asserts the Phase 5 -> Phase 3 cross-phase contract
// (design spec §4 / D7):
//   - an UNRESTRICTED rwa/{id} round-trips Vertix<->Vertix over ICS-20;
//   - a RESTRICTED rwa/{id} (non-empty allow/deny list) is REJECTED at the
//     source boundary (the bank SendRestrictionFn blocks the escrow send).
//
// Enabled only after x/rwa (Phase 3) exists. Run with `go test -tags rwa`.
func TestRWAPortability_UnrestrictedRoundTrips(t *testing.T) {
	t.Skip("blocked on x/rwa (Phase 3): register an rwa asset, mint rwa/{id}, then ICS-20 round-trip it")
}

func TestRWAPortability_RestrictedRejectedAtSource(t *testing.T) {
	t.Skip("blocked on x/rwa (Phase 3): a restricted rwa/{id} send into the IBC escrow account must be rejected")
}
