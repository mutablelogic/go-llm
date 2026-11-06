// The presets of providers which can be added, as the command-line tool's
// provider-create commands are: each one's type, its name, unless another is
// given, its URL, which is the type's default, when it's empty, whether its
// URL is asked for, as it's a server of its own, and whether the credential
// of its API key is needed, can be given, or isn't asked for

export interface Preset {
    id: string;
    label: string;
    type: string;
    name: string;
    url: string;
    askURL: boolean;
    urlLabel?: string;
    credential: 'required' | 'optional' | 'none';
}

export const presets: Preset[] = [
    { id: 'ollama', label: 'Ollama', type: 'ollama', name: 'ollama', url: 'http://localhost:11434', askURL: true, urlLabel: 'URL of the Ollama server', credential: 'optional' },
    { id: 'compatible', label: 'OpenAI-compatible', type: 'openai', name: '', url: 'http://localhost:1234/v1', askURL: true, urlLabel: 'URL of the server, such as LM Studio or vLLM', credential: 'optional' },
    { id: 'openai', label: 'OpenAI', type: 'openai', name: 'openai', url: '', askURL: false, credential: 'required' },
    { id: 'mistral', label: 'Mistral', type: 'openai', name: 'mistral', url: 'https://api.mistral.ai/v1', askURL: false, credential: 'required' },
    { id: 'deepseek', label: 'DeepSeek', type: 'openai', name: 'deepseek', url: 'https://api.deepseek.com', askURL: false, credential: 'required' },
    { id: 'gowhisper', label: 'go-whisper', type: 'gowhisper', name: 'gowhisper', url: 'http://localhost:8081', askURL: true, urlLabel: 'URL of the go-whisper server', credential: 'none' },
    { id: 'elevenlabs', label: 'ElevenLabs', type: 'elevenlabs', name: 'elevenlabs', url: '', askURL: false, credential: 'required' },
    { id: 'eliza', label: 'Eliza, for testing', type: 'eliza', name: 'eliza', url: '', askURL: false, credential: 'none' },
];

// fixedHosts are the hosts of the services whose URL isn't asked for, such
// as Mistral's, which the server may store with a path, such as /v1
const fixedHosts = ['api.openai.com', 'api.mistral.ai', 'api.deepseek.com', 'api.elevenlabs.io'];

// presetOf returns the preset with the identifier, or the first
export function presetOf(id?: string): Preset {
    return presets.find((preset) => preset.id === id) ?? presets[0];
}

// asksURL returns whether a provider of the type, with the URL, has a URL of
// its own, which is asked for, when it's edited, such as an Ollama server,
// rather than a service's, such as Mistral's
export function asksURL(type: string, url?: string): boolean {
    switch (type) {
    case 'ollama':
    case 'gowhisper':
        return true;
    case 'openai':
        return !!url && !fixedHosts.includes(hostOf(url));
    }
    return false;
}

// hostOf returns the URL's host, in lower case, or empty, when it isn't a URL
function hostOf(url: string): string {
    try {
        return new URL(url).hostname.toLowerCase();
    } catch {
        return '';
    }
}
