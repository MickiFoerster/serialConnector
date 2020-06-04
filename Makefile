TARGETS=uds-server-go
TEMPLATES=$(shell find templates -name "*.gotmpl")

all: $(TARGETS)

uds-server.go: $(TEMPLATES)
	touch $@
uds-server-go: uds-server.go request-response-handling.go
	go build -o $@ $^

clean: 
	rm -f $(TARGETS) *.o *.c *.h serial

.PHONY: clean all
