#include <assert.h>
#include <ctype.h>
#include <errno.h>
#include <fcntl.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/time.h>
#include <termios.h>
#include <unistd.h>

#include "config.h"
#include "serial-channel.h"

static bool serial_open_channel(const char *portname);
static void serial_close_channel(void);
static ssize_t serial_read_channel(unsigned char *buf, size_t count);
static ssize_t serial_write_channel(const unsigned char *buf, size_t count);

serialChannelType serialChannel = {
    .portname = "/dev/ttyUSB0",
    .sock = -1,
    .Open = serial_open_channel,
    .Close = serial_close_channel,
    .Read = serial_read_channel,
    .Write = serial_write_channel,
};

static int set_interface_attribs(int fd, int speed, int parity,
                                 int should_block) {
  struct termios tty;
  if (tcgetattr(fd, &tty) != 0) {
    error_message("error: tcgetattr failed: %s", strerror(errno));
    return -1;
  }

  cfsetospeed(&tty, speed);
  cfsetispeed(&tty, speed);

  tty.c_cflag = (tty.c_cflag & ~CSIZE) | CS8; // 8-bit chars
  // disable IGNBRK for mismatched speed tests; otherwise receive break
  // as \000 chars
  tty.c_iflag &= ~IGNBRK; // disable break processing
  tty.c_lflag = 0;        // no signaling chars, no echo,
                          // no canonical processing
  tty.c_oflag = 0;        // no remapping, no delays
  tty.c_cc[VMIN] = 0;     // read doesn't block
  tty.c_cc[VTIME] = 5;    // 0.5 seconds read timeout

  tty.c_iflag &= ~(IXON | IXOFF | IXANY); // shut off xon/xoff ctrl

  tty.c_cflag |= (CLOCAL | CREAD);   // ignore modem controls,
                                     // enable reading
  tty.c_cflag &= ~(PARENB | PARODD); // shut off parity
  tty.c_cflag |= parity;
  tty.c_cflag &= ~CSTOPB;
  tty.c_cflag &= ~CRTSCTS;

  tty.c_cc[VMIN] = should_block ? 1 : 0;
  tty.c_cc[VTIME] = 5; // 0.5 seconds read timeout

  if (tcsetattr(fd, TCSANOW, &tty) != 0) {
    error_message("error: tcsetattr failed: %s\n", strerror(errno));
    exit(1);
  }
  return 0;
}

static bool serial_open_channel(const char *portname) {
  int fd = open(portname, O_RDWR | O_NOCTTY | O_SYNC);
  if (fd < 0) {
    error_message("error: could not open serial device '%s': %s\n", portname,
                  strerror(errno));
    exit(1);
  }

  set_interface_attribs(fd, B115200,
                        0,  // set speed to 115,200 bps, 8n1 (no parity)
                        0); // set no blocking

  serialChannel.sock = fd;
  atexit(serialChannel.Close);
  return true;
}

static void serial_close_channel(void) {
  for (;;) {
    int rc = close(serialChannel.sock);
    if (rc == -1) {
      switch (errno) {
      case EINTR:
        continue;
      }
      error_message("error: could not close serial channel properly: %s\n",
                    strerror(errno));
    }
    serialChannel.sock = -1;
    break; // exit loop
  }
  fprintf(stderr, "info: Serial channel closed\n");
}

static ssize_t serial_read_channel(unsigned char *buf, size_t count) {
  return read(serialChannel.sock, buf, count);
}

static ssize_t serial_write_channel(const unsigned char *buf, size_t count) {
  return write(serialChannel.sock, buf, count);
}
