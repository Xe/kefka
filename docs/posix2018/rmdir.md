#### []{#tag_20_113_01}NAME {#name .mansect}

> rmdir - remove directories

#### []{#tag_20_113_02}SYNOPSIS {#synopsis .mansect}

> `rmdir`` `**`[`**`-p`**`]`**` `*`dir`*`...`

#### []{#tag_20_113_03}DESCRIPTION {#description .mansect}

> The *rmdir* utility shall remove the directory entry specified by each
> *dir* operand.
>
> For each *dir* operand, the *rmdir* utility shall perform actions
> equivalent to the [*rmdir*()](../functions/rmdir.html) function called
> with the *dir* operand as its only argument.
>
> Directories shall be processed in the order specified. If a directory
> and a subdirectory of that directory are specified in a single
> invocation of the *rmdir* utility, the application shall specify the
> subdirectory before the parent directory so that the parent directory
> will be empty when the *rmdir* utility tries to remove it.

#### []{#tag_20_113_04}OPTIONS {#options .mansect}

> The *rmdir* utility shall conform to XBD [*Utility Syntax
> Guidelines*](../basedefs/V1_chap12.html#tag_12_02).
>
> The following option shall be supported:
>
> **-p**
> :   Remove all directories in a pathname. For each *dir* operand:
>     1.  The directory entry it names shall be removed.
>
>     2.  If the *dir* operand includes more than one pathname
>         component, effects equivalent to the following command shall
>         occur:
>
>
>             rmdir -p $(dirname dir)

#### []{#tag_20_113_05}OPERANDS {#operands .mansect}

> The following operand shall be supported:
>
> *dir*
> :   A pathname of an empty directory to be removed.

#### []{#tag_20_113_06}STDIN {#stdin .mansect}

> Not used.

#### []{#tag_20_113_07}INPUT FILES {#input-files .mansect}

> None.

#### []{#tag_20_113_08}ENVIRONMENT VARIABLES {#environment-variables .mansect}

> The following environment variables shall affect the execution of
> *rmdir*:
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

#### []{#tag_20_113_09}ASYNCHRONOUS EVENTS {#asynchronous-events .mansect}

> Default.

#### []{#tag_20_113_10}STDOUT {#stdout .mansect}

> Not used.

#### []{#tag_20_113_11}STDERR {#stderr .mansect}

> The standard error shall be used only for diagnostic messages.

#### []{#tag_20_113_12}OUTPUT FILES {#output-files .mansect}

> None.

#### []{#tag_20_113_13}EXTENDED DESCRIPTION {#extended-description .mansect}

> None.

#### []{#tag_20_113_14}EXIT STATUS {#exit-status .mansect}

> The following exit values shall be returned:
>
>  0
> :   Each directory entry specified by a *dir* operand was removed
>     successfully.
>
> \>0
> :   An error occurred.

#### []{#tag_20_113_15}CONSEQUENCES OF ERRORS {#consequences-of-errors .mansect}

> Default.

------------------------------------------------------------------------

::: box
*The following sections are informative.*
:::

#### []{#tag_20_113_16}APPLICATION USAGE {#application-usage .mansect}

> The definition of an empty directory is one that contains, at most,
> directory entries for dot and dot-dot.

#### []{#tag_20_113_17}EXAMPLES {#examples .mansect}

> If a directory **a** in the current directory is empty except it
> contains a directory **b** and **a/b** is empty except it contains a
> directory **c**:
>
>
>     rmdir -p a/b/c
>
> removes all three directories.

#### []{#tag_20_113_18}RATIONALE {#rationale .mansect}

> On historical System V systems, the **-p** option also caused a
> message to be written to the standard output. The message indicated
> whether the whole path was removed or whether part of the path
> remained for some reason. The STDERR section requires this diagnostic
> when the entire path specified by a *dir* operand is not removed, but
> does not allow the status message reporting success to be written as a
> diagnostic.
>
> The *rmdir* utility on System V also included a **-s** option that
> suppressed the informational message output by the **-p** option. This
> option has been omitted because the informational message is not
> specified by this volume of POSIX.1-2017.

#### []{#tag_20_113_19}FUTURE DIRECTIONS {#future-directions .mansect}

> None.

#### []{#tag_20_113_20}SEE ALSO {#see-also .mansect}

> [*rm*](../utilities/rm.html#)
>
> XBD [*Environment Variables*](../basedefs/V1_chap08.html#tag_08),
> [*Utility Syntax Guidelines*](../basedefs/V1_chap12.html#tag_12_02)
>
> XSH [*remove*](../functions/remove.html#),
> [*rmdir*](../functions/rmdir.html#tag_16_491),
> [*unlink*](../functions/unlink.html#tag_16_635)

#### []{#tag_20_113_21}CHANGE HISTORY {#change-history .mansect}

> First released in Issue 2.

#### []{#tag_20_113_22}Issue 6 {#issue-6 .mansect}

> The normative text is reworded to avoid use of the term \"must\" for
> application requirements.

