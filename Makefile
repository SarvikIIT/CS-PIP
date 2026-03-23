CC      = gcc
CFLAGS  = -Wall -Wextra -std=c11 -D_GNU_SOURCE -I.
LDFLAGS =

RUNTIME_DIR = internal/runtime
CMD_DIR     = cmd/cspip
TEST_DIR    = tests
BUILD_DIR   = build

RUNTIME_SRCS = \
	$(RUNTIME_DIR)/container.c \
	$(RUNTIME_DIR)/namespace.c \
	$(RUNTIME_DIR)/cgroup.c \
	$(RUNTIME_DIR)/rootfs.c \
	$(RUNTIME_DIR)/network.c

CMD_SRCS  = $(CMD_DIR)/main.c
TEST_SRCS = $(TEST_DIR)/runtime_test.c

RUNTIME_OBJS = $(patsubst %.c, $(BUILD_DIR)/%.o, $(RUNTIME_SRCS))
CMD_OBJS     = $(patsubst %.c, $(BUILD_DIR)/%.o, $(CMD_SRCS))
TEST_OBJS    = $(patsubst %.c, $(BUILD_DIR)/%.o, $(TEST_SRCS))

TARGET      = $(BUILD_DIR)/cspip
TEST_TARGET = $(BUILD_DIR)/runtime_test

.PHONY: all clean test rootfs

all: $(TARGET)

$(TARGET): $(RUNTIME_OBJS) $(CMD_OBJS)
	$(CC) $(CFLAGS) -o $@ $^ $(LDFLAGS)

$(TEST_TARGET): $(RUNTIME_OBJS) $(TEST_OBJS)
	$(CC) $(CFLAGS) -o $@ $^ $(LDFLAGS)

$(BUILD_DIR)/%.o: %.c
	@mkdir -p $(dir $@)
	$(CC) $(CFLAGS) -c -o $@ $<

test: $(TEST_TARGET)
	sudo $(TEST_TARGET)

# Set up a minimal busybox rootfs (requires busybox installed on host)
rootfs:
	@echo "Setting up minimal rootfs..."
	mkdir -p rootfs/bin rootfs/etc rootfs/proc rootfs/tmp rootfs/dev
	@if command -v busybox >/dev/null 2>&1; then \
		cp $$(which busybox) rootfs/bin/busybox; \
		cd rootfs/bin && for cmd in sh ls ps cat echo mkdir rm cp mv; do \
			ln -sf busybox $$cmd; \
		done; \
	else \
		echo "busybox not found — install it or copy a static binary to rootfs/bin/"; \
	fi
	@echo "hostname" > rootfs/etc/hostname
	@echo "nameserver 8.8.8.8" > rootfs/etc/resolv.conf
	@echo "rootfs ready."

clean:
	rm -rf $(BUILD_DIR)

install: $(TARGET)
	install -m 755 $(TARGET) /usr/local/bin/cspip
