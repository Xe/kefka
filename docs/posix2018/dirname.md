#### []{#tag_20_35_01}NAME {#name .mansect}

> dirname - return the directory portion of a pathname

#### []{#tag_20_35_02}SYNOPSIS {#synopsis .mansect}

> `dirname`` `*`string`*

#### []{#tag_20_35_03}DESCRIPTION {#description .mansect}

> The *string* operand shall be treated as a pathname, as defined in XBD
> [*Pathname*](../basedefs/V1_chap03.html#tag_03_271). The string
> *string* shall be converted to the name of the directory containing
> the filename corresponding to the last pathname component in *string*,
> performing actions equivalent to the following steps in order:
>
> 1.  If *string* is **//**, skip steps 2 to 5.
>
> 2.  If *string* consists entirely of \<slash\> characters, *string*
>     shall be set to a single \<slash\> character. In this case, skip
>     steps 3 to 8.
>
> 3.  If there are any trailing \<slash\> characters in *string*, they
>     shall be removed.
>
> 4.  If there are no \<slash\> characters remaining in *string*,
>     *string* shall be set to a single \<period\> character. In this
>     case, skip steps 5 to 8.
>
> 5.  If there are any trailing non- \<slash\> characters in *string*,
>     they shall be removed.
>
> 6.  If the remaining *string* is **//**, it is implementation-defined
>     whether steps 7 and 8 are skipped or processed.
>
> 7.  If there are any trailing \<slash\> characters in *string*, they
>     shall be removed.
>
> 8.  If the remaining *string* is empty, *string* shall be set to a
>     single \<slash\> character.
>
> The resulting string shall be written to standard output.

#### []{#tag_20_35_04}OPTIONS {#options .mansect}

> None.

#### []{#tag_20_35_05}OPERANDS {#operands .mansect}

> The following operand shall be supported:
>
> *string*
> :   A string.

#### []{#tag_20_35_06}STDIN {#stdin .mansect}

> Not used.

#### []{#tag_20_35_07}INPUT FILES {#input-files .mansect}

> None.

#### []{#tag_20_35_08}ENVIRONMENT VARIABLES {#environment-variables .mansect}

> The following environment variables shall affect the execution of
> *dirname*:
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

#### []{#tag_20_35_09}ASYNCHRONOUS EVENTS {#asynchronous-events .mansect}

> Default.

#### []{#tag_20_35_10}STDOUT {#stdout .mansect}

> The *dirname* utility shall write a line to the standard output in the
> following format:
>
>
>     "%s\n", <resulting string>

#### []{#tag_20_35_11}STDERR {#stderr .mansect}

> The standard error shall be used only for diagnostic messages.

#### []{#tag_20_35_12}OUTPUT FILES {#output-files .mansect}

> None.

#### []{#tag_20_35_13}EXTENDED DESCRIPTION {#extended-description .mansect}

> None.

#### []{#tag_20_35_14}EXIT STATUS {#exit-status .mansect}

> The following exit values shall be returned:
>
>  0
> :   Successful completion.
>
> \>0
> :   An error occurred.

#### []{#tag_20_35_15}CONSEQUENCES OF ERRORS {#consequences-of-errors .mansect}

> Default.

------------------------------------------------------------------------

::: box
*The following sections are informative.*
:::

#### []{#tag_20_35_16}APPLICATION USAGE {#application-usage .mansect}

> The definition of *pathname* specifies implementation-defined behavior
> for pathnames starting with two \<slash\> characters. Therefore,
> applications shall not arbitrarily add \<slash\> characters to the
> beginning of a pathname unless they can ensure that there are more or
> less than two or are prepared to deal with the implementation-defined
> consequences.

#### []{#tag_20_35_17}EXAMPLES {#examples .mansect}

> The EXAMPLES section of the [*basename*()](../functions/basename.html)
> function (see XSH [*basename*](../functions/basename.html#tag_16_32))
> includes a table showing examples of the results of processing several
> sample pathnames by the [*basename*()](../functions/basename.html) and
> [*dirname*()](../functions/dirname.html) functions and by the
> [*basename*](../utilities/basename.html) and *dirname* utilities.
>
> See also the examples for the [*basename*](../utilities/basename.html)
> utility.

#### []{#tag_20_35_18}RATIONALE {#rationale .mansect}

> The behaviors of [*basename*](../utilities/basename.html) and
> *dirname* in this volume of POSIX.1-2017 have been coordinated so that
> when *string* is a valid pathname:
>
>
>     $(basename -- "string")
>
> would be a valid filename for the file in the directory:
>
>
>     $(dirname -- "string")
>
> This would not work for the versions of these utilities in early
> proposals due to the way processing of trailing \<slash\> characters
> was specified. Consideration was given to leaving processing
> unspecified if there were trailing \<slash\> characters, but this
> cannot be done; XBD
> [*Pathname*](../basedefs/V1_chap03.html#tag_03_271) allows trailing
> \<slash\> characters. The [*basename*](../utilities/basename.html) and
> *dirname* utilities have to specify consistent handling for all valid
> pathnames.

#### []{#tag_20_35_19}FUTURE DIRECTIONS {#future-directions .mansect}

> None.

#### []{#tag_20_35_20}SEE ALSO {#see-also .mansect}

> [*Parameters and Variables*](../utilities/V3_chap02.html#tag_18_05),
> [*basename*](../utilities/basename.html#tag_20_07)
>
> XBD [*Pathname*](../basedefs/V1_chap03.html#tag_03_271), [*Environment
> Variables*](../basedefs/V1_chap08.html#tag_08)
>
> XSH [*basename*](../functions/basename.html#tag_16_32),
> [*dirname*](../functions/dirname.html#tag_16_91)

#### []{#tag_20_35_21}CHANGE HISTORY {#change-history .mansect}

> First released in Issue 2.

#### []{#tag_20_35_22}Issue 7 {#issue-7 .mansect}

> POSIX.1-2008, Technical Corrigendum 1, XCU/TC1-2008/0083 \[192,430\],
> XCU/TC1-2008/0084 \[192\], and XCU/TC1-2008/0085 \[192\] are applied.
>
> POSIX.1-2008, Technical Corrigendum 2, XCU/TC2-2008/0086 \[612\],
> XCU/TC2-2008/0087 \[620\], and XCU/TC2-2008/0088 \[612\] are applied.

