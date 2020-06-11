package main

import (
	"log"
	"net"
	"strings"
)

type reaction struct {
	req      string
	res      string
	reaction func(req, res string, c net.Conn)
}

// * "Passwort:" handler1

var reactions = []reaction{}

var commands = []string{
	"hostname",
	"id",
	"sudo apt update",
	"sudo apt upgrade -y",
}

func fill_reactions() {
	reactions = []reaction{
		/*
			reaction{
				req: "*",
				res: "Last login: ",
				reaction: func(req, res string, c net.Conn) {
					err := write_uds_message(c, udsmsg_host2serial, "date\n")
					if err != nil {
						log.Printf("error while sending command 'date' to client: %v\n", err)
						close_uds_channel(c)
					}
				},
			},*/
		reaction{
			req: "*",
			res: " login: ",
			reaction: func(req, res string, c net.Conn) {
				u := username
				if u[len(u)-1] != '\n' {
					u += "\n"
				}
				err := write_uds_message(c, udsmsg_host2serial, u)
				if err != nil {
					log.Printf("error while sending command username to client: %v\n", err)
					close_uds_channel(c)
				}
			},
		},
		reaction{
			req: username,
			res: "Password: ",
			reaction: func(req, res string, c net.Conn) {
				p := password
				if p[len(p)-1] != '\n' {
					p += "\n"
				}
				err := write_uds_message(c, udsmsg_host2serial, p)
				if err != nil {
					log.Printf("error while sending command password to client: %v\n", err)
					close_uds_channel(c)
				}
			},
		},
		reaction{
			req: "*",
			res: "Password: ",
			reaction: func(req, res string, c net.Conn) {
				err := write_uds_message(c, udsmsg_host2serial, "exit\n")
				if err != nil {
					log.Printf("error while sending command 'exit' to client: %v\n", err)
					close_uds_channel(c)
				}
			},
		},
		reaction{
			req: password,
			res: "Login incorrect",
			reaction: func(req, res string, c net.Conn) {
				log.Println("error: Cannot login with provided username/password")
				err := write_uds_message(c, udsmsg_control, "")
				if err != nil {
					log.Printf("error while sending command to client: %v\n", err)
					close_uds_channel(c)
				}
			},
		},
		reaction{
			req: password,
			res: "~$ ",
			reaction: func(req, res string, c net.Conn) {
				if len(commands) > 0 {
					cmd := commands[0]
					if cmd[len(cmd)-1] != '\n' {
						cmd += "\n"
					}
					err := write_uds_message(c, udsmsg_host2serial, cmd)
					if err != nil {
						log.Printf("error while sending command %q to client: %v\n", cmd, err)
						close_uds_channel(c)
					}
				} else {
					cmd := "exit\n"
					err := write_uds_message(c, udsmsg_host2serial, cmd)
					if err != nil {
						log.Printf("error while sending command %q to client: %v\n", cmd, err)
						close_uds_channel(c)
					}
					close_uds_channel(c)
				}
			},
		},
		reaction{
			req: "~$ " + commands[0],
			res: "~$ ",
			reaction: func(req, res string, c net.Conn) {
				commands = commands[1:]
				if len(commands) > 0 {
					cmd := commands[0]
					commands = commands[1:]
					if cmd[len(cmd)-1] != '\n' {
						cmd += "\n"
					}
					err := write_uds_message(c, udsmsg_host2serial, cmd)
					if err != nil {
						log.Printf("error while sending command %q to client: %v\n", cmd, err)
						close_uds_channel(c)
					}
				} else {
					cmd := "exit\n"
					err := write_uds_message(c, udsmsg_host2serial, cmd)
					if err != nil {
						log.Printf("error while sending command %q to client: %v\n", cmd, err)
						close_uds_channel(c)
					}
					close_uds_channel(c)
				}
			},
		},
	}
}

func interpreter(c net.Conn) {
	for i, r := range reactions {
		req := r.req
		if len(req) == 0 {
			log.Fatalf("request may not be empty: reaction[%d]\n", i)
		}
		if req != "*" &&
			strings.Index(string(request), req) == -1 {
			log.Printf("request %v not found in %v\n", req, string(request))
			continue
		}

		if strings.Index(string(received), r.res) == -1 {
			log.Printf("%v not found in %v\n", r.res, string(received))
			continue
		}

		log.Printf("Reaction %d machtes: Found %q after sending %q, so calling handler.\n", i+1, r.res, req)
		request = []byte{}
		received = []byte{}
		r.reaction(req, r.res, c)
		break
	}
}
