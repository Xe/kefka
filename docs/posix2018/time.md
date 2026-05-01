#### []{#tag_20_129_01}NAME {#name .mansect}

> time - time a simple command

#### []{#tag_20_129_02}SYNOPSIS {#synopsis .mansect}

> `time`` `**`[`**`-p`**`]`**` `*`utility`*` `**`[`***`argument`*`...`**`]`**

#### []{#tag_20_129_03}DESCRIPTION {#description .mansect}

> The *time* utility shall invoke the utility named by the *utility*
> operand with arguments supplied as the *argument* operands and write a
> message to standard error that lists timing statistics for the
> utility. The message shall include the following information:
>
> - The elapsed (real) time between invocation of *utility* and its
>   termination.
>
> - The User CPU time, equivalent to the sum of the *tms_utime* and
>   *tms_cutime* fields returned by the
>   [*times*()](../functions/times.html) function defined in the System
>   Interfaces volume of POSIX.1-2017 for the process in which *utility*
>   is executed.
>
> - The System CPU time, equivalent to the sum of the *tms_stime* and
>   *tms_cstime* fields returned by the
>   [*times*()](../functions/times.html) function for the process in
>   which *utility* is executed.
>
> The precision of the timing shall be no less than the granularity
> defined for the size of the clock tick unit on the system, but the
> results shall be reported in terms of standard time units (for
> example, 0.02 seconds, 00:00:00.02, 1m33.75s, 365.21 seconds), not
> numbers of clock ticks.
>
> When *time* is used as part of a pipeline, the times reported are
> unspecified, except when it is the sole command within a grouping
> command (see [*Grouping
> Commands*](../utilities/V3_chap02.html#tag_18_09_04_01)) in that
> pipeline. For example, the commands on the left are unspecified; those
> on the right report on utilities **a** and **c**, respectively:
>
>
>     time a | b | c    { time a; } | b | c
>     a | b | time c    a | b | (time c)

#### []{#tag_20_129_04}OPTIONS {#options .mansect}

> The *time* utility shall conform to XBD [*Utility Syntax
> Guidelines*](../basedefs/V1_chap12.html#tag_12_02) .
>
> The following option shall be supported:
>
> **-p**
> :   Write the timing output to standard error in the format shown in
>     the STDERR section.

#### []{#tag_20_129_05}OPERANDS {#operands .mansect}

> The following operands shall be supported:
>
> *utility*
> :   The name of a utility that is to be invoked. If the *utility*
>     operand names any of the special built-in utilities in [*Special
>     Built-In Utilities*](../utilities/V3_chap02.html#tag_18_14), the
>     results are undefined.
>
> *argument*
> :   Any string to be supplied as an argument when invoking the utility
>     named by the *utility* operand.

#### []{#tag_20_129_06}STDIN {#stdin .mansect}

> Not used.

#### []{#tag_20_129_07}INPUT FILES {#input-files .mansect}

> None.

#### []{#tag_20_129_08}ENVIRONMENT VARIABLES {#environment-variables .mansect}

> The following environment variables shall affect the execution of
> *time*:
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
>     contents of diagnostic and informative messages written to
>     standard error.
>
> *LC_NUMERIC*
> :   Determine the locale for numeric formatting.
>
> *NLSPATH*
> :   ^\[[XSI](javascript:open_code('XSI'))\]^ ![\[Option
>     Start\]](../images/opt-start.gif){border="0"} Determine the
>     location of message catalogs for the processing of *LC_MESSAGES.*
>     ![\[Option End\]](../images/opt-end.gif){border="0"}
>
> *PATH*
> :   Determine the search path that shall be used to locate the utility
>     to be invoked; see XBD [*Environment
>     Variables*](../basedefs/V1_chap08.html#tag_08).

#### []{#tag_20_129_09}ASYNCHRONOUS EVENTS {#asynchronous-events .mansect}

> Default.

#### []{#tag_20_129_10}STDOUT {#stdout .mansect}

> Not used.

#### []{#tag_20_129_11}STDERR {#stderr .mansect}

> If the *utility* utility is invoked, the standard error shall be used
> to write the timing statistics and may be used to write a diagnostic
> message if the utility terminates abnormally; otherwise, the standard
> error shall be used to write diagnostic messages and may also be used
> to write the timing statistics.
>
> If **-p** is specified, the following format shall be used for the
> timing statistics in the POSIX locale:
>
>
>     "real %f\nuser %f\nsys %f\n", <real seconds>, <user seconds>,
>         <system seconds>
>
> where each floating-point number shall be expressed in seconds. The
> precision used may be less than the default six digits of `%f`, but
> shall be sufficiently precise to accommodate the size of the clock
> tick on the system (for example, if there were 60 clock ticks per
> second, at least two digits shall follow the radix character). The
> number of digits following the radix character shall be no less than
> one, even if this always results in a trailing zero. The
> implementation may append white space and additional information
> following the format shown here. The implementation may also prepend a
> single empty line before the format shown here.

#### []{#tag_20_129_12}OUTPUT FILES {#output-files .mansect}

> None.

#### []{#tag_20_129_13}EXTENDED DESCRIPTION {#extended-description .mansect}

> None.

#### []{#tag_20_129_14}EXIT STATUS {#exit-status .mansect}

> If the *utility* utility is invoked, the exit status of *time* shall
> be the exit status of *utility*; otherwise, the *time* utility shall
> exit with one of the following values:
>
> 1-125
> :   An error occurred in the *time* utility.
>
>   126
> :   The utility specified by *utility* was found but could not be
>     invoked.
>
>   127
> :   The utility specified by *utility* could not be found.

#### []{#tag_20_129_15}CONSEQUENCES OF ERRORS {#consequences-of-errors .mansect}

> Default.

------------------------------------------------------------------------

::: box
*The following sections are informative.*
:::

#### []{#tag_20_129_16}APPLICATION USAGE {#application-usage .mansect}

> The [*command*](../utilities/command.html),
> [*env*](../utilities/env.html), [*nice*](../utilities/nice.html),
> [*nohup*](../utilities/nohup.html), *time*, and
> [*xargs*](../utilities/xargs.html) utilities have been specified to
> use exit code 127 if an error occurs so that applications can
> distinguish \"failure to find a utility\" from \"invoked utility
> exited with an error indication\". The value 127 was chosen because it
> is not commonly used for other meanings; most utilities use small
> values for \"normal error conditions\" and the values above 128 can be
> confused with termination due to receipt of a signal. The value 126
> was chosen in a similar manner to indicate that the utility could be
> found, but not invoked. Some scripts produce meaningful error messages
> differentiating the 126 and 127 cases. The distinction between exit
> codes 126 and 127 is based on KornShell practice that uses 127 when
> all attempts to *exec* the utility fail with \[ENOENT\], and uses 126
> when any attempt to *exec* the utility fails for any other reason.

#### []{#tag_20_129_17}EXAMPLES {#examples .mansect}

> It is frequently desirable to apply *time* to pipelines or lists of
> commands. This can be done by placing pipelines and command lists in a
> single file; this file can then be invoked as a utility, and the
> *time* applies to everything in the file.
>
> Alternatively, the following command can be used to apply *time* to a
> complex command:
>
>
>     time sh -c 'complex-command-line'

#### []{#tag_20_129_18}RATIONALE {#rationale .mansect}

> When the *time* utility was originally proposed to be included in the
> ISO POSIX-2:1993 standard, questions were raised about its suitability
> for inclusion on the grounds that it was not useful for conforming
> applications, specifically:
>
> - The underlying CPU definitions from the System Interfaces volume of
>   POSIX.1-2017 are vague, so the numeric output could not be compared
>   accurately between systems or even between invocations.
>
> - The creation of portable benchmark programs was outside the scope
>   this volume of POSIX.1-2017.
>
> However, *time* does fit in the scope of user portability. Human
> judgement can be applied to the analysis of the output, and it could
> be very useful in hands-on debugging of applications or in providing
> subjective measures of system performance. Hence it has been included
> in this volume of POSIX.1-2017.
>
> The default output format has been left unspecified because historical
> implementations differ greatly in their style of depicting this
> numeric output. The **-p** option was invented to provide scripts with
> a common means of obtaining this information.
>
> In the KornShell, *time* is a shell reserved word that can be used to
> time an entire pipeline, rather than just a simple command. The POSIX
> definition has been worded to allow this implementation. Consideration
> was given to invalidating this approach because of the historical
> model from the C shell and System V shell. However, since the System V
> *time* utility historically has not produced accurate results in
> pipeline timing (because the constituent processes are not all owned
> by the same parent process, as allowed by POSIX), it did not seem
> worthwhile to break historical KornShell usage.
>
> The term *utility* is used, rather than *command*, to highlight the
> fact that shell compound commands, pipelines, special built-ins, and
> so on, cannot be used directly. However, *utility* includes user
> application programs and shell scripts, not just the standard
> utilities.

#### []{#tag_20_129_19}FUTURE DIRECTIONS {#future-directions .mansect}

> None.

#### []{#tag_20_129_20}SEE ALSO {#see-also .mansect}

> [*Shell Command Language*](../utilities/V3_chap02.html#tag_18),
> [*sh*](../utilities/sh.html#)
>
> XBD [*Environment Variables*](../basedefs/V1_chap08.html#tag_08),
> [*Utility Syntax Guidelines*](../basedefs/V1_chap12.html#tag_12_02)
>
> XSH [*times*](../functions/times.html#tag_16_617)

#### []{#tag_20_129_21}CHANGE HISTORY {#change-history .mansect}

> First released in Issue 2.

#### []{#tag_20_129_22}Issue 6 {#issue-6 .mansect}

> This utility is marked as part of the User Portability Utilities
> option.

#### []{#tag_20_129_23}Issue 7 {#issue-7 .mansect}

> The *time* utility is moved from the User Portability Utilities option
> to the Base. User Portability Utilities is now an option for
> interactive utilities.
>
> SD5-XCU-ERN-115 is applied, updating the example in the DESCRIPTION.
>
> POSIX.1-2008, Technical Corrigendum 1, XCU/TC1-2008/0144 \[266\] is
> applied.
>
> POSIX.1-2008, Technical Corrigendum 2, XCU/TC2-2008/0194 \[723\] is
> applied.

