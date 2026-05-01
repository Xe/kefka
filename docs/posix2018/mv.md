#### []{#tag_20_82_01}NAME {#name .mansect}

> mv - move files

#### []{#tag_20_82_02}SYNOPSIS {#synopsis .mansect}

> `mv`` `**`[`**`-if`**`]`**` `*`source_file target_file`*\
> \
> `mv`` `**`[`**`-if`**`]`**` `*`source_file`*`...`` `*`target_dir`*\

#### []{#tag_20_82_03}DESCRIPTION {#description .mansect}

> In the first synopsis form, the *mv* utility shall move the file named
> by the *source_file* operand to the destination specified by the
> *target_file*. This first synopsis form is assumed when the final
> operand does not name an existing directory and is not a symbolic link
> referring to an existing directory. In this case, if *source_file*
> names a non-directory file and *target_file* ends with a trailing
> \<slash\> character, *mv* shall treat this as an error and no
> *source_file* operands will be processed.
>
> In the second synopsis form, *mv* shall move each file named by a
> *source_file* operand to a destination file in the existing directory
> named by the *target_dir* operand, or referenced if *target_dir* is a
> symbolic link referring to an existing directory. The destination path
> for each *source_file* shall be the concatenation of the target
> directory, a single \<slash\> character if the target did not end in a
> \<slash\>, and the last pathname component of the *source_file*. This
> second form is assumed when the final operand names an existing
> directory.
>
> If any operand specifies an existing file of a type not specified by
> the System Interfaces volume of POSIX.1-2017, the behavior is
> implementation-defined.
>
> For each *source_file* the following steps shall be taken:
>
> 1.  If the destination path exists, the **-f** option is not
>     specified, and either of the following conditions is true:
>
>     a.  The permissions of the destination path do not permit writing
>         and the standard input is a terminal.
>
>     b.  The **-i** option is specified.
>
>     the *mv* utility shall write a prompt to standard error and read a
>     line from standard input. If the response is not affirmative, *mv*
>     shall do nothing more with the current *source_file* and go on to
>     any remaining *source_file*s.
>
> 2.  If the *source_file* operand and destination path resolve to
>     either the same existing directory entry or different directory
>     entries for the same existing file, then the destination path
>     shall not be removed, and one of the following shall occur:
>
>     a.  No change is made to *source_file*, no error occurs, and no
>         diagnostic is issued.
>
>     b.  No change is made to *source_file*, a diagnostic is issued to
>         standard error identifying the two names, and the exit status
>         is affected.
>
>     c.  If the *source_file* operand and destination path name
>         distinct directory entries, then the *source_file* operand is
>         removed, no error occurs, and no diagnostic is issued.
>
>     The *mv* utility shall do nothing more with the current
>     *source_file*, and go on to any remaining *source_file*s.
>
> 3.  The *mv* utility shall perform actions equivalent to the
>     [*rename*()](../functions/rename.html) function defined in the
>     System Interfaces volume of POSIX.1-2017, called with the
>     following arguments:
>
>     a.  The *source_file* operand is used as the *old* argument.
>
>     b.  The destination path is used as the *new* argument.
>
>     If this succeeds, *mv* shall do nothing more with the current
>     *source_file* and go on to any remaining *source_file*s. If this
>     fails for any reasons other than those described for the *errno*
>     \[EXDEV\] in the System Interfaces volume of POSIX.1-2017, *mv*
>     shall write a diagnostic message to standard error, do nothing
>     more with the current *source_file*, and go on to any remaining
>     *source_file*s.
>
> 4.  If the destination path exists, and it is a file of type directory
>     and *source_file* is not a file of type directory, or it is a file
>     not of type directory and *source_file* is a file of type
>     directory, *mv* shall write a diagnostic message to standard
>     error, do nothing more with the current *source_file*, and go on
>     to any remaining *source_file*s. If the destination path exists
>     and was created by a previous step, it is unspecified whether this
>     will treated as an error or the destination path will be
>     overwritten.
>
> 5.  If the destination path exists, *mv* shall attempt to remove it.
>     If this fails for any reason, *mv* shall write a diagnostic
>     message to standard error, do nothing more with the current
>     *source_file*, and go on to any remaining *source_file*s.
>
> 6.  The file hierarchy rooted in *source_file* shall be duplicated as
>     a file hierarchy rooted in the destination path. If *source_file*
>     or any of the files below it in the hierarchy are symbolic links,
>     the links themselves shall be duplicated, including their
>     contents, rather than any files to which they refer. The following
>     characteristics of each file in the file hierarchy shall be
>     duplicated:
>
>     - The time of last data modification and time of last access
>
>     - The user ID and group ID
>
>     - The file mode
>
>     If the user ID, group ID, or file mode of a regular file cannot be
>     duplicated, the file mode bits S_ISUID and S_ISGID shall not be
>     duplicated.
>
>     When files are duplicated to another file system, the
>     implementation may require that the process invoking *mv* has read
>     access to each file being duplicated.
>
>     If files being duplicated to another file system have hard links
>     to other files, it is unspecified whether the files copied to the
>     new file system have the hard links preserved or separate copies
>     are created for the linked files.
>
>     If the duplication of the file hierarchy fails for any reason,
>     *mv* shall write a diagnostic message to standard error, do
>     nothing more with the current *source_file*, and go on to any
>     remaining *source_file*s.
>
>     If the duplication of the file characteristics fails for any
>     reason, *mv* shall write a diagnostic message to standard error,
>     but this failure shall not cause *mv* to modify its exit status.
>
> 7.  The file hierarchy rooted in *source_file* shall be removed. If
>     this fails for any reason, *mv* shall write a diagnostic message
>     to the standard error, do nothing more with the current
>     *source_file*, and go on to any remaining *source_file*s.

