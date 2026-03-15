package flags

import "testing"

func TestStoreTenantIsolation(t *testing.T) {
	store := NewStore()
	store.UpsertFlag("tenant-a", Flag{Key: "new-ui", Enabled: true, Rollout: 100})

	if _, ok := store.GetFlag("tenant-b", "new-ui"); ok {
		t.Fatalf("flag leaked across tenants")
	}
}

func TestEvaluateDeterministicForSameInput(t *testing.T) {
	store := NewStore()
	store.UpsertFlag("tenant-a", Flag{Key: "checkout-v2", Enabled: true, Rollout: 42})

	first, ok := store.Evaluate("tenant-a", "checkout-v2", "user-123")
	if !ok {
		t.Fatalf("expected flag to exist")
	}

	for i := 0; i < 20; i++ {
		next, exists := store.Evaluate("tenant-a", "checkout-v2", "user-123")
		if !exists {
			t.Fatalf("expected flag to exist in iteration %d", i)
		}
		if next.Enabled != first.Enabled {
			t.Fatalf("non-deterministic enabled in iteration %d", i)
		}
		if next.HashSlot != first.HashSlot {
			t.Fatalf("non-deterministic hash slot in iteration %d", i)
		}
	}
}

func TestDeleteFlagRemovesConfig(t *testing.T) {
	store := NewStore()
	store.UpsertFlag("tenant-a", Flag{Key: "promo", Enabled: true, Rollout: 100})

	deleted := store.DeleteFlag("tenant-a", "promo")
	if !deleted {
		t.Fatalf("expected delete to return true")
	}
	if _, ok := store.GetFlag("tenant-a", "promo"); ok {
		t.Fatalf("expected deleted flag to be unavailable")
	}
}
