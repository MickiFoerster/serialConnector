package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
)

type udsMessage struct {
	typ     uint8
	len     uint32
	payload []byte
}

const (
	uds_file_path = "/tmp/ASDF"
)

const (
	undefined = iota
	udsmsg_serial2host
	udsmsg_host2serial
	udsmsg_info
	udsmsg_control
)

var (
	username string
	password string

	empty_uds_message = udsMessage{
		typ: undefined,
		len: 0,
	}
	request = []byte{}
	received = []byte{}

    terminate_signal = make(chan struct{})
)

func init() {
	os.Remove(uds_file_path)
}

func handleClient(c net.Conn) chan struct{} {
    ch := reader(c)

	// initial starting communication by sending a request to see how target
	// will response
	write_uds_message(c, udsmsg_host2serial, "exit\n")

    return ch
}

func main() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)
	go func() {
		<-ch
		terminate_signal <- struct{}{}
	}()

	flag.StringVar(&username, "username", "pi\n", "username for login")
	flag.StringVar(&password, "password", "raspberry\n", "password for login")
	flag.Parse()
    log.Printf("username: %v\n", username)
    log.Printf("password: %v\n", password)
    fill_reactions()

    serverdone := server()

    // wait for SIGINT
    <-terminate_signal

    // stop server
    log.Println("send server to stop ...")
    serverdone<-struct{}{}
    log.Println("wait for server to stop ...")
    <-serverdone
    log.Println("terminating main")
}


func server() chan struct{} {
    serverdone := make(chan struct{})
    listener := make(chan net.Conn)

    go func() { // listener routine
        l, err := net.Listen("unix", uds_file_path)
        if err != nil {
            log.Fatal("listen failed:", err)
        }
        log.Println("Listener created successfully")
        defer os.Remove(uds_file_path)

        for {
            c, err := l.Accept()
            if err != nil {
                log.Fatal("accept() failed:", err)
            }
            listener<-c
        }
    }()

    go func() { // event loop
        clients := []chan struct{}{}
        serverloop:
        for {
            select {
            case <-serverdone:
                log.Println("terminate signal received, so exit server")
                break serverloop
            case client:=<-listener:
                chan2client := handleClient(client)
                clients = append(clients, chan2client)
            }
        }
        log.Println("signal clients to terminate")
        for i, chan2client := range(clients) {
            log.Printf("signal client %v to terminate", i)
            chan2client<-struct{}{}
            log.Println("wait for client to complete shutdown")
            <-chan2client
        }
        log.Println("server signals back that shutdown is complete")
        serverdone<-struct{}{}
    }()

    return serverdone
}

func read_uds_message_error(err error, c net.Conn) udsMessage {
	if err != io.EOF {
		log.Fatalf("error while reading from client: %s\n", err)
	}
	return empty_uds_message
}

func read_uds_message(c net.Conn) udsMessage {
	var msg udsMessage
	buf := make([]byte, 1)
	n, err := c.Read(buf)
	if err != nil {
		return read_uds_message_error(err, c)
	}
	if n != 1 {
		log.Fatal("not expected number of bytes read: wanted %d, read %d", len(buf), n)
	}
	reader := bytes.NewReader(buf)
	err = binary.Read(reader, binary.LittleEndian, &msg.typ)
	if err != nil {
		log.Fatal("error while writing message type: %v\n", err)
	}

	buf = make([]byte, 4)
	n, err = c.Read(buf)
	if err != nil {
		return read_uds_message_error(err, c)
	}
	if n != len(buf) {
		log.Fatal("not expected number of bytes read: wanted %d, read %d", len(buf), n)
	}
	reader = bytes.NewReader(buf)
	err = binary.Read(reader, binary.LittleEndian, &msg.len)
	if err != nil {
		log.Fatal("error while writing message type: %v\n", err)
	}

	msg.payload = make([]byte, msg.len)
	n, err = c.Read(msg.payload)
	if err != nil {
		return read_uds_message_error(err, c)
	}
	if n != len(msg.payload) {
		log.Fatal("not expected number of bytes read: wanted %d, read %d", len(msg.payload), n)
	}

	//log.Printf("msg read: %v\n", msg)
	return msg
}

func reader(c net.Conn) chan struct{} {
    ch := make(chan struct{})
    go func() {
        sigterm_recvd := false
        loop:
        for {
            select {
            case <-ch:
                log.Println("terminate signal received, so exit reader")
                sigterm_recvd = true
                break loop
            default:
                log.Println("no terminate signal received, so continue")
            }
            msg := read_uds_message(c)
            switch msg.typ {
            case undefined:
                log.Println("reader stops due to reading error")
                break loop
            case udsmsg_serial2host:
                fmt.Print("<-")
                for i := uint32(0); i < msg.len; i++ {
                    switch msg.payload[i] {
                    case '\t':
                        fmt.Printf("\\t")
                    case '\r':
                        fmt.Printf("\\r")
                    case '\n':
                        fmt.Printf("\\n")
                    default:
                        fmt.Printf("%c", msg.payload[i])
                    }
                }
                fmt.Printf("\n")
                // hand over message to response interpreter
                received = append(received, msg.payload...)
                interpreter(c)

            case udsmsg_host2serial:
                log.Fatal("not expected type host2serial")
            case udsmsg_info:
                fmt.Printf("info: %s\n", string(msg.payload))
            case udsmsg_control:
                fmt.Printf("control: %s\n", string(msg.payload))
            default:
                log.Fatal(fmt.Sprintf("error: not expected message: %v", msg))
            }
        }
        write_uds_message(c, udsmsg_control, "")
        c.Close()
        log.Printf("connection closed\n")
        if sigterm_recvd {
            log.Printf("signal back to server that client connection has been closed\n")
            ch<-struct{}{}
        }
    }()

    return ch
}

func write_uds_message(c net.Conn, typ int, cmd string) {
	msg := udsMessage{
		typ:     uint8(typ),
		len:     uint32(len(cmd)),
		payload: []byte(cmd),
	}
	buf := new(bytes.Buffer)
	// write type
	err := binary.Write(buf, binary.LittleEndian, msg.typ)
	if err != nil {
		log.Fatalf("error: could not write type: %v\n", err)
	}
	n, err := buf.WriteTo(c)
	if err != nil || n != 1 {
		log.Fatalf("error: could not write to socket: %v\n", err)
	}

	// write length
	err = binary.Write(buf, binary.LittleEndian, msg.len)
	if err != nil {
		log.Fatalf("error: could not write length: %v\n", err)
	}
	n, err = buf.WriteTo(c)
	if err != nil || n != 4 {
		log.Fatalf("error: could not write to socket: %v\n", err)
	}

	// write payload
	m, err := c.Write(msg.payload)
	if err != nil || uint32(m) != msg.len {
		log.Fatalf("error: could not write to socket: %v\n", err)
	}

	// store request for reference reasons for interpreter
	request = msg.payload

	// user output
	fmt.Print("->")
	for i := uint32(0); i < msg.len; i++ {
		switch msg.payload[i] {
		case '\t':
			fmt.Printf("\\t")
		case '\r':
			fmt.Printf("\\r")
		case '\n':
			fmt.Printf("\\n")
		default:
			fmt.Printf("%c", msg.payload[i])
		}
	}
	fmt.Printf("\n")
}
