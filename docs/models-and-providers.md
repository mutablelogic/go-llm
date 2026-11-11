# Models and Providers

A provider is a service, or a server, whose models go-llm can use. Each has a name, unique in go-llm, so there can be more than one of a type, such as two Ollama servers.

## Providers

| Type | Provider | Generate | Embed | OCR | Transcribe | Translate | Speak | Images | Models |
|------|----------|:--------:|:-----:|:---:|:----------:|:---------:|:-----:|:------:|--------|
| `ollama` | [Ollama](https://ollama.com) | ✅ | ✅ | ✅ (images) | | | ✅ (phonon) | | Download, delete, unload |
| `lmstudio` | [LM Studio](https://lmstudio.ai) | ✅ | ✅ | ✅ (images, olmOCR) | | | | | Download, load, unload |
| `openai` | [OpenAI](https://platform.openai.com) | ✅ | ✅ | | ✅ | ✅ (whisper) | ✅ (tts) | | |
| `anthropic` | [Anthropic](https://www.anthropic.com) | ✅ | | | | | | | |
| `gemini` | [Google Gemini](https://ai.google.dev) | ✅ | ✅ | | ✅ | | ✅ (tts) | ✅ (Nano Banana) | |
| `openai` | [Mistral](https://mistral.ai) | ✅ | ✅ | ✅ | ✅ | | | | |
| `openai` | [DeepSeek](https://www.deepseek.com), and other OpenAI-compatible servers | ✅ | | | | | | | |
| `gowhisper` | [go-whisper](https://github.com/mutablelogic/go-whisper) | | | | ✅ | ✅ | | | Download, delete |
| `elevenlabs` | [ElevenLabs](https://elevenlabs.io) | | | | ✅ | | ✅ | | |
| `eliza` | A mock chatbot, for testing | ✅ | | | | | | | |

- **Adding them:** **Add provider** on the Models page, or `llm provider-create-<type>`. See [Getting Started](getting-started.md).
- **OpenAI's speech models** shut down on 6 January 2027 ([#286](https://github.com/mutablelogic/go-llm/issues/286)).

## Models

- **By name:** a model is referred to by its name, such as `llama3.2:latest`, or by its provider and name, such as `ollama:llama3.2:latest`, when more than one provider has it.
- **Its details:** the Models page shows each model's capabilities, and its input and output token limits, when they're known.
- **Local models:** load one, downloading it when it isn't stored, with **Load model**, or `llm model-load`. Unload it with `llm model-unload`, and delete it with `llm model-delete`.

## Capabilities

What a model can do, as its provider reports it. A capability which isn't reported isn't assumed, and a request for something a model is known not to do is rejected before it's sent.

| Capability | Meaning |
|------------|---------|
| `chat` | Replies to messages |
| `tools` | Calls tools |
| `thinking` | Reasons before replying |
| `image_input`, `audio_input`, `video_input`, `document_input` | Takes images, audio, video or documents in a conversation |
| `image_output` | Makes images |
| `speech_output` | Speaks text |
| `embedding` | Embeds text and media |
| `ocr` | Reads the text in images and documents |
| `transcription`, `translation` | Transcribes speech, or translates it into English |

```bash
llm models --capability image_input,tools
llm models --capability speech_output
```

## Voices

A model which speaks has voices, chosen by their ID, or their name, such as `Matilda`, ignoring case.

- **Listing them:** `llm model-voices <model>`, with `--voice` to find one by part of its name, a page at a time, or `GET /api/llm/model/{name}/voice`.
- **What's known of each:** its title, description, gender, age, accent, the languages it's verified to speak, and whether it's recommended for the model, when the provider says.
- **Which:** OpenAI's and Gemini's fixed voices, the voices Ollama shows for a speech model, such as phonon, and your ElevenLabs account's.

```bash
llm model-voices elevenlabs:eleven_v4_turbo --voice matilda
llm speak --model elevenlabs:eleven_v4_turbo --voice Matilda -o hello.mp3 "Hello"
```
