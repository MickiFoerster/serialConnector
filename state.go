package main

import (
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"
)

type State struct {
	name         string
	entranceTime time.Time
	sent         []byte
	received     []byte
	enterHook    func()
	exitHook     func()
}

type condition func(from *State, to *State) bool

type transition struct {
	from       *State
	to         *State
	conditions []condition
}

var (
	start = State{
		name: "undefined",
		enterHook: func() {
			fmt.Println("enterHook of 'undefined' called")
		},
		exitHook: func() {
			fmt.Println("exitHook of 'undefined' called")
		},
	}
	loggedoff = State{
		name: "loggedoff",
		enterHook: func() {
			fmt.Println("enterHook of 'loggedoff' called")
		},
		exitHook: func() {
			fmt.Println("exitHook of 'loggedoff' called")
		},
	}
	loggedin = State{
		name: "loggedin",
		enterHook: func() {
			fmt.Println("enterHook of 'loggedin' called")
			cmd := `hostname
            id
            sudo apt update 
            sudo apt upgrade -y 
            exit
            `
			writerinput <- udsMessage{
				typ:     udsmsg_host2serial,
				len:     uint32(len(cmd)),
				payload: []byte(cmd),
			}
		},
		exitHook: func() {
			fmt.Println("exitHook of 'loggedin' called")
		},
	}
	currentstate *State = &start

	transitions = []transition{
		transition{
			from: &start, to: &loggedoff,
			conditions: []condition{
				func(from *State, to *State) bool {
					switch {
					case strings.Index(string(from.received), " login: ") != -1:
						return true
					case strings.Index(string(from.received), "\nLogin incorrect ") != -1:
						return true
					case strings.Index(string(from.received), "\nPassword:") != -1:
						from.received = []byte{}
						cmd := "exit\n"
						writerinput <- udsMessage{
							typ:     udsmsg_host2serial,
							len:     uint32(len(cmd)),
							payload: []byte(cmd),
						}
						return true
					}
					return false
				},
			},
		},
		transition{
			from: &loggedoff, to: &loggedin,
			conditions: []condition{
				func(from *State, to *State) bool {
					re := regexp.MustCompile(username + `@.*:~\$`)
					switch {
					case strings.Index(string(from.received), "\nPassword:") != -1:
						from.received = []byte{}
						cmd := password + "\n"
						writerinput <- udsMessage{
							typ:     udsmsg_host2serial,
							len:     uint32(len(cmd)),
							payload: []byte(cmd),
						}
					case strings.Index(string(from.received), " login: ") != -1 &&
						strings.Index(string(from.sent), password) == -1:
						from.received = []byte{}
						cmd := username + "\n"
						writerinput <- udsMessage{
							typ:     udsmsg_host2serial,
							len:     uint32(len(cmd)),
							payload: []byte(cmd),
						}
					case re.Match(currentstate.received):
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
			time.Sleep(1 * time.Second)
			// go through all transitions and test if current node fits and when so if one condition is true
			for _, t := range transitions {
				// if current node is the origin node in the transition we are looking at in this iteration
				if t.from == currentstate {
					// check that all conditions are true otherwise transition must not be done
					condition_true := true
					for _, cond := range t.conditions {
						condition_true = condition_true && cond(t.from, t.to)
						if !condition_true {
							break
						}
					}
					if condition_true {
						if currentstate.exitHook != nil {
							currentstate.exitHook()
						}
						sent := currentstate.sent
						received := currentstate.received
						currentstate = t.to
						currentstate.sent = sent
						currentstate.received = received
						if currentstate.enterHook != nil {
							currentstate.enterHook()
						}
						log.Println("Found new next state: ", currentstate.name)
					}
				}
			}
		}
	}()
}

func updateCurrentStateRecv(msg udsMessage) {
	//log.Println("updateCurrentStateRecv called")
	currentstate.received = append(currentstate.received, msg.payload...)

}

func updateCurrentStateSent(msg udsMessage) {
	//log.Println("updateCurrentStateSent called")
	currentstate.sent = append(currentstate.sent, msg.payload...)
}
