package rotator

import "time"

func getLogTimePassed(timePassed time.Duration) string {
	if time.Duration(time.Microsecond*100) >= timePassed {
		return "100us"
	} else if time.Duration(time.Microsecond*250) >= timePassed {
		return "250us"
	} else if time.Duration(time.Microsecond*500) >= timePassed {
		return "500us"
	} else if time.Duration(time.Microsecond*1000) >= timePassed {
		return "1000us"
	} else if time.Duration(time.Millisecond*5) >= timePassed {
		return "5ms"
	} else {
		return ">5ms"
	}
}
