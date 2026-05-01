#### []{#tag_20_132_01}NAME {#name .mansect}

> tr - translate characters

#### []{#tag_20_132_02}SYNOPSIS {#synopsis .mansect}

> `tr`` `**`[`**`-c|-C`**`] [`**`-s`**`]`**` `*`string1 string2`*\
> \
> `tr -s`` `**`[`**`-c|-C`**`]`**` `*`string1`*\
> \
> `tr -d`` `**`[`**`-c|-C`**`]`**` `*`string1`*\
> \
> `tr -ds`` `**`[`**`-c|-C`**`]`**` `*`string1 string2`*\

#### []{#tag_20_132_03}DESCRIPTION {#description .mansect}

> The *tr* utility shall copy the standard input to the standard output
> with substitution or deletion of selected characters. The options
> specified and the *string1* and *string2* operands shall control
> translations that occur while copying characters and single-character
> collating elements.

#### []{#tag_20_132_04}OPTIONS {#options .mansect}

> The *tr* utility shall conform to XBD [*Utility Syntax
> Guidelines*](../basedefs/V1_chap12.html#tag_12_02) .
>
> The following options shall be supported:
>
> **-c**
> :   Complement the set of values specified by *string1*. See the
>     EXTENDED DESCRIPTION section.
>
> **-C**
> :   Complement the set of characters specified by *string1*. See the
>     EXTENDED DESCRIPTION section.
>
> **-d**
> :   Delete all occurrences of input characters that are specified by
>     *string1*.
>
> **-s**
> :   Replace instances of repeated characters with a single character,
>     as described in the EXTENDED DESCRIPTION section.

#### []{#tag_20_132_05}OPERANDS {#operands .mansect}

> The following operands shall be supported:
>
> *string1*, *string2*
> :   Translation control strings. Each string shall represent a set of
>     characters to be converted into an array of characters used for
>     the translation. For a detailed description of how the strings are
>     interpreted, see the EXTENDED DESCRIPTION section.

#### []{#tag_20_132_06}STDIN {#stdin .mansect}

> The standard input can be any type of file.

#### []{#tag_20_132_07}INPUT FILES {#input-files .mansect}

> None.

#### []{#tag_20_132_08}ENVIRONMENT VARIABLES {#environment-variables .mansect}

> The following environment variables shall affect the execution of
> *tr*:
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
> :   Determine the locale for the behavior of range expressions and
>     equivalence classes.
>
> *LC_CTYPE*
> :   Determine the locale for the interpretation of sequences of bytes
>     of text data as characters (for example, single-byte as opposed to
>     multi-byte characters in arguments) and the behavior of character
>     classes.
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

#### []{#tag_20_132_09}ASYNCHRONOUS EVENTS {#asynchronous-events .mansect}

> Default.

#### []{#tag_20_132_10}STDOUT {#stdout .mansect}

> The *tr* output shall be identical to the input, with the exception of
> the specified transformations.

#### []{#tag_20_132_11}STDERR {#stderr .mansect}

> The standard error shall be used only for diagnostic messages.

#### []{#tag_20_132_12}OUTPUT FILES {#output-files .mansect}

> None.

#### []{#tag_20_132_13}EXTENDED DESCRIPTION {#extended-description .mansect}

> The operands *string1* and *string2* (if specified) define two arrays
> of characters. The constructs in the following list can be used to
> specify characters or single-character collating elements. If any of
> the constructs result in multi-character collating elements, *tr*
> shall exclude, without a diagnostic, those multi-character elements
> from the resulting array.
>
> *character*
> :   Any character not described by one of the conventions below shall
>     represent itself.
>
> \\*octal*
> :   Octal sequences can be used to represent characters with specific
>     coded values. An octal sequence shall consist of a \<backslash\>
>     followed by the longest sequence of one, two, or three-octal-digit
>     characters (01234567). The sequence shall cause the value whose
>     encoding is represented by the one, two, or three-digit octal
>     integer to be placed into the array. Multi-byte characters require
>     multiple, concatenated escape sequences of this type, including
>     the leading \<backslash\> for each byte.
>
> \\*character*
> :   The \<backslash\>-escape sequences in XBD [*Escape Sequences and
>     Associated Actions*](../basedefs/V1_chap05.html#tagtcjh_2) (
>     `'\\'`, `'\a'`, `'\b'`, `'\f'`, `'\n'`, `'\r'`, `'\t'`, `'\v'` )
>     shall be supported. The results of using any other character,
>     other than an octal digit, following the \<backslash\> are
>     unspecified. Also, if there is no character following the
>     \<backslash\>, the results are unspecified.
>
> *c*-*c*
>
> :   In the POSIX locale, this construct shall represent the range of
>     collating elements between the range endpoints (as long as neither
>     endpoint is an octal sequence of the form \\*octal*), inclusive,
>     as defined by the collation sequence. The characters or collating
>     elements in the range shall be placed in the array in ascending
>     collation sequence. If the second endpoint precedes the starting
>     endpoint in the collation sequence, it is unspecified whether the
>     range of collating elements is empty, or this construct is treated
>     as invalid. In locales other than the POSIX locale, this construct
>     has unspecified behavior.
>
>     If either or both of the range endpoints are octal sequences of
>     the form \\*octal*, this shall represent the range of specific
>     coded values between the two range endpoints, inclusive.
>
> \[:*class*:\]
>
> :   Represents all characters belonging to the defined character
>     class, as defined by the current setting of the *LC_CTYPE* locale
>     category. The following character class names shall be accepted
>     when specified in *string1*:
>
>       ----------- ----------- ----------- ----------- ----------- ------------
>       **alnum**   **blank**   **digit**   **lower**   **punct**   **upper**
>       **alpha**   **cntrl**   **graph**   **print**   **space**   **xdigit**
>       ----------- ----------- ----------- ----------- ----------- ------------
>
>     ^\[[XSI](javascript:open_code('XSI'))\]^ ![\[Option
>     Start\]](../images/opt-start.gif){border="0"} In addition,
>     character class expressions of the form \[: *name*:\] shall be
>     recognized in those locales where the *name* keyword has been
>     given a **charclass** definition in the *LC_CTYPE* category.
>     ![\[Option End\]](../images/opt-end.gif){border="0"}
>
>     When both the **-d** and **-s** options are specified, any of the
>     character class names shall be accepted in *string2*. Otherwise,
>     only character class names **lower** or **upper** are valid in
>     *string2* and then only if the corresponding character class
>     (**upper** and **lower**, respectively) is specified in the same
>     relative position in *string1*. Such a specification shall be
>     interpreted as a request for case conversion. When \[: *lower*:\]
>     appears in *string1* and \[: *upper*:\] appears in *string2*, the
>     arrays shall contain the characters from the **toupper** mapping
>     in the *LC_CTYPE* category of the current locale. When \[:
>     *upper*:\] appears in *string1* and \[: *lower*:\] appears in
>     *string2*, the arrays shall contain the characters from the
>     **tolower** mapping in the *LC_CTYPE* category of the current
>     locale. The first character from each mapping pair shall be in the
>     array for *string1* and the second character from each mapping
>     pair shall be in the array for *string2* in the same relative
>     position.
>
>     Except for case conversion, the characters specified by a
>     character class expression shall be placed in the array in an
>     unspecified order.
>
>     If the name specified for *class* does not define a valid
>     character class in the current locale, the behavior is undefined.
>
> \[=*equiv*=\]
> :   Represents all characters or collating elements belonging to the
>     same equivalence class as *equiv*, as defined by the current
>     setting of the *LC_COLLATE* locale category. An equivalence class
>     expression shall be allowed only in *string1*, or in *string2*
>     when it is being used by the combined **-d** and **-s** options.
>     The characters belonging to the equivalence class shall be placed
>     in the array in an unspecified order.
>
> \[*x*\**n*\]
> :   Represents *n* repeated occurrences of the character *x*. Because
>     this expression is used to map multiple characters to one, it is
>     only valid when it occurs in *string2*. If *n* is omitted or is
>     zero, it shall be interpreted as large enough to extend the
>     *string2*-based sequence to the length of the *string1*-based
>     sequence. If *n* has a leading zero, it shall be interpreted as an
>     octal value. Otherwise, it shall be interpreted as a decimal
>     value.
>
> When the **-d** option is not specified:
>
> - If *string2* is present, each input character found in the array
>   specified by *string1* shall be replaced by the character in the
>   same relative position in the array specified by *string2*. If the
>   array specified by *string2* is shorter that the one specified by
>   *string1*, or if a character occurs more than once in *string1*, the
>   results are unspecified.
>
> - If the **-C** option is specified, the complements of the characters
>   specified by *string1* (the set of all characters in the current
>   character set, as defined by the current setting of *LC_CTYPE,*
>   except for those actually specified in the *string1* operand) shall
>   be placed in the array in ascending collation sequence, as defined
>   by the current setting of *LC_COLLATE.*
>
> - If the **-c** option is specified, the complement of the values
>   specified by *string1* shall be placed in the array in ascending
>   order by binary value.
>
> - Because the order in which characters specified by character class
>   expressions or equivalence class expressions is undefined, such
>   expressions should only be used if the intent is to map several
>   characters into one. An exception is case conversion, as described
>   previously.
>
> When the **-d** option is specified:
>
> - Input characters found in the array specified by *string1* shall be
>   deleted.
>
> - When the **-C** option is specified with **-d**, all characters
>   except those specified by *string1* shall be deleted. The contents
>   of *string2* are ignored, unless the **-s** option is also
>   specified.
>
> - When the **-c** option is specified with **-d**, all values except
>   those specified by *string1* shall be deleted. The contents of
>   *string2* shall be ignored, unless the **-s** option is also
>   specified.
>
> - The same string cannot be used for both the **-d** and the **-s**
>   option; when both options are specified, both *string1* (used for
>   deletion) and *string2* (used for squeezing) shall be required.
>
> When the **-s** option is specified, after any deletions or
> translations have taken place, repeated sequences of the same
> character shall be replaced by one occurrence of the same character,
> if the character is found in the array specified by the last operand.
> If the last operand contains a character class, such as the following
> example:
>
>
>     tr -s '[:space:]'
>
> the last operand\'s array shall contain all of the characters in that
> character class. However, in a case conversion, as described
> previously, such as:
>
>
>     tr -s '[:upper:]' '[:lower:]'
>
> the last operand\'s array shall contain only those characters defined
> as the second characters in each of the **toupper** or **tolower**
> character pairs, as appropriate.
>
> An empty string used for *string1* or *string2* produces undefined
> results.

#### []{#tag_20_132_14}EXIT STATUS {#exit-status .mansect}

> The following exit values shall be returned:
>
>  0
> :   All input was processed successfully.
>
> \>0
> :   An error occurred.

#### []{#tag_20_132_15}CONSEQUENCES OF ERRORS {#consequences-of-errors .mansect}

> Default.

------------------------------------------------------------------------

::: box
*The following sections are informative.*
:::

#### []{#tag_20_132_16}APPLICATION USAGE {#application-usage .mansect}

> If necessary, *string1* and *string2* can be quoted to avoid pattern
> matching by the shell.
>
> If an ordinary digit (representing itself) is to follow an octal
> sequence, the octal sequence must use the full three digits to avoid
> ambiguity.
>
> When *string2* is shorter than *string1*, a difference results between
> historical System V and BSD systems. A BSD system pads *string2* with
> the last character found in *string2*. Thus, it is possible to do the
> following:
>
>
>     tr 0123456789 d
>
> which would translate all digits to the letter `'d'`. Since this area
> is specifically unspecified in this volume of POSIX.1-2017, both the
> BSD and System V behaviors are allowed, but a conforming application
> cannot rely on the BSD behavior. It would have to code the example in
> the following way:
>
>
>     tr 0123456789 '[d*]'
>
> It should be noted that, despite similarities in appearance, the
> string operands used by *tr* are not regular expressions.
>
> Unlike some historical implementations, this definition of the *tr*
> utility correctly processes NUL characters in its input stream. NUL
> characters can be stripped by using:
>
>
>     tr -d '\000'

#### []{#tag_20_132_17}EXAMPLES {#examples .mansect}

> 1.  The following example creates a list of all words in **file1** one
>     per line in **file2**, where a word is taken to be a maximal
>     string of letters.
>
>
>         tr -cs "[:alpha:]" "[\n*]" <file1 >file2
>
> 2.  The next example translates all lowercase characters in **file1**
>     to uppercase and writes the results to standard output.
>
>
>         tr "[:lower:]" "[:upper:]" <file1
>
> 3.  This example uses an equivalence class to identify accented
>     variants of the base character `'e'` in **file1**, which are
>     stripped of diacritical marks and written to **file2**.
>
>
>         tr "[=e=]" "[e*]" <file1 >file2

#### []{#tag_20_132_18}RATIONALE {#rationale .mansect}

> In some early proposals, an explicit option **-n** was added to
> disable the historical behavior of stripping NUL characters from the
> input. It was considered that automatically stripping NUL characters
> from the input was not correct functionality. However, the removal of
> **-n** in a later proposal does not remove the requirement that *tr*
> correctly process NUL characters in its input stream. NUL characters
> can be stripped by using *tr* **-d** \'\\000\'.
>
> Historical implementations of *tr* differ widely in syntax and
> behavior. For example, the BSD version has not needed the bracket
> characters for the repetition sequence. The *tr* utility syntax is
> based more closely on the System V and XPG3 model while attempting to
> accommodate historical BSD implementations. In the case of the short
> *string2* padding, the decision was to unspecify the behavior and
> preserve System V and XPG3 scripts, which might find difficulty with
> the BSD method. The assumption was made that BSD users of *tr* have to
> make accommodations to meet the syntax defined here. Since it is
> possible to use the repetition sequence to duplicate the desired
> behavior, whereas there is no simple way to achieve the System V
> method, this was the correct, if not desirable, approach.
>
> The use of octal values to specify control characters, while having
> historical precedents, is not portable. The introduction of escape
> sequences for control characters should provide the necessary
> portability. It is recognized that this may cause some historical
> scripts to break.
>
> An early proposal included support for multi-character collating
> elements. It was pointed out that, while *tr* does employ some
> syntactical elements from REs, the aim of *tr* is quite different;
> ranges, for example, do not have a similar meaning (\`\`any of the
> chars in the range matches\", *versus* \"translate each character in
> the range to the output counterpart\"). As a result, the previously
> included support for multi-character collating elements has been
> removed. What remains are ranges in current collation order (to
> support, for example, accented characters), character classes, and
> equivalence classes.
>
> In XPG3 the \[: *class*:\] and \[= *equiv*=\] conventions are shown
> with double brackets, as in RE syntax. However, *tr* does not
> implement RE principles; it just borrows part of the syntax.
> Consequently, \[: *class*:\] and \[= *equiv*=\] should be regarded as
> syntactical elements on a par with \[ *x*\* *n*\], which is not an RE
> bracket expression.
>
> The standard developers will consider changes to *tr* that allow it to
> translate characters between different character encodings, or they
> will consider providing a new utility to accomplish this.
>
> On historical System V systems, a range expression requires enclosing
> square-brackets, such as:
>
>
>     tr '[a-z]' '[A-Z]'
>
> However, BSD-based systems did not require the brackets, and this
> convention is used here to avoid breaking large numbers of BSD
> scripts:
>
>
>     tr a-z A-Z
>
> The preceding System V script will continue to work because the
> brackets, treated as regular characters, are translated to themselves.
> However, any System V script that relied on `"a-z"` representing the
> three characters `'a'`, `'-'`, and `'z'` have to be rewritten as
> `"az-"`.
>
> The ISO POSIX-2:1993 standard had a **-c** option that behaved
> similarly to the **-C** option, but did not supply functionality
> equivalent to the **-c** option specified in POSIX.1-2017.
>
> The earlier version also said that octal sequences referred to
> collating elements and could be placed adjacent to each other to
> specify multi-byte characters. However, it was noted that this caused
> ambiguities because *tr* would not be able to tell whether adjacent
> octal sequences were intending to specify multi-byte characters or
> multiple single byte characters. POSIX.1-2017 specifies that octal
> sequences always refer to single byte binary values when used to
> specify an endpoint of a range of collating elements.
>
> Earlier versions of this standard allowed for implementations with
> bytes other than eight bits, but this has been modified in this
> version.

#### []{#tag_20_132_19}FUTURE DIRECTIONS {#future-directions .mansect}

> None.

#### []{#tag_20_132_20}SEE ALSO {#see-also .mansect}

> [*sed*](../utilities/sed.html#)
>
> XBD [*Escape Sequences and Associated
> Actions*](../basedefs/V1_chap05.html#tagtcjh_2), [*Environment
> Variables*](../basedefs/V1_chap08.html#tag_08), [*Utility Syntax
> Guidelines*](../basedefs/V1_chap12.html#tag_12_02)

#### []{#tag_20_132_21}CHANGE HISTORY {#change-history .mansect}

> First released in Issue 2.

#### []{#tag_20_132_22}Issue 6 {#issue-6 .mansect}

> The **-C** operand is added, and the description of the **-c** operand
> is changed to align with the IEEE P1003.2b draft standard.
>
> The normative text is reworded to avoid use of the term \"must\" for
> application requirements.
>
> IEEE Std 1003.1-2001/Cor 2-2004, item XCU/TC2/D6/31 is applied,
> removing text describing behavior on systems with bytes consisting of
> more than eight bits.
>
> IEEE Std 1003.1-2001/Cor 2-2004, item XCU/TC2/D6/32 is applied,
> updating an example in the EXAMPLES section to avoid using unspecified
> behavior.
>
> IEEE Std 1003.1-2001/Cor 2-2004, item XCU/TC2/D6/33 is applied, making
> a correction to the RATIONALE.

#### []{#tag_20_132_23}Issue 7 {#issue-7 .mansect}

> SD5-XCU-ERN-30 is applied.
>
> SD5-XCU-ERN-97 is applied, updating the SYNOPSIS.\
>
> Austin Group Interpretation 1003.1-2001 #132 is applied, adding
> rationale to the \\ *character* construct.
>
> POSIX.1-2008, Technical Corrigendum 1, XCU/TC1-2008/0145 \[325\] is
> applied.
>
> POSIX.1-2008, Technical Corrigendum 2, XCU/TC2-2008/0196 \[663\] is
> applied.

