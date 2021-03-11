import React from 'react'

export interface BatchChangesListEmptyProps {
    // Nothing for now.
}

export const BatchChangesListEmpty: React.FunctionComponent<BatchChangesListEmptyProps> = () => (
    <div className="web-content">
        <h2 className="mb-4">Get started with batch changes</h2>
        <h3 className="mb-3">Tutorials to help with your first batch change</h3>
        <div className="row">
            <div className="col-12 col-md-6 mb-2">
                <div className="card h-100 p-2">
                    <div className="card-body d-flex">
                        <FindReplaceIcon className="mr-3" />
                        <div>
                            <h4>
                                <a
                                    href="https://docs.sourcegraph.com/campaigns/tutorials/search_and_replace_specific_terms"
                                    rel="noopener"
                                >
                                    Finding and replacing code and text
                                </a>
                            </h4>
                            <p className="text-muted mb-0">
                                A Sourcegraph query plus a simple <code>sed</code> command creates changesets required
                                to manage a large scale change.
                            </p>
                        </div>
                    </div>
                </div>
            </div>
            <div className="col-12 col-md-6 mb-2">
                <div className="card h-100 p-2">
                    <div className="card-body d-flex">
                        <RefactorCombyIcon className="mr-3" />
                        <div>
                            <h4>
                                <a
                                    href="https://docs.sourcegraph.com/campaigns/tutorials/updating_go_import_statements"
                                    rel="noopener"
                                >
                                    Refactoring with language aware search
                                </a>
                            </h4>
                            <p className="text-muted mb-0">
                                Using{' '}
                                <a href="https://comby.dev/" rel="noopener">
                                    Comby's
                                </a>{' '}
                                language-aware structural search to refactor Go statements to a semantically equivalent,
                                but clearer execution.
                            </p>
                        </div>
                    </div>
                </div>
            </div>
            <div className="col-12 mb-4">
                <p>
                    <a href="https://docs.sourcegraph.com/campaigns/tutorials" rel="noopener">
                        More tutorials
                    </a>
                </p>
            </div>
        </div>

        <div className="row mb-4">
            <div className="col-12 col-md-4 mb-3">
                <p>
                    <strong>Quickstart</strong>
                </p>
                <p>Create your first Sourcegraph batch change in 10 minutes or less.</p>
                <a href="https://docs.sourcegraph.com/campaigns/quickstart" rel="noopener">
                    Batch changes quickstart
                </a>
            </div>
            <div className="col-12 col-md-4 mb-3">
                <p>
                    <strong>Introduction</strong>
                </p>
                <p>Learn how batch changes enables large-scale code changes across many repositories and code hosts.</p>
                <a href="https://docs.sourcegraph.com/campaigns/explanations/introduction_to_campaigns" rel="noopener">
                    Introduction to batch changes
                </a>
            </div>
            <div className="col-12 col-md-4 mb-3">
                <p>
                    <strong>Documentation</strong>
                </p>
                <p>
                    Take a look at the{' '}
                    <a
                        href="https://docs.sourcegraph.com/campaigns/references/campaign_spec_yaml_reference"
                        rel="noopener"
                    >
                        batch spec YAML reference
                    </a>
                    , learn about its powerful{' '}
                    <a href="https://docs.sourcegraph.com/campaigns/references/campaign_spec_templating">
                        templating language
                    </a>
                    ,{' '}
                    <a
                        href="https://docs.sourcegraph.com/campaigns/explanations/permissions_in_campaigns"
                        rel="noopener"
                    >
                        permissions
                    </a>{' '}
                    and more in the{' '}
                    <a href="https://docs.sourcegraph.com/campaigns" rel="noopener">
                        Batch Changes documentation
                    </a>
                    .
                </p>
            </div>
        </div>
        <h2>Batch changes demo</h2>
        <p className="text-muted">
            This demo shows how to refactor code and manage changesets across many repositories and multiple code hosts.
        </p>
        <div className="text-center">
            <iframe
                className="percy-hide chromatic-ignore"
                width="100%"
                height="600"
                src="https://www.youtube.com/embed/EfKwKFzOs3E"
                frameBorder="0"
                allow="accelerometer; autoplay; encrypted-media; gyroscope; picture-in-picture"
                allowFullScreen={true}
            />
        </div>
    </div>
)

