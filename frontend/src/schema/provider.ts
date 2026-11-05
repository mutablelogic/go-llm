// The providers' schema, which mirrors the registry's

// Provider is a provider of models, by its name in the registry, and its
// type, such as ollama
export interface Provider {
    name: string;
    provider: string;
    url?: string;
    serial?: number;

    // Why the provider can't be reached, when it can't
    error?: string;
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
