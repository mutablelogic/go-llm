#!/bin/sh
set -e

# If UID or GID are set, remap the llm user and group, so the server runs
# as that user and group, such as the owner of a mounted /data. An ID which
# another user, or group, already has, such as the users group's 100, is
# shared with it.
if [ -n "${GID}" ] && [ "${GID}" != "$(id -g llm)" ]; then
    groupmod -o -g "${GID}" llm
fi
if [ -n "${UID}" ] && [ "${UID}" != "$(id -u llm)" ]; then
    usermod -o -u "${UID}" llm
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

# Run the server, or the command, as the llm user, unless the server is
# run for a new installation, which has no providers yet
if [ "$1" != "run" ] || [ -f "${DATA}/llm/providers.json" ]; then
    exec gosu llm /usr/local/bin/llm "$@"
fi

# A new installation runs the server, and, once it answers, adds the Eliza
# provider, unless NO_ELIZA is set, and those whose variables are set:
# OLLAMA_ADDR and GOWHISPER_ADDR, as their URLs, and OPENAI_API_KEY,
# DEEPSEEK_API_KEY, MISTRAL_API_KEY and ELEVENLABS_API_KEY, by the names of
# their credentials, which are read when they're used, so the keys aren't
# stored. One which can't be added, such as one which can't be reached, is
# said to be, and can be added later, with provider-create.

# The server's address, and prefix, which are its flags, when they're given,
# such as --http.addr=0.0.0.0:9090, or else its variables, such as ADDR, are
# the clients' too, so they ask the server where it is
CLIENT=""
PREVIOUS=""
for ARG in "$@"; do
    case "${PREVIOUS}" in
    --http.addr | --http.prefix) CLIENT="${CLIENT} ${PREVIOUS}=${ARG}" ;;
    esac
    case "${ARG}" in
    --http.addr=* | --http.prefix=*) CLIENT="${CLIENT} ${ARG}" ;;
    esac
    PREVIOUS="${ARG}"
done

gosu llm /usr/local/bin/llm "$@" &
PID=$!

# The server is stopped, as the container is, at once, as is the client, or
# the wait, which is running, so its stop isn't held up by them
STOPPING=""
HELPER=""
trap 'STOPPING=1; kill -TERM "${PID}" ${HELPER} 2>/dev/null' TERM INT

# background runs the command in the background, and waits for it, which a
# signal interrupts, returning its status, or failing, once the server is
# stopping
background() {
    if [ -n "${STOPPING}" ]; then
        return 1
    fi
    "$@" &
    HELPER=$!
    STATUS=0
    wait "${HELPER}" || STATUS=$?
    if [ -n "${STOPPING}" ]; then
        wait "${HELPER}" 2>/dev/null || true
        HELPER=""
        return 1
    fi
    HELPER=""
    return "${STATUS}"
}

# client runs the llm command as a client, as the llm user, of the server,
# which is local, so it isn't asked through a proxy
client() {
    # shellcheck disable=SC2086
    background env NO_PROXY='*' no_proxy='*' gosu llm /usr/local/bin/llm "$@" ${CLIENT} >/dev/null 2>&1
}

TRIES=60
until client providers; do
    TRIES=$((TRIES - 1))
    if [ -n "${STOPPING}" ]; then
        break
    elif [ "${TRIES}" -le 0 ] || ! kill -0 "${PID}" 2>/dev/null; then
        echo "entrypoint: the server didn't answer, so no providers were added" >&2
        break
    fi
    background sleep 1 || true
done

# add adds a provider, with the command which creates it, such as
# provider-create-ollama, saying whether it was added
add() {
    NAME="${1#provider-create-}"
    if client "$@"; then
        echo "entrypoint: added the ${NAME} provider" >&2
    elif [ -z "${STOPPING}" ]; then
        echo "entrypoint: the ${NAME} provider wasn't added, which llm $1 can add later" >&2
    fi
}

if [ "${TRIES}" -gt 0 ] && [ -z "${STOPPING}" ]; then
    [ -z "${NO_ELIZA}" ] && add provider-create-eliza
    [ -n "${OLLAMA_ADDR}" ] && add provider-create-ollama "${OLLAMA_ADDR}"
    [ -n "${GOWHISPER_ADDR}" ] && add provider-create-gowhisper "${GOWHISPER_ADDR}"
    [ -n "${OPENAI_API_KEY}" ] && add provider-create-openai --api-key-credential OPENAI_API_KEY
    [ -n "${DEEPSEEK_API_KEY}" ] && add provider-create-deepseek --api-key-credential DEEPSEEK_API_KEY
    [ -n "${MISTRAL_API_KEY}" ] && add provider-create-mistral --api-key-credential MISTRAL_API_KEY
    [ -n "${ELEVENLABS_API_KEY}" ] && add provider-create-elevenlabs --api-key-credential ELEVENLABS_API_KEY
fi

# Wait for the server, which is stopped, as the container is, by the trap,
# and exit as it does
set +e
wait "${PID}"
STATUS=$?
while kill -0 "${PID}" 2>/dev/null; do
    wait "${PID}"
    STATUS=$?
done
exit "${STATUS}"
