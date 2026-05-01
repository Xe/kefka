#### []{#tag_20_154_01}NAME {#name .mansect}

> wc - word, line, and byte or character count

#### []{#tag_20_154_02}SYNOPSIS {#synopsis .mansect}

> `wc`` `**`[`**`-c|-m`**`] [`**`-lw`**`] [`***`file`*`...`**`]`**

#### []{#tag_20_154_03}DESCRIPTION {#description .mansect}

> The *wc* utility shall read one or more input files and, by default,
> write the number of \<newline\> characters, words, and bytes contained
> in each input file to the standard output.
>
> The utility also shall write a total count for all named files, if
> more than one input file is specified.
>
> The *wc* utility shall consider a *word* to be a non-zero-length
> string of characters delimited by white space.

#### []{#tag_20_154_04}OPTIONS {#options .mansect}

> The *wc* utility shall conform to XBD [*Utility Syntax
> Guidelines*](../basedefs/V1_chap12.html#tag_12_02) .
>
> The following options shall be supported:
>
> **-c**
> :   Write to the standard output the number of bytes in each input
>     file.
>
> **-l**
> :   Write to the standard output the number of \<newline\> characters
>     in each input file.
>
> **-m**
> :   Write to the standard output the number of characters in each
>     input file.
>
> **-w**
> :   Write to the standard output the number of words in each input
>     file.
>
> When any option is specified, *wc* shall report only the information
> requested by the specified options.

#### []{#tag_20_154_05}OPERANDS {#operands .mansect}

> The following operand shall be supported:
>
> *file*
> :   A pathname of an input file. If no *file* operands are specified,
>     the standard input shall be used.

#### []{#tag_20_154_06}STDIN {#stdin .mansect}

> The standard input shall be used if no *file* operands are specified,
> and shall be used if a *file* operand is `'-'` and the implementation
> treats the `'-'` as meaning standard input. Otherwise, the standard
> input shall not be used. See the INPUT FILES section.

#### []{#tag_20_154_07}INPUT FILES {#input-files .mansect}

> The input files may be of any type.

#### []{#tag_20_154_08}ENVIRONMENT VARIABLES {#environment-variables .mansect}

> The following environment variables shall affect the execution of
> *wc*:
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
> *LC_CTYPE*
> :   Determine the locale for the interpretation of sequences of bytes
>     of text data as characters (for example, single-byte as opposed to
>     multi-byte characters in arguments and input files) and which
>     characters are defined as white-space characters.
>
> *LC_MESSAGES*
> :   Determine the locale that should be used to affect the format and
>     contents of diagnostic messages written to standard error and
>     informative messages written to standard output.
>
> *NLSPATH*
> :   ^\[[XSI](javascript:open_code('XSI'))\]^ ![\[Option
>     Start\]](../images/opt-start.gif){border="0"} Determine the
>     location of message catalogs for the processing of *LC_MESSAGES.*
>     ![\[Option End\]](../images/opt-end.gif){border="0"}

#### []{#tag_20_154_09}ASYNCHRONOUS EVENTS {#asynchronous-events .mansect}

> Default.

#### []{#tag_20_154_10}STDOUT {#stdout .mansect}

> By default, the standard output shall contain an entry for each input
> file of the form:
>
>
>     "%d %d %d %s\n", <newlines>, <words>, <bytes>, <file>
>
> If the **-m** option is specified, the number of characters shall
> replace the \<*bytes*\> field in this format.
>
> If any options are specified and the **-l** option is not specified,
> the number of \<newline\> characters shall not be written.
>
> If any options are specified and the **-w** option is not specified,
> the number of words shall not be written.
>
> If any options are specified and neither **-c** nor **-m** is
> specified, the number of bytes or characters shall not be written.
>
> If no input *file* operands are specified, no name shall be written
> and no \<blank\> characters preceding the pathname shall be written.
>
> If more than one input *file* operand is specified, an additional line
> shall be written, of the same format as the other lines, except that
> the word **total** (in the POSIX locale) shall be written instead of a
> pathname and the total of each column shall be written as appropriate.
> Such an additional line, if any, is written at the end of the output.

#### []{#tag_20_154_11}STDERR {#stderr .mansect}

> The standard error shall be used only for diagnostic messages.

#### []{#tag_20_154_12}OUTPUT FILES {#output-files .mansect}

> None.

#### []{#tag_20_154_13}EXTENDED DESCRIPTION {#extended-description .mansect}

> None.

#### []{#tag_20_154_14}EXIT STATUS {#exit-status .mansect}

> The following exit values shall be returned:
>
>  0
> :   Successful completion.
>
> \>0
> :   An error occurred.

#### []{#tag_20_154_15}CONSEQUENCES OF ERRORS {#consequences-of-errors .mansect}

> Default.

------------------------------------------------------------------------

::: box
*The following sections are informative.*
:::

#### []{#tag_20_154_16}APPLICATION USAGE {#application-usage .mansect}

> The **-m** option is not a switch, but an option at the same level as
> **-c**. Thus, to produce the full default output with character counts
> instead of bytes, the command required is:
>
>
>     wc -mlw

#### []{#tag_20_154_17}EXAMPLES {#examples .mansect}

> None.

#### []{#tag_20_154_18}RATIONALE {#rationale .mansect}

> The output file format pseudo- [*printf*()](../functions/printf.html)
> string differs from the System V version of *wc*:
>
>
>     "%7d%7d%7d %s\n"
>
> which produces possibly ambiguous and unparsable results for very
> large files, as it assumes no number shall exceed six digits.
>
> Some historical implementations use only \<space\>, \<tab\>, and
> \<newline\> as word separators. The equivalent of the ISO C standard
> [*isspace*()](../functions/isspace.html) function is more appropriate.
>
> The **-c** option stands for \"character\" count, even though it
> counts bytes. This stems from the sometimes erroneous historical view
> that bytes and characters are the same size. Due to international
> requirements, the **-m** option (reminiscent of \"multi-byte\") was
> added to obtain actual character counts.
>
> Early proposals only specified the results when input files were text
> files. The current specification more closely matches historical
> practice. (Bytes, words, and \<newline\> characters are counted
> separately and the results are written when an end-of-file is
> detected.)
>
> Historical implementations of the *wc* utility only accepted one
> argument to specify the options **-c**, **-l**, and **-w**. Some of
> them also had multiple occurrences of an option cause the
> corresponding count to be written multiple times and had the order of
> specification of the options affect the order of the fields on output,
> but did not document either of these. Because common usage either
> specifies no options or only one option, and because none of this was
> documented, the changes required by this volume of POSIX.1-2017 should
> not break many historical applications (and do not break any
> historical conforming applications).

#### []{#tag_20_154_19}FUTURE DIRECTIONS {#future-directions .mansect}

> None.

#### []{#tag_20_154_20}SEE ALSO {#see-also .mansect}

> [*cksum*](../utilities/cksum.html#)
>
> XBD [*Environment Variables*](../basedefs/V1_chap08.html#tag_08),
> [*Utility Syntax Guidelines*](../basedefs/V1_chap12.html#tag_12_02)

#### []{#tag_20_154_21}CHANGE HISTORY {#change-history .mansect}

> First released in Issue 2.

#### []{#tag_20_154_22}Issue 7 {#issue-7 .mansect}

> Austin Group Interpretation 1003.1-2001 #092 is applied.
>
> SD5-XCU-ERN-97 is applied, updating the SYNOPSIS.

