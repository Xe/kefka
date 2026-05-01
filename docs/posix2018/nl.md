#### []{#tag_20_85_01}NAME {#name .mansect}

> nl - line numbering filter

#### []{#tag_20_85_02}SYNOPSIS {#synopsis .mansect}

> ::: box
> ^`[`[`XSI`](javascript:open_code('XSI'))`]`^` `![`[Option Start]`](../images/opt-start.gif){border="0"}` nl`` `**`[`**`-p`**`] [`**`-b`` `*`type`***`] [`**`-d`` `*`delim`***`] [`**`-f`` `*`type`***`] [`**`-h`` `*`type`***`] [`**`-i`` `*`incr`***`] [`**`-l`` `*`num`***`]`\**
> ` ``      `` `**`[`**`-n`` `*`format`***`] [`**`-s`` `*`sep`***`] [`**`-v`` `*`startnum`***`] [`**`-w`` `*`width`***`] [`***`file`***`]`**![`[Option End]`](../images/opt-end.gif){border="0"}
> :::

#### []{#tag_20_85_03}DESCRIPTION {#description .mansect}

> The *nl* utility shall read lines from the named *file* or the
> standard input if no *file* is named and shall reproduce the lines to
> standard output. Lines shall be numbered on the left. Additional
> functionality may be provided in accordance with the command options
> in effect.
>
> The *nl* utility views the text it reads in terms of logical pages.
> Line numbering shall be reset at the start of each logical page. A
> logical page consists of a header, a body, and a footer section. Empty
> sections are valid. Different line numbering options are independently
> available for header, body, and footer (for example, no numbering of
> header and footer lines while numbering blank lines only in the body).
>
> The starts of logical page sections shall be signaled by input lines
> containing nothing but the following delimiter characters:
>
>   **Line**    **Start of**
>   ----------- --------------
>   \\:\\:\\:   Header
>   \\:\\:      Body
>   \\:         Footer
>
> Unless otherwise specified, *nl* shall assume the text being read is
> in a single logical page body.

#### []{#tag_20_85_04}OPTIONS {#options .mansect}

