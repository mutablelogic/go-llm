// The providers' API

import { request } from './client';
import type { ProviderList } from '../schema/provider';

// listProviders returns the providers
export function listProviders(signal?: AbortSignal): Promise<ProviderList> {
    return request<ProviderList>('GET', '/llm/provider', { signal });
}
