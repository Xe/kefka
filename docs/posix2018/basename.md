#### []{#tag_20_07_01}NAME {#name .mansect}

> basename - return non-directory portion of a pathname

#### []{#tag_20_07_02}SYNOPSIS {#synopsis .mansect}

> `basename`` `*`string`*` `**`[`***`suffix`***`]`**

#### []{#tag_20_07_03}DESCRIPTION {#description .mansect}

> The *string* operand shall be treated as a pathname, as defined in XBD
> [*Pathname*](../basedefs/V1_chap03.html#tag_03_271). The string
> *string* shall be converted to the filename corresponding to the last
> pathname component in *string* and then the suffix string *suffix*, if
> present, shall be removed. This shall be done by performing actions
> equivalent to the following steps in order:
>
> 1.  If *string* is a null string, it is unspecified whether the
>     resulting string is `'.'` or a null string. In either case, skip
>     steps 2 through 6.
>
> 2.  If *string* is `"//"`, it is implementation-defined whether steps
>     3 to 6 are skipped or processed.
>
> 3.  If *string* consists entirely of \<slash\> characters, *string*
>     shall be set to a single \<slash\> character. In this case, skip
>     steps 4 to 6.
>
> 4.  If there are any trailing \<slash\> characters in *string*, they
>     shall be removed.
>
> 5.  If there are any \<slash\> characters remaining in *string*, the
>     prefix of *string* up to and including the last \<slash\>
>     character in *string* shall be removed.
>
> 6.  If the *suffix* operand is present, is not identical to the
>     characters remaining in *string*, and is identical to a suffix of
>     the characters remaining in *string*, the suffix *suffix* shall be
>     removed from *string*. Otherwise, *string* is not modified by this
>     step. It shall not be considered an error if *suffix* is not found
>     in *string*.
>
> The resulting string shall be written to standard output.

#### []{#tag_20_07_04}OPTIONS {#options .mansect}

> None.

#### []{#tag_20_07_05}OPERANDS {#operands .mansect}

> The following operands shall be supported:
>
> *string*
> :   A string.
>
> *suffix*
> :   A string.

#### []{#tag_20_07_06}STDIN {#stdin .mansect}

> Not used.

#### []{#tag_20_07_07}INPUT FILES {#input-files .mansect}

> None.

#### []{#tag_20_07_08}ENVIRONMENT VARIABLES {#environment-variables .mansect}

> The following environment variables shall affect the execution of
> *basename*:
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

#### []{#tag_20_07_09}ASYNCHRONOUS EVENTS {#asynchronous-events .mansect}

> Default.

#### []{#tag_20_07_10}STDOUT {#stdout .mansect}

> The *basename* utility shall write a line to the standard output in
> the following format:
>
>
>     "%s\n", <resulting string>

#### []{#tag_20_07_11}STDERR {#stderr .mansect}

> The standard error shall be used only for diagnostic messages.

#### []{#tag_20_07_12}OUTPUT FILES {#output-files .mansect}

> None.

#### []{#tag_20_07_13}EXTENDED DESCRIPTION {#extended-description .mansect}

> None.

#### []{#tag_20_07_14}EXIT STATUS {#exit-status .mansect}

> The following exit values shall be returned:
>
>  0
> :   Successful completion.
>
> \>0
> :   An error occurred.

#### []{#tag_20_07_15}CONSEQUENCES OF ERRORS {#consequences-of-errors .mansect}

> Default.

------------------------------------------------------------------------

::: box
*The following sections are informative.*
:::

#### []{#tag_20_07_16}APPLICATION USAGE {#application-usage .mansect}

> The definition of *pathname* specifies implementation-defined behavior
> for pathnames starting with two \<slash\> characters. Therefore,
> applications shall not arbitrarily add \<slash\> characters to the
> beginning of a pathname unless they can ensure that there are more or
> less than two or are prepared to deal with the implementation-defined
> consequences.

#### []{#tag_20_07_17}EXAMPLES {#examples .mansect}

> If the string *string* is a valid pathname:
>
>
>     $(basename -- "string")
>
> produces a filename that could be used to open the file named by
> *string* in the directory returned by:
>
>
>     $(dirname -- "string")
>
> If the string *string* is not a valid pathname, the same algorithm is
> used, but the result need not be a valid filename. The *basename*
> utility is not expected to make any judgements about the validity of
> *string* as a pathname; it just follows the specified algorithm to
> produce a result string.
>
> The following shell script compiles **/usr/src/cmd/cat.c** and moves
> the output to a file named **cat** in the current directory when
> invoked with the argument **/usr/src/cmd/cat** or with the argument
> **/usr/src/cmd/cat.c**:
>
>
>     c99 -- "$(dirname -- "$1")/$(basename -- "$1" .c).c" &&
>     mv a.out "$(basename -- "$1" .c)"
>
> The EXAMPLES section of the [*basename*()](../functions/basename.html)
> function (see XSH [*basename*](../functions/basename.html#tag_16_32))
> includes a table showing examples of the results of processing several
> sample pathnames by the [*basename*()](../functions/basename.html) and
> [*dirname*()](../functions/dirname.html) functions and by the
> *basename* and [*dirname*](../utilities/dirname.html) utilities.

#### []{#tag_20_07_18}RATIONALE {#rationale .mansect}

> The behaviors of *basename* and [*dirname*](../utilities/dirname.html)
> have been coordinated so that when *string* is a valid pathname:
>
>
>     $(basename -- "string")
>
> would be a valid filename for the file in the directory:
>
>
>     $(dirname -- "string")
>
> This would not work for the early proposal versions of these utilities
> due to the way it specified handling of trailing \<slash\> characters.
>
> Since the definition of *pathname* specifies implementation-defined
> behavior for pathnames starting with two \<slash\> characters, this
> volume of POSIX.1-2017 specifies similar implementation-defined
> behavior for the *basename* and [*dirname*](../utilities/dirname.html)
> utilities.

#### []{#tag_20_07_19}FUTURE DIRECTIONS {#future-directions .mansect}

> None.

#### []{#tag_20_07_20}SEE ALSO {#see-also .mansect}

> [*Parameters and Variables*](../utilities/V3_chap02.html#tag_18_05),
> [*dirname*](../utilities/dirname.html#tag_20_35)
>
> XBD [*Pathname*](../basedefs/V1_chap03.html#tag_03_271), [*Environment
> Variables*](../basedefs/V1_chap08.html#tag_08)
>
> XSH [*basename*](../functions/basename.html#tag_16_32),
> [*dirname*](../functions/dirname.html#tag_16_91)

#### []{#tag_20_07_21}CHANGE HISTORY {#change-history .mansect}

> First released in Issue 2.

#### []{#tag_20_07_22}Issue 6 {#issue-6 .mansect}

> IEEE PASC Interpretation 1003.2 #164 is applied.
>
> The normative text is reworded to avoid use of the term \"must\" for
> application requirements.

#### []{#tag_20_07_23}Issue 7 {#issue-7 .mansect}

> POSIX.1-2008, Technical Corrigendum 1, XCU/TC1-2008/0065 \[192,538\],
> XCU/TC1-2008/0066 \[192,538\], and XCU/TC1-2008/0067 \[192,430,538\]
> are applied.
>
> POSIX.1-2008, Technical Corrigendum 2, XCU/TC2-2008/0065 \[612\] is
> applied.

