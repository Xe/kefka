#### []{#tag_20_111_01}NAME {#name .mansect}

> rm - remove directory entries

#### []{#tag_20_111_02}SYNOPSIS {#synopsis .mansect}

> `rm`` `**`[`**`-iRr`**`]`**` `*`file`*`...`\
> \
> `rm -f`` `**`[`**`-iRr`**`]`**` `**`[`***`file`*`...`**`]`**\

#### []{#tag_20_111_03}DESCRIPTION {#description .mansect}

> The *rm* utility shall remove the directory entry specified by each
> *file* argument.
>
> If either of the files dot or dot-dot are specified as the basename
> portion of an operand (that is, the final pathname component) or if an
> operand resolves to the root directory, *rm* shall write a diagnostic
> message to standard error and do nothing more with such operands.
>
> For each *file* the following steps shall be taken:
>
> 1.  If the *file* does not exist:
>
>     a.  If the **-f** option is not specified, *rm* shall write a
>         diagnostic message to standard error.
>
>     b.  Go on to any remaining *files*.
>
> 2.  If *file* is of type directory, the following steps shall be
>     taken:
>
>     a.  If neither the **-R** option nor the **-r** option is
>         specified, *rm* shall write a diagnostic message to standard
>         error, do nothing more with *file*, and go on to any remaining
>         files.
>
>     b.  If *file* is an empty directory, *rm* may skip to step 2d. If
>         the **-f** option is not specified, and either the permissions
>         of *file* do not permit writing and the standard input is a
>         terminal or the **-i** option is specified, *rm* shall write a
>         prompt to standard error and read a line from the standard
>         input. If the response is not affirmative, *rm* shall do
>         nothing more with the current file and go on to any remaining
>         files.
>
>     c.  For each entry contained in *file*, other than dot or dot-dot,
>         the four steps listed here (1 to 4) shall be taken with the
>         entry as if it were a *file* operand. The *rm* utility shall
>         not traverse directories by following symbolic links into
>         other parts of the hierarchy, but shall remove the links
>         themselves.
>
>     d.  If the **-i** option is specified, *rm* shall write a prompt
>         to standard error and read a line from the standard input. If
>         the response is not affirmative, *rm* shall do nothing more
>         with the current file, and go on to any remaining files.
>
> 3.  If *file* is not of type directory, the **-f** option is not
>     specified, and either the permissions of *file* do not permit
>     writing and the standard input is a terminal or the **-i** option
>     is specified, *rm* shall write a prompt to the standard error and
>     read a line from the standard input. If the response is not
>     affirmative, *rm* shall do nothing more with the current file and
>     go on to any remaining files.
>
> 4.  If the current file is a directory, *rm* shall perform actions
>     equivalent to the [*rmdir*()](../functions/rmdir.html) function
>     defined in the System Interfaces volume of POSIX.1-2017 called
>     with a pathname of the current file used as the *path* argument.
>     If the current file is not a directory, *rm* shall perform actions
>     equivalent to the [*unlink*()](../functions/unlink.html) function
>     defined in the System Interfaces volume of POSIX.1-2017 called
>     with a pathname of the current file used as the *path* argument.
>
>     If this fails for any reason, *rm* shall write a diagnostic
>     message to standard error, do nothing more with the current file,
>     and go on to any remaining files.
>
> The *rm* utility shall be able to descend to arbitrary depths in a
> file hierarchy, and shall not fail due to path length limitations
> (unless an operand specified by the user exceeds system limitations).

#### []{#tag_20_111_04}OPTIONS {#options .mansect}

