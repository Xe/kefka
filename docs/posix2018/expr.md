#### []{#tag_20_42_01}NAME {#name .mansect}

> expr - evaluate arguments as an expression

#### []{#tag_20_42_02}SYNOPSIS {#synopsis .mansect}

> `expr`` `*`operand`*`...`

#### []{#tag_20_42_03}DESCRIPTION {#description .mansect}

> The *expr* utility shall evaluate an expression and write the result
> to standard output.

#### []{#tag_20_42_04}OPTIONS {#options .mansect}

> None.

#### []{#tag_20_42_05}OPERANDS {#operands .mansect}

> The single expression evaluated by *expr* shall be formed from the
> *operand* operands, as described in the EXTENDED DESCRIPTION section.
> The application shall ensure that each of the expression operator
> symbols:
>
>
>     (  )  |  &  =  >  >=  <  <=  !=  +  -  *  /  %  :
>
> and the symbols *integer* and *string* in the table are provided as
> separate arguments to *expr*.

#### []{#tag_20_42_06}STDIN {#stdin .mansect}

> Not used.

#### []{#tag_20_42_07}INPUT FILES {#input-files .mansect}

> None.

#### []{#tag_20_42_08}ENVIRONMENT VARIABLES {#environment-variables .mansect}

> The following environment variables shall affect the execution of
> *expr*:
>
> *LANG*
> :   Provide a default value for the internationalization variables
>     that are unset or null. (See XBD [*Internationalization
>     Variables*](../basedefs/V1_chap08.html#tag_08_02) for the
>     precedence of internationalization variables used to determine the
>     values of locale categories.)
>
> *LC_ALL*
> :   If set to a non-empty string value, override the values of all the
>     other internationalization variables.
>
> *LC_COLLATE*
> :   Determine the locale for the behavior of ranges, equivalence
>     classes, and multi-character collating elements within regular
>     expressions and by the string comparison operators.
>
> *LC_CTYPE*
> :   Determine the locale for the interpretation of sequences of bytes
>     of text data as characters (for example, single-byte as opposed to
>     multi-byte characters in arguments) and the behavior of character
>     classes within regular expressions.
>
> *LC_MESSAGES*
> :   Determine the locale that should be used to affect the format and
>     contents of diagnostic messages written to standard error.
>
> *NLSPATH*
> :   ^\[[XSI](javascript:open_code('XSI'))\]^ ![\[Option
>     Start\]](../images/opt-start.gif){border="0"} Determine the
>     location of message catalogs for the processing of *LC_MESSAGES.*
>     ![\[Option End\]](../images/opt-end.gif){border="0"}

#### []{#tag_20_42_09}ASYNCHRONOUS EVENTS {#asynchronous-events .mansect}

> Default.

#### []{#tag_20_42_10}STDOUT {#stdout .mansect}

> The *expr* utility shall evaluate the expression and write the result,
> followed by a \<newline\>, to standard output.

#### []{#tag_20_42_11}STDERR {#stderr .mansect}

> The standard error shall be used only for diagnostic messages.

#### []{#tag_20_42_12}OUTPUT FILES {#output-files .mansect}

> None.

#### []{#tag_20_42_13}EXTENDED DESCRIPTION {#extended-description .mansect}

> The formation of the expression to be evaluated is shown in the
> following table. The symbols *expr*, *expr1*, and *expr2* represent
> expressions formed from *integer* and *string* symbols and the
> expression operator symbols (all separate arguments) by recursive
> application of the constructs described in the table. The expressions
> are listed in order of decreasing precedence, with equal-precedence
> operators grouped between horizontal lines. All of the operators shall
> be left-associative.
>
> +-----------------------------------+-----------------------------------+
> | **Expression**                    | **Description**                   |
> +:==================================+:==================================+
> | *integer*                         | An argument consisting only of an |
> |                                   | (optional) unary minus followed   |
> | *string*                          | by digits.                        |
> |                                   |                                   |
> |                                   | A string argument; see below.     |
> +-----------------------------------+-----------------------------------+
> | ( *expr* )                        | Grouping symbols. Any expression  |
> |                                   | can be placed within parentheses. |
> |                                   | Parentheses can be nested to a    |
> |                                   | depth of {EXPR_NEST_MAX}.         |
> +-----------------------------------+-----------------------------------+
> | *expr1* : *expr2*                 | Matching expression; see below.   |
> +-----------------------------------+-----------------------------------+
> | *expr1* \* *expr2*                | Multiplication of decimal         |
> |                                   | integer-valued arguments.         |
> | *expr1* / *expr2*                 |                                   |
> |                                   | Integer division of decimal       |
> | *expr1* % *expr2*                 | integer-valued arguments,         |
> |                                   | producing an integer result.      |
> |                                   |                                   |
> |                                   | Remainder of integer division of  |
> |                                   | decimal integer-valued arguments. |
> +-----------------------------------+-----------------------------------+
> | *expr1* + *expr2*                 | Addition of decimal               |
> |                                   | integer-valued arguments.         |
> | *expr1* - *expr2*                 |                                   |
> |                                   | Subtraction of decimal            |
> |                                   | integer-valued arguments.         |
> +-----------------------------------+-----------------------------------+
> |  \                                | Returns the result of a decimal   |
> |                                   | integer comparison if both        |
> |                                   | arguments are integers;           |
> | *expr1* = *expr2*                 | otherwise, returns the result of  |
> |                                   | a string comparison using the     |
> | *expr1* \> *expr2*                | locale-specific collation         |
> |                                   | sequence. The result of each      |
> | *expr1* \>= *expr2*               | comparison is 1 if the specified  |
> |                                   | relationship is true, or 0 if the |
> | *expr1* \< *expr2*                | relationship is false.            |
> |                                   |                                   |
> | *expr1* \<= *expr2*               | Equal.                            |
> |                                   |                                   |
> | *expr1* != *expr2*                | Greater than.                     |
> |                                   |                                   |
> |                                   | Greater than or equal.            |
> |                                   |                                   |
> |                                   | Less than.                        |
> |                                   |                                   |
> |                                   | Less than or equal.               |
> |                                   |                                   |
> |                                   | Not equal.                        |
> +-----------------------------------+-----------------------------------+
> | *expr1* & *expr2*                 | Returns the evaluation of *expr1* |
> |                                   | if neither expression evaluates   |
> |                                   | to null or zero; otherwise,       |
> |                                   | returns zero.                     |
> +-----------------------------------+-----------------------------------+
> | *expr1* \| *expr2*                | Returns the evaluation of *expr1* |
> |                                   | if it is neither null nor zero;   |
> |                                   | otherwise, returns the evaluation |
> |                                   | of *expr2* if it is not null;     |
> |                                   | otherwise, zero.                  |
> +-----------------------------------+-----------------------------------+
>
> ##### []{#tag_20_42_13_01}Matching Expression
>
> The `':'` matching operator shall compare the string resulting from
> the evaluation of *expr1* with the regular expression pattern
> resulting from the evaluation of *expr2*. Regular expression syntax
> shall be that defined in XBD [*Basic Regular
> Expressions*](../basedefs/V1_chap09.html#tag_09_03), except that all
> patterns are anchored to the beginning of the string (that is, only
> sequences starting at the first character of a string are matched by
> the regular expression) and, therefore, it is unspecified whether
> `'^'` is a special character in that context. Usually, the matching
> operator shall return a string representing the number of characters
> matched ( `'0'` on failure). Alternatively, if the pattern contains at
> least one regular expression subexpression `"[\(...\)]"`, the string
> matched by the back-reference expression `"\1"` shall be returned. If
> the back-reference expression `"\1"` does not match, then the null
> string shall be returned.
>
> ##### []{#tag_20_42_13_02}Identification as Integer or String
>
> An argument or the value of a subexpression that consists only of an
> optional unary minus followed by digits is a candidate for treatment
> as an integer if it is used as the left argument to the `|` operator
> or as either argument to any of the following operators:
> `& = > >= < <= != + - * / %`. Otherwise, the argument or subexpression
> value shall be treated as a string.
>
> The use of string arguments **length**, **substr**, **index**, or
> **match** produces unspecified results.

#### []{#tag_20_42_14}EXIT STATUS {#exit-status .mansect}

> The following exit values shall be returned:
>
>  0
> :   The *expression* evaluates to neither null nor zero.
>
>  1
> :   The *expression* evaluates to null or zero.
>
>  2
> :   Invalid *expression*.
>
> \>2
> :   An error occurred.

#### []{#tag_20_42_15}CONSEQUENCES OF ERRORS {#consequences-of-errors .mansect}

> Default.

------------------------------------------------------------------------

::: box
*The following sections are informative.*
:::

#### []{#tag_20_42_16}APPLICATION USAGE {#application-usage .mansect}

> The *expr* utility has a rather difficult syntax:
>
> - Many of the operators are also shell control operators or reserved
>   words, so they have to be escaped on the command line.
>
> - Each part of the expression is composed of separate arguments, so
>   liberal usage of \<blank\> characters is required. For example:
>
>     **Invalid**            **Valid**
>     ---------------------- ---------------------------
>     *expr* `1+2`           *expr* `1 + 2`
>     *expr* `"1 + 2"`       *expr* `1 + 2`
>     *expr* `1 + (2 * 3)`   *expr* `1 + \( 2 \* 3 \)`
>
> In many cases, the arithmetic and string features provided as part of
> the shell command language are easier to use than their equivalents in
> *expr*. Newly written scripts should avoid *expr* in favor of the new
> features within the shell; see [*Parameters and
> Variables*](../utilities/V3_chap02.html#tag_18_05) and [*Arithmetic
> Expansion*](../utilities/V3_chap02.html#tag_18_06_04).
>
> After argument processing by the shell, *expr* is not required to be
> able to tell the difference between an operator and an operand except
> by the value. If `"$a"` is `'='`, the command:
>
>
>     expr "$a" = '='
>
> looks like:
>
>
>     expr = = =
>
> as the arguments are passed to *expr* (and they all may be taken as
> the `'='` operator). The following works reliably:
>
>
>     expr "X$a" = X=
>
> Also note that this volume of POSIX.1-2017 permits implementations to
> extend utilities. The *expr* utility permits the integer arguments to
> be preceded with a unary minus. This means that an integer argument
> could look like an option. Therefore, the conforming application must
> employ the `"--"` construct of Guideline 10 of XBD [*Utility Syntax
> Guidelines*](../basedefs/V1_chap12.html#tag_12_02) to protect its
> operands if there is any chance the first operand might be a negative
> integer (or any string with a leading minus).
>
> For testing string equality the [*test*](../utilities/test.html)
> utility is preferred over *expr*, as it is usually implemented as a
> shell built-in. However, the functionality is not quite the same
> because the *expr* `=` and `!=` operators check whether strings
> collate equally, whereas [*test*](../utilities/test.html) checks
> whether they are identical. Therefore, they can produce different
> results in locales where the collation sequence does not have a total
> ordering of all characters (see XBD
> [*LC_COLLATE*](../basedefs/V1_chap07.html#tag_07_03_02)).\

#### []{#tag_20_42_17}EXAMPLES {#examples .mansect}

> The following command:
>
>
>     a=$(expr "$a" + 1)
>
> adds 1 to the variable *a*.
>
> The following command, for `"$a"` equal to either **/usr/abc/file** or
> just **file**:
>
>
>     expr $a : '.*/\(.*\)' \| $a
>
> returns the last segment of a pathname (that is, **file**).
> Applications should avoid the character `'/'` used alone as an
> argument; *expr* may interpret it as the division operator.
>
> The following command:
>
>
>     expr "//$a" : '.*/\(.*\)'
>
> is a better representation of the previous example. The addition of
> the `"//"` characters eliminates any ambiguity about the division
> operator and simplifies the whole expression. Also note that pathnames
> may contain characters contained in the *IFS* variable and should be
> quoted to avoid having `"$a"` expand into multiple arguments.
>
> The following command:
>
>
>     expr "X$VAR" : '.*' - 1
>
> returns the number of characters in *VAR*.

#### []{#tag_20_42_18}RATIONALE {#rationale .mansect}

> In an early proposal, EREs were used in the matching expression
> syntax. This was changed to BREs to avoid breaking historical
> applications.
>
> The use of a leading \<circumflex\> in the BRE is unspecified because
> many historical implementations have treated it as a special
> character, despite their system documentation. For example:
>
>
>     expr foo : ^foo     expr ^foo : ^foo
>
> return 3 and 0, respectively, on those systems; their documentation
> would imply the reverse. Thus, the anchoring condition is left
> unspecified to avoid breaking historical scripts relying on this
> undocumented feature.

#### []{#tag_20_42_19}FUTURE DIRECTIONS {#future-directions .mansect}

> None.

#### []{#tag_20_42_20}SEE ALSO {#see-also .mansect}

> [*Parameters and Variables*](../utilities/V3_chap02.html#tag_18_05),
> [*Arithmetic Expansion*](../utilities/V3_chap02.html#tag_18_06_04)
>
> XBD [*LC_COLLATE*](../basedefs/V1_chap07.html#tag_07_03_02),
> [*Environment Variables*](../basedefs/V1_chap08.html#tag_08), [*Basic
> Regular Expressions*](../basedefs/V1_chap09.html#tag_09_03), [*Utility
> Syntax Guidelines*](../basedefs/V1_chap12.html#tag_12_02)

#### []{#tag_20_42_21}CHANGE HISTORY {#change-history .mansect}

> First released in Issue 2.

#### []{#tag_20_42_22}Issue 5 {#issue-5 .mansect}

> The FUTURE DIRECTIONS section is added.

#### []{#tag_20_42_23}Issue 6 {#issue-6 .mansect}

> The *expr* utility is aligned with the IEEE P1003.2b draft standard,
> to include resolution of IEEE PASC Interpretation 1003.2 #104.
>
> The normative text is reworded to avoid use of the term \"must\" for
> application requirements.

#### []{#tag_20_42_24}Issue 7 {#issue-7 .mansect}

> Austin Group Interpretation 1003.1-2001 #036 is applied, clarifying
> the behavior for BREs.
>
> The SYNOPSIS and OPERANDS sections are revised to explicitly state
> that the name of each of the operands is *operand*.
>
> POSIX.1-2008, Technical Corrigendum 2, XCU/TC2-2008/0094 \[942\],
> XCU/TC2-2008/0095 \[709\], XCU/TC2-2008/0096 \[942\],
> XCU/TC2-2008/0097 \[963\], and XCU/TC2-2008/0098 \[942\] are applied.

