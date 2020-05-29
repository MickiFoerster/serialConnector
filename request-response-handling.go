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

var reactions = []reaction{
	reaction{
		req: "*",
		res: " login: ",
		reaction: func(req, res string, c net.Conn) {
			u := username
			if u[len(u)-1] != '\n' {
				u += "\n"
			}
			write_uds_message(c, u)
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
			write_uds_message(c, p)
		},
	},
	reaction{
		req: "*",
		res: "Password: ",
		reaction: func(req, res string, c net.Conn) {
			write_uds_message(c, "exit\n")
		},
	},
	reaction{
		req: password,
		res: "Login incorrect",
		reaction: func(req, res string, c net.Conn) {
			log.Println("error: Cannot login with provided username/password")
			c.Close()
		},
	},
	reaction{
		req: password,
		res: ":~$ ",
		reaction: func(req, res string, c net.Conn) {
			write_uds_message(c, "echo hello ; sleep 5\n")
		},
	},
}

func interpreter(c net.Conn) {
	for i, r := range reactions {
		if len(r.req) == 0 {
			continue
		}
		if r.req != "*" &&
			strings.Index(string(last_msg.payload), r.req) == -1 {
			continue
		}

		if strings.Index(string(received), r.res) == -1 {
			continue
		}

		log.Printf("Reaction %d machtes: Found %q after sending %q, so calling handler.\n", i+1, r.res, r.req)
		r.reaction(r.req, r.res, c)
		received = []byte{}
        break;
	}
}
