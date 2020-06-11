package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
)

type udsMessage struct {
	typ     uint8
	len     uint32
	payload []byte
}

const (
	uds_file_path               = "/tmp/uds-server.uds"
	serial_device_config_string = "100:0:1cb2:0:3:1c:7f:15:4:5:0:0:11:13:1a:0:12:f:17:16:0:0:0:0:0:0:0:0:0:0:0:0:0:0:0:0"
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
	device   string

	empty_uds_message = udsMessage{
		typ: undefined,
		len: 0,
	}

	terminate_signal = make(chan struct{})
)

func init() {
	os.Remove(uds_file_path)
	start_statemachine()
}

func main() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)
	go func() {
		caught := 0
		for {
			<-ch
			caught += 1
			if caught > 1 {
				log.Fatalf("SIGINT received already twice, so exit the hard way\n")
			}
			terminate_signal <- struct{}{}
		}
	}()

	flag.StringVar(&username, "username", "pi", "username for login")
	flag.StringVar(&password, "password", "raspberry", "password for login")
	flag.StringVar(&device, "device", "/dev/ttyUSB0", "device for serial connection")
	flag.Parse()
	log.Printf("username: %v\n", username)
	log.Printf("password: %v\n", password)

	// init serial device
	cmd := exec.Command("stty",
		fmt.Sprintf("--file=%s", device),
		serial_device_config_string)
	err := cmd.Run()
	if err != nil {
		log.Fatalf("error while configuring tty device %v: %v\n", device, err)
	}
	log.Printf("serial device %v was configured with tty successfully\n", device)

	client_sync := make(chan struct{})
	server_done, err := server(client_sync)
	if err != nil {
		log.Fatalf("error: server could not be started: %v\n", err)
	}
	client_done, err := client(client_sync)
	if err != nil {
		log.Fatalf("error: client build/execution failed: %v\n", err)
	}

	alldone := make(chan struct{})
	go func() {
		for {
			select {
			case <-terminate_signal:
				log.Println("send server to stop ...")
				server_done <- struct{}{}
			case <-server_done:
				log.Println("server shutdown is complete")
				alldone <- struct{}{}
			case <-client_done:
				log.Println("client shutdown is complete")
				alldone <- struct{}{}
			}
		}
	}()

	for i := 0; i < 2; i++ {
		<-alldone
	}
	log.Println("terminating main")
}
