#### []{#tag_20_48_01}NAME {#name .mansect}

> fold - filter for folding lines

#### []{#tag_20_48_02}SYNOPSIS {#synopsis .mansect}

> `fold`` `**`[`**`-bs`**`] [`**`-w`` `*`width`***`] [`***`file`*`...`**`]`**

#### []{#tag_20_48_03}DESCRIPTION {#description .mansect}

> The *fold* utility is a filter that shall fold lines from its input
> files, breaking the lines to have a maximum of *width* column
> positions (or bytes, if the **-b** option is specified). Lines shall
> be broken by the insertion of a \<newline\> such that each output line
> (referred to later in this section as a *segment*) is the maximum
> width possible that does not exceed the specified number of column
> positions (or bytes). A line shall not be broken in the middle of a
> character. The behavior is undefined if *width* is less than the
> number of columns any single character in the input would occupy.
>
> If the \<carriage-return\>, \<backspace\>, or \<tab\> characters are
> encountered in the input, and the **-b** option is not specified, they
> shall be treated specially:
>
> \<backspace\>
> :   The current count of line width shall be decremented by one,
>     although the count never shall become negative. The *fold* utility
>     shall not insert a \<newline\> immediately before or after any
>     \<backspace\>, unless the following character has a width greater
>     than 1 and would cause the line width to exceed *width*.
>
> \<carriage-return\>
> :   The current count of line width shall be set to zero. The *fold*
>     utility shall not insert a \<newline\> immediately before or after
>     any \<carriage-return\>.
>
> \<tab\>
> :   Each \<tab\> encountered shall advance the column position pointer
>     to the next tab stop. Tab stops shall be at each column position
>     *n* such that *n* modulo 8 equals 1.

#### []{#tag_20_48_04}OPTIONS {#options .mansect}

> The *fold* utility shall conform to XBD [*Utility Syntax
> Guidelines*](../basedefs/V1_chap12.html#tag_12_02) .
>
> The following options shall be supported:
>
> **-b**
> :   Count *width* in bytes rather than column positions.
>
> **-s**
> :   If a segment of a line contains a \<blank\> within the first
>     *width* column positions (or bytes), break the line after the last
>     such \<blank\> meeting the width constraints. If there is no
>     \<blank\> meeting the requirements, the **-s** option shall have
>     no effect for that output segment of the input line.
>
> **-w ** *width*
> :   Specify the maximum line length, in column positions (or bytes if
>     **-b** is specified). The results are unspecified if *width* is
>     not a positive decimal number. The default value shall be 80.

#### []{#tag_20_48_05}OPERANDS {#operands .mansect}

> The following operand shall be supported:
>
> *file*
> :   A pathname of a text file to be folded. If no *file* operands are
>     specified, the standard input shall be used.

#### []{#tag_20_48_06}STDIN {#stdin .mansect}

> The standard input shall be used if no *file* operands are specified,
> and shall be used if a *file* operand is `'-'` and the implementation
> treats the `'-'` as meaning standard input. Otherwise, the standard
> input shall not be used. See the INPUT FILES section.

#### []{#tag_20_48_07}INPUT FILES {#input-files .mansect}

> If the **-b** option is specified, the input files shall be text files
> except that the lines are not limited to {LINE_MAX} bytes in length.
> If the **-b** option is not specified, the input files shall be text
> files.

#### []{#tag_20_48_08}ENVIRONMENT VARIABLES {#environment-variables .mansect}

> The following environment variables shall affect the execution of
> *fold*:
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
>     multi-byte characters in arguments and input files), and for the
>     determination of the width in column positions each character
>     would occupy on a constant-width font output device.
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

#### []{#tag_20_48_09}ASYNCHRONOUS EVENTS {#asynchronous-events .mansect}

> Default.

#### []{#tag_20_48_10}STDOUT {#stdout .mansect}

> The standard output shall be a file containing a sequence of
> characters whose order shall be preserved from the input files,
> possibly with inserted \<newline\> characters.

#### []{#tag_20_48_11}STDERR {#stderr .mansect}

> The standard error shall be used only for diagnostic messages.

