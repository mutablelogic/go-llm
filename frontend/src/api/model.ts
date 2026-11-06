// The models' API

import { request, requestStream } from './client';
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

// LoadProgress is how a model's download is getting on: its status, and how
// complete it is, as a percentage, when it's known
export interface LoadProgress {
    status?: string;
    percent?: number;
}

// loadModel loads the model with the provider, which downloads it when it
// isn't stored, calling the function with its progress, as it's streamed,
// and returns the model, once it's loaded. It throws the error of an error
// event.
export async function loadModel(provider: string, name: string, onProgress: (progress: LoadProgress) => void, signal?: AbortSignal): Promise<Model> {
    let model: Model | undefined;
    let error: string | undefined;
    await requestStream('POST', '/llm/model', (event, data) => {
        switch (event) {
            case 'progress':
                onProgress(data as LoadProgress);
                break;
            case 'result':
                model = data as Model;
                break;
            case 'error':
                error = (data as { error?: string }).error;
                break;
        }
    }, { body: { provider, name }, signal });
    if (error !== undefined) {
        throw new Error(error);
    } else if (!model) {
        throw new Error('the download ended before it was complete');
    }
    return model;
}
