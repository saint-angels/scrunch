package main

import (
	"sync"
	"time"
)

type Standing struct {
	User      string `json:"user"`
	StartedAt int64  `json:"startedAt"`
	EndsAt    int64  `json:"endsAt"`
}

type Room struct {
	mu        sync.Mutex
	standings map[string]Standing
	timers    map[string]*time.Timer
}

func NewRoom() *Room {
	return &Room{
		standings: make(map[string]Standing),
		timers:    make(map[string]*time.Timer),
	}
}

func (r *Room) Stand(user string, duration int, onTimeUp func(string)) Standing {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.sitLocked(user)

	now := time.Now().UnixMilli()
	endsAt := now + int64(duration)*1000
	s := Standing{User: user, StartedAt: now, EndsAt: endsAt}
	r.standings[user] = s
	r.timers[user] = time.AfterFunc(time.Duration(duration)*time.Second, func() {
		r.mu.Lock()
		delete(r.timers, user)
		r.mu.Unlock()
		onTimeUp(user)
	})

	return s
}

func (r *Room) Sit(user string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sitLocked(user)
}

func (r *Room) sitLocked(user string) {
	if t, ok := r.timers[user]; ok {
		t.Stop()
		delete(r.timers, user)
	}
	delete(r.standings, user)
}

func (r *Room) IsStanding(user string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, ok := r.standings[user]
	return ok
}

func (r *Room) GetStandings() []Standing {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Standing, 0, len(r.standings))
	for _, s := range r.standings {
		out = append(out, s)
	}
	return out
}
