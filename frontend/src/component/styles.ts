// Styles which are shared by components

import { css } from 'lit';

// tableStyles are the styles of a table, whose rows are clicked, rather than
// their text selected. A row's name is a button, which looks like its text,
// so the row can be opened with the keyboard, as its click reaches the row.
export const tableStyles = css`
    cds-table {
        user-select: none;
    }

    cds-table-row {
        cursor: pointer;
    }

    button.open {
        all: unset;
        cursor: pointer;
    }

    button.open:focus-visible {
        outline: 2px solid var(--cds-focus, #0f62fe);
        outline-offset: 2px;
    }

    button.open:disabled {
        cursor: default;
    }
`;
