<!DOCTYPE html>
<html>
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<link rel="stylesheet" href="ivy.css">
<script src="ivy.js"></script>
<title>Ivy Demo</title>
</head>
<body onload="ivyStartup();">

# Ivy Demo

This is a demo of Ivy, running in your web browser.

Each step in the demo is one line of input followed by some output from Ivy, rendered like this:

```ivy
2+2
-- out --
4
```

The first line you see above (`2+2`) is input; the next (`4`) is output from a running ivy.

To execute Ivy text, click the Play button (“<small>▶️</small>”) next to it. Try this one:

```ivy
2*3
```

Whenever you like, you can edit the text yourself and then re-execute it by clicking Play again.
If you are using a desktop computer, as an alternative to clicking Play,
you can use Control-Enter to execute the Ivy text your cursor is editing.

Let's start the actual demo.

Arithmetic has the obvious operations: `+` `-` `*` etc. `**` is exponentiation. `mod` is modulo.

```ivy
23
```

```ivy
23 + 45
```

```ivy
23 * 45
```

```ivy
23 - 45
```

```ivy
7 ** 3
```

```ivy
7 mod 3
```

Operator precedence is unusual.

Unary operators operate on everything to the right.

Binary operators operate on the item immediately to the left, and everything to the right.

```ivy
2*3+4     # Parsed as 2*(3+4), not the usual (2*3)+4.
```

```ivy
2**2+3    # 2**5, not (2**2) + 3
```

```ivy
(2**2)+3  # Use parentheses if you need to group differently.
```

Ivy can do rational arithmetic, so `1/3` is really 1/3, not 0.333....

```ivy
1/3
```

```ivy
1/3 + 4/5
```

```ivy
1/3 ** 2
```

We'll see non-integral exponents later.

Even when a number is input in floating notation, it is still an exact rational number inside.

```ivy
1.2
```

In fact, ivy is a "bignum" calculator that can handle huge numbers and rationals made of huge numbers.

These are integers:

```ivy
1e10
```

```ivy
1e100
```

These are exact rationals:

```ivy
1e10/3
```

```ivy
3/1e10
```

They can get pretty big:

```ivy
2**64
```

They can get really big:

```ivy
2**640
```

They can get really really big:

```ivy
2**6400
```

Ivy also has characters, which represent a Unicode code point.

```ivy
'x'
```

`char` is an operator, returning the character with the given value.

```ivy
char 0x61
```

```ivy
char 0x1f4a9
```

`code` is `char`'s inverse, the value of the given character:

```ivy
code '💩'
```

Everything in ivy can be placed into a vector.

Vectors are written and displayed with spaces between the elements.

```ivy
1 2 3
```

```ivy
1 4/3 5/3 (2+1/3)
```

Note that without the parens, this becomes `(1 4/3 5/3 2)+1/3`

```ivy
1 4/3 5/3 2+1/3
```

Vectors of characters print without quotes or spaces.

```ivy
'h' 'e' 'l' 'l' 'o'
```

This is a nicer but equivalent way to write 'h' 'e' 'l' 'l' 'o':

```ivy
'hello'
```

Arithmetic works elementwise on vectors.

```ivy
1 2 3 + 4 5 6
```

Arithmetic between scalar and vector also works, either way.

```ivy
23 + 1 2 3
```

Note the grouping here. A vector is a single value.

```ivy
1 2 3 + 23
```

More fun with scalar and vector.

```ivy
1 << 1 2 3 4 5
```

```ivy
(1 << 1 2 3 4 5) == (2 ** 1 2 3 4 5)
```

Note that true is 1 and false is 0.

`iota` is an "index generator": It counts from 1.

```ivy
iota 10
```

```ivy
2 ** iota 5
```

```ivy
(1 << iota 100) == 2 ** iota 100
```

Again, notice how the precedence rules work.

```ivy
2 ** -1 + iota 32
```

The `take` operator removes n items from the beginning of the vector.
A negative n takes from the end.

```ivy
3 take iota 10
```

```ivy
-3 take iota 10
```

Drop is the other half: it drops n from the vector.

```ivy
3 drop iota 10
```

```ivy
-3 drop iota 10
```

```ivy
6 drop 'hello world'
```

## Reduction

```ivy
iota 15
```

Add them up:

```ivy
1 + 2 + 3 + 4 + 5 + 6 + 7 + 8 + 9 + 10 + 11 + 12 + 13 + 14 + 15
```

Automate this by reducing `+` over the vector, like this:

```ivy
+/iota 15
```

We can reduce using any binary operator. This is factorial:

```ivy
1 * 2 * 3 * 4 * 5 * 6 * 7 * 8 * 9 * 10
```

```ivy
*/iota 10
```

```ivy
*/iota 100
```

