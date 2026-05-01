#### []{#tag_20_97_01}NAME {#name .mansect}

> pwd - return working directory name

#### []{#tag_20_97_02}SYNOPSIS {#synopsis .mansect}

> `pwd`` `**`[`**`-L|-P`**`]`**

#### []{#tag_20_97_03}DESCRIPTION {#description .mansect}

> The *pwd* utility shall write to standard output an absolute pathname
> of the current working directory, which does not contain the filenames
> dot or dot-dot.

#### []{#tag_20_97_04}OPTIONS {#options .mansect}

> The *pwd* utility shall conform to XBD [*Utility Syntax
> Guidelines*](../basedefs/V1_chap12.html#tag_12_02) .
>
> The following options shall be supported by the implementation:
>
> **-L**
> :   If the *PWD* environment variable contains an absolute pathname of
>     the current directory and the pathname does not contain any
>     components that are dot or dot-dot, *pwd* shall write this
>     pathname to standard output, except that if the *PWD* environment
>     variable is longer than {PATH_MAX} bytes including the terminating
>     null, it is unspecified whether *pwd* writes this pathname to
>     standard output or behaves as if the **-P** option had been
>     specified. Otherwise, the **-L** option shall behave as the **-P**
>     option.
>
> **-P**
> :   The pathname written to standard output shall not contain any
>     components that refer to files of type symbolic link. If there are
>     multiple pathnames that the *pwd* utility could write to standard
>     output, one beginning with a single \<slash\> character and one or
>     more beginning with two \<slash\> characters, then it shall write
>     the pathname beginning with a single \<slash\> character. The
>     pathname shall not contain any unnecessary \<slash\> characters
>     after the leading one or two \<slash\> characters.
>
> If both **-L** and **-P** are specified, the last one shall apply. If
> neither **-L** nor **-P** is specified, the *pwd* utility shall behave
> as if **-L** had been specified.

#### []{#tag_20_97_05}OPERANDS {#operands .mansect}

> None.

#### []{#tag_20_97_06}STDIN {#stdin .mansect}

> Not used.

#### []{#tag_20_97_07}INPUT FILES {#input-files .mansect}

> None.

#### []{#tag_20_97_08}ENVIRONMENT VARIABLES {#environment-variables .mansect}

> The following environment variables shall affect the execution of
> *pwd*:
>
> *LANG*
> :   Provide a default value for the internationalization variables
>     that are unset or null. (See XBD [*Internationalization
>     Variables*](../basedefs/V1_chap08.html#tag_08_02) the precedence
>     of internationalization variables used to determine the values of
>     locale categories.)
>
> *LC_ALL*
> :   If set to a non-empty string value, override the values of all the
>     other internationalization variables.
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
>
> *PWD*
> :   An absolute pathname of the current working directory. If an
>     application sets or unsets the value of *PWD,* the behavior of
>     *pwd* is unspecified.

#### []{#tag_20_97_09}ASYNCHRONOUS EVENTS {#asynchronous-events .mansect}

> Default.

#### []{#tag_20_97_10}STDOUT {#stdout .mansect}

> The *pwd* utility output is an absolute pathname of the current
> working directory:
>
>
>     "%s\n", <directory pathname>

#### []{#tag_20_97_11}STDERR {#stderr .mansect}

> The standard error shall be used only for diagnostic messages.

#### []{#tag_20_97_12}OUTPUT FILES {#output-files .mansect}

> None.

#### []{#tag_20_97_13}EXTENDED DESCRIPTION {#extended-description .mansect}

> None.

#### []{#tag_20_97_14}EXIT STATUS {#exit-status .mansect}

> The following exit values shall be returned:
>
>  0
> :   Successful completion.
>
> \>0
> :   An error occurred.

#### []{#tag_20_97_15}CONSEQUENCES OF ERRORS {#consequences-of-errors .mansect}

> If an error is detected, output shall not be written to standard
> output, a diagnostic message shall be written to standard error, and
> the exit status is not zero.

------------------------------------------------------------------------

::: box
*The following sections are informative.*
:::

#### []{#tag_20_97_16}APPLICATION USAGE {#application-usage .mansect}

> If the pathname obtained from *pwd* is longer than {PATH_MAX} bytes,
> it could produce an error if passed to [*cd*](../utilities/cd.html).
> Therefore, in order to return to that directory it may be necessary to
> break the pathname into sections shorter than {PATH_MAX} and call
> [*cd*](../utilities/cd.html) on each section in turn (the first
> section being an absolute pathname and subsequent sections being
> relative pathnames).

#### []{#tag_20_97_17}EXAMPLES {#examples .mansect}

> None.

#### []{#tag_20_97_18}RATIONALE {#rationale .mansect}

> Some implementations have historically provided *pwd* as a shell
> special built-in command.
>
> In most utilities, if an error occurs, partial output may be written
> to standard output. This does not happen in historical implementations
> of *pwd*. Because *pwd* is frequently used in historical shell scripts
> without checking the exit status, it is important that the historical
> behavior is required here; therefore, the CONSEQUENCES OF ERRORS
> section specifically disallows any partial output being written to
> standard output.
>
> An earlier version of this standard stated that the *PWD* environment
> variable was affected when the **-P** option was in effect. This was
> incorrect; conforming implementations do not do this.

#### []{#tag_20_97_19}FUTURE DIRECTIONS {#future-directions .mansect}

> None.

#### []{#tag_20_97_20}SEE ALSO {#see-also .mansect}

> [*cd*](../utilities/cd.html#)
>
> XBD [*Environment Variables*](../basedefs/V1_chap08.html#tag_08),
> [*Utility Syntax Guidelines*](../basedefs/V1_chap12.html#tag_12_02)
>
> XSH [*getcwd*](../functions/getcwd.html#)

#### []{#tag_20_97_21}CHANGE HISTORY {#change-history .mansect}

> First released in Issue 2.

#### []{#tag_20_97_22}Issue 6 {#issue-6 .mansect}

> The **-P** and **-L** options are added to describe actions relating
> to symbolic links as specified in the IEEE P1003.2b draft standard.

#### []{#tag_20_97_23}Issue 7 {#issue-7 .mansect}

> Austin Group Interpretation 1003.1-2001 #097 is applied.
>
> SD5-XCU-ERN-97 is applied, updating the SYNOPSIS.
>
> Changes to the *pwd* utility and *PWD* environment variable have been
> made to match the changes to the
> [*getcwd*()](../functions/getcwd.html) function made for Austin Group
> Interpretation 1003.1-2001 #140.
>
> POSIX.1-2008, Technical Corrigendum 2, XCU/TC2-2008/0161 \[471\] is
> applied.

