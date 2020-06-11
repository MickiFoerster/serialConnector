package main

import (
	"log"
	"os"
	"time"
)

type State struct {
	name         string
	entranceTime time.Time
	sent         []byte
	received     []byte
	enterHook    func(interface{})
	exitHook     func(interface{})
}

type condition func(from *State, to *State) bool

type transition struct {
	from       *State
	to         *State
	conditions []condition
}

var (
	start               = State{name: "undefined"}
	loggedoff           = State{name: "loggedoff"}
	loggedin            = State{name: "loggedin"}
	currentstate *State = &start

	transitions = []transition{
		transition{
			from: &start, to: &loggedoff,
			conditions: []condition{
				func(from *State, to *State) bool {
					if from.name == "undefined" {
						return true
					}
					return false
				},
			},
		},
	}
)

func start_statemachine() {
	start.entranceTime = time.Now()

	go func() {
		for {
			// go through all transitions and test if current node fits and when so if one condition is true
			for _, t := range transitions {
				// if current node is the origin node in the transition we are looking at in this iteration
				if t.from == currentstate {
					// check that all conditions are true otherwise transition must not be done
					c := true
					for _, cond := range t.conditions {
						c = c && cond(t.from, t.to)
						if !c {
							break
						}
					}
					if c {
						currentstate = t.to
						log.Println("Found new next state: ", currentstate.name)
						os.Exit(1)
					}
				}
			}
		}
	}()
}

func updateCurrentStateRecv(msg udsMessage) {
}

func updateCurrentStateSent(msg udsMessage) {
}
