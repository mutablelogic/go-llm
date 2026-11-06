// The resources' schema, which mirrors the resource package's

// Resource is content, such as an image, a document or audio, which is kept
// once, by its hash, its SHA-256, and referred to as resource://<hash>, in a
// session, or on its own
export interface Resource {
    hash: string;
    size: number;
    name?: string;
    description?: string;
    type?: string;
    session?: string;
    created?: string;
    used?: string;
}

// ResourceList is a page of the resources, the most recently used first,
// and how many there are
export interface ResourceList {
    count: number;
    body?: Resource[];
}

// resourceURI returns the URI of the resource, by which a message's
// attachment, or a tool, refers to it
export function resourceURI(resource: Resource): string {
    return 'resource://' + resource.hash;
}

// resourceName returns the resource's name, or else its type, or else the
// start of its hash
export function resourceName(resource: Resource): string {
    return resource.name || resource.type || resource.hash.slice(0, 12);
}
