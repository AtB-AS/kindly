package kindly

import (
	"time"
)

// Time is a convenience type to work with times in the Kindly API
type Time struct {
	time.Time
}

// UnmarshalJSON implements json.Unmarshaler
func (t *Time) UnmarshalJSON(data []byte) error {
	const layout = "2006-01-02T15:04:05.000000"
	tm, err := time.Parse(layout, string(data[1:len(data)-1]))
	if err != nil {
		return err
	}

	t.Time = tm
	return nil
}
