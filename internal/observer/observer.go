package observer

import (
	"time"
)

type Action string

const (
	ActionShorten Action = "shorten"
	ActionFollow  Action = "follow"
)

type Event struct {
	Timestamp int64  `json:"ts"`
	Action    Action `json:"action"`
	UserID    string `json:"user_id"`
	URL       string `json:"url"`
}

func NewEvent(action Action, userID, url string) Event {
	return Event{
		Timestamp: time.Now().Unix(),
		Action:    action,
		UserID:    userID,
		URL:       url,
	}
}

type Observer interface {
	Handle(event Event) error
}

type Subject struct {
	observers []Observer
}

func NewSubject() *Subject {
	return &Subject{}
}

func (s *Subject) Subscribe(o Observer) {
	s.observers = append(s.observers, o)
}

func (s *Subject) Notify(event Event) []error {
	var errs []error
	for _, o := range s.observers {
		if err := o.Handle(event); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}
