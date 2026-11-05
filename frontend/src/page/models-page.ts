// The page of the models, of every provider, or of one, with an action
// which loads them again

import { customElement } from 'lit/decorators.js';
import iconLoader from '@carbon/web-components/es/globals/internal/icon-loader.js';
import MachineLearningModel32 from '@carbon/icons/es/machine-learning-model/32.js';
import { Page } from './page';
import { refreshAction } from './refresh';

@customElement('llm-models-page')
export class ModelsPage extends Page {
    heading = 'Models';
    description = 'The models of the providers, and what they can do';

    protected renderIcon(): unknown {
        return iconLoader(MachineLearningModel32);
    }

    protected renderActions(): unknown {
        return refreshAction(this);
    }
}

declare global {
    interface HTMLElementTagNameMap {
        'llm-models-page': ModelsPage;
    }
}
