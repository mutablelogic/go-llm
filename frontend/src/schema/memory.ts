// The memory's schema, of its details, which are what's known of the
// person, and the assistant

// Detail is a detail's value, or that the person declined to say it
export interface Detail {
    value?: string;
    declined?: boolean;
}

// Details are the details which are known, or declined
export interface Details {
    name?: Detail;
    assistant?: Detail;
    location?: Detail;
    timezone?: Detail;
    language?: Detail;
}

// detailValue returns the detail's value, when it's known, and not declined
export function detailValue(detail?: Detail): string | undefined {
    return detail && !detail.declined && detail.value ? detail.value : undefined;
}