#### []{#tag_20_48_12}OUTPUT FILES {#output-files .mansect}

> None.

#### []{#tag_20_48_13}EXTENDED DESCRIPTION {#extended-description .mansect}

> None.

#### []{#tag_20_48_14}EXIT STATUS {#exit-status .mansect}

> The following exit values shall be returned:
>
>  0
> :   All input files were processed successfully.
>
> \>0
> :   An error occurred.

#### []{#tag_20_48_15}CONSEQUENCES OF ERRORS {#consequences-of-errors .mansect}

> Default.

------------------------------------------------------------------------

::: box
*The following sections are informative.*
:::

#### []{#tag_20_48_16}APPLICATION USAGE {#application-usage .mansect}

> The [*cut*](../utilities/cut.html) and *fold* utilities can be used to
> create text files out of files with arbitrary line lengths. The
> [*cut*](../utilities/cut.html) utility should be used when the number
> of lines (or records) needs to remain constant. The *fold* utility
> should be used when the contents of long lines need to be kept
> contiguous.
>
> The *fold* utility is frequently used to send text files to printers
> that truncate, rather than fold, lines wider than the printer is able
> to print (usually 80 or 132 column positions).

#### []{#tag_20_48_17}EXAMPLES {#examples .mansect}

> An example invocation that submits a file of possibly long lines to
> the printer (under the assumption that the user knows the line width
> of the printer to be assigned by [*lp*](../utilities/lp.html)):
>
>
>     fold -w 132 bigfile | lp

#### []{#tag_20_48_18}RATIONALE {#rationale .mansect}

> Although terminal input in canonical processing mode requires the
> erase character (frequently set to \<backspace\>) to erase the
> previous character (not byte or column position), terminal output is
> not buffered and is extremely difficult, if not impossible, to parse
> correctly; the interpretation depends entirely on the physical device
> that actually displays/prints/stores the output. In all known
> internationalized implementations, the utilities producing output for
> mixed column-width output assume that a \<backspace\> character backs
> up one column position and outputs enough \<backspace\> characters to
> return to the start of the character when \<backspace\> is used to
> provide local line motions to support underlining and emboldening
> operations. Since *fold* without the **-b** option is dealing with
> these same constraints, \<backspace\> is always treated as backing up
> one column position rather than backing up one character.
>
> Historical versions of the *fold* utility assumed 1 byte was one
> character and occupied one column position when written out. This is
> no longer always true. Since the most common usage of *fold* is
> believed to be folding long lines for output to limited-length output
> devices, this capability was preserved as the default case. The **-b**
> option was added so that applications could *fold* files with
> arbitrary length lines into text files that could then be processed by
> the standard utilities. Note that although the width for the **-b**
> option is in bytes, a line is never split in the middle of a
> character. (It is unspecified what happens if a width is specified
> that is too small to hold a single character found in the input
> followed by a \<newline\>.)
>
> The tab stops are hardcoded to be every eighth column to meet
> historical practice. No new method of specifying other tab stops was
> invented.

#### []{#tag_20_48_19}FUTURE DIRECTIONS {#future-directions .mansect}

> None.

#### []{#tag_20_48_20}SEE ALSO {#see-also .mansect}

> [*cut*](../utilities/cut.html#)
>
> XBD [*Environment Variables*](../basedefs/V1_chap08.html#tag_08),
> [*Utility Syntax Guidelines*](../basedefs/V1_chap12.html#tag_12_02)

#### []{#tag_20_48_21}CHANGE HISTORY {#change-history .mansect}

> First released in Issue 4.

#### []{#tag_20_48_22}Issue 6 {#issue-6 .mansect}

> The normative text is reworded to avoid use of the term \"must\" for
> application requirements.

#### []{#tag_20_48_23}Issue 7 {#issue-7 .mansect}

> Austin Group Interpretation 1003.1-2001 #092 is applied.\
>
> Austin Group Interpretation 1003.1-2001 #204 is applied, updating the
> DESCRIPTION to clarify when a \<newline\> can be inserted before or
> after a \<backspace\>.
>
> SD5-XCU-ERN-97 is applied, updating the SYNOPSIS.