#### []{#tag_20_82_04}OPTIONS {#options .mansect}

> The *mv* utility shall conform to XBD [*Utility Syntax
> Guidelines*](../basedefs/V1_chap12.html#tag_12_02) .
>
> The following options shall be supported:
>
> **-f**
> :   Do not prompt for confirmation if the destination path exists. Any
>     previous occurrence of the **-i** option is ignored.
>
> **-i**
> :   Prompt for confirmation if the destination path exists. Any
>     previous occurrence of the **-f** option is ignored.
>
> Specifying more than one of the **-f** or **-i** options shall not be
> considered an error. The last option specified shall determine the
> behavior of *mv*.

#### []{#tag_20_82_05}OPERANDS {#operands .mansect}

> The following operands shall be supported:
>
> *source_file*
> :   A pathname of a file or directory to be moved.
>
> *target_file*
> :   A new pathname for the file or directory being moved.
>
> *target_dir*
> :   A pathname of an existing directory into which to move the input
>     files.

#### []{#tag_20_82_06}STDIN {#stdin .mansect}

> The standard input shall be used to read an input line in response to
> each prompt specified in the STDERR section. Otherwise, the standard
> input shall not be used.

#### []{#tag_20_82_07}INPUT FILES {#input-files .mansect}

> The input files specified by each *source_file* operand can be of any
> file type.

#### []{#tag_20_82_08}ENVIRONMENT VARIABLES {#environment-variables .mansect}

> The following environment variables shall affect the execution of
> *mv*:
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
>     classes, and multi-character collating elements used in the
>     extended regular expression defined for the **yesexpr** locale
>     keyword in the *LC_MESSAGES* category.
>
> *LC_CTYPE*
> :   Determine the locale for the interpretation of sequences of bytes
>     of text data as characters (for example, single-byte as opposed to
>     multi-byte characters in arguments and input files), the behavior
>     of character classes used in the extended regular expression
>     defined for the **yesexpr** locale keyword in the *LC_MESSAGES*
>     category.
>
> *LC_MESSAGES*
> :   Determine the locale used to process affirmative responses, and
>     the locale used to affect the format and contents of diagnostic
>     messages and prompts written to standard error.
>
> *NLSPATH*
> :   ^\[[XSI](javascript:open_code('XSI'))\]^ ![\[Option
>     Start\]](../images/opt-start.gif){border="0"} Determine the
>     location of message catalogs for the processing of *LC_MESSAGES.*
>     ![\[Option End\]](../images/opt-end.gif){border="0"}

#### []{#tag_20_82_09}ASYNCHRONOUS EVENTS {#asynchronous-events .mansect}

> Default.

#### []{#tag_20_82_10}STDOUT {#stdout .mansect}

> Not used.

#### []{#tag_20_82_11}STDERR {#stderr .mansect}

> Prompts shall be written to the standard error under the conditions
> specified in the DESCRIPTION section. The prompts shall contain the
> destination pathname, but their format is otherwise unspecified.
> Otherwise, the standard error shall be used only for diagnostic
> messages.

#### []{#tag_20_82_12}OUTPUT FILES {#output-files .mansect}

> The output files may be of any file type.

#### []{#tag_20_82_13}EXTENDED DESCRIPTION {#extended-description .mansect}

> None.

#### []{#tag_20_82_14}EXIT STATUS {#exit-status .mansect}

> The following exit values shall be returned:
>
>  0
> :   All input files were moved successfully.
>
> \>0
> :   An error occurred.

#### []{#tag_20_82_15}CONSEQUENCES OF ERRORS {#consequences-of-errors .mansect}

> If the copying or removal of *source_file* is prematurely terminated
> by a signal or error, *mv* may leave a partial copy of *source_file*
> at the source or destination. The *mv* utility shall not modify both
> *source_file* and the destination path simultaneously; termination at
> any point shall leave either *source_file* or the destination path
> complete.

------------------------------------------------------------------------

::: box
*The following sections are informative.*
:::

#### []{#tag_20_82_16}APPLICATION USAGE {#application-usage .mansect}

> Some implementations mark for update the last file status change
> timestamp of renamed files and some do not. Applications which make
> use of the last file status change timestamp may behave differently
> with respect to renamed files unless they are designed to allow for
> either behavior.
>
> The specification ensures that *mv* **a** **a** will not alter the
> contents of file **a**, and allows the implementation to issue an
> error that a file cannot be moved onto itself. Likewise, when **a**
> and **b** are hard links to the same file, *mv* **a** **b** will not
> alter **b**, but if a diagnostic is not issued, then it is unspecified
> whether **a** is left untouched (as it would be by the
> [*rename*()](../functions/rename.html) function) or unlinked (reducing
> the link count of **b**).

#### []{#tag_20_82_17}EXAMPLES {#examples .mansect}

> If the current directory contains only files **a** (of any type
> defined by the System Interfaces volume of POSIX.1-2017), **b** (also
> of any type), and a directory **c**:
>
>
>     mv a b c
>     mv c d
>
> results with the original files **a** and **b** residing in the
> directory **d** in the current directory.

#### []{#tag_20_82_18}RATIONALE {#rationale .mansect}

> Early proposals diverged from the SVID and BSD historical practice in
> that they required that when the destination path exists, the **-f**
> option is not specified, and input is not a terminal, *mv* fails. This
> was done for compatibility with [*cp*](../utilities/cp.html). The
> current text returns to historical practice. It should be noted that
> this is consistent with the [*rename*()](../functions/rename.html)
> function defined in the System Interfaces volume of POSIX.1-2017,
> which does not require write permission on the target.
>
> For absolute clarity, paragraph (1), describing the behavior of *mv*
> when prompting for confirmation, should be interpreted in the
> following manner:
>
>
>     if (exists AND (NOT f_option) AND
>         ((not_writable AND input_is_terminal) OR i_option))
>
> The **-i** option exists on BSD systems, giving applications and users
> a way to avoid accidentally unlinking files when moving others. When
> the standard input is not a terminal, the 4.3 BSD *mv* deletes all
> existing destination paths without prompting, even when **-i** is
> specified; this is inconsistent with the behavior of the 4.3 BSD
> [*cp*](../utilities/cp.html) utility, which always generates an error
> when the file is unwritable and the standard input is not a terminal.
> The standard developers decided that use of **-i** is a request for
> interaction, so when the destination path exists, the utility takes
> instructions from whatever responds to standard input.
>
> The [*rename*()](../functions/rename.html) function is able to move
> directories within the same file system. Some historical versions of
> *mv* have been able to move directories, but not to a different file
> system. The standard developers considered that this was an annoying
> inconsistency, so this volume of POSIX.1-2017 requires directories to
> be able to be moved even across file systems. There is no **-R**
> option to confirm that moving a directory is actually intended, since
> such an option was not required for moving directories in historical
> practice. Requiring the application to specify it sometimes, depending
> on the destination, seemed just as inconsistent. The semantics of the
> [*rename*()](../functions/rename.html) function were preserved as much
> as possible. For example, *mv* is not permitted to \"rename\" files to
> or from directories, even though they might be empty and removable.
>
> Historic implementations of *mv* did not exit with a non-zero exit
> status if they were unable to duplicate any file characteristics when
> moving a file across file systems, nor did they write a diagnostic
> message for the user. The former behavior has been preserved to
> prevent scripts from breaking; a diagnostic message is now required,
> however, so that users are alerted that the file characteristics have
> changed.
>
> The exact format of the interactive prompts is unspecified. Only the
> general nature of the contents of prompts are specified because
> implementations may desire more descriptive prompts than those used on
> historical implementations. Therefore, an application not using the
> **-f** option or using the **-i** option relies on the system to
> provide the most suitable dialog directly with the user, based on the
> behavior specified.
>
> When *mv* is dealing with a single file system and *source_file* is a
> symbolic link, the link itself is moved as a consequence of the
> dependence on the [*rename*()](../functions/rename.html)
> functionality, per the DESCRIPTION. Across file systems, this has to
> be made explicit.

#### []{#tag_20_82_19}FUTURE DIRECTIONS {#future-directions .mansect}

> None.

#### []{#tag_20_82_20}SEE ALSO {#see-also .mansect}

> [*cp*](../utilities/cp.html#), [*ln*](../utilities/ln.html#)
>
> XBD [*Environment Variables*](../basedefs/V1_chap08.html#tag_08),
> [*Utility Syntax Guidelines*](../basedefs/V1_chap12.html#tag_12_02)
>
> XSH [*rename*](../functions/rename.html#)

#### []{#tag_20_82_21}CHANGE HISTORY {#change-history .mansect}

> First released in Issue 2.

#### []{#tag_20_82_22}Issue 6 {#issue-6 .mansect}

> The *mv* utility is changed to describe processing of symbolic links
> as specified in the IEEE P1003.2b draft standard.
>
> The APPLICATION USAGE section is added.

#### []{#tag_20_82_23}Issue 7 {#issue-7 .mansect}

> Austin Group Interpretation 1003.1-2001 #016 is applied.
>
> Austin Group Interpretation 1003.1-2001 #126 is applied, changing the
> description of the *LC_MESSAGES* environment variable.
>
> Austin Group Interpretations 1003.1-2001 #164, #168, and #169 are
> applied.
>
> SD5-XCU-ERN-13 is applied, making an editorial correction to the
> SYNOPSIS.
>
> SD5-XCU-ERN-51 is applied to the DESCRIPTION, defining the behavior
> for when files are being duplicated to another file system while
> having hard links.
>
> Changes are made related to support for finegrained timestamps.
>
> POSIX.1-2008, Technical Corrigendum 1, XCU/TC1-2008/0124 \[48\] is
> applied.
>
> POSIX.1-2008, Technical Corrigendum 2, XCU/TC2-2008/0147 \[534\] is
> applied.

