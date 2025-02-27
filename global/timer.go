package global

import (
	"time"
)

type SipTimer struct {
	DoneCh chan any
	Tmr    *time.Timer
}
