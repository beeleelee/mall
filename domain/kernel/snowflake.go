package kernel

import (
	"sync"
	"time"
)

const (
	snowflakeEpoch     = 1700000000000
	workerBits         = 10
	sequenceBits       = 12
	workerMax          = -1 ^ (-1 << workerBits)
	sequenceMask       = -1 ^ (-1 << sequenceBits)
	workerShift        = sequenceBits
	timestampLeftShift = sequenceBits + workerBits
)

type Snowflake struct {
	mu        sync.Mutex
	workerID  int64
	sequence  int64
	lastStamp int64
}

func NewSnowflake(workerID int64) (*Snowflake, error) {
	if workerID < 0 || workerID > workerMax {
		return nil, NewDomainError(ErrInvalidArgument, "worker ID out of range")
	}
	return &Snowflake{workerID: workerID}, nil
}

func (s *Snowflake) NextID() (ID, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UnixMilli()
	if now < s.lastStamp {
		return 0, NewDomainError(ErrInternal, "clock moved backwards")
	}

	if now == s.lastStamp {
		s.sequence = (s.sequence + 1) & sequenceMask
		if s.sequence == 0 {
			for now <= s.lastStamp {
				now = time.Now().UnixMilli()
			}
		}
	} else {
		s.sequence = 0
	}

	s.lastStamp = now
	id := (now-snowflakeEpoch)<<timestampLeftShift | s.workerID<<workerShift | s.sequence
	return ID(id), nil
}
