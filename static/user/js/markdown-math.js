const assets = '/user/vendor/katex-0.19.1/';

export async function typesetMath(root) {
    if (!root) return;
    const nodes = root.querySelectorAll('.math-inline:not([data-typeset]), .math-display:not([data-typeset]), pre > code.language-math:only-child');
    if (!nodes.length) return;

    if (!document.querySelector('link[data-katex]')) {
        const css = document.createElement('link');
        css.rel = 'stylesheet';
        css.href = assets + 'katex.min.css';
        css.dataset.katex = '';
        document.head.append(css);
    }
    const {render, ParseError} = await import(assets + 'katex.mjs');
    for (const node of nodes) {
        if (!node.isConnected || node.hasAttribute('data-typeset')) continue;
        const fenced = node.matches('pre > code.language-math');
        const text = node.textContent;
        const width = text.startsWith('$$') || text.startsWith('\\') ? 2 : 1;
        const tex = fenced ? text : text.slice(width, -width);
        const display = fenced || node.classList.contains('math-display');
        const output = document.createElement(fenced ? 'div' : 'span');
        try {
            // Existing notes use bare \\ for display line breaks and w^\* for an optimum.
            render(tex, output, {displayMode: display, strict: false, trust: false, macros: {'\\*': '\\ast'}});
        } catch (error) {
            if (!(error instanceof ParseError)) throw error;
            node.title = error.message;
            node.dataset.typeset = 'error';
            continue;
        }
        if (fenced) {
            output.className = 'math-display';
            output.dataset.typeset = 'done';
            node.parentElement.replaceWith(output);
        } else {
            node.replaceChildren(...output.childNodes);
            node.dataset.typeset = 'done';
        }
    }
}
