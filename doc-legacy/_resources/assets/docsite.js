// Theme
const currentThemePreference = () => localStorage.getItem('theme-preference') || 'auto'
const themePreferenceButtons = () => document.querySelectorAll('body > #sidebar #theme button[data-theme-preference]')
const currentTheme = (pref = currentThemePreference()) => pref === 'auto' ? window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light' : pref
const applyThemePreference = (pref) => {
  localStorage.setItem('theme-preference', pref)
  applyTheme()
}
const applyTheme = () => {
  const pref = currentThemePreference()
  for (const button of themePreferenceButtons()) {
    button.classList.toggle('active', button.dataset.themePreference === pref)
  }

  const theme = currentTheme(pref)
  document.body.classList.toggle('theme-light', theme === 'light')
  document.body.classList.toggle('theme-dark', theme === 'dark')
}
document.addEventListener('DOMContentLoaded', () => {
  window.matchMedia('(prefers-color-scheme: dark)').addListener(e => applyTheme())
  applyTheme()
  for (const button of themePreferenceButtons()) {
    button.addEventListener('click', e => {
      applyThemePreference(e.currentTarget.dataset.themePreference)
    });
  }
  // Toggle theme on Option+T/Alt+T/Meta+T
  document.body.addEventListener('keydown', e => {
    if ((e.metaKey || e.altKey) && e.key === 't') {
      const theme = currentTheme();
      applyThemePreference(theme === 'light' ? 'dark' : 'light')
    }
  })
})

// Sidebar
const pagePath = location.pathname
const quote = str => JSON.stringify(str.replace(/[^a-zA-Z0-9._\/-]/g, ''))
document.addEventListener('DOMContentLoaded', () => {
  const style = document.createElement('style')
  let currentElement = null;
  for (const link of document.querySelectorAll('body > #sidebar .nav-section.tree a')) {
    const current = link.pathname === pagePath

    // Store the current element, so we can scroll it into view later
    if (current && !currentElement) {
      currentElement = link;
    }

    const expand = current || pagePath.startsWith(link.pathname + '/')
    const subsection = link.pathname.split('/').length >= 3

    const item = link.parentNode
    item.classList.toggle('current', current)
    item.classList.toggle('expand', expand)
    item.classList.toggle('active-subsection', subsection && expand)
    item.classList.toggle('collapse', !expand)
  }

  // Scroll matching sidebar item into view
  if (currentElement) {
    currentElement.scrollIntoView({ block: 'center' })
  }
})

// JSON Schema keys
document.addEventListener('DOMContentLoaded', () => {
  const schemaDocs = document.querySelectorAll('.json-schema-doc')
  // Find spans that contain a key and swap them for anchors for hover functionality
  schemaDocs.forEach(schemaDoc => {
    schemaDoc.querySelectorAll(`span`).forEach(el => {
      const keyNameMatch = el.innerText.match(/^"(.*)"/)
      const isKey = el.nextSibling && el.nextSibling.textContent.includes(':')
      if (!isKey || !keyNameMatch) {
        return
      }

      // Add a named anchor to get the hover functionality we need
      const keyText = keyNameMatch[1]
      const id = keyText.replace(/\./g, '-')
      const anchor = document.createElement('a')
      anchor.id = id
      anchor.className = 'schema-doc-key'
      anchor.href = '#' + id
      anchor.rel = 'nofollow'
      anchor.textContent = `"${keyText}"`
      anchor.style.color = el.style.color
      el.replaceWith(anchor)
    })
  })
  // If URL hash is set and matches a schema key, scroll to it
  if (window.location.hash) {
    let targetKey = document.getElementById(window.location.hash.slice(1))
    if (targetKey && Array.from(schemaDocs).some(e => e.contains(targetKey))) {
      setTimeout(() => {
        // Scroll to slightly above the element so the docstring is visible.
        targetKey.scrollIntoView(true)
        window.scrollBy(0, -150)
      }, 0)
    }
  }
})

