// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package eqnml

var preamble = `
% TeXbook Appendix F, pp 434-439.
% 1. Lowercase Greek letters
\def\alpha{α}
\def\beta{β}
\def\gamma{γ}
\def\delta{δ}
\def\epsilon{ϵ}
\def\varepsilon{ε}
\def\zeta{ζ}
\def\eta{η}
\def\theta{θ}
\def\vartheta{ϑ}
\def\iota{ι}
\def\kappa{κ}
\def\lambda{λ}
\def\mu{μ}
\def\nu{ν}
\def\xi{ξ}
\def\pi{π}
\def\varpi{ϖ}
\def\rho{ρ}
\def\varrho{ϱ}
\def\sigma{σ}
\def\varsigma{ς}
\def\tau{τ}
\def\upsilon{υ}
\def\phi{ϕ}
\def\varphi{φ}
\def\chi{χ}
\def\psi{ψ}
\def\omega{ω}

% 2. Uppercase Greek letters
\def\Gamma{Γ}
\def\Delta{Δ}
\def\Theta{Θ}
\def\Lambda{Λ}
\def\Xi{Ξ}
\def\Pi{Π}
\def\Sigma{Σ}
\def\Upsilon{Υ}
\def\Phi{Φ}
\def\Psi{Ψ}
\def\Omega{Ω}

% 3. TODO Caligraphic capitals and other alphabets

% 4. Miscellaneous symbols of type Ord.
\def\aleph{ℵ}
\def\hbar{ħ}
\def\imath{𝚤}
\def\jmath{𝚥}
\def\ell{ℓ}
\def\wp{℘}
\def\Re{ℜ}
\def\Im{ℑ}
\def\partial{∂}
\def\infty{∞}
\def\prime{′}
\def\emptyset{∅}
\def\nabla{∇}
\def\surd{√}
\def\top{⊤}
\def\bot{⊥}
\def\|{‖}
\def\angle{∠}
\def\triangle{△}
% \backslash handled specially
\def\forall{∀}
\def\exists{∃}
\def\neg{¬}
\def\flat{♭}
\def\natural{♮}
\def\sharp{♯}
\def\clubsuit{♣}
\def\diamondsuit{♢}
\def\heartsuit{♡}
\def\spadesuit{♠}

% 5. Digits.
% TODO {\it ...} {\mathit ...} {\bf ...} {\mathbf ...} {\oldstyle ...}

% 6. Large operators
\def\sum{∑}
\def\prod{∏}
\def\coprod{∐}
\def\int{∫}
\def\oint{∮}
\def\bigcap{⋂}
\def\bigcup{⋃}
\def\bigsqcup{⨆}
\def\bigvee{⋁}
\def\bigwedge{⋀}
\def\bigodot{⨀}
\def\bigotimes{⨂}
\def\bigoplus{⨁}
\def\biguplus{⨄}

% 7. Binary operations.
% TODO distinguish from the big ones
\def\pm{±}
\def\mp{∓}
\def\setminus⧵  % TODO make an operato{r}
\def\cdot{⋅}
\def\times{×}
\def\ast{∗}
\def\star{⭑}
\def\diamond{◇}
\def\circ{○}
\def\bullet{•}
\def\div{÷}
\def\cap{∩}
\def\cup{∪}
\def\uplus{⊎}
\def\sqcap{⊓}
\def\sqcup{⊔}
\def\triangleleft{◁}
\def\triangleright{▷}
\def\wr{≀}
\def\bigcirc{◯}
\def\bigtriangleup{△}
\def\bigtriangledown{▽}
\def\vee{∨}
\def\wedge{∧}
\def\oplus{⊕}
\def\ominus{⊖}
\def\otimes{⊗}
\def\oslash{⊘}
\def\odot{⊙}
\def\dagger{†}
\def\ddagger{‡}
\def\amalg{⨿}

% 8. Relations
\def\leq{≤}
\def\prec{≺}
\def\preceq{⪯}
\def\ll{≪}
\def\subset{⊂}
\def\subseteq{⊆}
\def\sqsubseteq{⊑}
\def\in{∈}
\def\vdash{⊢}
\def\smile{⌣}
\def\frown{⌢}
\def\geq{≥}
\def\succ{≻}
\def\succeq{⪰}
\def\gg{≫}
\def\supset{⊃}
\def\supseteq{⊇}
\def\sqsupseteq{⊒}
\def\ni{∋}
\def\dashv{⊣}
\def\mid{∣}
\def\parallel{∥}
\def\equiv{≡}
\def\sim{∼}
\def\simeq{≃}
\def\asymp{≍}
\def\approx{≈}
\def\cong{≅}
\def\bowtie{⋈}
\def\propto{∝}
\def\models{⊧}
\def\doteq{≐}
\def\perp{⟂}

% 9. Negated relations
% TODO

% 10. Arrows
\def\leftarrow{←}
\def\Leftarrow{⇐}
\def\rightarrow{→}
\def\Rightarrow{⇒}
\def\leftrightarrow{↔}
\def\Leftrightarrow{⇔}
\def\mapsto{↦}
\def\hookleftarrow{↩}
\def\leftharpoonup{↼}
\def\leftharpoondown{↽}
\def\rightleftharpoons{⇌}
\def\longleftarrow{⟵}
\def\Longleftarrow{⟸}
\def\longrightarrow{⟶}
\def\Longrightarrow{⟹}
\def\longleftrightarrow{⟷}
\def\Longleftrightarrow{⟺}
\def\longmapsto{⟼}
\def\hookrightarrow{↪}
\def\rightharpoonup{⇀}
\def\rightharpoondown{⇁}
\def\uparrow{↑}
\def\Uparrow{⇑}
\def\downarrow{↓}
\def\Downarrow{⇓}
\def\updownarrow{↕}
\def\Updownarrow{⇕}
\def\nearrow{↗}
\def\searrow{↘}
\def\swarrow{↙}
\def\nwarrow{↖}

% 11. Openings
% 12. Closings
\def\lbrack{[}
\def\lbrace{\{}
\def\lfloor{⌊}
\def\langle{⟨}
\def\lceil{⌈}
\def\lgroup{⟮}
\def\lmoustache{⎰}
\def\rbrack{]}
\def\rbrace{\}}
\def\rfloor{⌋}
\def\rangle{⟩}
\def\rceil{⌉}
\def\rgroup{⟯}
\def\rmoustache{⎱}

% 13. Punctuation
% TODO spacing around colon
\def\colon{:}

% 14. Alternate names.
% TODO \let
\def\ne{≠}
\def\neq{≠}
\def\le{≤}
\def\ge{≥}
% \{ and \} handled specially
\def\to{→}
\def\gets{←}
\def\owns{∋}
\def\land{∧}
\def\lor{∨}
\def\lnot{¬}
\def\vert{|}
\def\Vert{‖}

% 15. Non-math symbols.
\def\S{§}
\def\P{¶}
\def\dag{†}
\def\ddag{‡}

% TODO expand according to https://www.ctan.org/pkg/lshort-english

\def\log{{\rm log}}

\def\\{\cr}
\def\choose{\atopwithdelims()}
\def\overline#1{\mover{#1}{\_}}
\def\phantom#1{\mphantom{#1}}
\def\matrix#1{\mtable{#1}}
\def\pmatrix#1{\left(\mtable{#1}\right)}
\def\sqrt#1{\msqrt{#1}}
\def\underline#1{\munder{#1}{\_}}
\def\quad{\mspace{1em}{}}
\def\qquad{\mspace{2em}{}}
\def\,{\mspace{0.166em}{}}
\def\>{\mspace{0.222em}{}}
\def\;{\mspace{0.277em}{}}
`
