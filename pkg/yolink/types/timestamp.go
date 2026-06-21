package types

import (
	"encoding/json"
	"time"
)

type Timestamp struct {
	time.Time
}

func (ts Timestamp) MarshalJSON() ([]byte, error) {
	return json.Marshal(ts.UnixMilli()) // Marshal as epoch timestamp
}

func (ts *Timestamp) UnmarshalJSON(data []byte) error {
	var timestamp int64
	if err := json.Unmarshal(data, &timestamp); err != nil {
		return err
	}
	ts.Time = time.UnixMilli(timestamp)
	return nil
}
