#### []{#tag_20_118_01}NAME {#name .mansect}

> sleep - suspend execution for an interval

#### []{#tag_20_118_02}SYNOPSIS {#synopsis .mansect}

> `sleep`` `*`time`*

#### []{#tag_20_118_03}DESCRIPTION {#description .mansect}

> The *sleep* utility shall suspend execution for at least the integral
> number of seconds specified by the *time* operand.

#### []{#tag_20_118_04}OPTIONS {#options .mansect}

> None.

#### []{#tag_20_118_05}OPERANDS {#operands .mansect}

> The following operand shall be supported:
>
> *time*
> :   A non-negative decimal integer specifying the number of seconds
>     for which to suspend execution.

#### []{#tag_20_118_06}STDIN {#stdin .mansect}

> Not used.

#### []{#tag_20_118_07}INPUT FILES {#input-files .mansect}

> None.

#### []{#tag_20_118_08}ENVIRONMENT VARIABLES {#environment-variables .mansect}

> The following environment variables shall affect the execution of
> *sleep*:
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

#### []{#tag_20_118_09}ASYNCHRONOUS EVENTS {#asynchronous-events .mansect}

> If the *sleep* utility receives a SIGALRM signal, one of the following
> actions shall be taken:
>
> 1.  Terminate normally with a zero exit status.
>
> 2.  Effectively ignore the signal.
>
> 3.  Provide the default behavior for signals described in the
>     ASYNCHRONOUS EVENTS section of [*Utility Description
>     Defaults*](../utilities/V3_chap01.html#tag_17_04). This could
>     include terminating with a non-zero exit status.
>
> The *sleep* utility shall take the standard action for all other
> signals.

#### []{#tag_20_118_10}STDOUT {#stdout .mansect}

> Not used.

#### []{#tag_20_118_11}STDERR {#stderr .mansect}

> The standard error shall be used only for diagnostic messages.

#### []{#tag_20_118_12}OUTPUT FILES {#output-files .mansect}

> None.

#### []{#tag_20_118_13}EXTENDED DESCRIPTION {#extended-description .mansect}

> None.

#### []{#tag_20_118_14}EXIT STATUS {#exit-status .mansect}

> The following exit values shall be returned:
>
>  0
> :   The execution was successfully suspended for at least *time*
>     seconds, or a SIGALRM signal was received. See the ASYNCHRONOUS
>     EVENTS section.
>
> \>0
> :   An error occurred.

#### []{#tag_20_118_15}CONSEQUENCES OF ERRORS {#consequences-of-errors .mansect}

> Default.

------------------------------------------------------------------------

::: box
*The following sections are informative.*
:::

#### []{#tag_20_118_16}APPLICATION USAGE {#application-usage .mansect}

> None.

#### []{#tag_20_118_17}EXAMPLES {#examples .mansect}

> The *sleep* utility can be used to execute a command after a certain
> amount of time, as in:
>
>
>     (sleep 105; command) &
>
> or to execute a command every so often, as in:
>
>
>     while true
>     do
>         command    
>        sleep 37
>     done

#### []{#tag_20_118_18}RATIONALE {#rationale .mansect}

> The exit status is allowed to be zero when *sleep* is interrupted by
> the SIGALRM signal because most implementations of this utility rely
> on the arrival of that signal to notify them that the requested
> finishing time has been successfully attained. Such implementations
> thus do not distinguish this situation from the successful completion
> case. Other implementations are allowed to catch the signal and go
> back to sleep until the requested time expires or to provide the
> normal signal termination procedures.
>
> As with all other utilities that take integral operands and do not
> specify subranges of allowed values, *sleep* is required by this
> volume of POSIX.1-2017 to deal with *time* requests of up to
> 2147483647 seconds. This may mean that some implementations have to
> make multiple calls to the delay mechanism of the underlying operating
> system if its argument range is less than this.

#### []{#tag_20_118_19}FUTURE DIRECTIONS {#future-directions .mansect}

> None.

#### []{#tag_20_118_20}SEE ALSO {#see-also .mansect}

> [*wait*](../utilities/wait.html#tag_20_153)
>
> XBD [*Environment Variables*](../basedefs/V1_chap08.html#tag_08)
>
> XSH [*alarm*](../functions/alarm.html#),
> [*sleep*](../functions/sleep.html#tag_16_560)

#### []{#tag_20_118_21}CHANGE HISTORY {#change-history .mansect}

> First released in Issue 2.

