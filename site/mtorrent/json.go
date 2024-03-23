package mtorrent

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

type Time struct {
	time.Time
}

func (ct *Time) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf(`"%s"`, ct.Format(time.DateTime))), nil
}

func (ct *Time) UnmarshalJSON(data []byte) error {
	parsedTime, err := time.Parse(time.DateTime, string(data[1:len(data)-1]))
	if err != nil {
		return err
	}
	ct.Time = parsedTime
	return nil
}

func (ct *Time) UnixWithDefault(defaultValue int64) int64 {
	if ct == nil {
		return defaultValue
	}

	return ct.Unix()
}

type Int64 int64

func (ci *Int64) UnmarshalJSON(data []byte) error {
	var raw json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	var num int64
	if err := json.Unmarshal(raw, &num); err == nil {
		*ci = Int64(num)
		return nil
	}

	var str string
	if err := json.Unmarshal(raw, &str); err != nil {
		return err
	}

	num, err := strconv.ParseInt(str, 10, 64)
	if err != nil {
		return err
	}

	*ci = Int64(num)
	return nil
}

func (ci *Int64) MarshalJSON() ([]byte, error) {
	return json.Marshal(strconv.FormatInt(int64(*ci), 10))
}

func (ci *Int64) Value() int64 {
	return int64(*ci)
}
