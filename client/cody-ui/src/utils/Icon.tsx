import React from 'react'

export const Icon: React.FunctionComponent<{ svgPath: string; className?: string }> = ({ svgPath, className }) => (
    <svg role="img" height={24} width={24} viewBox="0 0 24 24" fill="currentColor" className={className}>
        <path d={svgPath} />
    </svg>
)