const FindReplaceIcon: React.FunctionComponent<{ className?: string }> = ({ className }) => (
    <svg
        width="180"
        height="78"
        viewBox="0 0 134 58"
        fill="none"
        className={className}
        xmlns="http://www.w3.org/2000/svg"
    >
        <rect x="10.5" y="0.569092" width="54.9031" height="8" rx="2.07182" fill="#CAD2E2" />
        <rect x="70.5827" y="0.569092" width="21.2361" height="8" rx="2.07182" fill="#CAD2E2" />
        <rect x="96.9984" y="0.569092" width="6.21545" height="8" rx="2.07182" fill="#CAD2E2" />
        <rect x="30.5" y="12.7127" width="71" height="8" rx="2.07182" fill="#CAD2E2" />
        <rect x="30.5" y="24.8564" width="38.8466" height="8" rx="2.07182" fill="#CAD2E2" />
        <rect x="74.5261" y="24.8564" width="38.8466" height="8" rx="2.07182" fill="#F96216" />
        <rect x="30.5" y="37" width="54" height="8" rx="2.07182" fill="#CAD2E2" />
        <rect x="10.5" y="49.1436" width="6.21545" height="8" rx="2.07182" fill="#CAD2E2" />
        <g filter="url(#filter0_d)">
            <rect x="80.6899" y="15.7872" width="44.3626" height="10" rx="2.07182" fill="#00B4F2" />
        </g>
        <defs>
            <filter
                id="filter0_d"
                x="78.6181"
                y="15.7872"
                width="48.5062"
                height="14.1436"
                filterUnits="userSpaceOnUse"
                colorInterpolationFilters="sRGB"
            >
                <feFlood floodOpacity="0" result="BackgroundImageFix" />
                <feColorMatrix in="SourceAlpha" type="matrix" values="0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 127 0" />
                <feOffset dy="2.07182" />
                <feGaussianBlur stdDeviation="1.03591" />
                <feColorMatrix type="matrix" values="0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0.25 0" />
                <feBlend mode="normal" in2="BackgroundImageFix" result="effect1_dropShadow" />
                <feBlend mode="normal" in="SourceGraphic" in2="effect1_dropShadow" result="shape" />
            </filter>
        </defs>
    </svg>
)

const RefactorCombyIcon: React.FunctionComponent<{ className?: string }> = ({ className }) => (
    <svg
        width="180"
        height="78"
        viewBox="0 0 133 58"
        fill="none"
        className={className}
        xmlns="http://www.w3.org/2000/svg"
    >
        <rect x="15.5" y="0.788292" width="53.935" height="8" rx="2.03528" fill="#CAD2E2" />
        <rect x="74.5233" y="0.788292" width="31.5469" height="8" rx="2.03528" fill="#CAD2E2" />
        <rect x="111.158" y="0.788292" width="6.10585" height="8" rx="2.03528" fill="#CAD2E2" />
        <rect x="25.4979" y="12.8589" width="14.4969" height="8" rx="2.03528" fill="#F96216" />
        <rect x="44.9938" y="12.8589" width="38.1616" height="8" rx="2.03528" fill="#F96216" />
        <rect x="88.1543" y="12.8589" width="21.4955" height="8" rx="2.03528" fill="#00B4F2" />
        <rect x="114.649" y="12.8589" width="6.10585" height="8" rx="2.03528" fill="#F96216" />
        <rect x="25.4979" y="24.9294" width="38.1616" height="8" rx="2.03528" fill="#94D82D" />
        <rect x="68.7476" y="24.9294" width="21.5" height="8" rx="2.03528" fill="#00B4F2" />
        <rect x="95.3359" y="24.9294" width="6.10585" height="8" rx="2.03528" fill="#94D82D" />
        <g filter="url(#filter0_d)">
            <rect x="25.4979" y="37" width="25" height="8" rx="2.03528" fill="#CAD2E2" />
        </g>
        <rect x="15.5" y="49.0706" width="6.10585" height="8" rx="2.03528" fill="#CAD2E2" />
        <defs>
            <filter
                id="filter0_d"
                x="23.4626"
                y="37"
                width="29.0706"
                height="12.0706"
                filterUnits="userSpaceOnUse"
                colorInterpolationFilters="sRGB"
            >
                <feFlood floodOpacity="0" result="BackgroundImageFix" />
                <feColorMatrix in="SourceAlpha" type="matrix" values="0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 127 0" />
                <feOffset dy="2.03528" />
                <feGaussianBlur stdDeviation="1.01764" />
                <feColorMatrix type="matrix" values="0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0.25 0" />
                <feBlend mode="normal" in2="BackgroundImageFix" result="effect1_dropShadow" />
                <feBlend mode="normal" in="SourceGraphic" in2="effect1_dropShadow" result="shape" />
            </filter>
        </defs>
    </svg>
)
