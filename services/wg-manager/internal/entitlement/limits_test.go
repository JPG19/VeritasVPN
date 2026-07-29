package entitlement

import "testing"

func TestNormalizeTier(t *testing.T) {
	if got := NormalizeTier(""); got != TierFree {
		t.Fatalf("empty -> free, got %q", got)
	}
	if got := NormalizeTier("PREMIUM"); got != TierPremium {
		t.Fatalf("PREMIUM -> premium, got %q", got)
	}
	if got := NormalizeTier("unknown"); got != TierFree {
		t.Fatalf("unknown -> free, got %q", got)
	}
}

func TestCheckCreatePeerDeviceLimit(t *testing.T) {
	err := CheckCreatePeer(TierFree, 1, "", nil)
	if err == nil {
		t.Fatal("expected free device limit")
	}
	pe, ok := err.(*PlanError)
	if !ok || pe.Code != "plan_device_limit_free" {
		t.Fatalf("unexpected error: %#v", err)
	}

	if err := CheckCreatePeer(TierPremium, 4, "", nil); err != nil {
		t.Fatalf("premium under limit: %v", err)
	}
	if err := CheckCreatePeer(TierPremium, 5, "", nil); err == nil {
		t.Fatal("expected premium device limit")
	}
}

func TestCheckCreatePeerRegion(t *testing.T) {
	allowed := []string{"local"}
	if err := CheckCreatePeer(TierFree, 0, "eu-west", allowed); err == nil {
		t.Fatal("expected region denied")
	}
	if err := CheckCreatePeer(TierFree, 0, "local", allowed); err != nil {
		t.Fatalf("local should be allowed: %v", err)
	}
	if err := CheckCreatePeer(TierPremium, 0, "eu-west", allowed); err != nil {
		t.Fatalf("premium ignores free allow-list: %v", err)
	}
}

func TestCheckSelectedRegion(t *testing.T) {
	allowed := []string{"local"}
	if err := CheckSelectedRegion(TierFree, "ams", allowed); err == nil {
		t.Fatal("expected deny")
	}
	if err := CheckSelectedRegion(TierFree, "local", allowed); err != nil {
		t.Fatal(err)
	}
	if err := CheckSelectedRegion(TierFree, "ams", nil); err != nil {
		t.Fatal("empty allow-list should allow any")
	}
}
