package main

import (
	"bytes"
	"encoding/binary"
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
	username = "pi\n"
	password = "raspberry\n"

	uds_file_path = "/tmp/ASDF"
)

const (
	undefined = iota
	udsmsg_serial2host
	udsmsg_host2serial
	udsmsg_info
)

var (
	last_msg          udsMessage
	empty_uds_message = udsMessage{
		typ: undefined,
		len: 0,
	}
	received = []byte{}
)

func init() {
	os.Remove(uds_file_path)
}

func handleClient(c net.Conn) {
	go reader(c)

	// initial starting communication by sending a request to see how target
	// will response
	write_uds_message(c, "exit\n")
}

func main() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)
	go func() {
		<-ch
		os.Remove(uds_file_path)
		log.Printf("file %s has been deleted\n", uds_file_path)
		os.Exit(0)
	}()

	l, err := net.Listen("unix", uds_file_path)
	if err != nil {
		log.Fatal("listen failed:", err)
	}

	for {
		c, err := l.Accept()
		if err != nil {
			log.Fatal("accept() failed:", err)
		}

		go handleClient(c)
	}
}

func read_uds_message_error(err error, c net.Conn) udsMessage {
	if err != io.EOF {
		log.Fatalf("error while reading from client: %s\n", err)
	}
	log.Printf("EOF received from client. Close connection.\n")
	c.Close()
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

func reader(c net.Conn) {
loop:
	for {
		msg := read_uds_message(c)
		switch msg.typ {
		case undefined:
			log.Println("connection closed, finishing reading messages")
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
		default:
			log.Fatal(fmt.Sprintf("error: not expected message: %v", msg))
		}

	}
}

func write_uds_message(c net.Conn, cmd string) {
	msg := udsMessage{
		typ:     uint8(udsmsg_host2serial),
		len:     uint32(len(cmd)),
		payload: []byte(cmd),
	}
	buf := new(bytes.Buffer)
	// write type
	err := binary.Write(buf, binary.LittleEndian, msg.typ)
	if err != nil {
		log.Fatal("error: could not write type: %v", err)
	}
	n, err := buf.WriteTo(c)
	if err != nil || n != 1 {
		log.Fatal("error: could not write to socket: %v", err)
	}

	// write length
	err = binary.Write(buf, binary.LittleEndian, msg.len)
	if err != nil {
		log.Fatal("error: could not write length: %v", err)
	}
	n, err = buf.WriteTo(c)
	if err != nil || n != 4 {
		log.Fatal("error: could not write to socket: %v", err)
	}

	// write payload
	m, err := c.Write(msg.payload)
	if err != nil || uint32(m) != msg.len {
		log.Fatal("error: could not write to socket: %v", err)
	}

	// store request for reference reasons for interpreter
	last_msg = msg

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
