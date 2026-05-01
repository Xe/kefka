#### []{#tag_20_41_01}NAME {#name .mansect}

> expand - convert tabs to spaces

#### []{#tag_20_41_02}SYNOPSIS {#synopsis .mansect}

> `expand`` `**`[`**`-t`` `*`tablist`***`] [`***`file`*`...`**`]`**

#### []{#tag_20_41_03}DESCRIPTION {#description .mansect}

> The *expand* utility shall write files or the standard input to the
> standard output with \<tab\> characters replaced with one or more
> \<space\> characters needed to pad to the next tab stop. Any
> \<backspace\> characters shall be copied to the output and cause the
> column position count for tab stop calculations to be decremented; the
> column position count shall not be decremented below zero.

#### []{#tag_20_41_04}OPTIONS {#options .mansect}

> The *expand* utility shall conform to XBD [*Utility Syntax
> Guidelines*](../basedefs/V1_chap12.html#tag_12_02).
>
> The following option shall be supported:
>
> **-t ** *tablist*
>
> :   Specify the tab stops. The application shall ensure that the
>     argument *tablist* consists of either a single positive decimal
>     integer or a list of tabstops. If a single number is given, tabs
>     shall be set that number of column positions apart instead of the
>     default 8.
>
>     If a list of tabstops is given, the application shall ensure that
>     it consists of a list of two or more positive decimal integers,
>     separated by \<blank\> or \<comma\> characters, in ascending
>     order. The \<tab\> characters shall be set at those specific
>     column positions. Each tab stop *N* shall be an integer value
>     greater than zero, and the list is in strictly ascending order.
>     This is taken to mean that, from the start of a line of output,
>     tabbing to position *N* shall cause the next character output to
>     be in the (*N*+1)th column position on that line.
>
>     In the event of *expand* having to process a \<tab\> at a position
>     beyond the last of those specified in a multiple tab-stop list,
>     the \<tab\> shall be replaced by a single \<space\> in the output.

#### []{#tag_20_41_05}OPERANDS {#operands .mansect}

> The following operand shall be supported:
>
> *file*
> :   The pathname of a text file to be used as input.

#### []{#tag_20_41_06}STDIN {#stdin .mansect}

> See the INPUT FILES section.

#### []{#tag_20_41_07}INPUT FILES {#input-files .mansect}

> Input files shall be text files.

#### []{#tag_20_41_08}ENVIRONMENT VARIABLES {#environment-variables .mansect}

> The following environment variables shall affect the execution of
> *expand*:
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
>     multi-byte characters in arguments and input files), the
>     processing of \<tab\> and \<space\> characters, and for the
>     determination of the width in column positions each character
>     would occupy on an output device.
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

#### []{#tag_20_41_09}ASYNCHRONOUS EVENTS {#asynchronous-events .mansect}

> Default.

#### []{#tag_20_41_10}STDOUT {#stdout .mansect}

> The standard output shall be equivalent to the input files with
> \<tab\> characters converted into the appropriate number of \<space\>
> characters.

#### []{#tag_20_41_11}STDERR {#stderr .mansect}

> The standard error shall be used only for diagnostic messages.

#### []{#tag_20_41_12}OUTPUT FILES {#output-files .mansect}

> None.

#### []{#tag_20_41_13}EXTENDED DESCRIPTION {#extended-description .mansect}

> None.

#### []{#tag_20_41_14}EXIT STATUS {#exit-status .mansect}

> The following exit values shall be returned:
>
>  0
> :   Successful completion
>
> \>0
> :   An error occurred.

#### []{#tag_20_41_15}CONSEQUENCES OF ERRORS {#consequences-of-errors .mansect}

> The *expand* utility shall terminate with an error message and
> non-zero exit status upon encountering difficulties accessing one of
> the *file* operands.

------------------------------------------------------------------------

::: box
*The following sections are informative.*
:::

#### []{#tag_20_41_16}APPLICATION USAGE {#application-usage .mansect}

> None.

#### []{#tag_20_41_17}EXAMPLES {#examples .mansect}

> None.

#### []{#tag_20_41_18}RATIONALE {#rationale .mansect}

> The *expand* utility is useful for preprocessing text files (before
> sorting, looking at specific columns, and so on) that contain \<tab\>
> characters.
>
> See XBD [*Column Position*](../basedefs/V1_chap03.html#tag_03_103).
>
> The *tablist* option-argument consists of integers in ascending order.
> Utility Syntax Guideline 8 mandates that *expand* shall accept the
> integers (within the single argument) separated using either \<comma\>
> or \<blank\> characters.
>
> Earlier versions of this standard allowed the following form in the
> SYNOPSIS:
>
>
>     expand [-tabstop][-tab1,tab2,...,tabn][file ...]
>
> This form is no longer specified by POSIX.1-2017 but may be present in
> some implementations.

#### []{#tag_20_41_19}FUTURE DIRECTIONS {#future-directions .mansect}

> None.

#### []{#tag_20_41_20}SEE ALSO {#see-also .mansect}

> [*tabs*](../utilities/tabs.html#),
> [*unexpand*](../utilities/unexpand.html#)
>
> XBD [*Column Position*](../basedefs/V1_chap03.html#tag_03_103),
> [*Environment Variables*](../basedefs/V1_chap08.html#tag_08),
> [*Utility Syntax Guidelines*](../basedefs/V1_chap12.html#tag_12_02)

#### []{#tag_20_41_21}CHANGE HISTORY {#change-history .mansect}

> First released in Issue 4.

#### []{#tag_20_41_22}Issue 6 {#issue-6 .mansect}

> This utility is marked as part of the User Portability Utilities
> option.
>
> The APPLICATION USAGE section is added.
>
> The obsolescent SYNOPSIS is removed.
>
> The *LC_CTYPE* environment variable description is updated to align
> with the IEEE P1003.2b draft standard.
>
> The normative text is reworded to avoid use of the term \"must\" for
> application requirements.

#### []{#tag_20_41_23}Issue 7 {#issue-7 .mansect}

> Austin Group Interpretation 1003.1-2001 #027 is applied.
>
> SD5-XCU-ERN-97 is applied, updating the SYNOPSIS.
>
> The *expand* utility is moved from the User Portability Utilities
> option to the Base. User Portability Utilities is now an option for
> interactive utilities.

