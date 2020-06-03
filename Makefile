TARGETS=uds-server-go

all: $(TARGETS)

uds-server-go: uds-server.go request-response-handling.go
	go build -o $@ $^

clean: 
	rm -f $(TARGETS) *.o *.c *.h

.PHONY: clean all
