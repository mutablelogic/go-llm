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

# llm runs the command as the llm user
llm() {
    gosu llm /usr/local/bin/llm "$@"
}

# Run the server, or the command, as the llm user, unless the server is
# run for a new installation, which has no providers yet
if [ "$1" != "run" ] || [ -f "${DATA}/llm/providers.json" ]; then
    exec gosu llm /usr/local/bin/llm "$@"
fi

# A new installation runs the server, and, once it answers, adds the Eliza
# provider, unless NOELIZA is set, and those whose variables are set:
# OLLAMA_ADDR and GOWHISPER_ADDR, as their URLs, and OPENAI_API_KEY,
# DEEPSEEK_API_KEY, MISTRAL_API_KEY and ELEVENLABS_API_KEY, by the names of
# their credentials, which are read when they're used, so the keys aren't
# stored. One which can't be added, such as one which can't be reached, is
# said to be, and can be added later, with provider-create.
gosu llm /usr/local/bin/llm "$@" &
PID=$!
trap 'kill -TERM "${PID}" 2>/dev/null' TERM INT

TRIES=60
until llm providers >/dev/null 2>&1; do
    TRIES=$((TRIES - 1))
    if [ "${TRIES}" -le 0 ] || ! kill -0 "${PID}" 2>/dev/null; then
        echo "entrypoint: the server didn't answer, so no providers were added" >&2
        break
    fi
    sleep 1
done

# add adds a provider, with the command which creates it, such as
# provider-create-ollama, saying whether it was added
add() {
    NAME="${1#provider-create-}"
    if llm "$@" >/dev/null 2>&1; then
        echo "entrypoint: added the ${NAME} provider" >&2
    else
        echo "entrypoint: the ${NAME} provider wasn't added, which llm $1 can add later" >&2
    fi
}

if [ "${TRIES}" -gt 0 ]; then
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
if kill -0 "${PID}" 2>/dev/null; then
    wait "${PID}"
    STATUS=$?
fi
exit "${STATUS}"
