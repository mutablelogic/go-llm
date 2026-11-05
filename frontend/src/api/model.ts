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
