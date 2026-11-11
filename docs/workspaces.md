# Workspaces

{% hint style="info" %}
The Workspace is planned for go-llm 1.7 ([#246](https://github.com/mutablelogic/go-llm/issues/246)). Today, a conversation's files are its resources, which this page describes.
{% endhint %}

## Resources

A resource is a file go-llm keeps, such as an image, a document or audio.

- **Kept once,** by its SHA-256 hash, and referred to as `resource:///<hash>`.
- **A conversation's:** files you attach, or drop into the web UI, or send on Telegram, and files tools make, such as speech, images and transcripts.
- **Streamed** to and from the store, and downloadable in ranges, so long audio and video play as they load.
- **The Files panel** of a conversation lists its resources, plays audio and video, and attaches them to the next message.

```bash
llm resource-create report.pdf
llm ask -a resource:///<hash> "What does this say?"
llm resources
```

## Reading files

The `llm:resource` tool lists a conversation's files, and reads them as Markdown, a part at a time, carrying on where the last part ended. It reads text, HTML, CSV, PDFs, Word, OpenDocument, EPUB, spreadsheets, emails, calendars, subtitles and more, with [go-metadata](https://github.com/mutablelogic/go-metadata).

## Coming in the Workspace

- **Spaces:** a conversation's resources, connectors' MCP resources ([#292](https://github.com/mutablelogic/go-llm/issues/292)), and folders from your own filesystem.
- **Memory and help pages,** as spaces too.
- **Browsing and searching** across them, in the web UI and by the assistant.
