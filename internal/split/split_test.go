package split

import "testing"

func TestComputeParts(t *testing.T) {
	tests := []struct {
		name       string
		total      int64
		leftWeight int64
		rightWeight int64
		wantA      int64
		wantB      int64
	}{
		{"2to1", 1000, 2, 1, 666, 334},
		{"equal", 9, 1, 1, 4, 5},
		{"small", 1, 2, 1, 0, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a, b, err := ComputeParts(tt.total, tt.leftWeight, tt.rightWeight)
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if a != tt.wantA || b != tt.wantB {
				t.Fatalf("got (%d,%d), want (%d,%d)", a, b, tt.wantA, tt.wantB)
			}
		})
	}
}

func TestComputePartsInvalid(t *testing.T) {
	_, _, err := ComputeParts(10, 0, 1)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestComputePartsInvariant(t *testing.T) {
	total := int64(1000000)
	a, b, err := ComputeParts(total, 1, 3)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if a+b != total {
		t.Fatalf("invariant violated: %d + %d != %d", a, b, total)
	}
}
