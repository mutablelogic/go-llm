#!/bin/sh
set -e

# If UID or GID are set, remap the webhook user and group, so the server
# runs as that user and group, such as the owner of a mounted /data. An ID
# which another user, or group, already has is shared with it.
if [ -n "${GID}" ] && [ "${GID}" != "$(id -g webhook)" ]; then
    groupmod -o -g "${GID}" webhook
fi
if [ -n "${UID}" ] && [ "${UID}" != "$(id -u webhook)" ]; then
    usermod -o -u "${UID}" webhook
fi

# The server keeps its data in /data/webhook, which is the webhook user's, as
# is /data, so a volume, or a directory, which is mounted as either, and isn't,
# such as a new one, is made the webhook user's
DATA="${XDG_DATA_HOME:-/data}"
mkdir -p "${DATA}"
for DIR in "${DATA}" "${DATA}/webhook"; do
    if [ -d "${DIR}" ] && { [ "$(stat -c %u "${DIR}")" != "$(id -u webhook)" ] || [ "$(stat -c %g "${DIR}")" != "$(id -g webhook)" ]; }; then
        chown -R webhook:webhook "${DIR}"
    fi
done

# Run the server, by default, or the command, as the webhook user
if [ $# -eq 0 ]; then
    set -- run
fi
exec gosu webhook /usr/local/bin/webhook "$@"
