#### []{#tag_20_120_01}NAME {#name .mansect}

> split - split a file into pieces

#### []{#tag_20_120_02}SYNOPSIS {#synopsis .mansect}

> `split`` `**`[`**`-l`` `*`line_count`***`] [`**`-a`` `*`suffix_length`***`] [`***`file`*` `**`[`***`name`***`]]`**\
> \
> `split -b`` `*`n`***`[`**`k|m`**`] [`**`-a`` `*`suffix_length`***`] [`***`file`*` `**`[`***`name`***`]]`**\

#### []{#tag_20_120_03}DESCRIPTION {#description .mansect}

> The *split* utility shall read an input file and write zero or more
> output files. The default size of each output file shall be 1000
> lines. The size of the output files can be modified by specification
> of the **-b** or **-l** options. Each output file shall be created
> with a unique suffix. The suffix shall consist of exactly
> *suffix_length* lowercase letters from the POSIX locale. The letters
> of the suffix shall be used as if they were a base-26 digit system,
> with the first suffix to be created consisting of all `'a'`
> characters, the second with a `'b'` replacing the last `'a'`, and so
> on, until a name of all `'z'` characters is created. By default, the
> names of the output files shall be `'x'`, followed by a two-character
> suffix from the character set as described above, starting with
> `"aa"`, `"ab"`, `"ac"`, and so on, and continuing until the suffix
> `"zz"`, for a maximum of 676 files.
>
> If the number of files required exceeds the maximum allowed by the
> suffix length provided, such that the last allowable file would be
> larger than the requested size, the *split* utility shall fail after
> creating the last file with a valid suffix; *split* shall not delete
> the files it created with valid suffixes. If the file limit is not
> exceeded, the last file created shall contain the remainder of the
> input file, and may be smaller than the requested size. If the input
> is an empty file, no output file shall be created and this shall not
> be considered to be an error.

#### []{#tag_20_120_04}OPTIONS {#options .mansect}

