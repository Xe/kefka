#### []{#tag_20_133_01}NAME {#name .mansect}

> true - return true value

#### []{#tag_20_133_02}SYNOPSIS {#synopsis .mansect}

> `true`

#### []{#tag_20_133_03}DESCRIPTION {#description .mansect}

> The *true* utility shall return with exit code zero.

#### []{#tag_20_133_04}OPTIONS {#options .mansect}

> None.

#### []{#tag_20_133_05}OPERANDS {#operands .mansect}

> None.

#### []{#tag_20_133_06}STDIN {#stdin .mansect}

> Not used.

#### []{#tag_20_133_07}INPUT FILES {#input-files .mansect}

> None.

#### []{#tag_20_133_08}ENVIRONMENT VARIABLES {#environment-variables .mansect}

> None.

#### []{#tag_20_133_09}ASYNCHRONOUS EVENTS {#asynchronous-events .mansect}

> Default.

#### []{#tag_20_133_10}STDOUT {#stdout .mansect}

> Not used.

#### []{#tag_20_133_11}STDERR {#stderr .mansect}

> Not used.

#### []{#tag_20_133_12}OUTPUT FILES {#output-files .mansect}

> None.

#### []{#tag_20_133_13}EXTENDED DESCRIPTION {#extended-description .mansect}

> None.

#### []{#tag_20_133_14}EXIT STATUS {#exit-status .mansect}

> Zero.

#### []{#tag_20_133_15}CONSEQUENCES OF ERRORS {#consequences-of-errors .mansect}

> None.

------------------------------------------------------------------------

::: box
*The following sections are informative.*
:::

#### []{#tag_20_133_16}APPLICATION USAGE {#application-usage .mansect}

> This utility is typically used in shell scripts, as shown in the
> EXAMPLES section. The special built-in utility **:** is sometimes more
> efficient than *true*.

#### []{#tag_20_133_17}EXAMPLES {#examples .mansect}

> This command is executed forever:
>
>
>     while true
>     do
>         command
>     done

#### []{#tag_20_133_18}RATIONALE {#rationale .mansect}

> The *true* utility has been retained in this volume of POSIX.1-2017,
> even though the shell special built-in **:** provides similar
> functionality, because *true* is widely used in historical scripts and
> is less cryptic to novice script readers.

#### []{#tag_20_133_19}FUTURE DIRECTIONS {#future-directions .mansect}

> None.

#### []{#tag_20_133_20}SEE ALSO {#see-also .mansect}

> [*Shell Commands*](../utilities/V3_chap02.html#tag_18_09),
> [*false*](../utilities/false.html#)

#### []{#tag_20_133_21}CHANGE HISTORY {#change-history .mansect}

> First released in Issue 2.

#### []{#tag_20_133_22}Issue 6 {#issue-6 .mansect}

> IEEE Std 1003.1-2001/Cor 1-2002, item XCU/TC1/D6/39 is applied,
> replacing the terms \`\`None\'\' and \`\`Default\'\' from the STDERR
> and EXIT STATUS sections, respectively, with terms as defined in
> [*Utility Description
> Defaults*](../utilities/V3_chap01.html#tag_17_04).