> The *rm* utility shall conform to XBD [*Utility Syntax
> Guidelines*](../basedefs/V1_chap12.html#tag_12_02) .
>
> The following options shall be supported:
>
> **-f**
> :   Do not prompt for confirmation. Do not write diagnostic messages
>     or modify the exit status in the case of no file operands, or in
>     the case of operands that do not exist. Any previous occurrences
>     of the **-i** option shall be ignored.
>
> **-i**
> :   Prompt for confirmation as described previously. Any previous
>     occurrences of the **-f** option shall be ignored.
>
> **-R**
> :   Remove file hierarchies. See the DESCRIPTION.
>
> **-r**
> :   Equivalent to **-R**.

#### []{#tag_20_111_05}OPERANDS {#operands .mansect}

> The following operand shall be supported:
>
> *file*
> :   A pathname of a directory entry to be removed.

#### []{#tag_20_111_06}STDIN {#stdin .mansect}

> The standard input shall be used to read an input line in response to
> each prompt specified in the STDOUT section. Otherwise, the standard
> input shall not be used.

#### []{#tag_20_111_07}INPUT FILES {#input-files .mansect}

> None.

#### []{#tag_20_111_08}ENVIRONMENT VARIABLES {#environment-variables .mansect}

> The following environment variables shall affect the execution of
> *rm*:
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
>     multi-byte characters in arguments) and the behavior of character
>     classes within regular expressions used in the extended regular
>     expression defined for the **yesexpr** locale keyword in the
>     *LC_MESSAGES* category.
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

#### []{#tag_20_111_09}ASYNCHRONOUS EVENTS {#asynchronous-events .mansect}

> Default.

#### []{#tag_20_111_10}STDOUT {#stdout .mansect}

> Not used.

#### []{#tag_20_111_11}STDERR {#stderr .mansect}

> Prompts shall be written to standard error under the conditions
> specified in the DESCRIPTION and OPTIONS sections. The prompts shall
> contain the *file* pathname, but their format is otherwise
> unspecified. The standard error also shall be used for diagnostic
> messages.

#### []{#tag_20_111_12}OUTPUT FILES {#output-files .mansect}

> None.

#### []{#tag_20_111_13}EXTENDED DESCRIPTION {#extended-description .mansect}

> None.

#### []{#tag_20_111_14}EXIT STATUS {#exit-status .mansect}

> The following exit values shall be returned:
>
>  0
> :   Each directory entry was successfully removed, unless its removal
>     was canceled by a non-affirmative response to a prompt for
>     confirmation.
>
> \>0
> :   An error occurred.

#### []{#tag_20_111_15}CONSEQUENCES OF ERRORS {#consequences-of-errors .mansect}

> Default.

------------------------------------------------------------------------

::: box
*The following sections are informative.*
:::

#### []{#tag_20_111_16}APPLICATION USAGE {#application-usage .mansect}

> The *rm* utility is forbidden to remove the names dot and dot-dot in
> order to avoid the consequences of inadvertently doing something like:
>
>
>     rm -r .*
>
> Some implementations do not permit the removal of the last link to an
> executable binary file that is being executed; see the \[EBUSY\] error
> in the [*unlink*()](../functions/unlink.html) function defined in the
> System Interfaces volume of POSIX.1-2017. Thus, the *rm* utility can
> fail to remove such files.
>
> The **-i** option causes *rm* to prompt and read the standard input
> even if the standard input is not a terminal, but in the absence of
> **-i** the mode prompting is not done when the standard input is not a
> terminal.

#### []{#tag_20_111_17}EXAMPLES {#examples .mansect}

> 1.  The following command:
>
>
>         rm a.out core
>
>     removes the directory entries: **a.out** and **core**.
>
> 2.  The following command:
>
>
>         rm -Rf junk
>
>     removes the directory **junk** and all its contents, without
>     prompting.

#### []{#tag_20_111_18}RATIONALE {#rationale .mansect}

