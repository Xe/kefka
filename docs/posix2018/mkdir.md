#### []{#tag_20_79_01}NAME {#name .mansect}

> mkdir - make directories

#### []{#tag_20_79_02}SYNOPSIS {#synopsis .mansect}

> `mkdir`` `**`[`**`-p`**`] [`**`-m`` `*`mode`***`]`**` `*`dir`*`...`

#### []{#tag_20_79_03}DESCRIPTION {#description .mansect}

> The *mkdir* utility shall create the directories specified by the
> operands, in the order specified.
>
> For each *dir* operand, the *mkdir* utility shall perform actions
> equivalent to the [*mkdir*()](../functions/mkdir.html) function
> defined in the System Interfaces volume of POSIX.1-2017, called with
> the following arguments:
>
> 1.  The *dir* operand is used as the *path* argument.
>
> 2.  The value of the bitwise-inclusive OR of S_IRWXU, S_IRWXG, and
>     S_IRWXO is used as the *mode* argument. (If the **-m** option is
>     specified, the value of the [*mkdir*()](../functions/mkdir.html)
>     *mode* argument is unspecified, but the directory shall at no time
>     have permissions less restrictive than the **-m** *mode*
>     option-argument.)

#### []{#tag_20_79_04}OPTIONS {#options .mansect}