Try this:

```ivy
*/iota 10000
```

That printed using floating-point notation for manageability, but it is still an integer inside.

`max` and `min` are binary operators that do the obvious. (Use semicolons to separate expressions.)

```ivy
3 max 7; 'is max and'; 3 min 7; 'is min'
```

Like all binary arithmetic operators, `max` applies elementwise.

```ivy
2 3 4 max 4 3 2
```

Reduce using `max` to find maximum element in vector.

```ivy
max/2 34 42 233 2 2 521 14 1 4 1 55 133
```

Ivy allows multidimensional arrays. The binary shape operator, `rho`, builds them.
Dimension (which may be a vector) on the left, data on the right.

```ivy
5 rho 1
```

```ivy
5 5 rho 1
```

```ivy
5 5 rho 25
```

```ivy
5 5 rho iota 25
```

```ivy
3 5 5 rho iota 125
```

Unary `rho` tells us the shape of an item.

```ivy
x = 3 5 rho iota 15; x
```

```ivy
rho x
```

```ivy
x = 3 5 5 rho iota 75; x
```

```ivy
rho x
```

Arithmetic on matrices works as you would expect by now.

```ivy
x/2
```

```ivy
x**2
```

```ivy
x**3
```

```ivy
x**10
```

Inner product is written with a `.` between the operators.

For a dot product, multiply corresponding elements and add the result.

```ivy
1 2 3 4 +.* 2 3 4 5
```

Any operator works. How many items are the same?

```ivy
(1 2 3) +.== (1 3 3)
```

How many differ?

```ivy
(1 2 3) +.!= (1 3 3)
```

Outer product generates a matrix of all combinations applying the binary operator.

```ivy
(iota 5) o.* -1 + iota 5
```

That's a letter 'o', dot, star.

Any operator works; here is how to make an identity matrix.

```ivy
x = iota 5; x o.== x
```

Assignment is an operator, so you can save an intermediate expression.

```ivy
x o.== x = iota 5
```

Random numbers: Use a unary `?` to roll an n-sided die from 1 to n.

```ivy
?100
```

```ivy
?100 100 100 100 100
```

Twenty rolls of a 6-sided die:

```ivy
?20 rho 6
```

Indexing is easy.

```ivy
print x = ?20 rho 6
```

```ivy
x[1]
```

You can index with a vector.

```ivy
x[1 19 3]
```

Multiple index dimensions are separated by semicolons.

```ivy
(5 5 rho iota 25)[rot iota 5; iota 5]
```

The `up` and `down` operators generate index vectors that would sort the input.

```ivy
up x
```

```ivy
x[up x]
```

```ivy
x[down x]
```

```ivy
'hello world'[up 'hello world']
```

```ivy
'hello world'[down 'hello world']
```

More rolls of a die.

```ivy
?10 rho 6
```

Remember a set of rolls.

```ivy
x = ?10 rho 6; x
```

The outer product of `==` and the integers puts 1 in each row where that value appeared.

Compare the last row of the next result to the 6s in x.

```ivy
iota 6
x
(iota 6) o.== x
```

Count the number of times each value appears by reducing the matrix horizontally.

```ivy
+/(iota 6) o.== x
```

Do it for a much larger set of rolls: is the die fair?

```ivy
+/(iota 6) o.== ?60000 rho 6
```

Remember that ivy is a big number calculator.

```ivy
*/iota 100
```

```ivy
2**64
```

```ivy
2**iota 64
```

```ivy
-1+2**63
```

Settings are made and queried with a leading right paren. `)help` helps with settings and other commands.

```ivy
)help
```

Use `)base` to switch input and output to base 16.

```ivy
)base 16
```

```ivy
)base   # The input and output for settings is always base 10.
```

`_` is a variable that holds the most recently evaluated expression. It remembers our 63-bit number.

```ivy
_
```

16 powers of two (in base 16).

```ivy
1<<iota 10
```

The largest 64-bit number (in base 16).

```ivy
(2**40)-1
```

Output base 10, input base still 16.

```ivy
)obase 10
```

```ivy
)base
```

The largest 64-bit number, base 10.

```ivy
-1+2**40
```

The largest 63-bit number, base 10.

```ivy
-1+2**3F
```

Go back to base 10 input and output.

```ivy
)base 10
```

Rationals can be very big too.

```ivy
(2**1e3)/(3**1e2)
```

Such output can be unwieldy. Change the output format using a Printf string.

```ivy
)format '%.12g'
```

```ivy
_
```

We need more precision.

```ivy
)format "%.100g"
```

