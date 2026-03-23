cmake_minimum_required(VERSION 3.16)
project(cspip C)

set(CMAKE_C_STANDARD 11)
set(CMAKE_C_FLAGS "${CMAKE_C_FLAGS} -Wall -Wextra -D_GNU_SOURCE")

include_directories(${CMAKE_SOURCE_DIR})

set(RUNTIME_SOURCES
    internal/runtime/container.c
    internal/runtime/namespace.c
    internal/runtime/cgroup.c
    internal/runtime/rootfs.c
    internal/runtime/network.c
)

# Main binary
add_executable(cspip
    cmd/cspip/main.c
    ${RUNTIME_SOURCES}
)

# Test binary
add_executable(runtime_test
    tests/runtime_test.c
    ${RUNTIME_SOURCES}
)

install(TARGETS cspip DESTINATION bin)