> The *nl* utility shall conform to XBD [*Utility Syntax
> Guidelines*](../basedefs/V1_chap12.html#tag_12_02). Only one file can
> be named.
>
> The following options shall be supported:
>
> **-b ** *type*
>
> :   Specify which logical page body lines shall be numbered.
>     Recognized *types* and their meaning are:
>
>     **a**
>     :   Number all lines.
>
>     **t**
>     :   Number only non-empty lines.
>
>     **n**
>     :   No line numbering.
>
>     **p***string*
>     :   Number only lines that contain the basic regular expression
>         specified in *string*.
>
>     The default *type* for logical page body shall be **t** (text
>     lines numbered).
>
> **-d ** *delim*
> :   Specify the delimiter characters that indicate the start of a
>     logical page section. These can be changed from the default
>     characters `"\:"` to two user-specified characters. If only one
>     character is entered, the second character shall remain the
>     default character `':'`.
>
> **-f ** *type*
> :   Specify the same as **b** *type* except for footer. The default
>     for logical page footer shall be **n** (no lines numbered).
>
> **-h ** *type*
> :   Specify the same as **b** *type* except for header. The default
>     *type* for logical page header shall be **n** (no lines numbered).
>
> **-i ** *incr*
> :   Specify the increment value used to number logical page lines. The
>     default shall be 1.
>
> **-l ** *num*
> :   Specify the number of blank lines to be considered as one. For
>     example, **-l 2** results in only the second adjacent blank line
>     being numbered (if the appropriate **-h a**, **-b a**, or **-f a**
>     option is set). The default shall be 1.
>
> **-n ** *format*
> :   Specify the line numbering format. Recognized values are: **ln**,
>     left justified, leading zeros suppressed; **rn**, right justified,
>     leading zeros suppressed; **rz**, right justified, leading zeros
>     kept. The default *format* shall be **rn** (right justified).
>
> **-p**
> :   Specify that numbering should not be restarted at logical page
>     delimiters.
>
> **-s ** *sep*
> :   Specify the characters used in separating the line number and the
>     corresponding text line. The default *sep* shall be a \<tab\>.
>
> **-v ** *startnum*
> :   Specify the initial value used to number logical page lines. The
>     default shall be 1.
>
> **-w ** *width*
> :   Specify the number of characters to be used for the line number.
>     The default *width* shall be 6.

#### []{#tag_20_85_05}OPERANDS {#operands .mansect}

> The following operand shall be supported:
>
> *file*
> :   A pathname of a text file to be line-numbered.

#### []{#tag_20_85_06}STDIN {#stdin .mansect}

> The standard input shall be used if no *file* operand is specified,
> and shall be used if the *file* operand is `'-'` and the
> implementation treats the `'-'` as meaning standard input. Otherwise,
> the standard input shall not be used. See the INPUT FILES section.

#### []{#tag_20_85_07}INPUT FILES {#input-files .mansect}

> The input file shall be a text file.

#### []{#tag_20_85_08}ENVIRONMENT VARIABLES {#environment-variables .mansect}

> The following environment variables shall affect the execution of
> *nl*:
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
>     expressions.
>
> *LC_CTYPE*
> :   Determine the locale for the interpretation of sequences of bytes
>     of text data as characters (for example, single-byte as opposed to
>     multi-byte characters in arguments and input files), the behavior
>     of character classes within regular expressions, and for deciding
>     which characters are in character class **graph** (for the
>     **-b t**, **-f t**, and **-h t** options).
>
> *LC_MESSAGES*
> :   Determine the locale that should be used to affect the format and
>     contents of diagnostic messages written to standard error.
>
> *NLSPATH*
> :   Determine the location of message catalogs for the processing of
>     *LC_MESSAGES.*

#### []{#tag_20_85_09}ASYNCHRONOUS EVENTS {#asynchronous-events .mansect}

> Default.

#### []{#tag_20_85_10}STDOUT {#stdout .mansect}

> The standard output shall be a text file in the following format:
>
>
>     "%s%s%s", <line number>, <separator>, <input line>
>
> where \<*line number*\> is one of the following numeric formats:
>
> `%6d`
> :   When the **rn** format is used (the default; see **-n**).
>
> `%06d`
> :   When the **rz** format is used.
>
> `%-6d`
> :   When the **ln** format is used.
>
> \<empty\>
> :   When line numbers are suppressed for a portion of the page; the
>     \<*separator*\> is also suppressed.
>
> In the preceding list, the number 6 is the default width; the **-w**
> option can change this value.

#### []{#tag_20_85_11}STDERR {#stderr .mansect}

> The standard error shall be used only for diagnostic messages.

#### []{#tag_20_85_12}OUTPUT FILES {#output-files .mansect}

> None.

#### []{#tag_20_85_13}EXTENDED DESCRIPTION {#extended-description .mansect}

> None.

#### []{#tag_20_85_14}EXIT STATUS {#exit-status .mansect}

> The following exit values shall be returned:
>
>  0
> :   Successful completion.
>
> \>0
> :   An error occurred.

#### []{#tag_20_85_15}CONSEQUENCES OF ERRORS {#consequences-of-errors .mansect}

> Default.

------------------------------------------------------------------------

::: box
*The following sections are informative.*
:::

#### []{#tag_20_85_16}APPLICATION USAGE {#application-usage .mansect}

> In using the **-d** *delim* option, care should be taken to escape
> characters that have special meaning to the command interpreter.

#### []{#tag_20_85_17}EXAMPLES {#examples .mansect}

> The command:
>
>
>     nl -v 10 -i 10 -d \!+ file1
>
> numbers *file1* starting at line number 10 with an increment of 10.
> The logical page delimiter is `"!+"`. Note that the `'!'` has to be
> escaped when using *csh* as a command interpreter because of its
> history substitution syntax. For *ksh* and
> [*sh*](../utilities/sh.html) the escape is not necessary, but does not
> do any harm.

#### []{#tag_20_85_18}RATIONALE {#rationale .mansect}

> None.

#### []{#tag_20_85_19}FUTURE DIRECTIONS {#future-directions .mansect}

> None.

#### []{#tag_20_85_20}SEE ALSO {#see-also .mansect}

> [*pr*](../utilities/pr.html#)
>
> XBD [*Environment Variables*](../basedefs/V1_chap08.html#tag_08),
> [*Utility Syntax Guidelines*](../basedefs/V1_chap12.html#tag_12_02)

#### []{#tag_20_85_21}CHANGE HISTORY {#change-history .mansect}

> First released in Issue 2.

#### []{#tag_20_85_22}Issue 5 {#issue-5 .mansect}

> The option \[ **-f** *type*\] is added to the SYNOPSIS. The option
> descriptions are presented in alphabetic order. The description of
> **-bt** is changed to \"Number only non-empty lines\".

#### []{#tag_20_85_23}Issue 6 {#issue-6 .mansect}

> The obsolescent behavior allowing the options to be intermingled with
> the optional *file* operand is removed.

#### []{#tag_20_85_24}Issue 7 {#issue-7 .mansect}

> Austin Group Interpretation 1003.1-2001 #092 is applied.
>
> SD5-XCU-ERN-97 is applied, updating the SYNOPSIS.

