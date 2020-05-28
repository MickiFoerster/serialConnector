TARGETS=serial uds-server-go
CFLAGS=-g -Wall

%.o : %.c %.h Makefile
	$(CC) $(CFLAGS) -c -o $@ $<

all: $(TARGETS)
serial: serial.o uds-channel.o serial-channel.o
uds-server-go: uds-server.go request-response-handling.go
	go build -o $@ $^

clean: 
	rm -f $(TARGETS) *.o

.PHONY: clean all
