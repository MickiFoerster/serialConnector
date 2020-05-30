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

func fill_reactions() {
    reactions = []reaction{ 
        reaction{
            req: "*",
            res: "Last login: ",
            reaction: func(req, res string, c net.Conn) {
                write_uds_message(c, udsmsg_host2serial, "date\n")
            },
        },
        reaction{
            req: "*",
            res: " login: ",
            reaction: func(req, res string, c net.Conn) {
                u := username
                if u[len(u)-1] != '\n' {
                    u += "\n"
                }
                write_uds_message(c, udsmsg_host2serial, u)
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
                write_uds_message(c, udsmsg_host2serial, p)
            },
        },
        reaction{
            req: "*",
            res: "Password: ",
            reaction: func(req, res string, c net.Conn) {
                write_uds_message(c, udsmsg_host2serial, "exit\n")
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
            req: "*",
            res: ":~$ ",
            reaction: func(req, res string, c net.Conn) {
                write_uds_message(c, udsmsg_host2serial, "echo hello ; sleep 5\n")
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
			continue
		}

		if strings.Index(string(received), r.res) == -1 {
			continue
		}

		log.Printf("Reaction %d machtes: Found %q after sending %q, so calling handler.\n", i+1, r.res, req)
		request = []byte{}
		received = []byte{}
		r.reaction(req, r.res, c)
        break;
	}
}
