# Conversations and Tasks

## Conversations

A conversation is a session go-llm keeps: its messages, its model, its system prompt, its tools and its options.

- **Starting one:** **Add conversation** in the web UI, or `llm session-create`. New conversations in the web UI start with a memory window of 250K tokens.
- **Preferences:** the Conversation menu's **Preferences** set its model, its memory window, its instructions (the system prompt) and its toolkit.
- **Tools:** the model calls the conversation's tools until it replies. Its tools are given by name (`weatherapi:current`), by namespace (`weatherapi:*`), or all of them (`*`).
- **Switching models:** a conversation can change its model, such as from Claude to Gemini and back. A model that can't call tools keeps the conversation's tools for when it's changed back.
- **Everyone sees it:** every client of a conversation, such as the web UI, the terminal chat and Telegram, sees its new messages, and who's typing.
- **Where a message came from:** each message says where it was sent from, such as the web UI, the command line or Telegram, which the model is told.
- **Tracing:** with tracing on, the conversation shows the model's thinking, and the tools it calls, and their results. They're always kept.
- **Dictation:** the web UI's microphone, beside Send, dictates a message with the browser's speech recognition, in Chrome, Edge and Safari.

## Memory

### The memory window

A conversation is sent to its model within its memory window: the conversation's own setting, or the model's limit, when that's smaller.

- **How full it is:** the conversation's heading says, such as "Memory 38% full". From 80%, a warning icon shows.
- **Fitting:** when a reply doesn't fit, earlier tools' results are shortened, then removed, then earlier turns are dropped, for that request only.
- **Summarizing:** a conversation is summarized when it's 80% full, or when you choose **Summarize**. Its earlier messages are kept, and listed, but the model is sent a summary of them, which the built-in `compact` agent writes, with a title for the conversation.

### What every conversation knows

- **About the person:** their name, the assistant's name, their location, time zone and language, which the `memory` tools set.
- **Memories:** facts kept under keys, with versions, which the assistant searches, keeps and forgets.
- **Time:** each message is sent with its time, in the person's time zone.

## Tasks

A task is a prompt that runs once, or on a schedule, such as a reminder, or a morning briefing.

- **Scheduling:** ask the assistant, with the `heartbeat` tools, such as "remind me at 9 tomorrow to call Anna", or "every weekday at 7, tell me the weather". Schedules are cron expressions, in the person's time zone, and survive restarts.
- **Running:** a task runs in a session of its own, with the conversation's model and tools. The built-in `event` agent decides whether its outcome is worth telling you.
- **Delivery:** what's worth telling you is added to the conversation once it's idle, and sent to its channels, such as Telegram. An alert is delivered at once.
- **Seeing them:** the Conversation menu's **Tasks** lists the conversation's scheduled tasks, and what each said when it ran.
- **Triggers:** other systems can trigger a task with `POST /api/llm/event`, such as when a file changes.

## Files

Files you attach, and files tools make, such as speech and images, are the conversation's resources. See [Workspaces](workspaces.md).
