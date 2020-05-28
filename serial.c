#include <ctype.h>
#include <assert.h>
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
#include "uds-channel.h"

const char default_portname[] = "/dev/ttyUSB0";

static void init(int argc, char *argv[]) {
  const char *p = default_portname;
  if (argc > 1 && strstr(argv[1], "/dev/") != NULL) {
    p = argv[1];
  }
  if (!serialChannel.Open(p)) {
    error_message("error: could not create serial communication channel\n");
    exit(1);
  }

  if (!udsChannel.Open()) {
    error_message("error: could not create UDS communication channel\n");
    exit(1);
  }
}

int main(int argc, char* argv[]) {
  fd_set fds;

  init(argc, argv);
  for (;;) {
    struct timeval tv = {
        .tv_sec = 10,
        .tv_usec = 0,
    };

    FD_ZERO(&fds);
    FD_SET(udsChannel.sock, &fds);
    FD_SET(serialChannel.sock, &fds);
    int max = (serialChannel.sock < udsChannel.sock) ? udsChannel.sock
                                                     : serialChannel.sock;
    int active_socks = select(max + 1, &fds, NULL, NULL, &tv);
    switch (active_socks) {
    case 0:  // timeout
      fprintf(stderr, "error: select() returned with timeout\n");
      exit(1);
      break;
    case -1: // error
      fprintf(stderr, "error: select() failed: %s\n", strerror(errno));
      exit(1);
      break;
    default: // number of sockets is rc
      for (int i = 0; i < active_socks; ++i) {
        if (FD_ISSET(udsChannel.sock, &fds)) {
          udsMessage msg;
          if (udsChannel.Read(&msg)) {
            // write payload to serial channel
            ssize_t n = serialChannel.Write(msg.payload, msg.len);
            assert(n > 0);

            fprintf(stderr, "->");
            for (int i = 0; i < msg.len; ++i) {
              switch (msg.payload[i]) {
              case '\t':
                fprintf(stderr, "\\t");
                break;
              case '\r':
                fprintf(stderr, "\\r");
                break;
              case '\n':
                fprintf(stderr, "\\n");
                break;
              default:
                fprintf(stderr, "%c", msg.payload[i]);
              }
            }
            fprintf(stderr, "\n");
          }
        } else if (FD_ISSET(serialChannel.sock, &fds)) {
          unsigned char buf[4096];
          ssize_t n = serialChannel.Read(buf, sizeof buf);
          switch (n) {
          case -1:
            error_message("error: read() failed: %s\n", strerror(errno));
            exit(1);
            break;
          case 0:
            error_message("read() received zero/EOF\n");
            goto loop1;
            break;
          default:
            buf[n] = '\0';
            udsChannel.Write(udsmsg_serial2host, (char *)buf);
            // dump message to console
            fprintf(stderr, "<-");
            for (int i = 0; i < n; ++i) {
              switch (buf[i]) {
              case '\t':
                fprintf(stderr, "\\t");
                break;
              case '\r':
                fprintf(stderr, "\\r");
                break;
              case '\n':
                fprintf(stderr, "\\n");
                break;
              default:
                fprintf(stderr, "%c", buf[i]);
              }
            }
            fprintf(stderr, "\n");
            break;
          }
        }
      }
    }
  }
loop1:
#if 0
  short found = 127;
  const char *possible_responses[127];

  for(;;) {
    writestr(fd, "exit\n");
    const char login[] = "login: ";
    const char password[] = "Password: ";
    possible_responses[0] = login;
    possible_responses[1] = password;
    possible_responses[2] = NULL;
    found = expect(fd, possible_responses, 10);
    switch (found) {
    case 0:
      break;
    case 1:
      // In case of "Password:" we hit RETURN and expect "login:" again
      writestr(fd, "\n");
      possible_responses[1] = NULL;
      found = expect(fd, possible_responses, 10);
      if (found == 0) {
        break;
      }
      // in case of 127 fall-through to error message
    case 127:
      fprintf(
          stderr,
          "cannot recognize current state, so we continue to read from stdin and wait until stdin is silent\n");
      flush_stdin(fd, 15);
      fprintf( stderr, "continue ...\n");
      continue;
    default:
      assert(0 && "not expected case happened");
    }

    if (found == 0) {
      break; // exit from loop
    }
  }

  udsChannel.Write(udsmsg_info, "login prompt found\n");

  // login found
  writestr(fd, globalData.username);
  const char possible_response0[] = "Password: ";
  possible_responses[0] = possible_response0;
  possible_responses[1] = NULL;
  found = expect(fd, possible_responses, 15);
  switch (found) {
  case 0:
    break;
  case 127:
    fprintf(
        stderr,
        "cannot continue with test since expected response did not appear\n");
    return 0;
    break;
  default:
    assert(0 && "not expected case happened");
  }

  // password prompt found
  writestr(fd, globalData.password);
  const char user_shellprompt[] = "~$ ";
  const char root_shellprompt[] = "~# ";
  possible_responses[0] = user_shellprompt;
  possible_responses[1] = root_shellprompt;
  possible_responses[2] = NULL;
  found = expect(fd, possible_responses, 10);
  switch (found) {
  case 0:
  case 1:
    break;
  case 127:
    fprintf(
        stderr,
        "cannot continue with test since expected response did not appear\n");
    return 0;
    break;
  default:
    assert(0 && "not expected case happened");
  }

  writestr(fd, "hostname; echo DONE1234\n");
  const char end_mark1234[] = "\nDONE1234";
  possible_responses[0] = end_mark1234;
  possible_responses[1] = NULL;
  found = expect(fd, possible_responses, 5);
  switch (found) {
  case 0:
  case 1:
    break;
  case 127:
    fprintf(
        stderr,
        "cannot continue with test since expected response did not appear\n");
    return 0;
    break;
  default:
    assert(0 && "not expected case happened");
  }

  udsChannel.Write(udsmsg_info, "successfully logged in to host\n");

  udsMessage msg;
  ssize_t n = udsChannel.Read(&msg);
  if (n == 0) {
    fprintf(stderr, "could not read from channel\n");
  } else if (n < 0) {
    fprintf(stderr, "reading from channel failed\n");
  } else {
    fprintf(stderr, "read from UDS channel: %s\n", msg.payload);
    writestr(fd, (char *)msg.payload);
    possible_responses[0] = user_shellprompt;
    possible_responses[1] = root_shellprompt;
    possible_responses[2] = NULL;
    found = expect(fd, possible_responses, 10);
  }

#endif
 
  return 0;
}

