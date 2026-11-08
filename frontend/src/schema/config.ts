// Config mirrors the JSON of /config.json, which is the configuration of the
// web UI, from Config in frontend/cmd in Go

export interface Config {
    // The path prefix of the API, such as /api
    prefix: string;

    // The version of the server
    version?: string;

    // The path of the API's documentation, when the server serves it
    openapi?: string;

    // The server's public endpoint, such as https://llm.example.com/api,
    // when it was given its public name, such as that of a proxy it's
    // behind, with --tls.name
    endpoint?: string;
}
