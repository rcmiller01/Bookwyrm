package launcher

import "time"

type RestartPolicy struct {
	Limit     int
	Window    time.Duration
	BaseDelay time.Duration
	MaxDelay  time.Duration
	restarts  []time.Time
}

func (p *RestartPolicy) AllowRestart(now time.Time) (bool, time.Duration) {
	if p.Limit <= 0 {
		p.Limit = 1
	}
	if p.Window <= 0 {
		p.Window = 5 * time.Minute
	}
	if p.BaseDelay <= 0 {
		p.BaseDelay = 2 * time.Second
	}
	if p.MaxDelay <= 0 {
		p.MaxDelay = 30 * time.Second
	}

	cutoff := now.Add(-p.Window)
	filtered := make([]time.Time, 0, len(p.restarts))
	for _, ts := range p.restarts {
		if ts.After(cutoff) {
			filtered = append(filtered, ts)
		}
	}
	p.restarts = filtered
	if len(p.restarts) >= p.Limit {
		return false, 0
	}
	p.restarts = append(p.restarts, now)
	attempt := len(p.restarts)
	delay := p.BaseDelay * time.Duration(1<<(attempt-1))
	if delay > p.MaxDelay {
		delay = p.MaxDelay
	}
	return true, delay
}
