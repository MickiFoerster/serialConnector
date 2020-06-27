package main

import (
	"log"
	"time"
)

type State struct {
	name         string
	entranceTime time.Time
}

type condition func(from *State, to *State) bool

type transition struct {
	from *State
	to   *State
	cond condition
}

var (
	done = make(chan struct{})

	state_undefined        = State{name: "undefined"}
	state_loggedoff        = State{name: "loggedoff"}
	state_usernamesent     = State{name: "usernamesent"}
	state_awaitingPassword = State{name: "awaitingPassword"}
	state_passwordsent     = State{name: "passwordsent"}
	state_loggedin         = State{name: "loggedin"}
	state_loginfailed      = State{name: "loginfailed"}
	state_error            = State{name: "error"}

	currentstate *State = &state_undefined

	transitions = []transition{
		transition{
			from: &state_undefined, to: &state_loggedoff,
		},
	}
)

func statemachine() chan struct{} {
	currentstate.entranceTime = time.Now()

	go func() {
	loop:
		for {
			select {
			case <-done:
				log.Println("statemachine received signal to terminate execution")
				break loop
			case <-time.After(time.Second):
				valid, newState := checkTransitions()
				if valid {
					currentstate = newState
					currentstate.entranceTime = time.Now()
					log.Println("Found new next state: ", currentstate.name)
				}
			}
		}
		done <- struct{}{}
	}()

	return done
}

// checkTransitions() go through all transitions and test if current node fits
// and returns new state if transision is valid
func checkTransitions() (bool, *State) {
	for _, t := range transitions {
		// if current node is the origin node in the transition we are looking at in this iteration
		if t.from == currentstate {
			// check that all conditions are true otherwise transition must not be done
			if t.cond(t.from, t.to) {
				return true, t.to
			} else {
				log.Printf("Transition from %v to %v is not valid\n",
					t.from.name, t.to.name)
			}
		}
	}
	return false, nil
}

func updateCurrentStateRecv(msg udsMessage) {}

func updateCurrentStateSent(msg udsMessage) {}
