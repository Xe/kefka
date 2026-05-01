#### []{#tag_20_12_01}NAME {#name .mansect}

> cal - print a calendar

#### []{#tag_20_12_02}SYNOPSIS {#synopsis .mansect}

> ::: box
> ^`[`[`XSI`](javascript:open_code('XSI'))`]`^` `![`[Option Start]`](../images/opt-start.gif){border="0"}` cal`` `**`[[`***`month`***`]`**` `*`year`***`]`**![`[Option End]`](../images/opt-end.gif){border="0"}
> :::

#### []{#tag_20_12_03}DESCRIPTION {#description .mansect}

> The *cal* utility shall write a calendar to standard output using the
> Julian calendar for dates from January 1, 1 through September 2, 1752
> and the Gregorian calendar for dates from September 14, 1752 through
> December 31, 9999 as though the Gregorian calendar had been adopted on
> September 14, 1752.
>
> If no operands are given, *cal* shall produce a one-month calendar for
> the current month in the current year. If only the *year* operand is
> given, *cal* shall produce a calendar for all twelve months in the
> given calendar year. If both *month* and *year* operands are given,
> *cal* shall produce a one-month calendar for the given month in the
> given year.

#### []{#tag_20_12_04}OPTIONS {#options .mansect}

> None.

#### []{#tag_20_12_05}OPERANDS {#operands .mansect}

> The following operands shall be supported:
>
> *month*
> :   Specify the month to be displayed, represented as a decimal
>     integer from 1 (January) to 12 (December).
>
> *year*
> :   Specify the year for which the calendar is displayed, represented
>     as a decimal integer from 1 to 9999.

#### []{#tag_20_12_06}STDIN {#stdin .mansect}

> Not used.

#### []{#tag_20_12_07}INPUT FILES {#input-files .mansect}

> None.

#### []{#tag_20_12_08}ENVIRONMENT VARIABLES {#environment-variables .mansect}

> The following environment variables shall affect the execution of
> *cal*:
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
>     contents of diagnostic messages written to standard error, and
>     informative messages written to standard output.
>
> *LC_TIME*
> :   Determine the format and contents of the calendar.
>
> *NLSPATH*
> :   Determine the location of message catalogs for the processing of
>     *LC_MESSAGES.*
>
> *TZ*
> :   Determine the timezone used to calculate the value of the current
>     month.

#### []{#tag_20_12_09}ASYNCHRONOUS EVENTS {#asynchronous-events .mansect}

> Default.

#### []{#tag_20_12_10}STDOUT {#stdout .mansect}

> The standard output shall be used to display the calendar, in an
> unspecified format.

#### []{#tag_20_12_11}STDERR {#stderr .mansect}

> The standard error shall be used only for diagnostic messages.

#### []{#tag_20_12_12}OUTPUT FILES {#output-files .mansect}

> None.

#### []{#tag_20_12_13}EXTENDED DESCRIPTION {#extended-description .mansect}

> None.

#### []{#tag_20_12_14}EXIT STATUS {#exit-status .mansect}

> The following exit values shall be returned:
>
>  0
> :   Successful completion.
>
> \>0
> :   An error occurred.

#### []{#tag_20_12_15}CONSEQUENCES OF ERRORS {#consequences-of-errors .mansect}

> Default.

------------------------------------------------------------------------

::: box
*The following sections are informative.*
:::

#### []{#tag_20_12_16}APPLICATION USAGE {#application-usage .mansect}

> Note that:
>
>
>     cal 83
>
> refers to A.D. 83, not 1983.

#### []{#tag_20_12_17}EXAMPLES {#examples .mansect}

> None.

#### []{#tag_20_12_18}RATIONALE {#rationale .mansect}

> Earlier versions of this standard incorrectly required that the
> command:
>
>
>     cal 2000
>
> write a one-month calendar for the current calendar month (no matter
> what the current year is) in the year 2000 to standard output. This
> did not match historic practice in any known version of the *cal*
> utility. The description has been updated to match historic practice.
> When only the *year* operand is given, *cal* writes a twelve-month
> calendar for the specified year.

#### []{#tag_20_12_19}FUTURE DIRECTIONS {#future-directions .mansect}

> A future version of this standard may support locale-specific
> recognition of the date of adoption of the Gregorian calendar.

#### []{#tag_20_12_20}SEE ALSO {#see-also .mansect}

> XBD [*Environment Variables*](../basedefs/V1_chap08.html#tag_08)

#### []{#tag_20_12_21}CHANGE HISTORY {#change-history .mansect}

> First released in Issue 2.

#### []{#tag_20_12_22}Issue 6 {#issue-6 .mansect}

> The DESCRIPTION is updated to allow for traditional behavior for years
> before the adoption of the Gregorian calendar.

#### []{#tag_20_12_23}Issue 7 {#issue-7 .mansect}

> SD5-XCU-ERN-97 is applied, updating the SYNOPSIS.
>
> POSIX.1-2008, Technical Corrigendum 1, XCU/TC1-2008/0074 \[56\] and
> XCU/TC1-2008/0075 \[56\] are applied.

