# Options

This content needs reviewed and updated.

## Complete and Chat Options

These are the options you can use with the `Completion` and `Chat` methods.

<table>
<tr>
  <th>Ollama</th>
  <th>Anthropic</th>
  <th>Mistral</th>
  <th>OpenAI</th>
  <th>Gemini</th>
</tr>

<tr><td colspan="6">
  <code>llm.WithTemperature(float64)</code>
  What sampling temperature to use, between 0.0 and 1.0. Higher values like 0.7 will make the output more random, while lower values like 0.2 will make it more focused and deterministic.
</td></tr>
<tr style="border-bottom: 2px solid black;">
  <td>Yes</td>
  <td>Yes</td>
  <td>Yes</td>
  <td>Yes</td>
  <td>Yes</td>
</tr>

</table>

## Embedding Options

These are the options you can include for the `Embedding`method.

<table>
<tr>
  <th>Ollama</th>
  <th>Anthropic</th>
  <th>Mistral</th>
  <th>OpenAI</th>
  <th>Gemini</th>
</tr>

<tr><td colspan="6">
  <code>ollama.WithKeepAlive(time.Duration)</code>
  Controls how long the model will stay loaded into memory following the request
</td></tr>
<tr style="border-bottom: 2px solid black;">
  <td>Yes</td>
  <td>No</td>
  <td>No</td>
  <td>No</td>
  <td>No</td>
</tr>

<tr><td colspan="6">
  <code>ollama.WithTruncate()</code>
  Does not truncate the end of each input to fit within context length. Returns error if context length is exceeded.
</td></tr>
<tr style="border-bottom: 2px solid black;">
  <td>Yes</td>
  <td>No</td>
  <td>No</td>
  <td>No</td>
  <td>No</td>
</tr>

<tr><td colspan="6">
  <code>ollama.WithOption(string, any)</code>
  Set model-specific option value.
</td></tr>
<tr style="border-bottom: 2px solid black;">
  <td>Yes</td>
  <td>No</td>
  <td>No</td>
  <td>No</td>
  <td>No</td>
</tr>

<tr><td colspan="6">
  <code>openai.WithDimensions(uint64)</code>
  The number of dimensions the resulting output embeddings
  should have. Only supported in text-embedding-3 and later models.
</td></tr>
<tr style="border-bottom: 2px solid black;">
  <td>No</td>
  <td>No</td>
  <td>No</td>
  <td>Yes</td>
  <td>No</td>
</tr>

<tr><td colspan="6">
  <code>llm.WithFormat(string)</code>
  The format to return the embeddings in. Can be either .
</td></tr>
<tr style="border-bottom: 2px solid black;">
  <td>No</td>
  <td>No</td>
  <td>'float'</td>
  <td>'float' or 'base64'</td>
  <td>No</td>
</tr>

</table>

## Older Content

You can add options to sessions, or to prompts. Different providers and models support
different options.

```go
package llm 

type Model interface {
  // Set session-wide options
  Context(...Opt) Context

  // Create a completion from a text prompt
  Completion(context.Context, string, ...Opt) (Completion, error)

  // Embedding vector generation
  Embedding(context.Context, string, ...Opt) ([]float64, error)
}

type Context interface {
  // Generate a response from a user prompt (with attachments and
  // other options)
  FromUser(context.Context, string, ...Opt) error
}
```

The options are as follows:

