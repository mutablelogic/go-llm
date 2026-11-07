// The models' schema, which mirrors the registry's

// ModelCapabilities are what a model can do, as its provider says
export interface ModelCapabilities {
    completion?: boolean;
    tools?: boolean;
    thinking?: boolean;
    vision?: boolean;
    ocr?: boolean;
    transcription?: boolean;
    translation?: boolean;
    embedding?: boolean;
    speech?: boolean;
    image?: boolean;
    // Whether it takes a temperature, which is a setting, rather than what
    // it does, so it isn't one of the capabilityNames
    temperature?: boolean;
}

// Model is a model of a provider: its name, its provider's name, and its
// canonical name, which is both, such as ollama:llama3.2:latest
export interface Model {
    name: string;
    provider: string;
    canonical: string;
    description?: string;
    created?: string;
    aliases?: string[];
    meta?: Record<string, unknown>;
    input_token_limit?: number;
    output_token_limit?: number;
    capabilities?: ModelCapabilities;
}

// ModelList is a page of the models, and how many there are
export interface ModelList {
    count: number;
    body?: Model[];
}

// capabilityNames are the names of what a model can do, in the order
// they're shown
export const capabilityNames: (keyof ModelCapabilities)[] = ['completion', 'tools', 'thinking', 'vision', 'ocr', 'transcription', 'translation', 'embedding', 'speech', 'image'];

// modelCapabilities returns the names of what the model is known to be able
// to do, such as tools and vision, in the order they're shown
export function modelCapabilities(model: Model): string[] {
    return capabilityNames.filter((name) => model.capabilities?.[name] === true);
}

// cannot returns whether the model is known not to be able to do something,
// which isn't so when it isn't known, as its provider doesn't say
export function cannot(model: Model | undefined, capability: keyof ModelCapabilities): boolean {
    return model?.capabilities?.[capability] === false;
}