> The *mkdir* utility shall conform to XBD [*Utility Syntax
> Guidelines*](../basedefs/V1_chap12.html#tag_12_02).
>
> The following options shall be supported:
>
> **-m ** *mode*
> :   Set the file permission bits of the newly-created directory to the
>     specified *mode* value. The *mode* option-argument shall be the
>     same as the *mode* operand defined for the
>     [*chmod*](../utilities/chmod.html) utility. In the *symbolic_mode*
>     strings, the *op* characters `'+'` and `'-'` shall be interpreted
>     relative to an assumed initial mode of *a*= *rwx*; `'+'` shall add
>     permissions to the default mode, `'-'` shall delete permissions
>     from the default mode.
>
> **-p**
>
> :   Create any missing intermediate pathname components.
>
>     For each *dir* operand that does not name an existing directory,
>     before performing the actions described in the DESCRIPTION above,
>     the *mkdir* utility shall create any pathname components of the
>     path prefix of *dir* that do not name an existing directory by
>     performing actions equivalent to first calling the
>     [*mkdir*()](../functions/mkdir.html) function with the following
>     arguments:
>
>     1.  A pathname naming the missing pathname component, ending with
>         a trailing \<slash\> character, as the *path* argument
>
>     2.  The value zero as the *mode* argument
>
>     and then calling the [*chmod*()](../functions/chmod.html) function
>     with the following arguments:
>
>     1.  The same *path* argument as in the
>         [*mkdir*()](../functions/mkdir.html) call
>
>     2.  The value `(S_IWUSR|S_IXUSR|~`*`filemask`*`)&0777` as the
>         *mode* argument, where *filemask* is the file mode creation
>         mask of the process (see XSH
>         [*umask*](../functions/umask.html#tag_16_631))
>
>     Each *dir* operand that names an existing directory shall be
>     ignored without error.

#### []{#tag_20_79_05}OPERANDS {#operands .mansect}

> The following operand shall be supported:
>
> *dir*
> :   A pathname of a directory to be created.

#### []{#tag_20_79_06}STDIN {#stdin .mansect}

> Not used.

#### []{#tag_20_79_07}INPUT FILES {#input-files .mansect}

> None.

#### []{#tag_20_79_08}ENVIRONMENT VARIABLES {#environment-variables .mansect}

> The following environment variables shall affect the execution of
> *mkdir*:
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

#### []{#tag_20_79_09}ASYNCHRONOUS EVENTS {#asynchronous-events .mansect}

> Default.

#### []{#tag_20_79_10}STDOUT {#stdout .mansect}

> Not used.

#### []{#tag_20_79_11}STDERR {#stderr .mansect}

> The standard error shall be used only for diagnostic messages.

#### []{#tag_20_79_12}OUTPUT FILES {#output-files .mansect}

> None.

#### []{#tag_20_79_13}EXTENDED DESCRIPTION {#extended-description .mansect}

> None.

#### []{#tag_20_79_14}EXIT STATUS {#exit-status .mansect}

> The following exit values shall be returned:
>
>  0
> :   All the specified directories were created successfully, or the
>     **-p** option was specified and all the specified directories
>     either already existed or were created successfully.
>
> \>0
> :   An error occurred.

#### []{#tag_20_79_15}CONSEQUENCES OF ERRORS {#consequences-of-errors .mansect}

> Default.

------------------------------------------------------------------------

::: box
*The following sections are informative.*
:::

#### []{#tag_20_79_16}APPLICATION USAGE {#application-usage .mansect}

> The default file mode for directories is *a*= *rwx* (777 on most
> systems) with selected permissions removed in accordance with the file
> mode creation mask. For intermediate pathname components created by
> *mkdir*, the mode is the default modified by *u*+ *wx* so that the
> subdirectories can always be created regardless of the file mode
> creation mask; if different ultimate permissions are desired for the
> intermediate directories, they can be changed afterwards with
> [*chmod*](../utilities/chmod.html).
>
> Note that some of the requested directories may have been created even
> if an error occurs.

#### []{#tag_20_79_17}EXAMPLES {#examples .mansect}

> None.

#### []{#tag_20_79_18}RATIONALE {#rationale .mansect}

> The System V **-m** option was included to control the file mode.
>
> The System V **-p** option was included to create any needed
> intermediate directories and to complement the functionality provided
> by [*rmdir*](../utilities/rmdir.html) for removing directories in the
> path prefix as they become empty. Because no error is produced if any
> path component already exists, the **-p** option is also useful to
> ensure that a particular directory exists.
>
> The functionality of *mkdir* is described substantially through a
> reference to the [*mkdir*()](../functions/mkdir.html) function in the
> System Interfaces volume of POSIX.1-2017. For example, by default, the
> mode of the directory is affected by the file mode creation mask in
> accordance with the specified behavior of the
> [*mkdir*()](../functions/mkdir.html) function. In this way, there is
> less duplication of effort required for describing details of the
> directory creation.

#### []{#tag_20_79_19}FUTURE DIRECTIONS {#future-directions .mansect}

> None.

#### []{#tag_20_79_20}SEE ALSO {#see-also .mansect}

> [*chmod*](../utilities/chmod.html#tag_20_17),
> [*rm*](../utilities/rm.html#),
> [*rmdir*](../utilities/rmdir.html#tag_20_113),
> [*umask*](../utilities/umask.html#tag_20_138)
>
> XBD [*Environment Variables*](../basedefs/V1_chap08.html#tag_08),
> [*Utility Syntax Guidelines*](../basedefs/V1_chap12.html#tag_12_02)
>
> XSH [*mkdir*](../functions/mkdir.html#tag_16_325),
> [*umask*](../functions/umask.html#tag_16_631)

#### []{#tag_20_79_21}CHANGE HISTORY {#change-history .mansect}

> First released in Issue 2.

#### []{#tag_20_79_22}Issue 5 {#issue-5 .mansect}

> The FUTURE DIRECTIONS section is added.

#### []{#tag_20_79_23}Issue 7 {#issue-7 .mansect}

> SD5-XCU-ERN-56 is applied, aligning the **-m** option with the
> IEEE P1003.2b draft standard to clarify an ambiguity.
>
> SD5-XCU-ERN-97 is applied, updating the SYNOPSIS.
>
> POSIX.1-2008, Technical Corrigendum 1, XCU/TC1-2008/0122 \[161\] is
> applied.
>
> POSIX.1-2008, Technical Corrigendum 2, XCU/TC2-2008/0145 \[843\] is
> applied.

