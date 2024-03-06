import { useEffect } from 'react'

const mathjaxURL = `/.assets/mathjax/tex-mml-chtml.js`
export const mathjaxElementId = 'MathJax-script'

/**
 * useMathJax enables rendering mathematical expressions on the page.
 * @param enabled Set to `false` to disable the hook (default: `true`).
 *
 * @details
 * On component mount, useMathJax injects a script to load MathJax component
 * from the static files (.assets). This is the approach described in the
 * {@link https://docs.mathjax.org/en/latest/web/hosting.html#hosting-your-own-copy-of-mathjax | official documentation}.
 *
 * ```
 *  <script type="text/javascript" id="MathJax-script" async
 *      src="/.assets/mathjax/tex-mml-chtml.js">
 *  </script>
 * ```
 *
 * Although sourcing the files form the CDN is the *recommended* approach,
 * it is neither secure nor feasible for some of the on-prem environments
 * in which Sourcegraph may be deployed.
 * Once loaded, the component will scan the page and typeset any math in it.
 *
 * On component unmount, removes the script via useEffect destructor.
 */
export const useMathJax = (enabled: boolean = true) => {
    if (!enabled) {
        return
    }

    useEffect(() => {
        const mj = document.createElement('script')

        mj.setAttribute('type', 'text/javascript')
        mj.setAttribute('src', mathjaxURL)
        mj.setAttribute('async', '')
        mj.setAttribute('id', mathjaxElementId)

        document.head.appendChild(mj)

        return () => {
            mj.remove()
        }
    }, [])

    // When working with dynamic HTML, it can happen that MathJax typesets the page
    // before the dynamic part containing mathematics is loaded, in which case we
    // need to trigger typesetting by ourselves.
    useEffect(() => {
        if (window.MathJax) {
            window.MathJax.typeset()
        }
    }, [window.MathJax])
}

declare global {
    interface Window {
        MathJax: { typeset: () => void } | undefined
    }
}
