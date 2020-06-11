package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"syscall"
)

func server(client_sync chan struct{}) (chan struct{}, error) {
	server_done := make(chan struct{})

	l, err := net.Listen("unix", uds_file_path)
	if err != nil {
		return nil, fmt.Errorf("error: could not create passive socket: %v\n", err)
	}
	log.Println("Listener created successfully")

	go func() { // listener routine
		defer os.Remove(uds_file_path)

		log.Println("server: signal client that Accept now starts ...")
		client_sync <- struct{}{} // signal to client that server listens now
		c, err := l.Accept()
		if err != nil {
			log.Fatal("Accept() failed:", err)
		}

		reader(server_done, c)
		// initial starting communication by sending a request to see how target
		// will response
		err = write_uds_message(c, udsmsg_host2serial, "exit\n")
		if err != nil {
			log.Fatalf("error while sending command 'exit' to client: %v\n", err)
		}
	}()

	return server_done, nil
}

func reader(server_done chan struct{}, c net.Conn) {
	uds_ch := make(chan udsMessage)

	go func() {
		for {
			msg := read_uds_message(c)
			uds_ch <- msg
		}
	}()

	go func() {
	loop:
		for {
			select {
			case <-server_done:
				log.Println("reader received termination signal, so close connection")
				close_uds_channel(c)
				break loop
			case msg := <-uds_ch:
				err := process_received_uds_msg(msg, c)
				if err != nil {
					log.Printf("reader: terminate reader since error occurred: %v\n", err)
					break loop
				}
			}
		}
		// signal back that termination complete
		server_done <- struct{}{}
	}()
}

func process_received_uds_msg(msg udsMessage, c net.Conn) error {
	switch msg.typ {
	case undefined:
		return fmt.Errorf("error while reader occurred\n")
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
		updateCurrentStateRecv(msg)

	case udsmsg_host2serial:
		log.Fatal("not expected type host2serial")
	case udsmsg_info:
		fmt.Printf("info: %s\n", string(msg.payload))
	case udsmsg_control:
		fmt.Printf("control: %s\n", string(msg.payload))
	default:
		log.Fatal(fmt.Sprintf("error: not expected message: %v", msg))
	}
	return nil
}

func close_uds_channel(c net.Conn) {
	err := write_uds_message(c, udsmsg_control, "")
	if err != nil {
		log.Fatalf("signal child process to terminate failed: %v", err)
	}
	log.Println("closing UDS connection ...")
	err = c.Close()
	if err != nil {
		log.Printf("closing of UDS connection failed: %v", err)
		return
	}
	log.Println("UDS connection closed successfully")
}

func write_uds_message(c net.Conn, typ int, cmd string) error {
	msg := udsMessage{
		typ:     uint8(typ),
		len:     uint32(len(cmd)),
		payload: []byte(cmd),
	}
	buf := new(bytes.Buffer)
	// write type
	err := binary.Write(buf, binary.LittleEndian, msg.typ)
	if err != nil {
		return err
	}
	_, err = buf.WriteTo(c)
	if err != nil {
		return err
	}

	// write length
	err = binary.Write(buf, binary.LittleEndian, msg.len)
	if err != nil {
		return err
	}
	_, err = buf.WriteTo(c)
	if err != nil {
		return err
	}

	// write payload
	_, err = c.Write(msg.payload)
	if err != nil {
		return err
	}

	// store request for reference reasons for interpreter
	updateCurrentStateSent(msg)

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

	return nil
}

func read_uds_message_error(err error) udsMessage {
	switch err {
	case syscall.EPIPE:
		log.Printf("BROKEN PIPE occurred\n")
	case io.EOF:
		log.Printf("client closed the connection\n")
	default:
	}
	return empty_uds_message
}

func read_uds_message(c net.Conn) udsMessage {
	var msg udsMessage
	buf := make([]byte, 1)
	_, err := c.Read(buf)
	if err != nil {
		return read_uds_message_error(err)
	}
	reader := bytes.NewReader(buf)
	err = binary.Read(reader, binary.LittleEndian, &msg.typ)
	if err != nil {
		return read_uds_message_error(err)
	}

	buf = make([]byte, 4)
	_, err = c.Read(buf)
	if err != nil {
		return read_uds_message_error(err)
	}
	reader = bytes.NewReader(buf)
	err = binary.Read(reader, binary.LittleEndian, &msg.len)
	if err != nil {
		return read_uds_message_error(err)
	}

	msg.payload = make([]byte, msg.len)
	_, err = c.Read(msg.payload)
	if err != nil {
		return read_uds_message_error(err)
	}
	return msg
}
