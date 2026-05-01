#### []{#tag_20_33_01}NAME {#name .mansect}

> df - report free disk space

#### []{#tag_20_33_02}SYNOPSIS {#synopsis .mansect}

> ^`[`[`XSI`](javascript:open_code('XSI'))`]`^` df`` `**`[`**`-k`**`] [`**`-P|`![](../images/opt-start.gif){border="0"}`-t`**![](../images/opt-end.gif){border="0"}`] [`***`file`*`...`**`]`**

#### []{#tag_20_33_03}DESCRIPTION {#description .mansect}

> The *df* utility shall write the amount of available space
> ^\[[XSI](javascript:open_code('XSI'))\]^ ![\[Option
> Start\]](../images/opt-start.gif){border="0"}  and file slots
> ![\[Option End\]](../images/opt-end.gif){border="0"} for file systems
> on which the invoking user has appropriate read access. File systems
> shall be specified by the *file* operands; when none are specified,
> information shall be written for all file systems. The format of the
> default output from *df* is unspecified, but all space figures are
> reported in 512-byte units, unless the **-k** option is specified.
> This output shall contain at least the file system names, amount of
> available space on each of these file systems,
> ^\[[XSI](javascript:open_code('XSI'))\]^ ![\[Option
> Start\]](../images/opt-start.gif){border="0"}  and, if no options
> other than **-t** are specified, the number of free file slots, or
> *inode*s, available; when **-t** is specified, the output shall
> contain the total allocated space as well. ![\[Option
> End\]](../images/opt-end.gif){border="0"}

#### []{#tag_20_33_04}OPTIONS {#options .mansect}

