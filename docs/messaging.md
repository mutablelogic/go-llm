# Messaging

Connect chats on messaging services to a conversation, so you can talk to your assistant from your phone. Telegram is the first. Slack ([#249](https://github.com/mutablelogic/go-llm/issues/249)) and email ([#305](https://github.com/mutablelogic/go-llm/issues/305)) are planned.

## Telegram

1. Make a bot with [BotFather](https://t.me/BotFather), and set its token as `TELEGRAM_BOT_TOKEN`.
2. In a conversation whose model has the `messaging` tools, ask to talk on Telegram. You get a link, valid once for 15 minutes, which the web UI also shows as a QR code.
3. Open the link, or scan the code, and press Start.

```bash
llm session-create --model ollama:llama3.2 --tools "messaging:*"
llm chat "Let's talk on Telegram"
```

## Channels

- **Both ways:** what you say on Telegram goes to the conversation. Every reply, and every task's outcome worth telling you, goes to all its channels.
- **Managed by the assistant:** it connects, lists, sends to and disconnects chats, with the `messaging` tools.
- **More than one:** a conversation can have several chats, such as yours and a group's. A chat belongs to one conversation, so connecting it again moves it.
- **Groups:** everyone in a connected group can talk to the assistant, and see what it remembers about you. Only connect groups whose people you'd tell.
- **Who said it:** each message says where it came from, and who sent it.
- **Ignored:** messages from chats which aren't connected are ignored.

## Running the bot

- **No public address needed:** the server reads the bot's messages by long polling.
- **One server per bot:** don't share a bot's token between servers.
- **Progress:** while a reply takes a while, the chat shows what's happening, in one message, which is deleted when the reply arrives.

## Replies

- **Formatting:** Markdown is converted to Telegram's. Long replies are split between paragraphs.
- **Files:** pictures and speech a reply makes are sent with it. Images go as photos, Ogg audio as voice messages, MP3 and M4A as tracks, and anything else as documents, up to 50 MB.

## Voice notes

To talk by voice, the assistant needs a model that speaks, and one that transcribes. With the compose file's `ollama` and `whisper` profiles, these run on your own machine:

```bash
llm model-load ollama:phonon:latest          # speaks
llm model-load gowhisper:ggml-medium-q5_0    # transcribes
```

- **Tell the assistant which to use,** such as "speak with `ollama:phonon:latest` in Alfie's voice, and transcribe with `gowhisper:ggml-medium-q5_0`".
- **Speaking:** on Telegram, replies are sent as Ogg voice notes. Models that can't make them need `opusenc`, which the Docker image has.
- **Transcribing:** `ggml-medium-q5_0` runs on small machines, but can mishear names. `ggml-large-v3-turbo-q5_0` hears them better, on a faster machine.

## Files you send

- **Kept with the conversation:** photos, documents, voice messages, audio and video become the conversation's files. A caption is the message's text.
- **Albums** arrive as one message.
- **Over 20 MB:** bots can't download files this large, so the assistant is told it didn't get them.
