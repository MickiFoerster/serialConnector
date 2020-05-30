#pragma once

#include <stdbool.h>
#include <stdint.h>

typedef enum {
  undefined = 0,
  udsmsg_serial2host,
  udsmsg_host2serial,
  udsmsg_info,
  udsmsg_control,
} udsMessageType;

typedef struct {
  uint8_t typ;
  uint32_t len;
  unsigned char *payload;
} udsMessage;

typedef struct {
  int sock;
  bool (*Open)(void);
  void (*Close)(void);
  bool (*Read)(udsMessage *msg);
  void (*Write)(udsMessageType type, const char *str);
} udsChannelType;

extern udsChannelType udsChannel;
