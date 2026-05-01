#### []{#tag_20_57_01}NAME {#name .mansect}

> head - copy the first part of files

#### []{#tag_20_57_02}SYNOPSIS {#synopsis .mansect}

> `head`` `**`[`**`-n`` `*`number`***`] [`***`file`*`...`**`]`**

#### []{#tag_20_57_03}DESCRIPTION {#description .mansect}

> The *head* utility shall copy its input files to the standard output,
> ending the output for each file at a designated point.
>
> Copying shall end at the point in each input file indicated by the
> **-n** *number* option. The option-argument *number* shall be counted
> in units of lines.

#### []{#tag_20_57_04}OPTIONS {#options .mansect}

> The *head* utility shall conform to XBD [*Utility Syntax
> Guidelines*](../basedefs/V1_chap12.html#tag_12_02) .
>
> The following option shall be supported:
>
> **-n ** *number*
> :   The first *number* lines of each input file shall be copied to
>     standard output. The application shall ensure that the *number*
>     option-argument is a positive decimal integer.
>
> When a file contains less than *number* lines, it shall be copied to
> standard output in its entirety. This shall not be an error.
>
> If no options are specified, *head* shall act as if **-n 10** had been
> specified.

#### []{#tag_20_57_05}OPERANDS {#operands .mansect}

> The following operand shall be supported:
>
> *file*
> :   A pathname of an input file. If no *file* operands are specified,
>     the standard input shall be used.

#### []{#tag_20_57_06}STDIN {#stdin .mansect}

> The standard input shall be used if no *file* operands are specified,
> and shall be used if a *file* operand is `'-'` and the implementation
> treats the `'-'` as meaning standard input. Otherwise, the standard
> input shall not be used. See the INPUT FILES section.

#### []{#tag_20_57_07}INPUT FILES {#input-files .mansect}

> Input files shall be text files, but the line length is not restricted
> to {LINE_MAX} bytes.

#### []{#tag_20_57_08}ENVIRONMENT VARIABLES {#environment-variables .mansect}

> The following environment variables shall affect the execution of
> *head*:
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
>     multi-byte characters in arguments and input files).
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

#### []{#tag_20_57_09}ASYNCHRONOUS EVENTS {#asynchronous-events .mansect}

> Default.

#### []{#tag_20_57_10}STDOUT {#stdout .mansect}

> The standard output shall contain designated portions of the input
> files.
>
> If multiple *file* operands are specified, *head* shall precede the
> output for each with the header:
>
>
>     "\n==> %s <==\n", <pathname>
>
> except that the first header written shall not include the initial
> \<newline\>.

#### []{#tag_20_57_11}STDERR {#stderr .mansect}

> The standard error shall be used only for diagnostic messages.

#### []{#tag_20_57_12}OUTPUT FILES {#output-files .mansect}

> None.

#### []{#tag_20_57_13}EXTENDED DESCRIPTION {#extended-description .mansect}

> None.

#### []{#tag_20_57_14}EXIT STATUS {#exit-status .mansect}

> The following exit values shall be returned:
>
>  0
> :   Successful completion.
>
> \>0
> :   An error occurred.

#### []{#tag_20_57_15}CONSEQUENCES OF ERRORS {#consequences-of-errors .mansect}

> Default.

------------------------------------------------------------------------

::: box
*The following sections are informative.*
:::

#### []{#tag_20_57_16}APPLICATION USAGE {#application-usage .mansect}

> When using *head* to process pathnames, it is recommended that LC_ALL,
> or at least LC_CTYPE and LC_COLLATE, are set to POSIX or C in the
> environment, since pathnames can contain byte sequences that do not
> form valid characters in some locales, in which case the utility\'s
> behavior would be undefined. In the POSIX locale each byte is a valid
> single-byte character, and therefore this problem is avoided.

#### []{#tag_20_57_17}EXAMPLES {#examples .mansect}

> To write the first ten lines of all files (except those with a leading
> period) in the directory:
>
>
>     head -- *

#### []{#tag_20_57_18}RATIONALE {#rationale .mansect}

> Although it is possible to simulate *head* with
> [*sed*](../utilities/sed.html) 10q for a single file, the standard
> developers decided that the popularity of *head* on historical BSD
> systems warranted its inclusion alongside
> [*tail*](../utilities/tail.html).
>
> POSIX.1-2017 version of *head* follows the Utility Syntax Guidelines.
> The **-n** option was added to this new interface so that *head* and
> [*tail*](../utilities/tail.html) would be more logically related.
> Earlier versions of this standard allowed a **-number** option. This
> form is no longer specified by POSIX.1-2017 but may be present in some
> implementations.
>
> There is no **-c** option (as there is in
> [*tail*](../utilities/tail.html)) because it is not historical
> practice and because other utilities in this volume of POSIX.1-2017
> provide similar functionality.

#### []{#tag_20_57_19}FUTURE DIRECTIONS {#future-directions .mansect}

> None.

#### []{#tag_20_57_20}SEE ALSO {#see-also .mansect}

> [*sed*](../utilities/sed.html#), [*tail*](../utilities/tail.html#)
>
> XBD [*Environment Variables*](../basedefs/V1_chap08.html#tag_08),
> [*Utility Syntax Guidelines*](../basedefs/V1_chap12.html#tag_12_02)

#### []{#tag_20_57_21}CHANGE HISTORY {#change-history .mansect}

> First released in Issue 4.

#### []{#tag_20_57_22}Issue 6 {#issue-6 .mansect}

> The obsolescent **-number** form is removed.
>
> The normative text is reworded to avoid use of the term \"must\" for
> application requirements.
>
> The DESCRIPTION is updated to clarify that when a file contains less
> than the number of lines requested, the entire file is copied to
> standard output.

#### []{#tag_20_57_23}Issue 7 {#issue-7 .mansect}

> Austin Group Interpretations 1003.1-2001 #027 and #092 are applied.
>
> SD5-XCU-ERN-97 is applied, updating the SYNOPSIS.
>
> The APPLICATION USAGE section is removed and the EXAMPLES section is
> corrected.
>
> POSIX.1-2008, Technical Corrigendum 2, XCU/TC2-2008/0107 \[663\] is
> applied.

