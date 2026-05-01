#### []{#tag_20_67_01}NAME {#name .mansect}

> ln - link files

#### []{#tag_20_67_02}SYNOPSIS {#synopsis .mansect}

> `ln`` `**`[`**`-fs`**`] [`**`-L|-P`**`]`**` `*`source_file target_file`*\
> \
> `ln`` `**`[`**`-fs`**`] [`**`-L|-P`**`]`**` `*`source_file`*`...`` `*`target_dir`*\

#### []{#tag_20_67_03}DESCRIPTION {#description .mansect}

> In the first synopsis form, the *ln* utility shall create a new
> directory entry (link) at the destination path specified by the
> *target_file* operand. If the **-s** option is specified, a symbolic
> link shall be created for the file specified by the *source_file*
> operand. This first synopsis form shall be assumed when the final
> operand does not name an existing directory; if more than two operands
> are specified and the final is not an existing directory, an error
> shall result.
>
> In the second synopsis form, the *ln* utility shall create a new
> directory entry (link), or if the **-s** option is specified a
> symbolic link, for each file specified by a *source_file* operand, at
> a destination path in the existing directory named by *target_dir*.
>
> If the last operand specifies an existing file of a type not specified
> by the System Interfaces volume of POSIX.1-2017, the behavior is
> implementation-defined.
>
> The corresponding destination path for each *source_file* shall be the
> concatenation of the target directory pathname, a \<slash\> character
> if the target directory pathname did not end in a \<slash\>, and the
> last pathname component of the *source_file*. The second synopsis form
> shall be assumed when the final operand names an existing directory.
>
> For each *source_file*:
>
> 1.  If the destination path exists and was created by a previous step,
>     it is unspecified whether *ln* shall write a diagnostic message to
>     standard error, do nothing more with the current *source_file*,
>     and go on to any remaining *source_file*s; or will continue
>     processing the current *source_file*. If the destination path
>     exists:
>
>     a.  If the **-f** option is not specified, *ln* shall write a
>         diagnostic message to standard error, do nothing more with the
>         current *source_file*, and go on to any remaining
>         *source_file*s.
>
>     b.  If the destination path names the same directory entry as the
>         current *source_file* *ln* shall write a diagnostic message to
>         standard error, do nothing more with the current
>         *source_file*, and go on to any remaining *source_file*s*.*
>
>     c.  Actions shall be performed equivalent to the
>         [*unlink*()](../functions/unlink.html) function defined in the
>         System Interfaces volume of POSIX.1-2017, called using the
>         destination path as the *path* argument. If this fails for any
>         reason, *ln* shall write a diagnostic message to standard
>         error, do nothing more with the current *source_file*, and go
>         on to any remaining *source_file*s.
>
> 2.  If the **-s** option is specified, actions shall be performed
>     equivalent to the [*symlink*()](../functions/symlink.html)
>     function with *source_file* as the *path1* argument and the
>     destination path as the *path2* argument. The *ln* utility shall
>     do nothing more with *source_file* and shall go on to any
>     remaining files.
>
> 3.  If *source_file* is a symbolic link:
>
>     a.  If the **-P** option is in effect, actions shall be performed
>         equivalent to the [*linkat*()](../functions/linkat.html)
>         function with *source_file* as the *path1* argument, the
>         destination path as the *path2* argument, AT_FDCWD as the
>         *fd1* and *fd2* arguments, and zero as the *flag* argument.
>
>     b.  If the **-L** option is in effect, actions shall be performed
>         equivalent to the [*linkat*()](../functions/linkat.html)
>         function with *source_file* as the *path1* argument, the
>         destination path as the *path2* argument, AT_FDCWD as the
>         *fd1* and *fd2* arguments, and AT_SYMLINK_FOLLOW as the *flag*
>         argument.
>
>     The *ln* utility shall do nothing more with *source_file* and
>     shall go on to any remaining files.
>
> 4.  Actions shall be performed equivalent to the
>     [*link*()](../functions/link.html) function defined in the System
>     Interfaces volume of POSIX.1-2017 using *source_file* as the
>     *path1* argument, and the destination path as the *path2*
>     argument.

