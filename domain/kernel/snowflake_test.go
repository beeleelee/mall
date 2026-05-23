package kernel

import "testing"

func TestSnowflake_NextID(t *testing.T) {
	sf, err := NewSnowflake(1)
	if err != nil {
		t.Fatalf("NewSnowflake failed: %v", err)
	}

	ids := make(map[ID]bool)
	for i := 0; i < 1000; i++ {
		id, err := sf.NextID()
		if err != nil {
			t.Fatalf("NextID failed: %v", err)
		}
		if ids[id] {
			t.Fatalf("duplicate ID: %d", id)
		}
		ids[id] = true
	}
}

func TestSnowflake_WorkerIDOutOfRange(t *testing.T) {
	_, err := NewSnowflake(-1)
	if err == nil {
		t.Fatal("expected error for negative worker ID")
	}
	_, err = NewSnowflake(1024)
	if err == nil {
		t.Fatal("expected error for worker ID > 1023")
	}
}
