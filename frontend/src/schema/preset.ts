import Api16 from '@carbon/icons/es/api/16.js';
import Microphone16 from '@carbon/icons/es/microphone/16.js';
import ChatBot16 from '@carbon/icons/es/chat-bot/16.js';
import iconLoader from '@carbon/web-components/es/globals/internal/icon-loader.js';

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
    // The file name of the provider's logo, among the web UI's provider
    // icons, at icon/provider/, when it's a brand's
    icon?: string;
}

export const presets: Preset[] = [
    { id: 'ollama', label: 'Ollama', type: 'ollama', name: 'ollama', url: 'http://localhost:11434', askURL: true, urlLabel: 'URL of the Ollama server', credential: 'optional', icon: 'ollama.svg' },
    { id: 'compatible', label: 'OpenAI-compatible', type: 'openai', name: '', url: 'http://localhost:1234/v1', askURL: true, urlLabel: 'URL of the server, such as LM Studio or vLLM', credential: 'optional' },
    { id: 'openai', label: 'OpenAI', type: 'openai', name: 'openai', url: '', askURL: false, credential: 'required', icon: 'openai.svg' },
    { id: 'mistral', label: 'Mistral', type: 'openai', name: 'mistral', url: 'https://api.mistral.ai/v1', askURL: false, credential: 'required', icon: 'mistral.svg' },
    { id: 'anthropic', label: 'Anthropic', type: 'anthropic', name: 'anthropic', url: '', askURL: false, credential: 'required', icon: 'anthropic.svg' },
    { id: 'gemini', label: 'Google Gemini', type: 'gemini', name: 'gemini', url: '', askURL: false, credential: 'required', icon: 'gemini.svg' },
    { id: 'deepseek', label: 'DeepSeek', type: 'openai', name: 'deepseek', url: 'https://api.deepseek.com', askURL: false, credential: 'required', icon: 'deepseek.svg' },
    { id: 'gowhisper', label: 'go-whisper', type: 'gowhisper', name: 'gowhisper', url: 'http://localhost:8081', askURL: true, urlLabel: 'URL of the go-whisper server', credential: 'none' },
    { id: 'elevenlabs', label: 'ElevenLabs', type: 'elevenlabs', name: 'elevenlabs', url: '', askURL: false, credential: 'required', icon: 'elevenlabs.svg' },
    { id: 'eliza', label: 'Eliza, for testing', type: 'eliza', name: 'eliza', url: '', askURL: false, credential: 'none' },
];

// fixedHosts are the hosts of the services whose URL isn't asked for, such
// as Mistral's, which the server may store with a path, such as /v1
const fixedHosts = ['api.openai.com', 'api.mistral.ai', 'api.deepseek.com', 'api.elevenlabs.io', 'generativelanguage.googleapis.com'];

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

// presetIcon returns the URL of the preset's logo, or undefined, when it
// has none, as it isn't a brand's, such as a server of your own
export function presetIcon(preset: Preset): string | undefined {
    return preset.icon ? 'icon/provider/' + encodeURIComponent(preset.icon) : undefined;
}

// providerIcon returns the logo of a provider, by its type, and, for the
// openai type, which other services, and servers of your own, have too, by
// the host of its URL, as a URL of the web UI's, or an icon of what it is,
// such as a microphone, for go-whisper, or an API, for a server of your own
export function providerIcon(provider: { provider: string; url?: string }): string | unknown {
    const logo = (name: string) => 'icon/provider/' + name + '.svg';
    switch (provider.provider) {
    case 'ollama':
    case 'anthropic':
    case 'gemini':
    case 'elevenlabs':
        return logo(provider.provider);
    case 'gowhisper':
        return iconLoader(Microphone16);
    case 'eliza':
        return iconLoader(ChatBot16);
    case 'openai': {
        let host = '';
        try {
            host = provider.url ? new URL(provider.url).hostname.toLowerCase() : '';
        } catch {
            // Not a URL, which is a server of your own
            host = '?';
        }
        if (host === '' || host === 'api.openai.com') {
            return logo('openai');
        } else if (host === 'mistral.ai' || host.endsWith('.mistral.ai')) {
            return logo('mistral');
        } else if (host === 'deepseek.com' || host.endsWith('.deepseek.com')) {
            return logo('deepseek');
        }
        return iconLoader(Api16);
    }
    }
    return iconLoader(Api16);
}
