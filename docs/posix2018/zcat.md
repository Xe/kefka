#### []{#tag_20_160_01}NAME {#name .mansect}

> zcat - expand and concatenate data

#### []{#tag_20_160_02}SYNOPSIS {#synopsis .mansect}

> ::: box
> ^`[`[`XSI`](javascript:open_code('XSI'))`]`^` `![`[Option Start]`](../images/opt-start.gif){border="0"}` zcat`` `**`[`***`file`*`...`**`]`**![`[Option End]`](../images/opt-end.gif){border="0"}
> :::

#### []{#tag_20_160_03}DESCRIPTION {#description .mansect}

> The *zcat* utility shall write to standard output the uncompressed
> form of files that have been compressed using the
> [*compress*](../utilities/compress.html) utility. It is the equivalent
> of [*uncompress*](../utilities/uncompress.html) **-c**. Input files
> are not affected.

#### []{#tag_20_160_04}OPTIONS {#options .mansect}

> None.

#### []{#tag_20_160_05}OPERANDS {#operands .mansect}

> The following operand shall be supported:
>
> *file*
> :   The pathname of a file previously processed by the
>     [*compress*](../utilities/compress.html) utility. If *file*
>     already has the **.Z** suffix specified, it is used as submitted.
>     Otherwise, the **.Z** suffix is appended to the filename prior to
>     processing.

#### []{#tag_20_160_06}STDIN {#stdin .mansect}

> The standard input shall be used only if no *file* operands are
> specified, or if a *file* operand is `'-'` .

#### []{#tag_20_160_07}INPUT FILES {#input-files .mansect}

> Input files shall be compressed files that are in the format produced
> by the [*compress*](../utilities/compress.html) utility.

#### []{#tag_20_160_08}ENVIRONMENT VARIABLES {#environment-variables .mansect}

> The following environment variables shall affect the execution of
> *zcat*:
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
> :   Determine the location of message catalogs for the processing of
>     *LC_MESSAGES.*

#### []{#tag_20_160_09}ASYNCHRONOUS EVENTS {#asynchronous-events .mansect}

> Default.

#### []{#tag_20_160_10}STDOUT {#stdout .mansect}

> The compressed files given as input shall be written on standard
> output in their uncompressed form.

#### []{#tag_20_160_11}STDERR {#stderr .mansect}

> The standard error shall be used only for diagnostic messages.

#### []{#tag_20_160_12}OUTPUT FILES {#output-files .mansect}

> None.

#### []{#tag_20_160_13}EXTENDED DESCRIPTION {#extended-description .mansect}

> None.

#### []{#tag_20_160_14}EXIT STATUS {#exit-status .mansect}

> The following exit values shall be returned:
>
>  0
> :   Successful completion.
>
> \>0
> :   An error occurred.

#### []{#tag_20_160_15}CONSEQUENCES OF ERRORS {#consequences-of-errors .mansect}

> Default.

------------------------------------------------------------------------

::: box
*The following sections are informative.*
:::

#### []{#tag_20_160_16}APPLICATION USAGE {#application-usage .mansect}

> None.

#### []{#tag_20_160_17}EXAMPLES {#examples .mansect}

> None.

#### []{#tag_20_160_18}RATIONALE {#rationale .mansect}

> None.

#### []{#tag_20_160_19}FUTURE DIRECTIONS {#future-directions .mansect}

> None.

#### []{#tag_20_160_20}SEE ALSO {#see-also .mansect}

> [*compress*](../utilities/compress.html#),
> [*uncompress*](../utilities/uncompress.html#)
>
> XBD [*Environment Variables*](../basedefs/V1_chap08.html#tag_08)

#### []{#tag_20_160_21}CHANGE HISTORY {#change-history .mansect}

> First released in Issue 4.