#### []{#tag_20_67_04}OPTIONS {#options .mansect}

> The *ln* utility shall conform to XBD [*Utility Syntax
> Guidelines*](../basedefs/V1_chap12.html#tag_12_02) .
>
> The following options shall be supported:
>
> **-f**
> :   Force existing destination pathnames to be removed to allow the
>     link.
>
> **-L**
> :   For each *source_file* operand that names a file of type symbolic
>     link, create a (hard) link to the file referenced by the symbolic
>     link.
>
> **-P**
> :   For each *source_file* operand that names a file of type symbolic
>     link, create a (hard) link to the symbolic link itself.
>
> **-s**
> :   Create symbolic links instead of hard links. If the **-s** option
>     is specified, the **-L** and **-P** options shall be silently
>     ignored.
>
> Specifying more than one of the mutually-exclusive options **-L** and
> **-P** shall not be considered an error. The last option specified
> shall determine the behavior of the utility (unless the **-s** option
> causes it to be ignored).
>
> If the **-s** option is not specified and neither a **-L** nor a
> **-P** option is specified, it is implementation-defined which of the
> **-L** and **-P** options will be used as the default.

#### []{#tag_20_67_05}OPERANDS {#operands .mansect}

> The following operands shall be supported:
>
> *source_file*
> :   A pathname of a file to be linked. If the **-s** option is
>     specified, no restrictions on the type of file or on its existence
>     shall be made. If the **-s** option is not specified, whether a
>     directory can be linked is implementation-defined.
>
> *target_file*
> :   The pathname of the new directory entry to be created.
>
> *target_dir*
> :   A pathname of an existing directory in which the new directory
>     entries are created.

#### []{#tag_20_67_06}STDIN {#stdin .mansect}

> Not used.

#### []{#tag_20_67_07}INPUT FILES {#input-files .mansect}

> None.

#### []{#tag_20_67_08}ENVIRONMENT VARIABLES {#environment-variables .mansect}

> The following environment variables shall affect the execution of
> *ln*:
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

#### []{#tag_20_67_09}ASYNCHRONOUS EVENTS {#asynchronous-events .mansect}

> Default.

#### []{#tag_20_67_10}STDOUT {#stdout .mansect}

> Not used.

#### []{#tag_20_67_11}STDERR {#stderr .mansect}

> The standard error shall be used only for diagnostic messages.

#### []{#tag_20_67_12}OUTPUT FILES {#output-files .mansect}

> None.

#### []{#tag_20_67_13}EXTENDED DESCRIPTION {#extended-description .mansect}

> None.

#### []{#tag_20_67_14}EXIT STATUS {#exit-status .mansect}

> The following exit values shall be returned:
>
>  0
> :   All the specified files were linked successfully.
>
> \>0
> :   An error occurred.

#### []{#tag_20_67_15}CONSEQUENCES OF ERRORS {#consequences-of-errors .mansect}

> Default.

------------------------------------------------------------------------

::: box
*The following sections are informative.*
:::

#### []{#tag_20_67_16}APPLICATION USAGE {#application-usage .mansect}

> None.

#### []{#tag_20_67_17}EXAMPLES {#examples .mansect}

> None.

#### []{#tag_20_67_18}RATIONALE {#rationale .mansect}

> The CONSEQUENCES OF ERRORS section does not require *ln* **-f** *a b*
> to remove *b* if a subsequent link operation would fail.
>
> Some historic versions of *ln* (including the one specified by the
> SVID) unlink the destination file, if it exists, by default. If the
> mode does not permit writing, these versions prompt for confirmation
> before attempting the unlink. In these versions the **-f** option
> causes *ln* not to attempt to prompt for confirmation.
>
> This allows *ln* to succeed in creating links when the target file
> already exists, even if the file itself is not writable (although the
> directory must be). Early proposals specified this functionality.
>
> This volume of POSIX.1-2017 does not allow the *ln* utility to unlink
> existing destination paths by default for the following reasons:
>
> - The *ln* utility has historically been used to provide locking for
>   shell applications, a usage that is incompatible with *ln* unlinking
>   the destination path by default. There was no corresponding
>   technical advantage to adding this functionality.
>
> - This functionality gave *ln* the ability to destroy the link
>   structure of files, which changes the historical behavior of *ln*.
>
> - This functionality is easily replicated with a combination of
>   [*rm*](../utilities/rm.html) and *ln*.
>
> - It is not historical practice in many systems; BSD and BSD-derived
>   systems do not support this behavior. Unfortunately, whichever
>   behavior is selected can cause scripts written expecting the other
>   behavior to fail.
>
> - It is preferable that *ln* perform in the same manner as the
>   [*link*()](../functions/link.html) function, which does not permit
>   the target to exist already.
>
> This volume of POSIX.1-2017 retains the **-f** option to provide
> support for shell scripts depending on the SVID semantics. It seems
> likely that shell scripts would not be written to handle prompting by
> *ln* and would therefore have specified the **-f** option.
>
> The **-f** option is an undocumented feature of many historical
> versions of the *ln* utility, allowing linking to directories. These
> versions require modification.
>
> Early proposals of this volume of POSIX.1-2017 also required a **-i**
> option, which behaved like the **-i** options in
> [*cp*](../utilities/cp.html) and [*mv*](../utilities/mv.html),
> prompting for confirmation before unlinking existing files. This was
> not historical practice for the *ln* utility and has been omitted.
>
> The **-L** and **-P** options allow for implementing both common
> behaviors of the *ln* utility. Earlier versions of this standard did
> not specify these options and required the behavior now described for
> the **-L** option. Many systems by default or as an alternative
> provided a non-conforming *ln* utility with the behavior now described
> for the **-P** option. Since applications could not rely on *ln*
> following links in practice, the **-L** and **-P** options were added
> to specify the desired behavior for the application.
>
> The **-L** and **-P** options are ignored when **-s** is specified in
> order to allow an alias to be created to alter the default behavior
> when creating hard links (for example,
> [*alias*](../utilities/alias.html) *ln*=\' *ln* **-L**\'). They serve
> no purpose when **-s** is specified, since *source_file* is then just
> a string to be used as the contents of the created symbolic link and
> need not exist as a file.
>
> The specification ensures that *ln* **a** **a** with or without the
> **-f** option will not unlink the file **a**. Earlier versions of this
> standard were unclear in this case.

#### []{#tag_20_67_19}FUTURE DIRECTIONS {#future-directions .mansect}

> None.

#### []{#tag_20_67_20}SEE ALSO {#see-also .mansect}

> [*chmod*](../utilities/chmod.html#tag_20_17),
> [*find*](../utilities/find.html#), [*pax*](../utilities/pax.html#),
> [*rm*](../utilities/rm.html#)
>
> XBD [*Environment Variables*](../basedefs/V1_chap08.html#tag_08),
> [*Utility Syntax Guidelines*](../basedefs/V1_chap12.html#tag_12_02)
>
> XSH [*link*](../functions/link.html#tag_16_293),
> [*unlink*](../functions/unlink.html#tag_16_635)

#### []{#tag_20_67_21}CHANGE HISTORY {#change-history .mansect}

> First released in Issue 2.

#### []{#tag_20_67_22}Issue 6 {#issue-6 .mansect}

> The *ln* utility is updated to include symbolic link processing as
> defined in the IEEE P1003.2b draft standard.

#### []{#tag_20_67_23}Issue 7 {#issue-7 .mansect}

> Austin Group Interpretations 1003.1-2001 #164, #168, and #169 are
> applied.
>
> SD5-XCU-ERN-27 is applied, adding a new paragraph to the RATIONALE.
>
> SD5-XCU-ERN-97 is applied, updating the SYNOPSIS.
>
> The **-L** and **-P** options are added to make it
> implementation-defined whether the *ln* utility follows symbolic
> links.
>
> POSIX.1-2008, Technical Corrigendum 1, XCU/TC1-2008/0096 \[136\] is
> applied.
>
> POSIX.1-2008, Technical Corrigendum 2, XCU/TC2-2008/0113 \[930\] is
> applied.

