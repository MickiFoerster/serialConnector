package main

import (
    "net"
	"strings"
	"log"
)

type reaction struct {
    req string
    res string
    reaction func(req, res string, c net.Conn)
}
// * "Passwort:" handler1

var reactions = []reaction {
    reaction{
        req : "*",
        res : " login: ",
        reaction: func (req, res string, c net.Conn) {
            write_uds_message(c, username)
        },
    },
    reaction{
        req : "*",
        res : "\nPassword: ",
        reaction: func (req, res string, c net.Conn) {
            write_uds_message(c, password)
        },
    },
    reaction{
        req : "*",
        res : ":~$ ",
        reaction: func (req, res string, c net.Conn) {
            write_uds_message(c, "echo hello ; sleep 5\n")
        },
    },
}


func interpreter(c net.Conn) {
    for _, r := range(reactions) {
        if r.req != "*" &&
           strings.Index(string(last_msg.payload), r.req) == -1 {
            continue
        }

        if strings.Index(string(received), r.res) == -1 {
            continue
        }

        log.Printf("Found %q after sending %q, so calling handler.\n", r.res, r.req)
        r.reaction(r.req, r.res, c)
        received = []byte{}
    }
}