> The *split* utility shall conform to XBD [*Utility Syntax
> Guidelines*](../basedefs/V1_chap12.html#tag_12_02).
>
> The following options shall be supported:
>
> **-a ** *suffix_length*
> :   Use *suffix_length* letters to form the suffix portion of the
>     filenames of the split file. If **-a** is not specified, the
>     default suffix length shall be two. If the sum of the *name*
>     operand and the *suffix_length* option-argument would create a
>     filename exceeding {NAME_MAX} bytes, an error shall result;
>     *split* shall exit with a diagnostic message and no files shall be
>     created.
>
> **-b ** *n*
> :   Split a file into pieces *n* bytes in size.
>
> **-b ** *n***k**
> :   Split a file into pieces *n*\*1024 bytes in size.
>
> **-b ** *n***m**
> :   Split a file into pieces *n*\*1048576 bytes in size.
>
> **-l ** *line_count*
> :   Specify the number of lines in each resulting file piece. The
>     *line_count* argument is an unsigned decimal integer. The default
>     is 1000. If the input does not end with a \<newline\>, the partial
>     line shall be included in the last output file.

#### []{#tag_20_120_05}OPERANDS {#operands .mansect}

> The following operands shall be supported:
>
> *file*
> :   The pathname of the ordinary file to be split. If no input file is
>     given or *file* is `'-'`, the standard input shall be used.
>
> *name*
> :   The prefix to be used for each of the files resulting from the
>     split operation. If no *name* argument is given, `'x'` shall be
>     used as the prefix of the output files. The combined length of the
>     basename of *prefix* and *suffix_length* cannot exceed {NAME_MAX}
>     bytes. See the OPTIONS section.

#### []{#tag_20_120_06}STDIN {#stdin .mansect}

> See the INPUT FILES section.

#### []{#tag_20_120_07}INPUT FILES {#input-files .mansect}

> Any file can be used as input.

#### []{#tag_20_120_08}ENVIRONMENT VARIABLES {#environment-variables .mansect}

> The following environment variables shall affect the execution of
> *split*:
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

#### []{#tag_20_120_09}ASYNCHRONOUS EVENTS {#asynchronous-events .mansect}

> Default.

#### []{#tag_20_120_10}STDOUT {#stdout .mansect}

> Not used.

#### []{#tag_20_120_11}STDERR {#stderr .mansect}

> The standard error shall be used only for diagnostic messages.

#### []{#tag_20_120_12}OUTPUT FILES {#output-files .mansect}

> The output files contain portions of the original input file;
> otherwise, unchanged.

#### []{#tag_20_120_13}EXTENDED DESCRIPTION {#extended-description .mansect}

> None.

#### []{#tag_20_120_14}EXIT STATUS {#exit-status .mansect}

> The following exit values shall be returned:
>
>  0
> :   Successful completion.
>
> \>0
> :   An error occurred.

#### []{#tag_20_120_15}CONSEQUENCES OF ERRORS {#consequences-of-errors .mansect}

> Default.

------------------------------------------------------------------------

::: box
*The following sections are informative.*
:::

#### []{#tag_20_120_16}APPLICATION USAGE {#application-usage .mansect}

> None.

#### []{#tag_20_120_17}EXAMPLES {#examples .mansect}

> In the following examples **foo** is a text file that contains 5000
> lines.
>
> 1.  Create five files, **xaa**, **xab**, **xac**, **xad**, and
>     **xae**:
>
>
>         split foo
>
> 2.  Create five files, but the suffixed portion of the created files
>     consists of three letters, **xaaa**, **xaab**, **xaac**, **xaad**,
>     and **xaae**:
>
>
>         split -a 3 foo
>
> 3.  Create three files with four-letter suffixes and a supplied
>     prefix, **bar_aaaa**, **bar_aaab**, and **bar_aaac**:
>
>
>         split -a 4 -l 2000 foo bar_
>
> 4.  Create as many files as are necessary to contain at most 20\*1024
>     bytes, each with the default prefix of **x** and a five-letter
>     suffix:
>
>
>         split -a 5 -b 20k foo

#### []{#tag_20_120_18}RATIONALE {#rationale .mansect}

> The **-b** option was added to provide a mechanism for splitting files
> other than by lines. While most uses of the **-b** option are for
> transmitting files over networks, some believed it would have
> additional uses.
>
> The **-a** option was added to overcome the limitation of being able
> to create only 676 files.
>
> Consideration was given to deleting this utility, using the rationale
> that the functionality provided by this utility is available via the
> [*csplit*](../utilities/csplit.html) utility (see
> [*csplit*](../utilities/csplit.html#)). Upon reconsideration of the
> purpose of the User Portability Utilities option, it was decided to
> retain both this utility and the [*csplit*](../utilities/csplit.html)
> utility because users use both utilities and have historical
> expectations of their behavior. Furthermore, the splitting on byte
> boundaries in *split* cannot be duplicated with the historical
> [*csplit*](../utilities/csplit.html).
>
> The text \" *split* shall not delete the files it created with valid
> suffixes\" would normally be assumed, but since the related utility,
> [*csplit*](../utilities/csplit.html), does delete files under some
> circumstances, the historical behavior of *split* is made explicit to
> avoid misinterpretation.
>
> Earlier versions of this standard allowed a **-** *line_count* option.
> This form is no longer specified by POSIX.1-2017 but may be present in
> some implementations.

#### []{#tag_20_120_19}FUTURE DIRECTIONS {#future-directions .mansect}

> None.

#### []{#tag_20_120_20}SEE ALSO {#see-also .mansect}

> [*csplit*](../utilities/csplit.html#)
>
> XBD [*Environment Variables*](../basedefs/V1_chap08.html#tag_08),
> [*Utility Syntax Guidelines*](../basedefs/V1_chap12.html#tag_12_02)

#### []{#tag_20_120_21}CHANGE HISTORY {#change-history .mansect}

> First released in Issue 2.

#### []{#tag_20_120_22}Issue 6 {#issue-6 .mansect}

> This utility is marked as part of the User Portability Utilities
> option.
>
> The APPLICATION USAGE section is added.
>
> The obsolescent SYNOPSIS is removed.

#### []{#tag_20_120_23}Issue 7 {#issue-7 .mansect}

> Austin Group Interpretation 1003.1-2001 #027 is applied.
>
> The *split* utility is moved from the User Portability Utilities
> option to the Base. User Portability Utilities is now an option for
> interactive utilities.
>
> SD5-XCU-ERN-97 is applied, updating the SYNOPSIS.
>
> POSIX.1-2008, Technical Corrigendum 2, XCU/TC2-2008/0188 \[731\] is
> applied.

