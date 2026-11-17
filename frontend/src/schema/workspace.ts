// The types of the workspaces' API

// Workspace is a set of folders and resources, such as an MCP connector's,
// the memory, or a folder of files, with what it does: read, when its
// resources' content is read, and watch, when its changes are followed
export interface Workspace {
    name: string;
    title?: string;
    description?: string;
    version?: string;
    read?: boolean;
    watch?: boolean;
    tree?: boolean;
}

// WorkspaceList is the workspaces, in order of their names
export interface WorkspaceList {
    count: number;
    body?: Workspace[];
}

// ResourceMeta is a folder, or a resource, of a workspace, as it's listed
export interface ResourceMeta {
    uri: string;
    path?: string;
    name: string;
    title?: string;
    description?: string;
    type?: string;
    size?: number;
    modified?: string;
    meta?: ResourceMetaValue[];
    artwork?: Artwork[];
}

// ResourceMetaValue is a value of a resource's metadata, by its namespace
// and key, such as exif:DateTimeOriginal, as text, and as JSON
export interface ResourceMetaValue {
    ns: string;
    key: string;
    text: string;
    value?: unknown;
}

// Artwork is an image of a resource, such as its cover, or thumbnail
export interface Artwork {
    key: string;
    type?: string;
    uri: string;
}

// ResourceList is a page of a folder's folders, and resources, and the
// cursor of the next page, when there is one
export interface ResourceList {
    body?: ResourceMeta[];
    next?: string;
}

// FolderType is the type of a folder
export const FolderType = 'inode/directory';

// isFolder returns true when the resource is a folder
export function isFolder(resource: ResourceMeta): boolean {
    return resource.type === FolderType;
}
