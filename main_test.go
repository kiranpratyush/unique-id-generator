package main

import (
	"fmt"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	tests := []struct {
		name  string
		input snowflakeIdInput
	}{
		{"all zeros", snowflakeIdInput{0, 0, 0}},
		{"all max", snowflakeIdInput{MaxTimeStampNumber, MaxMachineIdNumber, MaxSequenceIdNumber}},
		{"max timestamp only", snowflakeIdInput{MaxTimeStampNumber, 0, 0}},
		{"max machine only", snowflakeIdInput{0, MaxMachineIdNumber, 0}},
		{"max sequence only", snowflakeIdInput{0, 0, MaxSequenceIdNumber}},
		{"sequence above 10 bits", snowflakeIdInput{12345, 7, 2050}},
		{"mixed", snowflakeIdInput{820_000_000_000, 513, 1}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			id, err := generateSnowflakeId(tc.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			got := decodeSnowflakeId(id)
			if got != tc.input {
				t.Errorf("round trip mismatch: got %+v, want %+v (id=%064b)", got, tc.input, id)
			}
		})
	}
}

func TestKnownLayout(t *testing.T) {
	// Each field = 1 must land at its exact bit position.
	id, err := generateSnowflakeId(snowflakeIdInput{1, 1, 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := uint64(1)<<22 | uint64(1)<<12 | 1
	if id != want {
		t.Errorf("got %064b, want %064b", id, want)
	}
}

func TestSignBitAlwaysZero(t *testing.T) {
	id, err := generateSnowflakeId(snowflakeIdInput{MaxTimeStampNumber, MaxMachineIdNumber, MaxSequenceIdNumber})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id>>63 != 0 {
		t.Errorf("sign bit set: %064b", id)
	}
	if id != (1<<63)-1 {
		t.Errorf("all-max should fill the low 63 bits: got %064b", id)
	}
}

func TestInvalidInputs(t *testing.T) {
	tests := []struct {
		name  string
		input snowflakeIdInput
	}{
		{"negative timestamp", snowflakeIdInput{-1, 0, 0}},
		{"timestamp too large", snowflakeIdInput{MaxTimeStampNumber + 1, 0, 0}},
		{"machine id too large", snowflakeIdInput{0, MaxMachineIdNumber + 1, 0}},
		{"sequence too large", snowflakeIdInput{0, 0, MaxSequenceIdNumber + 1}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			id, err := generateSnowflakeId(tc.input)
			if err == nil {
				t.Errorf("expected error, got id %d", id)
			}
		})
	}
}

func TestOrderingFollowsTimestamp(t *testing.T) {
	// A later timestamp must always produce a larger ID, regardless of the lower fields.
	earlier, _ := generateSnowflakeId(snowflakeIdInput{1000, MaxMachineIdNumber, MaxSequenceIdNumber})
	later, _ := generateSnowflakeId(snowflakeIdInput{1001, 0, 0})
	if later <= earlier {
		t.Errorf("later id %d should be > earlier id %d", later, earlier)
	}
}

func Example_decode() {
	id, _ := generateSnowflakeId(snowflakeIdInput{5, 3, 9})
	fmt.Printf("%+v\n", decodeSnowflakeId(id))
	// Output: {timestamp:5 machineID:3 sequenceID:9}
}
