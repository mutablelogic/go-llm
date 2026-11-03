# LLM Toolkit

## Tool Operations

Tools are functions which models can call, such as getting the weather. Each tool is in a namespace, such as `weatherapi`, and its canonical name is the namespace and the tool's name separated by a colon, such as `weatherapi:current`. The server's tools are set when it starts, so they can't be added or removed through the API.

### GET /llm/tool

List the tools, in order of canonical name, with each tool's description, and the JSON schemas of its input and output. The `namespace` parameter returns only the tools in that namespace, such as `weatherapi`. `count` is the number of tools. Returns 400 if a parameter is invalid, such as a `namespace` which isn't an identifier.

### GET /llm/tool/{name}

Get a tool by name, with its description, and the JSON schemas of its input and output. The name is the tool's canonical name, such as `weatherapi:current`, or its name without a namespace, such as `current`, when only one namespace has a tool with the name. Returns 404 if there is no tool with the name, or 409 if the name has no namespace and more than one namespace has a tool with the name.

### POST /llm/tool/{name}

Run the tool with the name, and return its result. The name is the tool's canonical name, such as `weatherapi:current`, or its name without a namespace, such as `current`, when only one namespace has a tool with the name. The request body has the tool's `input`, which must match the tool's input schema, and which can be left out for a tool without input. Returns the tool's result as JSON, or 204 with no content when the tool has no result. Text is returned as a JSON string, and a resource, such as an image, is returned with its `type` and base64-encoded `data`. Returns 400 if the request body is invalid, or the input doesn't match the tool's input schema, 404 if there is no tool with the name, 409 if the name has no namespace and more than one namespace has a tool with the name, or 500 if the tool's result isn't valid JSON. An error from the tool itself, such as an API it calls not being reachable, is returned with its status.
