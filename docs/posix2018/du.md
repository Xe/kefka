#### []{#tag_20_36_01}NAME {#name .mansect}

> du - estimate file space usage

#### []{#tag_20_36_02}SYNOPSIS {#synopsis .mansect}

> `du`` `**`[`**`-a|-s`**`] [`**`-kx`**`] [`**`-H|-L`**`] [`***`file`*`...`**`]`**

#### []{#tag_20_36_03}DESCRIPTION {#description .mansect}

> By default, the *du* utility shall write to standard output the size
> of the file space allocated to, and the size of the file space
> allocated to each subdirectory of, the file hierarchy rooted in each
> of the specified files. By default, when a symbolic link is
> encountered on the command line or in the file hierarchy, *du* shall
> count the size of the symbolic link (rather than the file referenced
> by the link), and shall not follow the link to another portion of the
> file hierarchy. The size of the file space allocated to a file of type
> directory shall be defined as the sum total of space allocated to all
> files in the file hierarchy rooted in the directory plus the space
> allocated to the directory itself.
>
> When *du* cannot [*stat*()](../functions/stat.html) files or
> [*stat*()](../functions/stat.html) or read directories, it shall
> report an error condition and the final exit status is affected. A
> file that occurs multiple times under one file operand and that has a
> link count greater than 1 shall be counted and written for only one
> entry. It is implementation-defined whether a file that has a link
> count no greater than 1 is counted and written just once, or is
> counted and written for each occurrence. It is implementation-defined
> whether a file that occurs under one file operand is counted for other
> file operands. The directory entry that is selected in the report is
> unspecified. By default, file sizes shall be written in 512-byte
> units, rounded up to the next 512-byte unit.

#### []{#tag_20_36_04}OPTIONS {#options .mansect}

