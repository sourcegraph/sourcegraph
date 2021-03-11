import React from 'react'

export interface BatchChangeCloseHeaderProps {
    // Nothing.
}

const BatchChangeCloseHeader: React.FunctionComponent<BatchChangeCloseHeaderProps> = () => (
    <>
        <span className="d-none d-md-block" />
        <h5 className="d-none d-md-block text-uppercase text-center text-nowrap">Action</h5>
        <h5 className="d-none d-md-block text-uppercase text-nowrap">Changeset information</h5>
        <h5 className="d-none d-md-block text-uppercase text-center text-nowrap">Check state</h5>
        <h5 className="d-none d-md-block text-uppercase text-center text-nowrap">Review state</h5>
        <h5 className="d-none d-md-block text-uppercase text-center text-nowrap">Changes</h5>
    </>
)

export const BatchChangeCloseHeaderWillCloseChangesets: React.FunctionComponent<BatchChangeCloseHeaderProps> = () => (
    <>
        <h2 className="batch-change-close-header__row test-batches-close-willclose-header">
            Closing the batch change will close the following changesets:
        </h2>
        <BatchChangeCloseHeader />
    </>
)

export const BatchChangeCloseHeaderWillKeepChangesets: React.FunctionComponent<BatchChangeCloseHeaderProps> = () => (
    <>
        <h2 className="batch-change-close-header__row">The following changesets will remain open:</h2>
        <BatchChangeCloseHeader />
    </>
)
