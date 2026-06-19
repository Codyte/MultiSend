package chunk

import "testing"

func TestAutoChunkSize(t *testing.T) {
	if got := AutoChunkSize(0); got != 2*MiB {
		t.Fatalf("got %d", got)
	}
	if got := AutoChunkSize(100*MiB - 1); got != 2*MiB {
		t.Fatalf("got %d", got)
	}
	if got := AutoChunkSize(100 * MiB); got != 4*MiB {
		t.Fatalf("got %d", got)
	}
	if got := AutoChunkSize(1024 * MiB); got != 4*MiB {
		t.Fatalf("got %d", got)
	}
	if got := AutoChunkSize(1024*MiB + 1); got != 8*MiB {
		t.Fatalf("got %d", got)
	}
	if got := AutoChunkSize(4 * 1024 * MiB); got != 8*MiB {
		t.Fatalf("got %d", got)
	}
	if got := AutoChunkSize(4*1024*MiB + 1); got != 16*MiB {
		t.Fatalf("got %d", got)
	}
}

func TestBuildPlanCases(t *testing.T) {
	tests := []struct {
		name      string
		total     int64
		chunkSize int64
		count     int64
		lastSize  int64
	}{
		{name: "zero", total: 0, chunkSize: 8 * MiB, count: 0, lastSize: 0},
		{name: "smaller", total: 3 * MiB, chunkSize: 8 * MiB, count: 1, lastSize: 3 * MiB},
		{name: "exact multiple", total: 16 * MiB, chunkSize: 8 * MiB, count: 2, lastSize: 8 * MiB},
		{name: "last partial", total: 17 * MiB, chunkSize: 8 * MiB, count: 3, lastSize: 1 * MiB},
		{name: "large", total: 10 * 1024 * MiB, chunkSize: 64 * MiB, count: 160, lastSize: 64 * MiB},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan, err := BuildPlan(tt.total, tt.chunkSize)
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if got := int64(len(plan.Chunks)); got != tt.count {
				t.Fatalf("count got %d want %d", got, tt.count)
			}
			if tt.count > 0 {
				if got := plan.Chunks[len(plan.Chunks)-1].Size; got != tt.lastSize {
					t.Fatalf("last size got %d want %d", got, tt.lastSize)
				}
			}
		})
	}
}

func TestInvalidChunkSize(t *testing.T) {
	if _, err := BuildPlan(10, 0); err == nil {
		t.Fatal("expected err")
	}
	if _, err := BuildPlan(-1, 8*MiB); err == nil {
		t.Fatal("expected err")
	}
}
