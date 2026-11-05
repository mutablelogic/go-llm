// The memory's API

import { request } from './client';
import type { Details } from '../schema/memory';

// getDetails returns the memory's details, such as the names of the person,
// and the assistant
export function getDetails(signal?: AbortSignal): Promise<Details> {
    return request<Details>('GET', '/memory/detail', { signal });
}
