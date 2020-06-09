package main

import "time"

type State struct {
	name         string
	entranceTime time.Time
	sent         string
	received     string
}

type Condition []func() bool

type transition struct {
	from       *State
	to         *State
	conditions []Condition
}

var (
	start        State {
        name: "undefined",
    }
	currentstate *State = &start
	loggedoff           = State{
		name: "loggedoff",
	}
	loggedin           = State{
		name: "loggedin",
	}

    t1 = transition {
        from: &start,
        to: &loggedoff,
        conditions: []Condition {
            Condition{},
        },
    }
)
