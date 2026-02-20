package pollerdb

import (
	"testing"
)

func TestAddIP_And_ListAllowedIPs(t *testing.T) {
	db := newTestDB(t)

	id, err := db.AddIP("192.168.1.1", "dev machine")
	if err != nil {
		t.Fatalf("AddIP() error = %v", err)
	}
	if id == 0 {
		t.Error("AddIP() should return non-zero ID")
	}

	db.AddIP("10.0.0.1", "office")

	entries, err := db.ListAllowedIPs()
	if err != nil {
		t.Fatalf("ListAllowedIPs() error = %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("len = %d, want 2", len(entries))
	}

	// Verify both IPs are present.
	ips := map[string]bool{}
	for _, e := range entries {
		ips[e.IP] = true
	}
	if !ips["192.168.1.1"] {
		t.Error("expected 192.168.1.1 in list")
	}
	if !ips["10.0.0.1"] {
		t.Error("expected 10.0.0.1 in list")
	}
}

func TestRemoveIP(t *testing.T) {
	db := newTestDB(t)

	id, _ := db.AddIP("1.2.3.4", "temp")

	if err := db.RemoveIP(int(id)); err != nil {
		t.Fatalf("RemoveIP() error = %v", err)
	}

	entries, _ := db.ListAllowedIPs()
	if len(entries) != 0 {
		t.Errorf("len = %d, want 0 after removal", len(entries))
	}
}

func TestRemoveIP_NotFound(t *testing.T) {
	db := newTestDB(t)

	err := db.RemoveIP(999)
	if err == nil {
		t.Error("RemoveIP() should error for nonexistent ID")
	}
}

func TestIsIPAllowed(t *testing.T) {
	db := newTestDB(t)

	db.AddIP("5.6.7.8", "allowed")

	allowed, err := db.IsIPAllowed("5.6.7.8")
	if err != nil {
		t.Fatalf("IsIPAllowed() error = %v", err)
	}
	if !allowed {
		t.Error("expected 5.6.7.8 to be allowed")
	}

	allowed, err = db.IsIPAllowed("9.9.9.9")
	if err != nil {
		t.Fatalf("IsIPAllowed() error = %v", err)
	}
	if allowed {
		t.Error("expected 9.9.9.9 to NOT be allowed")
	}
}

func TestLoadAllIPsIntoMap(t *testing.T) {
	db := newTestDB(t)

	db.AddIP("1.1.1.1", "one")
	db.AddIP("2.2.2.2", "two")
	db.AddIP("3.3.3.3", "three")

	ips, err := db.LoadAllIPsIntoMap()
	if err != nil {
		t.Fatalf("LoadAllIPsIntoMap() error = %v", err)
	}
	if len(ips) != 3 {
		t.Errorf("len = %d, want 3", len(ips))
	}
	if !ips["1.1.1.1"] {
		t.Error("expected 1.1.1.1 in map")
	}
	if !ips["2.2.2.2"] {
		t.Error("expected 2.2.2.2 in map")
	}
	if ips["4.4.4.4"] {
		t.Error("expected 4.4.4.4 NOT in map")
	}
}

func TestLoadAllIPsIntoMap_Empty(t *testing.T) {
	db := newTestDB(t)

	ips, err := db.LoadAllIPsIntoMap()
	if err != nil {
		t.Fatalf("LoadAllIPsIntoMap() error = %v", err)
	}
	if len(ips) != 0 {
		t.Errorf("len = %d, want 0", len(ips))
	}
}
