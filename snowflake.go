package main

import (
	"errors"
	"sync"
	"time"
)

const (
	TimeStampBitWidth   = 41
	MachineIdBitWidth   = 10
	SequenceIdBitWidth  = 12
	MaxTimeStampNumber  = (1 << TimeStampBitWidth) - 1
	MaxMachineIdNumber  = (1 << MachineIdBitWidth) - 1
	MaxSequenceIdNumber = (1 << SequenceIdBitWidth) - 1
)

type TimeStampGenerator func() int64

type snowflakeIdInput struct {
	timestamp  int64
	machineID  uint64
	sequenceID uint64
}

type snowflakeIdGenerator struct {
	lasttimestamp int64
	machineID     uint64
	sequenceID    uint64
	clock         TimeStampGenerator
	mu            sync.Mutex
}

func generateTimeStamp() TimeStampGenerator {
	epoch := time.Date(2000, 11, 23, 0, 0, 0, 0, time.UTC)
	return func() int64 {
		return time.Since(epoch).Milliseconds()
	}
}

func NewSnowflakeGenerator(machineID uint64, clock TimeStampGenerator) (*snowflakeIdGenerator, error) {
	if machineID > MaxMachineIdNumber {
		return nil, errors.New("machine id out of range")
	}
	return &snowflakeIdGenerator{
		lasttimestamp: -1,
		machineID:     machineID,
		sequenceID:    0,
		clock:         clock,
	}, nil
}

func generateSnowflakeId(input snowflakeIdInput) (uint64, error) {
	if input.timestamp < 0 || input.timestamp > MaxTimeStampNumber {
		return 0, errors.New("timestamp out of range")
	}
	unsignedTimeStamp := uint64(input.timestamp)
	if input.machineID > MaxMachineIdNumber {
		return 0, errors.New("machine id out of range")
	}
	if input.sequenceID > MaxSequenceIdNumber {
		return 0, errors.New("sequence out of range")
	}
	var snowflakeId uint64 = (unsignedTimeStamp << (MachineIdBitWidth + SequenceIdBitWidth)) |
		(input.machineID << SequenceIdBitWidth) | input.sequenceID
	return snowflakeId, nil
}
func decodeSnowflakeId(ID uint64) snowflakeIdInput {
	sequenceID := ID & MaxSequenceIdNumber
	ID = ID >> SequenceIdBitWidth
	machineID := ID & MaxMachineIdNumber
	ID = ID >> MachineIdBitWidth
	timestamp := ID & MaxTimeStampNumber

	return snowflakeIdInput{
		timestamp:  int64(timestamp),
		machineID:  machineID,
		sequenceID: sequenceID}
}

func (snf *snowflakeIdGenerator) Generate() (uint64, error) {
	snf.mu.Lock()
	defer snf.mu.Unlock()
	for {
		timestamp := snf.clock()
		if timestamp < snf.lasttimestamp {
			return 0, errors.New("clock moved backwards")
		}
		if timestamp == snf.lasttimestamp {
			if snf.sequenceID == MaxSequenceIdNumber {
				continue
			}
			snf.sequenceID += 1
			snowflakeIdParts := snowflakeIdInput{
				machineID:  snf.machineID,
				sequenceID: snf.sequenceID,
				timestamp:  timestamp,
			}
			return generateSnowflakeId(snowflakeIdParts)

		}
		snf.lasttimestamp = timestamp
		snf.sequenceID = 0

		snowflakeIdParts := snowflakeIdInput{
			timestamp:  timestamp,
			machineID:  snf.machineID,
			sequenceID: snf.sequenceID,
		}

		return generateSnowflakeId(snowflakeIdParts)
	}

}
