import { action } from '@storybook/addon-actions'
import classNames from 'classnames'
import { flow, startCase } from 'lodash'
import SearchIcon from 'mdi-react/SearchIcon'
import React from 'react'
import 'storybook-addon-designs'

import { preventDefault } from './utils'

type VariantType = 'btn' | 'btn-outline'

const variants: Record<VariantType, string[]> = {
    btn: ['primary', 'secondary', 'success', 'danger', 'warning', 'info'],
    'btn-outline': ['primary', 'secondary', 'danger'],
}

interface ButtonVariantsProps {
    variantType?: VariantType
}

export const ButtonVariants: React.FunctionComponent<ButtonVariantsProps> = ({ variantType = 'btn' }) => (
    <div
        // eslint-disable-next-line react/forbid-dom-props
        style={{
            display: 'grid',
            gridTemplateColumns: 'repeat(4, max-content)',
            gridAutoRows: 'max-content',
            gridGap: '1rem',
            marginBottom: '1rem',
        }}
    >
        {variants[variantType].map(variant => (
            <React.Fragment key={variant}>
                <button
                    type="button"
                    key={variant}
                    className={classNames('btn', `${variantType}-${variant}`)}
                    onClick={flow(preventDefault, action('button clicked'))}
                >
                    {startCase(variant)}
                </button>
                <button
                    type="button"
                    key={`${variantType} - ${variant} - focus`}
                    className={classNames('btn', `${variantType}-${variant}`, 'focus')}
                >
                    Focus
                </button>
                <button
                    type="button"
                    key={`${variantType} - ${variant} - disabled`}
                    className={classNames('btn', `${variantType}-${variant}`)}
                    disabled={true}
                >
                    Disabled
                </button>
                <button
                    type="button"
                    key={`${variantType} - ${variant} - icon`}
                    className={classNames('btn', `${variantType}-${variant}`)}
                >
                    <SearchIcon className="icon-inline mr-1" />
                    With icon
                </button>
            </React.Fragment>
        ))}
    </div>
)
