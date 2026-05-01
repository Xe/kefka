#### []{#tag_20_13_01}NAME {#name .mansect}

> cat - concatenate and print files

#### []{#tag_20_13_02}SYNOPSIS {#synopsis .mansect}

> `cat`` `**`[`**`-u`**`] [`***`file`*`...`**`]`**

#### []{#tag_20_13_03}DESCRIPTION {#description .mansect}

> The *cat* utility shall read files in sequence and shall write their
> contents to the standard output in the same sequence.

#### []{#tag_20_13_04}OPTIONS {#options .mansect}

> The *cat* utility shall conform to XBD [*Utility Syntax
> Guidelines*](../basedefs/V1_chap12.html#tag_12_02) .
>
> The following option shall be supported:
>
> **-u**
> :   Write bytes from the input file to the standard output without
>     delay as each is read.

#### []{#tag_20_13_05}OPERANDS {#operands .mansect}

> The following operand shall be supported:
>
> *file*
> :   A pathname of an input file. If no *file* operands are specified,
>     the standard input shall be used. If a *file* is `'-'`, the *cat*
>     utility shall read from the standard input at that point in the
>     sequence. The *cat* utility shall not close and reopen standard
>     input when it is referenced in this way, but shall accept multiple
>     occurrences of `'-'` as a *file* operand.

#### []{#tag_20_13_06}STDIN {#stdin .mansect}

> The standard input shall be used only if no *file* operands are
> specified, or if a *file* operand is `'-'`. See the INPUT FILES
> section.

#### []{#tag_20_13_07}INPUT FILES {#input-files .mansect}

> The input files can be any file type.

#### []{#tag_20_13_08}ENVIRONMENT VARIABLES {#environment-variables .mansect}

> The following environment variables shall affect the execution of
> *cat*:
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
>     multi-byte characters in arguments).
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

#### []{#tag_20_13_09}ASYNCHRONOUS EVENTS {#asynchronous-events .mansect}

> Default.

#### []{#tag_20_13_10}STDOUT {#stdout .mansect}

> The standard output shall contain the sequence of bytes read from the
> input files. Nothing else shall be written to the standard output. If
> the standard output is a regular file, and is the same file as any of
> the input file operands, the implementation may treat this as an
> error.

#### []{#tag_20_13_11}STDERR {#stderr .mansect}

> The standard error shall be used only for diagnostic messages.

#### []{#tag_20_13_12}OUTPUT FILES {#output-files .mansect}

> None.

#### []{#tag_20_13_13}EXTENDED DESCRIPTION {#extended-description .mansect}

> None.

#### []{#tag_20_13_14}EXIT STATUS {#exit-status .mansect}

> The following exit values shall be returned:
>
>  0
> :   All input files were output successfully.
>
> \>0
> :   An error occurred.

#### []{#tag_20_13_15}CONSEQUENCES OF ERRORS {#consequences-of-errors .mansect}

> Default.

------------------------------------------------------------------------

::: box
*The following sections are informative.*
:::

#### []{#tag_20_13_16}APPLICATION USAGE {#application-usage .mansect}

> The **-u** option has value in prototyping non-blocking reads from
> FIFOs. The intent is to support the following sequence:
>
>
>     mkfifo foo
>     cat -u foo > /dev/tty13 &
>     cat -u > foo
>
> It is unspecified whether standard output is or is not buffered in the
> default case. This is sometimes of interest when standard output is
> associated with a terminal, since buffering may delay the output. The
> presence of the **-u** option guarantees that unbuffered I/O is
> available. It is implementation-defined whether the *cat* utility
> buffers output if the **-u** option is not specified. Traditionally,
> the **-u** option is implemented using the equivalent of the
> [*setvbuf*()](../functions/setvbuf.html) function defined in the
> System Interfaces volume of POSIX.1-2017.

#### []{#tag_20_13_17}EXAMPLES {#examples .mansect}

> The following command:
>
>
>     cat myfile
>
> writes the contents of the file **myfile** to standard output.
>
> The following command:
>
>
>     cat doc1 doc2 > doc.all
>
> concatenates the files **doc1** and **doc2** and writes the result to
> **doc.all**.
>
> Because of the shell language mechanism used to perform output
> redirection, a command such as this:
>
>
>     cat doc doc.end > doc
>
> causes the original data in **doc** to be lost before *cat* even
> begins execution. This is true whether the *cat* command fails with an
> error or silently succeeds (the specification allows both behaviors).
> In order to append the contents of **doc.end** without losing the
> original contents of **doc**, this command should be used instead:
>
>
>     cat doc.end >> doc
>
> The command:
>
>
>     cat start - middle - end > file
>
> when standard input is a terminal, gets two arbitrary pieces of input
> from the terminal with a single invocation of *cat*. Note, however,
> that if standard input is a regular file, this would be equivalent to
> the command:
>
>
>     cat start - middle /dev/null end > file
>
> because the entire contents of the file would be consumed by *cat* the
> first time `'-'` was used as a *file* operand and an end-of-file
> condition would be detected immediately when `'-'` was referenced the
> second time.

#### []{#tag_20_13_18}RATIONALE {#rationale .mansect}

> Historical versions of the *cat* utility include the **-e**, **-t**,
> and **-v**, options which permit the ends of lines, \<tab\>
> characters, and invisible characters, respectively, to be rendered
> visible in the output. The standard developers omitted these options
> because they provide too fine a degree of control over what is made
> visible, and similar output can be obtained using a command such as:
>
>
>     sed -n l pathname
>
> The latter also has the advantage that its output is unambiguous,
> whereas the output of historical *cat* **-etv** is not.
>
> The **-s** option was omitted because it corresponds to different
> functions in BSD and System V-based systems. The BSD **-s** option to
> squeeze blank lines can be accomplished by the shell script shown in
> the following example:
>
>
>     sed -n '
>     # Write non-empty lines.
>     /./   {
>           p
>           d
>           }
>     # Write a single empty line, then look for more empty lines.
>     /^$/  p
>     # Get next line, discard the held <newline> (empty line),
>     # and look for more empty lines.
>     :Empty
>     /^$/  {
>           N
>           s/.//
>           b Empty
>           }
>     # Write the non-empty line before going back to search
>     # for the first in a set of empty lines.
>           p
>     '
>
> The System V **-s** option to silence error messages can be
> accomplished by redirecting the standard error. Note that the BSD
> documentation for *cat* uses the term \"blank line\" to mean the same
> as the POSIX \"empty line\'\': a line consisting only of a
> \<newline\>.
>
> The BSD **-n** option was omitted because similar functionality can be
> obtained from the **-n** option of the [*pr*](../utilities/pr.html)
> utility.

#### []{#tag_20_13_19}FUTURE DIRECTIONS {#future-directions .mansect}

> None.

#### []{#tag_20_13_20}SEE ALSO {#see-also .mansect}

> [*more*](../utilities/more.html#)
>
> XBD [*Environment Variables*](../basedefs/V1_chap08.html#tag_08),
> [*Utility Syntax Guidelines*](../basedefs/V1_chap12.html#tag_12_02)
>
> XSH [*setvbuf*](../functions/setvbuf.html#)

#### []{#tag_20_13_21}CHANGE HISTORY {#change-history .mansect}

> First released in Issue 2.

#### []{#tag_20_13_22}Issue 7 {#issue-7 .mansect}

> SD5-XCU-ERN-97 is applied, updating the SYNOPSIS.
>
> SD5-XCU-ERN-174 is applied, changing the RATIONALE.
>
> POSIX.1-2008, Technical Corrigendum 2, XCU/TC2-2008/0073 \[876\] is
> applied.

