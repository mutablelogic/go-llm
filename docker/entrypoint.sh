#!/bin/sh
set -e

# If UID or GID are set, remap the llm user and group, so the server runs
# as that user and group, such as the owner of a mounted /data
if [ -n "${GID}" ] && [ "${GID}" != "$(id -g llm)" ]; then
    groupmod -g "${GID}" llm
fi
if [ -n "${UID}" ] && [ "${UID}" != "$(id -u llm)" ]; then
    usermod -u "${UID}" llm
fi

# The server keeps its data in /data, which is the llm user's, so a volume,
# or a directory which is mounted, which isn't, such as a new one, is made
# the llm user's
DATA="${XDG_CACHE_HOME:-/data}"
mkdir -p "${DATA}"
if [ "$(stat -c %u "${DATA}")" != "$(id -u llm)" ] || [ "$(stat -c %g "${DATA}")" != "$(id -g llm)" ]; then
    chown -R llm:llm "${DATA}"
fi

# Default to "run" if no arguments are given
if [ $# -eq 0 ]; then
    set -- run
fi

# Drop privileges and exec llm
exec gosu llm /usr/local/bin/llm "$@"