> For absolute clarity, paragraphs (2b) and (3) in the DESCRIPTION of
> *rm* describing the behavior when prompting for confirmation, should
> be interpreted in the following manner:
>
>
>     if ((NOT f_option) AND
>         ((not_writable AND input_is_terminal) OR i_option))
>
> The exact format of the interactive prompts is unspecified. Only the
> general nature of the contents of prompts are specified because
> implementations may desire more descriptive prompts than those used on
> historical implementations. Therefore, an application not using the
> **-f** option, or using the **-i** option, relies on the system to
> provide the most suitable dialog directly with the user, based on the
> behavior specified.
>
> The **-r** option is historical practice on all known systems. The
> synonym **-R** option is provided for consistency with the other
> utilities in this volume of POSIX.1-2017 that provide options
> requesting recursive descent through the file hierarchy.
>
> The behavior of the **-f** option in historical versions of *rm* is
> inconsistent. In general, along with \"forcing\" the unlink without
> prompting for permission, it always causes diagnostic messages to be
> suppressed and the exit status to be unmodified for nonexistent
> operands and files that cannot be unlinked. In some versions, however,
> the **-f** option suppresses usage messages and system errors as well.
> Suppressing such messages is not a service to either shell scripts or
> users.
>
> It is less clear that error messages regarding files that cannot be
> unlinked (removed) should be suppressed. Although this is historical
> practice, this volume of POSIX.1-2017 does not permit the **-f**
> option to suppress such messages.
>
> When given the **-r** and **-i** options, historical versions of *rm*
> prompt the user twice for each directory, once before removing its
> contents and once before actually attempting to delete the directory
> entry that names it. This allows the user to \"prune\" the file
> hierarchy walk. Historical versions of *rm* were inconsistent in that
> some did not do the former prompt for directories named on the command
> line and others had obscure prompting behavior when the **-i** option
> was specified and the permissions of the file did not permit writing.
> The POSIX Shell and Utilities *rm* differs little from historic
> practice, but does require that prompts be consistent. Historical
> versions of *rm* were also inconsistent in that prompts were done to
> both standard output and standard error. This volume of POSIX.1-2017
> requires that prompts be done to standard error, for consistency with
> [*cp*](../utilities/cp.html) and [*mv*](../utilities/mv.html), and to
> allow historical extensions to *rm* that provide an option to list
> deleted files on standard output.
>
> The *rm* utility is required to descend to arbitrary depths so that
> any file hierarchy may be deleted. This means, for example, that the
> *rm* utility cannot run out of file descriptors during its descent
> (that is, if the number of file descriptors is limited, *rm* cannot be
> implemented in the historical fashion where one file descriptor is
> used per directory level). Also, *rm* is not permitted to fail because
> of path length restrictions, unless an operand specified by the user
> is longer than {PATH_MAX}.
>
> The *rm* utility removes symbolic links themselves, not the files they
> refer to, as a consequence of the dependence on the
> [*unlink*()](../functions/unlink.html) functionality, per the
> DESCRIPTION. When removing hierarchies with **-r** or **-R**, the
> prohibition on following symbolic links has to be made explicit.

#### []{#tag_20_111_19}FUTURE DIRECTIONS {#future-directions .mansect}

> None.

#### []{#tag_20_111_20}SEE ALSO {#see-also .mansect}

> [*rmdir*](../utilities/rmdir.html#tag_20_113)
>
> XBD [*Environment Variables*](../basedefs/V1_chap08.html#tag_08),
> [*Utility Syntax Guidelines*](../basedefs/V1_chap12.html#tag_12_02)
>
> XSH [*remove*](../functions/remove.html#),
> [*rmdir*](../functions/rmdir.html#tag_16_491),
> [*unlink*](../functions/unlink.html#tag_16_635)

#### []{#tag_20_111_21}CHANGE HISTORY {#change-history .mansect}

> First released in Issue 2.

#### []{#tag_20_111_22}Issue 5 {#issue-5 .mansect}

> The FUTURE DIRECTIONS section is added.

#### []{#tag_20_111_23}Issue 6 {#issue-6 .mansect}

> Text is added to clarify actions relating to symbolic links as
> specified in the IEEE P1003.2b draft standard.

#### []{#tag_20_111_24}Issue 7 {#issue-7 .mansect}

> Austin Group Interpretations 1003.1-2001 #019 and #091 are applied.
>
> Austin Group Interpretation 1003.1-2001 #126 is applied, changing the
> description of the *LC_MESSAGES* environment variable.
>
> POSIX.1-2008, Technical Corrigendum 2, XCU/TC2-2008/0163 \[542\],
> XCU/TC2-2008/0164 \[819\], and XCU/TC2-2008/0165 \[542\] are applied.

