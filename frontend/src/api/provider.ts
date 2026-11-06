// The providers' API

import { request } from './client';
import type { Provider, ProviderList, ProviderMeta } from '../schema/provider';

// listProviders returns the providers
export function listProviders(signal?: AbortSignal): Promise<ProviderList> {
    return request<ProviderList>('GET', '/llm/provider', { signal });
}

// createProvider creates a provider, which is checked to be reachable
export function createProvider(meta: ProviderMeta): Promise<Provider> {
    return request<Provider>('POST', '/llm/provider', { body: meta });
}

// updateProvider replaces the provider's URL, and patterns, and its API key,
// when it's set, which is otherwise kept
export function updateProvider(name: string, meta: ProviderMeta): Promise<Provider> {
    return request<Provider>('PATCH', '/llm/provider/' + encodeURIComponent(name), { body: meta });
}

// deleteProvider deletes the provider
export function deleteProvider(name: string): Promise<Provider> {
    return request<Provider>('DELETE', '/llm/provider/' + encodeURIComponent(name));
}
