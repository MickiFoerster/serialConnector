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
	from       State
	to         State
	conditions []Condition
}
