%{
package eqnml

import "html"

%}

%token	MATRIX ABOVE MROW VMATRIX BACKSLASH2
%term <s> EOF NOTEQN EQNSTART EQNEND QTEXT
%term <b> CONTIG
%type <b> eqn box eqnopt
%type <s> textopt
%type <bss> rows

%right SUP SUB

%union {
	s string
	b []byte
	bs [][]byte
	bss [][][]byte
	line line
}

%%

top:
	eqn EOF
	{yylex.(*lexer).math = $1; return 1}

eqn:
	box
	{$$ = $1}
|	eqn box
	{$$ = append($1, $2...)}

box:
	'{' eqn '}'
	{$$ = $2}
|	QTEXT
	{$$ = []byte("<mtext>"+html.EscapeString($1)+"</mtext>")}
|	CONTIG
	{$$ = $1}
|	'~'
	{$$ = []byte("<mspace class='s1'/>")}
|	'^'
	{$$ = []byte("<mspace class='s0'/>")}
|	MROW '{' eqn '}'
	{$$ = append(append([]byte("<mrow>"), $3...), "</mrow>"...)}
|	box SUB box
	{$$ = append(append(append(append([]byte("<msub><mrow>"), $1...), "</mrow><mrow>"...), $3...), "</msub>"...)}
|	box SUP box
	{$$ = append(append(append(append([]byte("<msup><mrow>"), $1...), "</mrow><mrow>"...), $3...), "</msup>"...)}
|	MATRIX textopt '{' rows '}'
	{$$ = vmatrix($2, $4)}

eqnopt:
	{$$ = nil}
|	eqn

rows:
	eqnopt
	{$$=[][][]byte{[][]byte{$1}}}
|	rows '&' eqnopt
	{$$ = $1; $$[len($$)-1] = append($$[len($$)-1], $3)}
|	rows BACKSLASH2 eqnopt
	{$$ = $1; $$ = append($$, [][]byte{$3})}

textopt:
	{$$ = ""}
|	QTEXT

%%


