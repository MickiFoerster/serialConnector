#pragma once

#include <stdbool.h>
#include <stdint.h>

typedef struct {
  const char *portname;
  int sock;
  bool (*Open)(const char *portname);
  void (*Close)(void);
  ssize_t (*Read)(unsigned char *buf, size_t count);
  ssize_t (*Write)(const unsigned char* buf, size_t count);
} serialChannelType;

extern serialChannelType serialChannel;
