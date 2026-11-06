// The providers' schema, which mirrors the registry's

// Provider is a provider of models, by its name in the registry, and its
// type, such as ollama
export interface Provider {
    name: string;
    provider: string;
    url?: string;
    include?: string[];
    exclude?: string[];
    serial?: number;

    // Whether it stores models, which can be downloaded, deleted, and
    // unloaded, such as Ollama
    downloader?: boolean;

    // Why the provider can't be reached, when it can't
    error?: string;
}

// ProviderMeta is what's set, to create or update a provider: its name and
// type, which aren't updated, its URL, the patterns of the names of its models
// which are included, and excluded, and its API key, which is a credential's
// placeholder, such as ${MISTRAL_API_KEY}, which is read from the server's
// environment, so the key isn't stored, and is kept, when it isn't set, as
// it isn't returned
export interface ProviderMeta {
    name?: string;
    provider?: string;
    url?: string;
    include?: string[];
    exclude?: string[];
    api_key?: string;
}

// ProviderList is the providers, and how many there are
export interface ProviderList {
    count: number;
    body?: Provider[];
}

// ProviderCount is a provider, and how many models it has, which is
// undefined when they can't be counted, such as when it can't be reached
export interface ProviderCount extends Provider {
    models?: number;
}