| Option | Ollama | Anthropic | Mistral | OpenAI | Description |
|--------|--------|-----------|---------|--------|-------------|
| `llm.WithTemperature(float64)` | Yes | Yes | Yes | Yes | What sampling temperature to use, between 0.0 and 1.0. Higher values like 0.7 will make the output more random, while lower values like 0.2 will make it more focused and deterministic. |
| `llm.WithTopP(float64)` | Yes | Yes | Yes | Yes | Nucleus sampling, where the model considers the results of the tokens with top_p probability mass. So 0.1 means only the tokens comprising the top 10% probability mass are considered. |
| `llm.WithTopK(uint64)` | Yes | Yes | No | No | Reduces the probability of generating nonsense. A higher value (e.g. 100) will give more diverse answers, while a lower value (e.g. 10) will be more conservative. |
| `llm.WithMaxTokens(uint64)` | No | Yes | Yes | Yes | The maximum number of tokens to generate in the response. |
| `llm.WithStream(func(llm.Completion))` | Can be enabled when tools are not used | Yes | Yes | Yes | Stream the response to a function. |
| `llm.WithToolChoice(string, string, ...)` | No | Use `auto`, `any` or a function name. Only the first argument is used. | Use `auto`, `any`, `none`, `required` or a function name. Only the first argument is used. | Use `auto`, `none`, `required` or a function name. Only the first argument is used. | The tool to use for the model. |
| `llm.WithToolKit(llm.ToolKit)` | Cannot be combined with streaming | Yes | Yes | Yes | The set of tools to use. |
| `llm.WithStopSequence(string, string, ...)` | Yes | Yes | Yes | Yes | Stop generation if one of these tokens is detected. |
| `llm.WithSystemPrompt(string)` | No | Yes | Yes | Yes | Set the system prompt for the model. |
| `llm.WithSeed(uint64)` | Yes | No | Yes | Yes | The seed to use for random sampling. If set, different calls will generate deterministic results. |
| `llm.WithFormat(string)` | Use `json` | No | Use `json_format` or `text` | Use `json_format` or `text` | The format of the response. For Mistral, you must also instruct the model to produce JSON yourself with a system or a user message. |
| `llm.WithPresencePenalty(float64)` | Yes | No | Yes | Yes | Determines how much the model penalizes the repetition of words or phrases. A higher presence penalty encourages the model to use a wider variety of words and phrases, making the output more diverse and creative. |
| `llm.WithFequencyPenalty(float64)` | Yes | No | Yes | Yes | Penalizes the repetition of words based on their frequency in the generated text. A higher frequency penalty discourages the model from repeating words that have already appeared frequently in the output, promoting diversity and reducing repetition. |
| `llm.WithPrediction(string)` | No | No | Yes | Yes | Enable users to specify expected results, optimizing response times by leveraging known or predictable content. This approach is especially effective for updating text documents or code files with minimal changes, reducing latency while maintaining high-quality results. |
| `llm.WithSafePrompt()` | No | No | Yes | No | Whether to inject a safety prompt before all conversations. |
| `llm.WithNumCompletions(uint64)` | No | No | Yes | Yes | Number of completions to return for each request. |
| `llm.WithAttachment(io.Reader)` | Yes | Yes | Yes | - | Attach a file to a user prompt. It is the responsibility of the caller to close the reader. |
| `llm.WithUser(string)` | No | Yes | No | Yes | A unique identifier representing your end-user |
| `antropic.WithEphemeral()` | No | Yes | No | - | Attachments should be cached server-side |
| `antropic.WithCitations()` | No | Yes | No | - | Attachments should be used in citations |
| `openai.WithStore(bool)` | No | No | No | Yes | Whether or not to store the output of this chat completion request |
| `openai.WithDimensions(uint64)` | No | No | No | Yes | The number of dimensions the resulting output embeddings should have. Only supported in text-embedding-3 and later models |
| `openai.WithReasoningEffort(string)` | No | No | No | Yes | The level of effort model should put into reasoning. |
| `openai.WithMetadata(string, string)` | No | No | No | Yes | Metadata to be logged with the completion. |
| `openai.WithLogitBias(uint64, int64)` | No | No | No | Yes | A token and their logit bias value. Call multiple times to add additional tokens |
| `openai.WithLogProbs()` | No | No | No | Yes | Include the log probabilities on the completion. |
| `openai.WithLogProbs()` | No | No | No | Yes | Include the log probabilities on the completion. |
| `openai.WithTopLogProbs(uint64)` | No | No | No | Yes | An integer between 0 and 20 specifying the number of most likely tokens to return at each token position. |
| `openai.WithAudio(string, string)` | No | No | No | Yes | Output audio (voice, format) for the completion. Can be used with certain models. |
| `openai.WithServiceTier(string)` | No | No | No | Yes | Specifies the latency tier to use for processing the request. |
| `openai.WithStreamOptions(func(llm.Completion), bool)` | No | No | No | Yes | Include usage information in the stream response |
| `openai.WithDisableParallelToolCalls()` | No | No | No | Yes | Call tools in serial, rather than in parallel |
