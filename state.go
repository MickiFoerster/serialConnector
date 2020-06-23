package main

import (
	"fmt"
	"log"
	"regexp"
	"strings"
	"sync"
	"time"
)

type State struct {
	name         string
	entranceTime time.Time
	sent         []byte
	sentMtx      sync.Mutex
	received     []byte
	receivedMtx  sync.Mutex
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
	done = make(chan struct{})

	commands = []string{
		"cat <<EOF > /tmp/asdf",
		`\
#!/bin/bash
hostname && echo OK
`,
		"EOF",
		"chmod 755 /tmp/asdf;",
		"/tmp/asdf;",
	}

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
		},
		exitHook: func() {
			fmt.Println("exitHook of 'loggedin' called")
		},
	}
	prompt = State{
		name: "prompt",
		enterHook: func() {
			fmt.Println("enterHook of 'prompt' called")
		},
		exitHook: func() {
			fmt.Println("exitHook of 'prompt' called")
		},
	}
	busy = State{
		name: "busy",
		enterHook: func() {
			fmt.Println("enterHook of 'busy' called")
		},
		exitHook: func() {
			fmt.Println("exitHook of 'busy' called")
		},
	}
	exit = State{
		name: "exit",
		enterHook: func() {
			fmt.Println("enterHook of 'exit' called")
			done <- struct{}{}
		},
		exitHook: func() {
			fmt.Println("exitHook of 'exit' called")
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
			from: &loggedoff, to: &loggedoff,
			conditions: []condition{
				func(from *State, to *State) bool {
					switch {
					case strings.Index(string(from.received), "\nPassword:") != -1 &&
						strings.Index(string(from.sent), username) == -1:
						cmd := "\n"
						writerinput <- udsMessage{
							typ:     udsmsg_host2serial,
							len:     uint32(len(cmd)),
							payload: []byte(cmd),
						}
						return true
					case strings.Index(string(from.received), "\nLogin timed out") != -1:
						fallthrough
					case strings.Index(string(from.received), "\nLogin incorrect") != -1:
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
					case strings.Index(string(from.received), "\nLast login: ") != -1 &&
						strings.Index(string(from.sent), password) != -1:
						return true
					}

					return false
				},
			},
		},
		transition{
			from: &loggedin, to: &prompt,
			conditions: []condition{
				func(from *State, to *State) bool {
					userprompt := regexp.MustCompile(username + `@.*:~\$ `)
					rootprompt := regexp.MustCompile(`root@.*:~\# `)
					switch {
					case rootprompt.Match(currentstate.received):
						return true
					case userprompt.Match(currentstate.received):
						return true
					default:
						return false
					}
				},
			},
		},
		transition{
			from: &prompt, to: &exit,
			conditions: []condition{
				func(from *State, to *State) bool {
					if len(commands) == 0 {
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
			from: &prompt, to: &busy,
			conditions: []condition{
				func(from *State, to *State) bool {
					if len(commands) > 0 {
						cmd := commands[0]
						if cmd[len(cmd)-1] != '\n' {
							cmd += "\n"
						}
						commands = commands[1:]
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
			from: &busy, to: &prompt,
			conditions: []condition{
				func(from *State, to *State) bool {
					userprompt := regexp.MustCompile(username + `@.*:~\$ `)
					rootprompt := regexp.MustCompile(`root@.*:~\# `)
					switch {
					case rootprompt.Match(currentstate.received):
						return true
					case userprompt.Match(currentstate.received):
						return true
					default:
						return false
					}
				},
			},
		},
	}
)

func statemachine() chan struct{} {
	start.entranceTime = time.Now()

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
					if currentstate.exitHook != nil {
						currentstate.exitHook()
					}
					//sent := currentstate.sent
					//received := currentstate.received
					currentstate = newState
					currentstate.sent = []byte{}
					currentstate.received = []byte{}
					currentstate.entranceTime = time.Now()
					if currentstate.enterHook != nil {
						currentstate.enterHook()
					}
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
			condition_true := true
			for _, cond := range t.conditions {
				condition_true = condition_true && cond(t.from, t.to)
				if !condition_true {
					break
				}
			}
			if condition_true {
				return true, t.to
			} else {
				log.Printf("Transition from %v to %v is not valid\n",
					t.from.name, t.to.name)
				fmt.Printf("DEBUG:sent: %s\n", t.from.sent)
				fmt.Printf("DEBUG:received: %s\n", t.from.received)
			}
		}
	}
	return false, nil
}

func updateCurrentStateRecv(msg udsMessage) {
	currentstate.receivedMtx.Lock()
	defer currentstate.receivedMtx.Unlock()
	currentstate.received = append(currentstate.received, msg.payload...)
}

func updateCurrentStateSent(msg udsMessage) {
	currentstate.sentMtx.Lock()
	defer currentstate.sentMtx.Unlock()
	currentstate.sent = append(currentstate.sent, msg.payload...)
}
