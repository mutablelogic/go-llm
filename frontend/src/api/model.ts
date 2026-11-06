// The models' API

import { request } from './client';
import type { Model, ModelList } from '../schema/model';

// listModels returns the models, of the provider, or every one, which the
// API returns all of, rather than a page
export function listModels(provider?: string, signal?: AbortSignal): Promise<ModelList> {
    return request<ModelList>('GET', '/llm/model', { query: { provider }, signal });
}

// getModel returns the model with the name, or its canonical name
export function getModel(name: string, signal?: AbortSignal): Promise<Model> {
    return request<Model>('GET', '/llm/model/' + encodeURIComponent(name), { signal });
}

// deleteModel deletes the model, by its canonical name, from its provider's
// storage, with a provider which stores models, such as Ollama
export function deleteModel(name: string): Promise<Model> {
    return request<Model>('DELETE', '/llm/model/' + encodeURIComponent(name));
}

// unloadModel unloads the model, by its canonical name, from its provider's
// memory, keeping it stored, so it's loaded again when it's next used
export function unloadModel(name: string): Promise<Model> {
    return request<Model>('POST', '/llm/model/' + encodeURIComponent(name) + '/unload');
}
