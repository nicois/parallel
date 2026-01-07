package dispatch

import (
	"time"
)

// Duration is a variant on time.Duration which also understands
// 'd' unit (for days) in addition to the notmal units
type Duration time.Duration

func (d *Duration) UnmarshalFlag(value string) error {
	v := value
	var days uint32
	for {
		if v[0] >= '0' && v[0] <= '9' {
			days = days*10 + uint32(v[0]-'0')
		} else if v[0] == 'd' {
			value = v[1:]
			if len(value) == 0 {
				*d = Duration(time.Hour * 24 * time.Duration(days))
				return nil
			}
			break
		} else {
			days = 0
			break
		}
		v = v[1:]
		if len(v) == 0 {
			days = 0
			break
		}
	}
	if duration, err := time.ParseDuration(value); err == nil {
		*d = Duration(duration + time.Hour*24*time.Duration(days))
		return nil
	} else {
		return err
	}
}
