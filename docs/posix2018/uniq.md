#### []{#tag_20_144_01}NAME {#name .mansect}

> uniq - report or filter out repeated lines in a file

#### []{#tag_20_144_02}SYNOPSIS {#synopsis .mansect}

> `uniq`` `**`[`**`-c|-d|-u`**`] [`**`-f`` `*`fields`***`] [`**`-s`` `*`char`***`] [`***`input_file`*` `**`[`***`output_file`***`]]`**

#### []{#tag_20_144_03}DESCRIPTION {#description .mansect}

> The *uniq* utility shall read an input file comparing adjacent lines,
> and write one copy of each input line on the output. The second and
> succeeding copies of repeated adjacent input lines shall not be
> written. The trailing \<newline\> of each line in the input shall be
> ignored when doing comparisons.
>
> Repeated lines in the input shall not be detected if they are not
> adjacent.

#### []{#tag_20_144_04}OPTIONS {#options .mansect}

> The *uniq* utility shall conform to XBD [*Utility Syntax
> Guidelines*](../basedefs/V1_chap12.html#tag_12_02) , except that `'+'`
> may be recognized as an option delimiter as well as `'-'`.
>
> The following options shall be supported:
>
> **-c**
> :   Precede each output line with a count of the number of times the
>     line occurred in the input.
>
> **-d**
> :   Suppress the writing of lines that are not repeated in the input.
>
> **-f ** *fields*
>
> :   Ignore the first *fields* fields on each input line when doing
>     comparisons, where *fields* is a positive decimal integer. A field
>     is the maximal string matched by the basic regular expression:
>
>
>         [[:blank:]]*[^[:blank:]]*
>
>     If the *fields* option-argument specifies more fields than appear
>     on an input line, a null string shall be used for comparison.
>
> **-s ** *chars*
> :   Ignore the first *chars* characters when doing comparisons, where
>     *chars* shall be a positive decimal integer. If specified in
>     conjunction with the **-f** option, the first *chars* characters
>     after the first *fields* fields shall be ignored. If the *chars*
>     option-argument specifies more characters than remain on an input
>     line, a null string shall be used for comparison.
>
> **-u**
> :   Suppress the writing of lines that are repeated in the input.

#### []{#tag_20_144_05}OPERANDS {#operands .mansect}

> The following operands shall be supported:
>
> *input_file*
> :   A pathname of the input file. If the *input_file* operand is not
>     specified, or if the *input_file* is `'-'`, the standard input
>     shall be used.
>
> *output_file*
> :   A pathname of the output file. If the *output_file* operand is not
>     specified, the standard output shall be used. The results are
>     unspecified if the file named by *output_file* is the file named
>     by *input_file*.

#### []{#tag_20_144_06}STDIN {#stdin .mansect}

> The standard input shall be used only if no *input_file* operand is
> specified or if *input_file* is `'-'`. See the INPUT FILES section.

#### []{#tag_20_144_07}INPUT FILES {#input-files .mansect}

> The input file shall be a text file.

#### []{#tag_20_144_08}ENVIRONMENT VARIABLES {#environment-variables .mansect}

> The following environment variables shall affect the execution of
> *uniq*:
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
>     characters constitute a \<blank\> in the current locale.
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

#### []{#tag_20_144_09}ASYNCHRONOUS EVENTS {#asynchronous-events .mansect}

> Default.

#### []{#tag_20_144_10}STDOUT {#stdout .mansect}

> The standard output shall be used if no *output_file* operand is
> specified, and shall be used if the *output_file* operand is `'-'` and
> the implementation treats the `'-'` as meaning standard output.
> Otherwise, the standard output shall not be used. See the OUTPUT FILES
> section.

#### []{#tag_20_144_11}STDERR {#stderr .mansect}

> The standard error shall be used only for diagnostic messages.

#### []{#tag_20_144_12}OUTPUT FILES {#output-files .mansect}

> If the **-c** option is specified, the output file shall be empty or
> each line shall be of the form:
>
>
>     "%d %s", <number of duplicates>, <line>
>
> otherwise, the output file shall be empty or each line shall be of the
> form:
>
>
>     "%s", <line>

#### []{#tag_20_144_13}EXTENDED DESCRIPTION {#extended-description .mansect}

> None.

#### []{#tag_20_144_14}EXIT STATUS {#exit-status .mansect}

> The following exit values shall be returned:
>
>  0
> :   The utility executed successfully.
>
> \>0
> :   An error occurred.

#### []{#tag_20_144_15}CONSEQUENCES OF ERRORS {#consequences-of-errors .mansect}

> Default.

------------------------------------------------------------------------

::: box
*The following sections are informative.*
:::

#### []{#tag_20_144_16}APPLICATION USAGE {#application-usage .mansect}

> If the collating sequence of the current locale has a total ordering
> of all characters, the [*sort*](../utilities/sort.html) utility can be
> used to cause repeated lines to be adjacent in the input file. If the
> collating sequence does not have a total ordering of all characters,
> the [*sort*](../utilities/sort.html) utility should still do this but
> it might not. To ensure that all duplicate lines are eliminated, and
> have the output sorted according the collating sequence of the current
> locale, applications should use:
>
>
>     LC_ALL=C sort -u | sort
>
> instead of:
>
>
>     sort | uniq
>
> To remove duplicate lines based on whether they collate equally
> instead of whether they are identical, applications should use:
>
>
>     sort -u
>
> instead of:
>
>
>     sort | uniq
>
> When using *uniq* to process pathnames, it is recommended that LC_ALL,
> or at least LC_CTYPE and LC_COLLATE, are set to POSIX or C in the
> environment, since pathnames can contain byte sequences that do not
> form valid characters in some locales, in which case the utility\'s
> behavior would be undefined. In the POSIX locale each byte is a valid
> single-byte character, and therefore this problem is avoided.

#### []{#tag_20_144_17}EXAMPLES {#examples .mansect}

> The following input file data (but flushed left) was used for a test
> series on *uniq*:
>
>
>     #01 foo0 bar0 foo1 bar1
>     #02 bar0 foo1 bar1 foo1
>     #03 foo0 bar0 foo1 bar1
>     #04
>     #05 foo0 bar0 foo1 bar1
>     #06 foo0 bar0 foo1 bar1
>     #07 bar0 foo1 bar1 foo0
>
> What follows is a series of test invocations of the *uniq* utility
> that use a mixture of *uniq* options against the input file data.
> These tests verify the meaning of *adjacent*. The *uniq* utility views
> the input data as a sequence of strings delimited by `'\n'`.
> Accordingly, for the *fields*th member of the sequence, *uniq*
> interprets unique or repeated adjacent lines strictly relative to the
> *fields*+1th member.
>
> 1.  This first example tests the line counting option, comparing each
>     line of the input file data starting from the second field:
>
>
>         uniq -c -f 1 uniq_0I.t
>             1 #01 foo0 bar0 foo1 bar1
>             1 #02 bar0 foo1 bar1 foo1
>             1 #03 foo0 bar0 foo1 bar1
>             1 #04
>             2 #05 foo0 bar0 foo1 bar1
>             1 #07 bar0 foo1 bar1 foo0
>
>     The number `'2'`, prefixing the fifth line of output, signifies
>     that the *uniq* utility detected a pair of repeated lines. Given
>     the input data, this can only be true when *uniq* is run using the
>     **-f 1** option (which shall cause *uniq* to ignore the first
>     field on each input line).
>
> 2.  The second example tests the option to suppress unique lines,
>     comparing each line of the input file data starting from the
>     second field:
>
>
>         uniq -d -f 1 uniq_0I.t
>         #05 foo0 bar0 foo1 bar1
>
> 3.  This test suppresses repeated lines, comparing each line of the
>     input file data starting from the second field:
>
>
>         uniq -u -f 1 uniq_0I.t
>         #01 foo0 bar0 foo1 bar1
>         #02 bar0 foo1 bar1 foo1
>         #03 foo0 bar0 foo1 bar1
>         #04
>         #07 bar0 foo1 bar1 foo0
>
> 4.  This suppresses unique lines, comparing each line of the input
>     file data starting from the third character:
>
>
>         uniq -d -s 2 uniq_0I.t
>
>     In the last example, the *uniq* utility found no input matching
>     the above criteria.

#### []{#tag_20_144_18}RATIONALE {#rationale .mansect}

> Some historical implementations have limited lines to be 1080 bytes in
> length, which does not meet the implied {LINE_MAX} limit.
>
> Earlier versions of this standard allowed the **-** *number* and **+**
> *number* options. These options are no longer specified by
> POSIX.1-2017 but may be present in some implementations.

#### []{#tag_20_144_19}FUTURE DIRECTIONS {#future-directions .mansect}

> None.

#### []{#tag_20_144_20}SEE ALSO {#see-also .mansect}

> [*comm*](../utilities/comm.html#), [*sort*](../utilities/sort.html#)
>
> XBD [*Environment Variables*](../basedefs/V1_chap08.html#tag_08),
> [*Utility Syntax Guidelines*](../basedefs/V1_chap12.html#tag_12_02)

#### []{#tag_20_144_21}CHANGE HISTORY {#change-history .mansect}

> First released in Issue 2.

#### []{#tag_20_144_22}Issue 6 {#issue-6 .mansect}

> The obsolescent SYNOPSIS and associated text are removed.
>
> The normative text is reworded to avoid use of the term \"must\" for
> application requirements.
>
> IEEE Std 1003.1-2001/Cor 1-2002, item XCU/TC1/D6/40 is applied, adding
> *LC_COLLATE* to the ENVIRONMENT VARIABLES section, and changing \"the
> application shall ensure that\" in the OUTPUT FILES section.

#### []{#tag_20_144_23}Issue 7 {#issue-7 .mansect}

> Austin Group Interpretation 1003.1-2001 #027 is applied, clarifying
> that `'+'` may be recognized as an option delimiter in the OPTIONS
> section.
>
> Austin Group Interpretation 1003.1-2001 #092 is applied.
>
> Austin Group Interpretation 1003.1-2001 #133 is applied, clarifying
> the behavior of the trailing \<newline\>.
>
> SD5-XCU-ERN-97 is applied, updating the SYNOPSIS.
>
> SD5-XCU-ERN-141 is applied, updating the EXAMPLES section.
>
> POSIX.1-2008, Technical Corrigendum 2, XCU/TC2-2008/0199 \[963\] and
> XCU/TC2-2008/0200 \[663\] are applied.

