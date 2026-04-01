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

CMD_SRCS  = cmd/cspip-runtime/main.c
TEST_SRCS = $(TEST_DIR)/runtime_test.c

RUNTIME_OBJS = $(patsubst %.c, $(BUILD_DIR)/%.o, $(RUNTIME_SRCS))
CMD_OBJS     = $(patsubst %.c, $(BUILD_DIR)/%.o, $(CMD_SRCS))
TEST_OBJS    = $(patsubst %.c, $(BUILD_DIR)/%.o, $(TEST_SRCS))

# C runtime binary (container lifecycle: run/exec/inspect/ps/stop/kill/rm)
RUNTIME_TARGET = $(BUILD_DIR)/cspip-runtime
# Go CLI binary  (wraps runtime + implements profiling and report command)
GO_TARGET      = $(BUILD_DIR)/cspip
TEST_TARGET    = $(BUILD_DIR)/runtime_test

.PHONY: all build build-runtime build-go clean test test-go rootfs install

all: build

build: build-runtime build-go

# Build the C container runtime binary.
build-runtime: $(RUNTIME_TARGET)

$(RUNTIME_TARGET): $(RUNTIME_OBJS) $(CMD_OBJS)
	$(CC) $(CFLAGS) -o $@ $^ $(LDFLAGS)

$(TEST_TARGET): $(RUNTIME_OBJS) $(TEST_OBJS)
	$(CC) $(CFLAGS) -o $@ $^ $(LDFLAGS)

$(BUILD_DIR)/%.o: %.c
	@mkdir -p $(dir $@)
	$(CC) $(CFLAGS) -c -o $@ $<

# Build the Go CLI binary.
build-go:
	@mkdir -p $(BUILD_DIR)
	go build -o $(GO_TARGET) ./cmd/cspip/

# Run C runtime unit tests (requires root).
test: $(TEST_TARGET)
	sudo $(TEST_TARGET)

# Run Go unit tests (no root required).
test-go:
	go test ./...

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

install: build
	install -m 755 $(RUNTIME_TARGET) /usr/local/bin/cspip-runtime
	install -m 755 $(GO_TARGET) /usr/local/bin/cspip
