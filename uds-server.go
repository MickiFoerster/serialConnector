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
	"os/exec"
	"os/signal"
	"syscall"
	"text/template"

	"github.com/fatih/color"
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
	clients  = map[chan struct{}]bool{}
	username string
	password string

	empty_uds_message = udsMessage{
		typ: undefined,
		len: 0,
	}
	request  = []byte{}
	received = []byte{}

	terminate_signal = make(chan struct{})

	template_serial_channel_c *template.Template
	template_uds_channel_c    *template.Template
	template_serial_c         *template.Template
)

func init() {
	os.Remove(uds_file_path)

	template_serial_channel_c = template.Must(template.ParseFiles("templates/serial-channel.gotmpl"))
	template_uds_channel_c = template.Must(template.ParseFiles("templates/uds_channel_c.gotmpl"))
	template_serial_c = template.Must(template.ParseFiles("templates/serial_c.gotmpl"))
}

func handleClient(c net.Conn) chan struct{} {
	ch := reader(c)

	// initial starting communication by sending a request to see how target
	// will response
	err := write_uds_message(c, udsmsg_host2serial, "exit\n")
	if err != nil {
		log.Printf("error while sending command 'exit' to client: %v\n", err)
		c.Close()
	}

	return ch
}

func main() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)
	go func() {
		<-ch
		terminate_signal <- struct{}{}
	}()

	flag.StringVar(&username, "username", "pi", "username for login")
	flag.StringVar(&password, "password", "raspberry", "password for login")
	flag.Parse()
	log.Printf("username: %v\n", username)
	log.Printf("password: %v\n", password)
	fill_reactions()

	serverdone := server()
	client()

	// wait for SIGINT
	<-terminate_signal

	// stop server
	log.Println("send server to stop ...")
	serverdone <- struct{}{}
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
			listener <- c
		}
	}()

	go func() { // event loop
	serverloop:
		for {
			select {
			case <-serverdone:
				log.Println("terminate signal received, so exit server")
				break serverloop
			case client := <-listener:
				chan2client := handleClient(client)
				clients[chan2client] = true
			}
		}
		log.Println("signal clients to terminate")
		i := 1
		for chan2client, _ := range clients {
			log.Printf("signal client %v to terminate", i)
			chan2client <- struct{}{}
			log.Println("wait for client to complete shutdown")
			<-chan2client
			i += 1
		}
		log.Println("server signals back that shutdown is complete")
		serverdone <- struct{}{}
	}()

	return serverdone
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

func reader(c net.Conn) chan struct{} {
	sigterm_chan := make(chan struct{})
	eventloop_reader_chan := make(chan udsMessage)
	go func() {
		sigterm_recvd := false
	eventloop:
		for {
			select {
			case <-sigterm_chan:
				log.Println("terminate signal received, so exit reader")
				sigterm_recvd = true
				break eventloop
			case msg := <-eventloop_reader_chan:
				log.Printf("UDS message received: %v\n", msg)
				switch msg.typ {
				case undefined:
					log.Println("reader stops due to reading error")
					break eventloop
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
		}
		close_uds_channel(c)
		if sigterm_recvd {
			log.Printf("signal back to server that client connection has been closed\n")
			sigterm_chan <- struct{}{}
		}
		close(sigterm_chan)
		delete(clients, sigterm_chan)
	}()

	go func() {
		for {
			log.Println("enter read_uds_message")
			eventloop_reader_chan <- read_uds_message(c)
			log.Println("back from read_uds_message")
		}
	}()

	return sigterm_chan
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

	return nil
}

func close_uds_channel(c net.Conn) {
	err := write_uds_message(c, udsmsg_control, "")
	if err != nil {
		log.Println("signal child process to terminate failed: %v", err)
	}
	log.Println("closing UDS connection ...")
	err = c.Close()
	if err != nil {
		log.Println("closing of UDS connection failed: %v", err)
		return
	}
	log.Println("UDS connection closed successfully")
}

func client() {
	fn := "serial.c"
	f, err := os.Create(fn)
	if err != nil {
		log.Fatal(err)
	}
	err = template_serial_c.Execute(f, nil)
	f.Close()
	cfiles := []string{fn}

	fn = "serial-channel.c"
	f, err = os.Create(fn)
	if err != nil {
		log.Fatal(err)
	}
	err = template_serial_channel_c.Execute(f, nil)
	f.Close()
	cfiles = append(cfiles, fn)

	fn = "uds-channel.c"
	f, err = os.Create(fn)
	if err != nil {
		log.Fatal(err)
	}
	err = template_uds_channel_c.Execute(f, nil)
	f.Close()
	cfiles = append(cfiles, fn)

	for _, cfile := range cfiles {
		compiler := "gcc"
		args := []string{"-c", "-std=c11", "-ggdb3", "-Wall", "-Werror", cfile}
		s := fmt.Sprint(compiler)
		for _, arg := range args {
			s += fmt.Sprintf(" %s", arg)
		}
		s += fmt.Sprint(": ")

		cmd := exec.Command(compiler, args...)
		err := cmd.Run()
		if err != nil {
			fmt.Printf("%-40s", s)
			color.Red(" %10s ", "[failed]")
			fmt.Println(err)
		} else {
			fmt.Printf("%-40s", s)
			color.Green(" %10s ", "[OK]")
		}
	}
}
