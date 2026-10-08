Eqnml converts an equation language similar to Unix's eqn(1)
to [MathML Core](https://www.w3.org/TR/mathml-core/) syntax that can be used in HTML pages.
As of this writing, the latest MathML Core draft is dated November 27, 2023
and is [supported by major browers](https://developer.mozilla.org/en-US/docs/Web/MathML#browser_compatibility) as of January 2023.
MathML Core thus provides a lighter-weight alternative to MathJAX and similar tools.

## API

The package provides a very simple API: the function [`ToHTML`](https://pkg.go.dev/rsc.io/eqnml#ToHTML)
and the string variable [`CSS`](https://pkg.go.dev/rsc.io/eqnml#CSS).
`ToHTML` converts equation language text into MathML Core markup.
The result must be wrapped in `<math class=eqn></math>` tags
(adding `display=block` for block-like equations),
and the page should include `CSS` in its CSS style sheet for
proper rendering of the output.

## Language

