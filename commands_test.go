package main

import "testing"

func TestParseDateRejectsNonexistent(t *testing.T) {
	for _, s := range []string{"2026-02-30", "2026-13-01", "2026-2-1", "20260201", "2026-02-29"} {
		if _, err := parseDate(s); err == nil {
			t.Fatalf("expected error for date %q", s)
		}
	}
	if _, err := parseDate("2024-02-29"); err != nil {
		t.Fatalf("leap day should be valid: %v", err)
	}
}

func TestSelftestPasses(t *testing.T) {
	if _, err := selftest(); err != nil {
		t.Fatalf("selftest failed: %v", err)
	}
}

func TestErrorsDoNotMutateState(t *testing.T) {
	st := newState()
	id := "eq"
	r := 10.0
	d := "2026-01-01"
	if _, err := handle(st, Command{
		Type: "register_equipment", EquipmentID: &id,
		InitialReading: &r, InitialDate: &d,
	}); err != nil {
		t.Fatal(err)
	}
	// 非法读数（负数）不得改变最新读数。
	bad := -1.0
	if _, err := handle(st, Command{
		Type: "add_reading", EquipmentID: &id, Date: strpTest("2026-01-02"), Reading: &bad,
	}); err == nil {
		t.Fatal("negative reading should be rejected")
	}
	if st.Equipment["eq"].LatestRead != 10 {
		t.Fatalf("state mutated by failed command: %v", st.Equipment["eq"].LatestRead)
	}
}

func strpTest(s string) *string { return &s }
