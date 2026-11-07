// The resources' schema, which mirrors the resource package's

// Resource is content, such as an image, a document or audio, which is kept
// once, by its hash, its SHA-256, and referred to as resource:///<hash>, in a
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
    return 'resource:///' + resource.hash;
}

// resourceHash returns the hash of a resource's URI, resource:///<hash>, or
// resource://<hash>, as it was before, or undefined, when it isn't one
export function resourceHash(uri: string): string | undefined {
    const match = uri.trim().match(/^resource:\/\/\/?([0-9a-fA-F]{64})$/);
    return match ? match[1].toLowerCase() : undefined;
}

// resourceTitle returns what the resource is called, for people: its
// description, which is its title, such as Ode to David, when it has one,
// on one line, or else its name
export function resourceTitle(resource: Resource): string {
    const title = resource.description?.split('\n')[0].trim();
    return title || resourceName(resource);
}

// resourceName returns the resource's name, or else its type, or else the
// start of its hash
export function resourceName(resource: Resource): string {
    return resource.name || resource.type || resource.hash.slice(0, 12);
}