(Double quotes work for strings too; there's no difference.)

```ivy
_
```

```ivy
)format '%#x'
```

```ivy
_
```

A nice format, easily available on the command line with `ivy -g`:

```ivy
)format '%.12g'
```

```ivy
_
```

```ivy
(3 4 rho iota 12)/4
```

Irrational functions cannot be represented precisely by rational numbers.

Ivy stores irrational results in high-precision (default 256-bit) floating point numbers.

```ivy
sqrt 2
```

`pi` and `e` are built-in, high-precision constants.

```ivy
pi
```

```ivy
e
```

```ivy
)format "%.100g"
```

```ivy
pi
```

```ivy
)format '%.12g'
```

```ivy
pi
```

Exponentials and logarithms.

Note: Non-integral exponent generates irrational result.

```ivy
2**1/2
```

```ivy
e**1e6
```

```ivy
log e**1e6
```

```ivy
log e**1e8
```

```ivy
log 1e1000000
```

Yes, that is 10 to the millionth power!

Transcendentals. (The low bit isn't always right...)

```ivy
sin pi/2
```

```ivy
cos .25*pi * -1 + iota 9
```

```ivy
log iota 6
```

Successive approximations to e. (We force the calculation to use float using the "float" unary operator. Why?)

```ivy
(float 1+10**-iota 9) ** 10**iota 9
```

Default precision is 256 bits of mantissa. We can go up to 10000.

Precision units are bits, not digits. `1000 * 2 log 10` is 3321; we add a few more bits for floating point errors.

```ivy
)prec 3350
```

```ivy
e
```

The units for formatting are digits, not bits. (Sorry for the inconsistency.)

```ivy
)format '%.1000g'
```

```ivy
e
```

```ivy
pi
```

```ivy
sqrt 2
```

```ivy
e**1e6
```

```ivy
log e**1e6
```

```ivy
(2**1e3)/(3**1e2)
```

User-defined operators are declared as unary or binary (or both). This one computes the (unary) average.

```ivy
op avg x = (+/x)/rho x
```

```ivy
avg iota 100
```

Here is a binary operator.

```ivy
op n largest x = n take x[down x]
```

```ivy
3 largest ? 100 rho 1000
```

```ivy
4 largest 'hello world'
```

Population count. Use `encode` to turn the value into a string of bits. Use `log` to decide how many.

```ivy
op a base b = ((ceil b log a) rho b) encode a
```

```ivy
7 base 2
```

```ivy
op popcount n = +/n base 2
```

```ivy
popcount 7
```

```ivy
popcount 1e6
```

```ivy
popcount 1e100
```

Here is one to sum the digits. The unary operator `text` turns its argument into text, like [fmt.Sprint](https://go.dev/pkg/fmt/#Sprint).

```ivy
op sumdigits x = t = text x; +/(code (t in '0123456789') sel t) - code '0'
```

Break it down:  The `sel` operator selects from the right based on the non-zero elements in the left.

The `in` operator generates a selector by choosing only the bytes that are ASCII digits.

```ivy
sumdigits 99
```

```ivy
sumdigits iota 10
```

```ivy
sumdigits '23 skidoo'
```

In the last example, note that `sumdigits` only counts the digits.

The binary `text` operator takes a format string (`%` optional) on the left and formats the value.

```ivy
'%x' text 1234
```

We can use this for another version of popcount: `%b` is binary.

```ivy
op popcount n = +/'1' == '%b' text n
```

```ivy
popcount 7
```

```ivy
popcount 1e6
```

```ivy
popcount 1e100
```

A classic (expensive!) algorithm to count primes.

```ivy
op primes N = (not T in T o.* T) sel T = 1 drop iota N
```

The assignment to `T` gives 2..N. We use outer product to build an array of all products.

Then we find all elements of `T` that appear in the product matrix, invert that, and select from the original.

```ivy
primes 100
```

A final trick.

The binary `?` operator "deals": `x?y` selects at random x distinct integers from 1..y inclusive.

```ivy
5?10
```

We can use this to shuffle a deck of cards. The suits are "♠♡♣♢", the values "A234567890JQK" (using 0 for 10, for simplicity).

Create the deck using outer product with the ravel operator:

```ivy
"A234567890JQK" o., "♠♡♣♢"
```

To shuffle it, ravel into into a vector and index that by 1 through 52, shuffled.

```ivy
(, "A234567890JQK" o., "♠♡♣♢")[52?52]
```

There is no looping construct in ivy, but there is a conditional evaluator.

Within a user-defined operator, one can write a condition expression
using a binary operator, ‘`:`’. If the left-hand operand is true (integer non-zero),
the user-defined operator will return the right-hand operand as its
result; otherwise execution continues.

```ivy
op a gcd b = a == b: a; a > b: b gcd a-b; a gcd b-a
```

```ivy
1562 gcd !11
```

That's it! Have fun.

For more information visit <https://pkg.go.dev/robpike.io/ivy>.

