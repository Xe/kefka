#### []{#tag_20_55_01}NAME {#name .mansect}

> grep - search a file for a pattern

#### []{#tag_20_55_02}SYNOPSIS {#synopsis .mansect}

> `grep`` `**`[`**`-E|-F`**`] [`**`-c|-l|-q`**`] [`**`-insvx`**`]`**` ``-e`` `*`pattern_list`\*
> ` ``      `` `**`[`**`-e`` `*`pattern_list`***`]`**`...`` `**`[`**`-f`` `*`pattern_file`***`]`**`...`` `**`[`***`file`*`...`**`]`**\
> \
> `grep`` `**`[`**`-E|-F`**`] [`**`-c|-l|-q`**`] [`**`-insvx`**`] [`**`-e`` `*`pattern_list`***`]...`\**
> ` ``      `` ``-f`` `*`pattern_file`*` `**`[`**`-f`` `*`pattern_file`***`]`**`...`` `**`[`***`file`*`...`**`]`**\
> \
> `grep`` `**`[`**`-E|-F`**`] [`**`-c|-l|-q`**`] [`**`-insvx`**`]`**` `*`pattern_list`*` `**`[`***`file`*`...`**`]`**\

#### []{#tag_20_55_03}DESCRIPTION {#description .mansect}

> The *grep* utility shall search the input files, selecting lines
> matching one or more patterns; the types of patterns are controlled by
> the options specified. The patterns are specified by the **-e**
> option, **-f** option, or the *pattern_list* operand. The
> *pattern_list*\'s value shall consist of one or more patterns
> separated by \<newline\> characters; the *pattern_file*\'s contents
> shall consist of one or more patterns terminated by a \<newline\>
> character. By default, an input line shall be selected if any pattern,
> treated as an entire basic regular expression (BRE) as described in
> XBD [*Basic Regular
> Expressions*](../basedefs/V1_chap09.html#tag_09_03), matches any part
> of the line excluding the terminating \<newline\>; a null BRE shall
> match every line. By default, each selected input line shall be
> written to the standard output.
>
> Regular expression matching shall be based on text lines. Since a
> \<newline\> separates or terminates patterns (see the **-e** and
> **-f** options below), regular expressions cannot contain a
> \<newline\>. Similarly, since patterns are matched against individual
> lines (excluding the terminating \<newline\> characters) of the input,
> there is no way for a pattern to match a \<newline\> found in the
> input.

#### []{#tag_20_55_04}OPTIONS {#options .mansect}

> The *grep* utility shall conform to XBD [*Utility Syntax
> Guidelines*](../basedefs/V1_chap12.html#tag_12_02) .
>
> The following options shall be supported:
>
> **-E**
> :   Match using extended regular expressions. Treat each pattern
>     specified as an ERE, as described in XBD [*Extended Regular
>     Expressions*](../basedefs/V1_chap09.html#tag_09_04). If any entire
>     ERE pattern matches some part of an input line excluding the
>     terminating \<newline\>, the line shall be matched. A null ERE
>     shall match every line.
>
> **-F**
> :   Match using fixed strings. Treat each pattern specified as a
>     string instead of a regular expression. If an input line contains
>     any of the patterns as a contiguous sequence of bytes, the line
>     shall be matched. A null string shall match every line.
>
> **-c**
> :   Write only a count of selected lines to standard output.
>
> **-e ** *pattern_list*
> :   Specify one or more patterns to be used during the search for
>     input. The application shall ensure that patterns in
>     *pattern_list* are separated by a \<newline\>. A null pattern can
>     be specified by two adjacent \<newline\> characters in
>     *pattern_list*. Unless the **-E** or **-F** option is also
>     specified, each pattern shall be treated as a BRE, as described in
>     XBD [*Basic Regular
>     Expressions*](../basedefs/V1_chap09.html#tag_09_03). Multiple
>     **-e** and **-f** options shall be accepted by the *grep* utility.
>     All of the specified patterns shall be used when matching lines,
>     but the order of evaluation is unspecified.
>
> **-f ** *pattern_file*
> :   Read one or more patterns from the file named by the pathname
>     *pattern_file*. Patterns in *pattern_file* shall be terminated by
>     a \<newline\>. A null pattern can be specified by an empty line in
>     *pattern_file*. Unless the **-E** or **-F** option is also
>     specified, each pattern shall be treated as a BRE, as described in
>     XBD [*Basic Regular
>     Expressions*](../basedefs/V1_chap09.html#tag_09_03).
>
> **-i**
> :   Perform pattern matching in searches without regard to case; see
>     XBD [*Regular Expression General
>     Requirements*](../basedefs/V1_chap09.html#tag_09_02).
>
> **-l**
> :   (The letter ell.) Write only the names of files containing
>     selected lines to standard output. Pathnames shall be written once
>     per file searched. If the standard input is searched, a pathname
>     of `"(standard input)"` shall be written, in the POSIX locale. In
>     other locales, `"standard input"` may be replaced by something
>     more appropriate in those locales.
>
> **-n**
> :   Precede each output line by its relative line number in the file,
>     each file starting at line 1. The line number counter shall be
>     reset for each file processed.
>
> **-q**
> :   Quiet. Nothing shall be written to the standard output, regardless
>     of matching lines. Exit with zero status if an input line is
>     selected.
>
> **-s**
> :   Suppress the error messages ordinarily written for nonexistent or
>     unreadable files. Other error messages shall not be suppressed.
>
> **-v**
> :   Select lines not matching any of the specified patterns. If the
>     **-v** option is not specified, selected lines shall be those that
>     match any of the specified patterns.
>
> **-x**
> :   Consider only input lines that use all characters in the line
>     excluding the terminating \<newline\> to match an entire fixed
>     string or regular expression to be matching lines.

#### []{#tag_20_55_05}OPERANDS {#operands .mansect}

> The following operands shall be supported:
>
> *pattern_list*
> :   Specify one or more patterns to be used during the search for
>     input. This operand shall be treated as if it were specified as
>     **-e** *pattern_list*.
>
> *file*
> :   A pathname of a file to be searched for the patterns. If no *file*
>     operands are specified, the standard input shall be used.

#### []{#tag_20_55_06}STDIN {#stdin .mansect}

> The standard input shall be used if no *file* operands are specified,
> and shall be used if a *file* operand is `'-'` and the implementation
> treats the `'-'` as meaning standard input. Otherwise, the standard
> input shall not be used. See the INPUT FILES section.

#### []{#tag_20_55_07}INPUT FILES {#input-files .mansect}

> The input files shall be text files.

#### []{#tag_20_55_08}ENVIRONMENT VARIABLES {#environment-variables .mansect}

> The following environment variables shall affect the execution of
> *grep*:
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
>     multi-byte characters in arguments and input files) and the
>     behavior of character classes within regular expressions.
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

#### []{#tag_20_55_09}ASYNCHRONOUS EVENTS {#asynchronous-events .mansect}

> Default.

#### []{#tag_20_55_10}STDOUT {#stdout .mansect}

> If the **-l** option is in effect, the following shall be written for
> each file containing at least one selected input line:
>
>
>     "%s\n", <file>
>
> Otherwise, if more than one *file* argument appears, and **-q** is not
> specified, the *grep* utility shall prefix each output line by:
>
>
>     "%s:", <file>
>
> The remainder of each output line shall depend on the other options
> specified:
>
> - If the **-c** option is in effect, the remainder of each output line
>   shall contain:
>
>
>       "%d\n", <count>
>
> - Otherwise, if **-c** is not in effect and the **-n** option is in
>   effect, the following shall be written to standard output:
>
>
>       "%d:", <line number>
>
> - Finally, the following shall be written to standard output:
>
>
>       "%s", <selected-line contents>

#### []{#tag_20_55_11}STDERR {#stderr .mansect}

> The standard error shall be used only for diagnostic messages.

#### []{#tag_20_55_12}OUTPUT FILES {#output-files .mansect}

> None.

#### []{#tag_20_55_13}EXTENDED DESCRIPTION {#extended-description .mansect}

> None.

#### []{#tag_20_55_14}EXIT STATUS {#exit-status .mansect}

> The following exit values shall be returned:
>
>  0
> :   One or more lines were selected.
>
>  1
> :   No lines were selected.
>
> \>1
> :   An error occurred.

#### []{#tag_20_55_15}CONSEQUENCES OF ERRORS {#consequences-of-errors .mansect}

> If the **-q** option is specified, the exit status shall be zero if an
> input line is selected, even if an error was detected. Otherwise,
> default actions shall be performed.

------------------------------------------------------------------------

::: box
*The following sections are informative.*
:::

#### []{#tag_20_55_16}APPLICATION USAGE {#application-usage .mansect}

> Care should be taken when using characters in *pattern_list* that may
> also be meaningful to the command interpreter. It is safest to enclose
> the entire *pattern_list* argument in single-quotes:
>
>
>     '...'
>
> The **-e** *pattern_list* option has the same effect as the
> *pattern_list* operand, but is useful when *pattern_list* begins with
> the \<hyphen-minus\> delimiter. It is also useful when it is more
> convenient to provide multiple patterns as separate arguments.
>
> Multiple **-e** and **-f** options are accepted and *grep* uses all of
> the patterns it is given while matching input text lines. (Note that
> the order of evaluation is not specified. If an implementation finds a
> null string as a pattern, it is allowed to use that pattern first,
> matching every line, and effectively ignore any other patterns.)
>
> The **-q** option provides a means of easily determining whether or
> not a pattern (or string) exists in a group of files. When searching
> several files, it provides a performance improvement (because it can
> quit as soon as it finds the first match) and requires less care by
> the user in choosing the set of files to supply as arguments (because
> it exits zero if it finds a match even if *grep* detected an access or
> read error on earlier *file* operands).
>
> When using *grep* to process pathnames, it is recommended that LC_ALL,
> or at least LC_CTYPE and LC_COLLATE, are set to POSIX or C in the
> environment, since pathnames can contain byte sequences that do not
> form valid characters in some locales, in which case the utility\'s
> behavior would be undefined. In the POSIX locale each byte is a valid
> single-byte character, and therefore this problem is avoided.

#### []{#tag_20_55_17}EXAMPLES {#examples .mansect}

> 1.  To find all uses of the word `"Posix"` (in any case) in file
>     **text.mm** and write with line numbers:
>
>
>         grep -i -n posix text.mm
>
> 2.  To find all empty lines in the standard input:
>
>
>         grep ^$
>
>     or:
>
>
>         grep -v .
>
> 3.  Both of the following commands print all lines containing strings
>     `"abc"` or `"def"` or both:
>
>
>         grep -E 'abc|def'
>
>
>         grep -F 'abc
>         def'
>
> 4.  Both of the following commands print all lines matching exactly
>     `"abc"` or `"def"` :
>
>
>         grep -E '^abc$|^def$'
>
>
>         grep -F -x 'abc
>         def'

#### []{#tag_20_55_18}RATIONALE {#rationale .mansect}

> This *grep* has been enhanced in an upwards-compatible way to provide
> the exact functionality of the historical *egrep* and *fgrep* commands
> as well. It was the clear intention of the standard developers to
> consolidate the three *grep*s into a single command.
>
> The old *egrep* and *fgrep* commands are likely to be supported for
> many years to come as implementation extensions, allowing historical
> applications to operate unmodified.
>
> Historical implementations usually silently ignored all but one of
> multiply-specified **-e** and **-f** options, but were not consistent
> as to which specification was actually used.
>
> The **-b** option was omitted from the OPTIONS section because block
> numbers are implementation-defined.
>
> The System V restriction on using **-** to mean standard input was
> omitted.
>
> A definition of action taken when given a null BRE or ERE is
> specified. This is an error condition in some historical
> implementations.
>
> The **-l** option previously indicated that its use was undefined when
> no files were explicitly named. This behavior was historical and
> placed an unnecessary restriction on future implementations. It has
> been removed.
>
> The historical BSD *grep* **-s** option practice is easily duplicated
> by redirecting standard output to **/dev/null**. The **-s** option
> required here is from System V.
>
> The **-x** option, historically available only with *fgrep*, is
> available here for all of the non-obsolescent versions.

#### []{#tag_20_55_19}FUTURE DIRECTIONS {#future-directions .mansect}

> None.

#### []{#tag_20_55_20}SEE ALSO {#see-also .mansect}

> [*sed*](../utilities/sed.html#)
>
> XBD [*Environment Variables*](../basedefs/V1_chap08.html#tag_08),
> [*Regular Expressions*](../basedefs/V1_chap09.html#tag_09), [*Utility
> Syntax Guidelines*](../basedefs/V1_chap12.html#tag_12_02)

#### []{#tag_20_55_21}CHANGE HISTORY {#change-history .mansect}

> First released in Issue 2.

#### []{#tag_20_55_22}Issue 6 {#issue-6 .mansect}

> The Open Group Corrigendum U029/5 is applied, correcting the SYNOPSIS.
>
> The normative text is reworded to avoid use of the term \"must\" for
> application requirements.
>
> IEEE Std 1003.1-2001/Cor 1-2002, item XCU/TC1/D6/28 is applied,
> correcting the examples using the *grep* **-F** option which did not
> match the normative description of the **-F** option.

#### []{#tag_20_55_23}Issue 7 {#issue-7 .mansect}

> Austin Group Interpretation 1003.1-2001 #092 is applied.
>
> SD5-XCU-ERN-97 is applied, updating the SYNOPSIS.
>
> SD5-XCU-ERN-98 is applied, updating the STDOUT section.
>
> POSIX.1-2008, Technical Corrigendum 2, XCU/TC2-2008/0105 \[584\] and
> XCU/TC2-2008/0106 \[663\] are applied.