> The *df* utility shall conform to XBD [*Utility Syntax
> Guidelines*](../basedefs/V1_chap12.html#tag_12_02) .
>
> The following options shall be supported:
>
> **-k**
> :   Use 1024-byte units, instead of the default 512-byte units, when
>     writing space figures.
>
> **-P**
> :   Produce output in the format described in the STDOUT section.
>
> **-t**
> :   ^\[[XSI](javascript:open_code('XSI'))\]^ ![\[Option
>     Start\]](../images/opt-start.gif){border="0"} Include total
>     allocated-space figures in the output. ![\[Option
>     End\]](../images/opt-end.gif){border="0"}

#### []{#tag_20_33_05}OPERANDS {#operands .mansect}

> The following operand shall be supported:
>
> *file*
> :   A pathname of a file within the hierarchy of the desired file
>     system. If a file other than a FIFO, a regular file, a directory,
>     ^\[[XSI](javascript:open_code('XSI'))\]^ ![\[Option
>     Start\]](../images/opt-start.gif){border="0"}  or a special file
>     representing the device containing the file system (for example,
>     **/dev/dsk/0s1**) ![\[Option
>     End\]](../images/opt-end.gif){border="0"} is specified, the
>     results are unspecified. If the *file* operand names a file other
>     than a special file containing a file system, *df* shall write the
>     amount of free space in the file system containing the specified
>     *file* operand. ^\[[XSI](javascript:open_code('XSI'))\]^
>     ![\[Option Start\]](../images/opt-start.gif){border="0"}
>      Otherwise, *df* shall write the amount of free space in that file
>     system. ![\[Option End\]](../images/opt-end.gif){border="0"}

#### []{#tag_20_33_06}STDIN {#stdin .mansect}

> Not used.

#### []{#tag_20_33_07}INPUT FILES {#input-files .mansect}

> None.

#### []{#tag_20_33_08}ENVIRONMENT VARIABLES {#environment-variables .mansect}

> The following environment variables shall affect the execution of
> *df*:
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
>     contents of diagnostic messages written to standard error and
>     informative messages written to standard output.
>
> *NLSPATH*
> :   ^\[[XSI](javascript:open_code('XSI'))\]^ ![\[Option
>     Start\]](../images/opt-start.gif){border="0"} Determine the
>     location of message catalogs for the processing of *LC_MESSAGES.*
>     ![\[Option End\]](../images/opt-end.gif){border="0"}

#### []{#tag_20_33_09}ASYNCHRONOUS EVENTS {#asynchronous-events .mansect}

> Default.

#### []{#tag_20_33_10}STDOUT {#stdout .mansect}

> When both the **-k** and **-P** options are specified, the following
> header line shall be written (in the POSIX locale):
>
>
>     "Filesystem 1024-blocks Used Available Capacity Mounted on\n"
>
> When the **-P** option is specified without the **-k** option, the
> following header line shall be written (in the POSIX locale):
>
>
>     "Filesystem 512-blocks Used Available Capacity Mounted on\n"
>
> The implementation may adjust the spacing of the header line and the
> individual data lines so that the information is presented in orderly
> columns.
>
> The remaining output with **-P** shall consist of one line of
> information for each specified file system. These lines shall be
> formatted as follows:
>
>
>     "%s %d %d %d %d%% %s\n", <file system name>, <total space>,
>         <space used>, <space free>, <percentage used>,
>         <file system root>
>
> In the following list, all quantities expressed in 512-byte units
> (1024-byte when **-k** is specified) shall be rounded up to the next
> higher unit. The fields are:
>
> \<*file system name*\>
> :   The name of the file system, in an implementation-defined format.
>
> \<*total space*\>
> :   The total size of the file system in 512-byte units. The exact
>     meaning of this figure is implementation-defined, but should
>     include \<*space used*\>, \<*space free*\>, plus any space
>     reserved by the system not normally available to a user.
>
> \<*space used*\>
> :   The total amount of space allocated to existing files in the file
>     system, in 512-byte units.
>
> \<*space free*\>
> :   The total amount of space available within the file system for the
>     creation of new files by unprivileged users, in 512-byte units.
>     When this figure is less than or equal to zero, it shall not be
>     possible to create any new files on the file system without first
>     deleting others, unless the process has appropriate privileges.
>     The figure written may be less than zero.
>
> \<*percentage used*\>
>
> :   The percentage of the normally available space that is currently
>     allocated to all files on the file system. This shall be
>     calculated using the fraction:
>
>
>         <space used>/( <space used>+ <space free>)
>
>     expressed as a percentage. This percentage may be greater than 100
>     if \<*space free*\> is less than zero. The percentage value shall
>     be expressed as a positive integer, with any fractional result
>     causing it to be rounded to the next highest integer.
>
> \<*file system root*\>
> :   The directory below which the file system hierarchy appears.
>
> ^\[[XSI](javascript:open_code('XSI'))\]^ ![\[Option
> Start\]](../images/opt-start.gif){border="0"} The output format is
> unspecified when **-t** is used. ![\[Option
> End\]](../images/opt-end.gif){border="0"}

#### []{#tag_20_33_11}STDERR {#stderr .mansect}

> The standard error shall be used only for diagnostic messages.

#### []{#tag_20_33_12}OUTPUT FILES {#output-files .mansect}

> None.

#### []{#tag_20_33_13}EXTENDED DESCRIPTION {#extended-description .mansect}

> None.

#### []{#tag_20_33_14}EXIT STATUS {#exit-status .mansect}

> The following exit values shall be returned:
>
>  0
> :   Successful completion.
>
> \>0
> :   An error occurred.

#### []{#tag_20_33_15}CONSEQUENCES OF ERRORS {#consequences-of-errors .mansect}

> Default.

------------------------------------------------------------------------

::: box
*The following sections are informative.*
:::

#### []{#tag_20_33_16}APPLICATION USAGE {#application-usage .mansect}

> On most systems, the \"name of the file system, in an
> implementation-defined format\" is the special file on which the file
> system is mounted.
>
> On large file systems, the calculation specified for percentage used
> can create huge rounding errors.

#### []{#tag_20_33_17}EXAMPLES {#examples .mansect}

> 1.  The following example writes portable information about the
>     **/usr** file system:
>
>
>         df -P /usr
>
> 2.  Assuming that **/usr/src** is part of the **/usr** file system,
>     the following produces the same output as the previous example:
>
>
>         df -P /usr/src

#### []{#tag_20_33_18}RATIONALE {#rationale .mansect}

> The behavior of *df* with the **-P** option is the default action of
> the 4.2 BSD *df* utility. The uppercase **-P** was selected to avoid
> collision with a known industry extension using **-p**.
>
> Historical *df* implementations vary considerably in their default
> output. It was therefore necessary to describe the default output in a
> loose manner to accommodate all known historical implementations and
> to add a portable option (**-P**) to provide information in a portable
> format.
>
> The use of 512-byte units is historical practice and maintains
> compatibility with [*ls*](../utilities/ls.html) and other utilities in
> this volume of POSIX.1-2017. This does not mandate that the file
> system itself be based on 512-byte blocks. The **-k** option was added
> as a compromise measure. It was agreed by the standard developers that
> 512 bytes was the best default unit because of its complete historical
> consistency on System V (*versus* the mixed 512/1024-byte usage on BSD
> systems), and that a **-k** option to switch to 1024-byte units was a
> good compromise. Users who prefer the more logical 1024-byte quantity
> can easily alias *df* to *df* **-k** without breaking many historical
> scripts relying on the 512-byte units.
>
> It was suggested that *df* and the various related utilities be
> modified to access a *BLOCKSIZE* environment variable to achieve
> consistency and user acceptance. Since this is not historical practice
> on any system, it is left as a possible area for system extensions and
> will be re-evaluated in a future version if it is widely implemented.

#### []{#tag_20_33_19}FUTURE DIRECTIONS {#future-directions .mansect}

> None.

#### []{#tag_20_33_20}SEE ALSO {#see-also .mansect}

> [*find*](../utilities/find.html#)
>
> XBD [*Environment Variables*](../basedefs/V1_chap08.html#tag_08),
> [*Utility Syntax Guidelines*](../basedefs/V1_chap12.html#tag_12_02)

#### []{#tag_20_33_21}CHANGE HISTORY {#change-history .mansect}

> First released in Issue 2.

#### []{#tag_20_33_22}Issue 6 {#issue-6 .mansect}

> This utility is marked as part of the User Portability Utilities
> option.

#### []{#tag_20_33_23}Issue 7 {#issue-7 .mansect}

> Austin Group Interpretation 1003.1-2001 #099 is applied.
>
> The *df* utility is removed from the User Portability Utilities
> option. User Portability Utilities is now an option for interactive
> utilities.
>
> SD5-XCU-ERN-97 is applied, updating the SYNOPSIS.
>
> POSIX.1-2008, Technical Corrigendum 1, XCU/TC1-2008/0082 \[156\] is
> applied.