// Conversion tracking for copying the quickstart command
document.addEventListener('DOMContentLoaded', () => {
  const startSourcegraphCommand = document.querySelector('.start-sourcegraph-command') // Assumes only one per page
  const gaConversionOnStartSourcegraphCommands = () => {
    // TODO(sqs): doesn't seem to be working: https://github.com/sourcegraph/sourcegraph/issues/14323
    if (window && window.gtag) {
      window.gtag('event', 'conversion', {
        'send_to': 'AW-868484203/vOYoCOCUj7EBEOuIkJ4D',
      });
    }
  }
  if (startSourcegraphCommand) {
    startSourcegraphCommand.addEventListener('click', gaConversionOnStartSourcegraphCommands)
  }
})

// Cloud CTA clicks
document.addEventListener('DOMContentLoaded', () => {
  const cloudCTAs = document.querySelectorAll('.cloud-cta')

  cloudCTAs.forEach(cloudCTA => {
    cloudCTA.addEventListener('click', () => {
      if (window && window.plausible) {
        window.plausible('ClickedOnFreeTrialCTA')
      }
    })
  })
})

// Promise to wait on for DOMContentLoaded
const domContentLoadedPromise = new Promise(resolve => {
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', resolve)
  } else {
    resolve()
  }
})

// Import Sourcegraph's EventLogger
const importEventLoggerPromise = import('https://cdn.skypack.dev/@sourcegraph/event-logger')

Promise.all([domContentLoadedPromise, importEventLoggerPromise]).then(([_, { EventLogger }]) => {
  const eventLogger = new EventLogger('https://sourcegraph.com')

  // Log a Docs page view event
  const eventArguments = { path: window.location.pathname }
  eventLogger.log('ViewStaticPage', eventArguments, eventArguments)

  // Log download links which have a "data-download-name" attribute
  // This is used to track Cody App download links
  document.addEventListener('click', event => {
    if (event.target.matches('a[data-download-name]')) {
      const downloadName = event.target.getAttribute('data-download-name')
      const eventArguments = {
        path: window.location.pathname,
        downloadSource: 'docs',
        downloadName,
        downloadLinkUrl: event.target.href,
      }

      eventLogger.log('DownloadClick', eventArguments, eventArguments)
    }
  })
})

// Add a "Copy" button to code blocks
document.addEventListener('DOMContentLoaded', () => {
  let copyButtonLabel = new DOMParser().parseFromString(
    '<svg xmlns="http://www.w3.org/2000/svg" height="18px" viewBox="0 0 24 24" width="18px" fill="currentColor"><path d="M0 0h24v24H0z" fill="none"/><path d="M16 1H4c-1.1 0-2 .9-2 2v14h2V3h12V1zm3 4H8c-1.1 0-2 .9-2 2v14c0 1.1.9 2 2 2h11c1.1 0 2-.9 2-2V7c0-1.1-.9-2-2-2zm0 16H8V7h11v14z"/></svg>',
    'application/xml'
  );

  // use a class selector if available
  let blocks = document.querySelectorAll("pre.chroma.sgquery, pre.chroma.js");

  blocks.forEach((block) => {
    // only add button if browser supports Clipboard API
    if (navigator.clipboard) {
      let button = document.createElement("button");

      button.appendChild(
        button.ownerDocument.importNode(copyButtonLabel.documentElement, true)
      );
      block.appendChild(button);

      button.addEventListener("click", async () => {
        await copyCode(block, button);
      });
    }
  });

  async function copyCode(block, button) {
    let text = block.innerText;

    await navigator.clipboard.writeText(text);

    // visual feedback that task is completed
    button.innerText = "Copied!";

    setTimeout(() => {
      button.replaceChild(
        button.ownerDocument.importNode(copyButtonLabel.documentElement, true),
        button.childNodes[0]
      );
    }, 900);
  }
})