> The *du* utility shall conform to XBD [*Utility Syntax
> Guidelines*](../basedefs/V1_chap12.html#tag_12_02) .
>
> The following options shall be supported:
>
> **-a**
> :   In addition to the default output, report the size of each file
>     not of type directory in the file hierarchy rooted in the
>     specified file. The **-a** option shall not affect whether
>     non-directories given as *file* operands are listed.
>
> **-H**
> :   If a symbolic link is specified on the command line, *du* shall
>     count the size of the file or file hierarchy referenced by the
>     link.
>
> **-k**
> :   Write the files sizes in units of 1024 bytes, rather than the
>     default 512-byte units.
>
> **-L**
> :   If a symbolic link is specified on the command line or encountered
>     during the traversal of a file hierarchy, *du* shall count the
>     size of the file or file hierarchy referenced by the link.
>
> **-s**
> :   Instead of the default output, report only the total sum for each
>     of the specified files.
>
> **-x**
> :   When evaluating file sizes, evaluate only those files that have
>     the same device as the file specified by the *file* operand.
>
> Specifying more than one of the mutually-exclusive options **-H** and
> **-L** shall not be considered an error. The last option specified
> shall determine the behavior of the utility.

#### []{#tag_20_36_05}OPERANDS {#operands .mansect}

> The following operand shall be supported:
>
> *file*
> :   The pathname of a file whose size is to be written. If no *file*
>     is specified, the current directory shall be used.

#### []{#tag_20_36_06}STDIN {#stdin .mansect}

> Not used.

#### []{#tag_20_36_07}INPUT FILES {#input-files .mansect}

> None.

#### []{#tag_20_36_08}ENVIRONMENT VARIABLES {#environment-variables .mansect}

> The following environment variables shall affect the execution of
> *du*:
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

#### []{#tag_20_36_09}ASYNCHRONOUS EVENTS {#asynchronous-events .mansect}

> Default.

#### []{#tag_20_36_10}STDOUT {#stdout .mansect}

> The output from *du* shall consist of the amount of space allocated to
> a file and the name of the file, in the following format:
>
>
>     "%d %s\n", <size>, <pathname>

#### []{#tag_20_36_11}STDERR {#stderr .mansect}

> The standard error shall be used only for diagnostic messages.

#### []{#tag_20_36_12}OUTPUT FILES {#output-files .mansect}

> None.

#### []{#tag_20_36_13}EXTENDED DESCRIPTION {#extended-description .mansect}

> None.

#### []{#tag_20_36_14}EXIT STATUS {#exit-status .mansect}

> The following exit values shall be returned:
>
>  0
> :   Successful completion.
>
> \>0
> :   An error occurred.

#### []{#tag_20_36_15}CONSEQUENCES OF ERRORS {#consequences-of-errors .mansect}

> Default.

------------------------------------------------------------------------

::: box
*The following sections are informative.*
:::

#### []{#tag_20_36_16}APPLICATION USAGE {#application-usage .mansect}

> None.

#### []{#tag_20_36_17}EXAMPLES {#examples .mansect}

> None.

#### []{#tag_20_36_18}RATIONALE {#rationale .mansect}

> The use of 512-byte units is historical practice and maintains
> compatibility with [*ls*](../utilities/ls.html) and other utilities in
> this volume of POSIX.1-2017. This does not mandate that the file
> system itself be based on 512-byte blocks. The **-k** option was added
> as a compromise measure. It was agreed by the standard developers that
> 512 bytes was the best default unit because of its complete historical
> consistency on System V (*versus* the mixed 512/1024-byte usage on BSD
> systems), and that a **-k** option to switch to 1024-byte units was a
> good compromise. Users who prefer the 1024-byte quantity can easily
> alias *du* to *du* **-k** without breaking the many historical scripts
> relying on the 512-byte units.
>
> The **-b** option was added to an early proposal to provide a
> resolution to the situation where System V and BSD systems give
> figures for file sizes in *blocks*, which is an implementation-defined
> concept. (In common usage, the block size is 512 bytes for System V
> and 1024 bytes for BSD systems.) However, **-b** was later deleted,
> since the default was eventually decided as 512-byte units.
>
> Historical file systems provided no way to obtain exact figures for
> the space allocation given to files. There are two known areas of
> inaccuracies in historical file systems: cases of *indirect blocks*
> being used by the file system or *sparse* files yielding incorrectly
> high values. An indirect block is space used by the file system in the
> storage of the file, but that need not be counted in the space
> allocated to the file. A *sparse* file is one in which an
> [*lseek*()](../functions/lseek.html) call has been made to a position
> beyond the end of the file and data has subsequently been written at
> that point. A file system need not allocate all the intervening
> zero-filled blocks to such a file. It is up to the implementation to
> define exactly how accurate its methods are.
>
> The **-a** and **-s** options were mutually-exclusive in the original
> version of *du*. The POSIX Shell and Utilities description is implied
> by the language in the SVID where **-s** is described as causing
> \"only the grand total\" to be reported. Some systems may produce
> output for **-sa**, but a Strictly Conforming POSIX Shell and
> Utilities Application cannot use that combination.
>
> The **-a** and **-s** options were adopted from the SVID except that
> the System V behavior of not listing non-directories explicitly given
> as operands, unless the **-a** option is specified, was considered a
> bug; the BSD-based behavior (report for all operands) is mandated. The
> default behavior of *du* in the SVID with regard to reporting the
> failure to read files (it produces no messages) was considered
> counter-intuitive, and thus it was specified that the POSIX Shell and
> Utilities default behavior shall be to produce such messages. These
> messages can be turned off with shell redirection to achieve the
> System V behavior.
>
> The **-x** option is historical practice on recent BSD systems. It has
> been adopted by this volume of POSIX.1-2017 because there was no other
> historical method of limiting the *du* search to a single file
> hierarchy. This limitation of the search is necessary to make it
> possible to obtain file space usage information about a file system on
> which other file systems are mounted, without having to resort to a
> lengthy [*find*](../utilities/find.html) and
> [*awk*](../utilities/awk.html) script.

#### []{#tag_20_36_19}FUTURE DIRECTIONS {#future-directions .mansect}

> A future version of this standard may require that a file that occurs
> multiple times shall be counted and written for only one entry, even
> if the occurrences are under different file operands.

#### []{#tag_20_36_20}SEE ALSO {#see-also .mansect}

> [*ls*](../utilities/ls.html#)
>
> XBD [*Environment Variables*](../basedefs/V1_chap08.html#tag_08),
> [*Utility Syntax Guidelines*](../basedefs/V1_chap12.html#tag_12_02)
>
> XSH [*fstatat*](../functions/fstatat.html#)

#### []{#tag_20_36_21}CHANGE HISTORY {#change-history .mansect}

> First released in Issue 2.

#### []{#tag_20_36_22}Issue 6 {#issue-6 .mansect}

> This utility is marked as part of the User Portability Utilities
> option.
>
> The APPLICATION USAGE section is added.
>
> The obsolescent **-r** option is removed.
>
> The Open Group Corrigendum U025/3 is applied. The *du* utility is
> reinstated, as it had incorrectly been marked LEGACY in Issue 5.
>
> The **-H** and **-L** options for symbolic links are added as
> described in the IEEE P1003.2b draft standard.

#### []{#tag_20_36_23}Issue 7 {#issue-7 .mansect}

> The *du* utility is moved from the User Portability Utilities option
> to the Base. User Portability Utilities is now an option for
> interactive utilities.
>
> SD5-XCU-ERN-97 is applied, updating the SYNOPSIS.
>
> POSIX.1-2008, Technical Corrigendum 2, XCU/TC2-2008/0089 \[527\] is
> applied.

