# Examples

These use go-llm's packages without the server. Each adds a provider to a registry, which is Ollama, when `OLLAMA_ADDR` is set, such as `localhost:11434`, OpenAI, when `OPENAI_API_KEY` is, or otherwise Eliza, which needs neither, and whose model is set with `-model`.

| Example | | Run |
|---------|-|-----|
| [ask](ask) | Ask a model a question, with a system prompt, and stream its reply, with the tokens used, and how long it took | `go run ./examples/ask "Explain a KV cache"` |
| [tools](tools) | A tool of your own, which rolls dice, in a namespace of a toolkit, which the registry runs as the model calls it, until it replies, streaming what the tool logs. Its model must be able to call tools, which Eliza can't | `go run ./examples/tools "Roll two dice"` |
| [chat](chat) | Chat in the terminal, with the conversation kept in memory, and sent to the model each time, shortened to fit its context, without a store of sessions | `go run ./examples/chat` |
| [agent](agent) | Run the example agent which translates text, from the agents folder, with a store of agents in memory. It replies with JSON, the translation, and the language of the text, as its schema says, which Eliza can't | `go run ./examples/agent "Guten Morgen"` |
| [ocr](ocr) | Read the text in an image, or a PDF, as Markdown, page by page. It needs Mistral, with `MISTRAL_API_KEY`, or Ollama, with an OCR model, such as `deepseek-ocr` | `go run ./examples/ocr scan.png` |
| [transcribe](transcribe) | Transcribe the speech in audio, streamed as it's transcribed, with `-segments` for when each was spoken, `-speakers` for who spoke it, or `-translate` into English. It needs OpenAI, Mistral, go-whisper, with `GOWHISPER_ADDR`, or ElevenLabs, with `ELEVENLABS_API_KEY` | `go run ./examples/transcribe -segments talk.mp3` |

The ocr and transcribe examples choose their providers differently, since Eliza can do neither, as their comments say. The agent example uses the in-memory store of agents in `registry/test`, which a system would replace with its own, such as a database.