#if 0
static void flush_stdin(int fd, int timeout) {
  size_t n;
  fd_set fds;
  struct timeval tv;
  int rc;
  char buf[4096];

  FD_ZERO(&fds);
  FD_SET(fd, &fds);
  tv.tv_sec = timeout;
  tv.tv_usec = 0;
  for(;;) {
    n = 0;
    rc = select(fd+1, &fds, NULL, NULL, &tv);
    switch(rc) {
      case -1:
        if (errno == EINTR || errno == EWOULDBLOCK) {
          continue;
        }
        error_message("error: select() failed: %s\n", strerror(errno));
        break;
      case 0:
        // time has expired without anything to read
        return;
      default:
        n = read(fd, buf, sizeof buf);
        buf[n] = '\0';
        udsChannel.Write(udsmsg_read, buf);
        break;
    }
    if (n<0) {
      error_message("error: read() failed: %s\n", strerror(errno));
      exit(1);
    }
  }
}

static short expect(int fd, const char *possible_responses[],
                    long timeout_in_seconds) {
  short found = 127;
  struct timeval start, current;
  char buf[4096];
  char *p = buf;

  gettimeofday(&start, NULL);
  for(;;) {
    gettimeofday(&current, NULL);
    long seconds = (current.tv_sec - start.tv_sec);
    long micros = 0;
    if (current.tv_usec >= start.tv_usec) {
      micros = current.tv_usec - start.tv_usec;
    } else {
      seconds--;
      micros = 1000000 + current.tv_usec - start.tv_usec;
    }
#if DEBUG
    fprintf(stderr, "\rtime spent inside loop: %03lds:%03ldms", seconds,
        micros / 1000);
#else
    (void)micros;
#endif
    if (seconds >= timeout_in_seconds) {
      break; // exit from loop when time expires
    }
    if (sizeof buf - (p - buf) <= 0) {
      fprintf(stderr, 
          "info: buffer for reading from channel is too small (%ld kb).\n"
          "We discard the content that came by until now.\n", sizeof buf / 1024);
      p = buf;
    }
    ssize_t n = read(fd, p, sizeof buf - (p - buf));
    if (n>0) {
      p[n] = '\0';
      udsChannel.Write(udsmsg_read, p);
      p += n;
      for (int i = 0; possible_responses[i] != NULL; ++i) {
        assert(i<127);
        if (strstr(buf, possible_responses[i])) {
          found = i;
          break;
        }
      }
      if (found != 127) {
        p = buf;
        break;
      }
    } else if (n < 0) {
      error_message("error: read() failed: %s\n", strerror(errno));
      exit(1);
      break;
    }
  }

  fprintf(stderr, "%s found\n", possible_responses[found]);
  return found;
}

#endif
