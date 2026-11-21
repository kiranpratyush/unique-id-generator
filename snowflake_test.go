package main

import (
	"sync"
	"testing"
)

// scriptedClock returns the given times in order, then keeps returning the
// last one.
func scriptedClock(times ...int64) TimeStampGenerator {
	i := 0
	return func() int64 {
		t := times[i]
		if i < len(times)-1 {
			i++
		}
		return t
	}
}

// fakeClock returns start for the first `repeat` calls, then advances by 1 ms
// on every call after that.
func fakeClock(start int64, repeat int) TimeStampGenerator {
	calls := 0
	now := start

	return func() int64 {
		if calls >= repeat {
			now++
		}
		calls++
		return now
	}
}

func TestSameMillisecondIncrementsSequenceAndNewMillisecondResetsIt(t *testing.T) {
	const machineID = 1
	sidg, err := NewSnowflakeGenerator(machineID, fakeClock(100, 2))
	if err != nil {
		t.Fatalf("creating snowflake generator: %v", err)
	}

	// Clock reads 100, 100, 101.
	want := []snowflakeIdInput{
		{timestamp: 100, machineID: machineID, sequenceID: 0},
		{timestamp: 100, machineID: machineID, sequenceID: 1},
		{timestamp: 101, machineID: machineID, sequenceID: 0},
	}

	var prev uint64
	for i, w := range want {
		id, err := sidg.Generate()
		if err != nil {
			t.Fatalf("call %d: generate failed: %v", i+1, err)
		}
		if i > 0 && id <= prev {
			t.Errorf("call %d: id %d not greater than previous id %d", i+1, id, prev)
		}
		prev = id

		if got := decodeSnowflakeId(id); got != w {
			t.Errorf("call %d: got %+v, want %+v", i+1, got, w)
		}
	}
}

func TestSequenceOverflowWaitsForNextMillisecond(t *testing.T) {
	const machineID = 1
	const perMs = MaxSequenceIdNumber + 1 // 4096 IDs fit in one millisecond

	// First 4096 clock reads return 100, then the clock advances.
	sidg, err := NewSnowflakeGenerator(machineID, fakeClock(100, perMs))
	if err != nil {
		t.Fatalf("creating snowflake generator: %v", err)
	}

	var prev uint64
	for i := 0; i <= perMs; i++ {
		id, err := sidg.Generate()
		if err != nil {
			t.Fatalf("call %d: generate failed: %v", i+1, err)
		}
		if i > 0 && id <= prev {
			t.Fatalf("call %d: id %d not greater than previous id %d", i+1, id, prev)
		}
		prev = id

		want := snowflakeIdInput{timestamp: 100, machineID: machineID, sequenceID: uint64(i)}
		if i == perMs {
			// 4097th call: sequence exhausted, so it must wait for the next ms.
			want = snowflakeIdInput{timestamp: 101, machineID: machineID, sequenceID: 0}
		}
		if got := decodeSnowflakeId(id); got != want {
			t.Fatalf("call %d: got %+v, want %+v", i+1, got, want)
		}
	}
}

func TestClockMovingBackwardsReturnsError(t *testing.T) {
	const machineID = 1
	// 100 → 99 (backwards) → 100 (caught up again).
	sidg, err := NewSnowflakeGenerator(machineID, scriptedClock(100, 99, 100))
	if err != nil {
		t.Fatalf("creating snowflake generator: %v", err)
	}

	first, err := sidg.Generate()
	if err != nil {
		t.Fatalf("call 1: generate failed: %v", err)
	}

	if id, err := sidg.Generate(); err == nil {
		t.Fatalf("call 2: expected error when clock moved backwards, got id %+v", decodeSnowflakeId(id))
	}

	// Once the clock is back at the last timestamp, generation resumes and
	// must not reuse the sequence issued before the error.
	third, err := sidg.Generate()
	if err != nil {
		t.Fatalf("call 3: generate failed after clock recovered: %v", err)
	}
	want := snowflakeIdInput{timestamp: 100, machineID: machineID, sequenceID: 1}
	if got := decodeSnowflakeId(third); got != want {
		t.Errorf("call 3: got %+v, want %+v", got, want)
	}
	if third <= first {
		t.Errorf("call 3: id %d not greater than first id %d", third, first)
	}
}

func TestConcurrentGenerateProducesUniqueIDs(t *testing.T) {
	const (
		goroutines   = 50
		perGoroutine = 2000
	)
	sidg, err := NewSnowflakeGenerator(1, generateTimeStamp())
	if err != nil {
		t.Fatalf("creating snowflake generator: %v", err)
	}

	// Each goroutine writes only to its own slice, so no locking is needed
	// while generating; results are merged after all goroutines finish.
	results := make([][]uint64, goroutines)
	var wg sync.WaitGroup
	for g := range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ids := make([]uint64, 0, perGoroutine)
			for range perGoroutine {
				id, err := sidg.Generate()
				if err != nil {
					t.Errorf("goroutine %d: generate failed: %v", g, err)
					return
				}
				ids = append(ids, id)
			}
			results[g] = ids
		}()
	}
	wg.Wait()

	seen := make(map[uint64]struct{}, goroutines*perGoroutine)
	for g, ids := range results {
		for _, id := range ids {
			if _, ok := seen[id]; ok {
				t.Fatalf("duplicate id %d (%+v) from goroutine %d", id, decodeSnowflakeId(id), g)
			}
			seen[id] = struct{}{}
		}
	}
	if len(seen) != goroutines*perGoroutine {
		t.Errorf("got %d unique ids, want %d", len(seen), goroutines*perGoroutine)
	}
}
