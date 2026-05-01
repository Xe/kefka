#### []{#tag_20_90_01}NAME {#name .mansect}

> patch - apply changes to files

#### []{#tag_20_90_02}SYNOPSIS {#synopsis .mansect}

> `patch`` `**`[`**`-blNR`**`] [`**`-c|-e|-n|-u`**`] [`**`-d`` `*`dir`***`] [`**`-D`` `*`define`***`] [`**`-i`` `*`patchfile`***`]`\**
> ` ``      `` `**`[`**`-o`` `*`outfile`***`] [`**`-p`` `*`num`***`] [`**`-r`` `*`rejectfile`***`] [`***`file`***`]`**

#### []{#tag_20_90_03}DESCRIPTION {#description .mansect}

> The *patch* utility shall read a source (patch) file containing any of
> four forms of difference (diff) listings produced by the
> [*diff*](../utilities/diff.html) utility (normal, copied context,
> unified context, or in the style of [*ed*](../utilities/ed.html)) and
> apply those differences to a file. By default, *patch* shall read from
> the standard input.
>
> The *patch* utility shall attempt to determine the type of the
> [*diff*](../utilities/diff.html) listing, unless overruled by a
> **-c**, **-e**, **-n**, or **-u** option.
>
> If the patch file contains more than one patch, *patch* shall attempt
> to apply each of them as if they came from separate patch files. (In
> this case, the application shall ensure that the name of the patch
> file is determinable for each [*diff*](../utilities/diff.html)
> listing.)

#### []{#tag_20_90_04}OPTIONS {#options .mansect}

> The *patch* utility shall conform to XBD [*Utility Syntax
> Guidelines*](../basedefs/V1_chap12.html#tag_12_02).
>
> The following options shall be supported:
>
> **-b**
> :   Save a copy of the original contents of each modified file, before
>     the differences are applied, in a file of the same name with the
>     suffix **.orig** appended to it. If the file already exists, it
>     shall be overwritten; if multiple patches are applied to the same
>     file, the **.orig** file shall be written only for the first
>     patch. When the **-o** *outfile* option is also specified, *file*
>     **.orig** shall not be created but, if *outfile* already exists,
>     *outfile* **.orig** shall be created.
>
> **-c**
> :   Interpret the patch file as a copied context difference (the
>     output of the utility [*diff*](../utilities/diff.html) when the
>     **-c** or **-C** options are specified).
>
> **-d ** *dir*
> :   Change the current directory to *dir* before processing as
>     described in the EXTENDED DESCRIPTION section.
>
> **-D ** *define*
>
> :   Mark changes with one of the following C preprocessor constructs:
>
>
>         #ifdef define
>         ...
>         #endif
>
>
>         #ifndef define
>         ...
>         #endif
>
>     optionally combined with the C preprocessor construct **#else**.
>     If the patched file is processed with the C preprocessor, where
>     the macro *define* is defined, the output shall contain the
>     changes from the patch file; otherwise, the output shall not
>     contain the patches specified in the patch file.
>
> **-e**
> :   Interpret the patch file as an [*ed*](../utilities/ed.html)
>     script, rather than a [*diff*](../utilities/diff.html) script.
>
> **-i ** *patchfile*
> :   Read the patch information from the file named by the pathname
>     *patchfile*, rather than the standard input.
>
> **-l**
> :   (The letter ell.) Cause any sequence of \<blank\> characters in
>     the difference script to match any sequence of \<blank\>
>     characters in the input file. Other characters shall be matched
>     exactly.
>
> **-n**
> :   Interpret the script as a normal difference.
>
> **-N**
> :   Ignore patches where the differences have already been applied to
>     the file; by default, already-applied patches shall be rejected.
>
> **-o ** *outfile*
> :   Instead of modifying the files (specified by the *file* operand or
>     the difference listings) directly, write a copy of the file
>     referenced by each patch, with the appropriate differences
>     applied, to *outfile*. Multiple patches for a single file shall be
>     applied to the intermediate versions of the file created by any
>     previous patches, and shall result in multiple, concatenated
>     versions of the file being written to *outfile*.
>
> **-p ** *num*
> :   For all pathnames in the patch file that indicate the names of
>     files to be patched, delete *num* pathname components from the
>     beginning of each pathname. If the pathname in the patch file is
>     absolute, any leading \<slash\> characters shall be considered the
>     first component (that is, **-p 1** shall remove the leading
>     \<slash\> characters). Specifying **-p 0** shall cause the full
>     pathname to be used. If **-p** is not specified, only the basename
>     (the final pathname component) shall be used.
>
> **-R**
> :   Reverse the sense of the patch script; that is, assume that the
>     difference script was created from the new version to the old
>     version. The **-R** option cannot be used with
>     [*ed*](../utilities/ed.html) scripts. The *patch* utility shall
>     attempt to reverse each portion of the script before applying it.
>     Rejected differences shall be saved in swapped format. If this
>     option is not specified, and until a portion of the patch file is
>     successfully applied, *patch* attempts to apply each portion in
>     its reversed sense as well as in its normal sense. If the attempt
>     is successful, the user shall be prompted to determine whether the
>     **-R** option should be set.
>
> **-r ** *rejectfile*
> :   Override the default reject filename. In the default case, the
>     reject file shall have the same name as the output file, with the
>     suffix **.rej** appended to it; see [Patch
>     Application](#tag_20_90_13_03).
>
> **-u**
> :   Interpret the patch file as a unified context difference (the
>     output of the [*diff*](../utilities/diff.html) utility when the
>     **-u** or **-U** options are specified).

#### []{#tag_20_90_05}OPERANDS {#operands .mansect}

> The following operand shall be supported:
>
> *file*
> :   A pathname of a file to patch.

#### []{#tag_20_90_06}STDIN {#stdin .mansect}

> See the INPUT FILES section.

#### []{#tag_20_90_07}INPUT FILES {#input-files .mansect}

> Input files shall be text files.

#### []{#tag_20_90_08}ENVIRONMENT VARIABLES {#environment-variables .mansect}

> The following environment variables shall affect the execution of
> *patch*:
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
> *LC_COLLATE*
> :   Determine the locale for the behavior of ranges, equivalence
>     classes, and multi-character collating elements used in the
>     extended regular expression defined for the **yesexpr** locale
>     keyword in the *LC_MESSAGES* category.
>
> *LC_CTYPE*
> :   Determine the locale for the interpretation of sequences of bytes
>     of text data as characters (for example, single-byte as opposed to
>     multi-byte characters in arguments and input files), and the
>     behavior of character classes used in the extended regular
>     expression defined for the **yesexpr** locale keyword in the
>     *LC_MESSAGES* category.
>
> *LC_MESSAGES*
> :   Determine the locale used to process affirmative responses, and
>     the locale used to affect the format and contents of diagnostic
>     messages and prompts written to standard error.
>
> *NLSPATH*
> :   ^\[[XSI](javascript:open_code('XSI'))\]^ ![\[Option
>     Start\]](../images/opt-start.gif){border="0"} Determine the
>     location of message catalogs for the processing of *LC_MESSAGES.*
>     ![\[Option End\]](../images/opt-end.gif){border="0"}
>
> *LC_TIME*
> :   Determine the locale for recognizing the format of file timestamps
>     written by the [*diff*](../utilities/diff.html) utility in a
>     context-difference input file.

#### []{#tag_20_90_09}ASYNCHRONOUS EVENTS {#asynchronous-events .mansect}

> Default.

#### []{#tag_20_90_10}STDOUT {#stdout .mansect}

> Not used.

#### []{#tag_20_90_11}STDERR {#stderr .mansect}

> The standard error shall be used for diagnostic and informational
> messages.

#### []{#tag_20_90_12}OUTPUT FILES {#output-files .mansect}

> The output of the *patch* utility, the save files (**.orig**
> suffixes), and the reject files (**.rej** suffixes) shall be text
> files.

#### []{#tag_20_90_13}EXTENDED DESCRIPTION {#extended-description .mansect}

> A patch file may contain patching instructions for more than one file;
> filenames shall be determined as specified in [Filename
> Determination](#tag_20_90_13_02). When the **-b** option is specified,
> for each patched file, the original shall be saved in a file of the
> same name with the suffix **.orig** appended to it.
>
> For each patched file, a reject file may also be created as noted in
> [Patch Application](#tag_20_90_13_03). In the absence of a **-r**
> option, the name of this file shall be formed by appending the suffix
> **.rej** to the original filename.
>
> ##### []{#tag_20_90_13_01}Patch File Format
>
> The patch file shall contain zero or more lines of header information
> followed by one or more patches. Each patch shall contain zero or more
> lines of filename identification in the format produced by the **-c**,
> **-C**, **-u**, or **-U** options of the
> [*diff*](../utilities/diff.html) utility, and one or more sets of
> [*diff*](../utilities/diff.html) output, which are customarily called
> *hunks*.
>
> The *patch* utility shall recognize the following expression in the
> header information:
>
> **Index: ** *pathname*
> :   The file to be patched is named *pathname*.
>
> If all lines (including headers) within a patch begin with the same
> leading sequence of \<blank\> characters, the *patch* utility shall
> remove this sequence before proceeding. Within each patch, if the type
> of difference is common context, the *patch* utility shall recognize
> the following expressions:
>
> \*\*\* *filename timestamp*
> :   The patches arose from *filename*.
>
> \-\-- *filename timestamp*
> :   The patches should be applied to *filename*.
>
> If the type of difference is unified context, the *patch* utility
> shall recognize the following expressions:
>
> \-\-- *filename timestamp*
> :   The patches arose from *filename*.
>
> +++ *filename timestamp*
> :   The patches should be applied to *filename*.
>
> Each hunk within a patch shall be the [*diff*](../utilities/diff.html)
> output to change a line range within the original file. The line
> numbers for successive hunks within a patch shall occur in ascending
> order.
>
> ##### []{#tag_20_90_13_02}Filename Determination
>
> If no *file* operand is specified, *patch* shall perform the following
> steps to determine the filename to use:
>
> 1.  If the type of [*diff*](../utilities/diff.html) is context, the
>     *patch* utility shall delete pathname components (as specified by
>     the **-p** option) from the filename on the line beginning with
>     `"***"` (if copied context) or `"---"` (if unified context), then
>     test for the existence of this file relative to the current
>     directory (or the directory specified with the **-d** option). If
>     the file exists, the *patch* utility shall use this filename.
>
> 2.  If the type of [*diff*](../utilities/diff.html) is context, the
>     *patch* utility shall delete the pathname components (as specified
>     by the **-p** option) from the filename on the line beginning with
>     `"---"` (if copied context) or `"+++"` (if unified context), then
>     test for the existence of this file relative to the current
>     directory (or the directory specified with the **-d** option). If
>     the file exists, the *patch* utility shall use this filename.
>
> 3.  If the header information contains a line beginning with the
>     string **Index:**, the *patch* utility shall delete pathname
>     components (as specified by the **-p** option) from this line,
>     then test for the existence of this file relative to the current
>     directory (or the directory specified with the **-d** option). If
>     the file exists, the *patch* utility shall use this filename.
>
> 4.  ^\[[XSI](javascript:open_code('XSI'))\]^ ![\[Option
>     Start\]](../images/opt-start.gif){border="0"} If an **SCCS**
>     directory exists in the current directory, *patch* shall attempt
>     to perform a [*get*](../utilities/get.html) **-e** **SCCS/s.**
>     *filename* command to retrieve an editable version of the file. If
>     the file exists, the *patch* utility shall use this filename.
>     ![\[Option End\]](../images/opt-end.gif){border="0"}
>
> 5.  The *patch* utility shall write a prompt to standard output and
>     request a filename interactively from the controlling terminal
>     (for example, **/dev/tty**).
>
> ##### []{#tag_20_90_13_03}Patch Application
>
> If the **-c**, **-e**, **-n**, or **-u** option is present, the
> *patch* utility shall interpret information within each hunk as a
> copied context difference, an [*ed*](../utilities/ed.html) difference,
> a normal difference, or a unified context difference, respectively. In
> the absence of any of these options, the *patch* utility shall
> determine the type of difference based on the format of information
> within the hunk.
>
> For each hunk, the *patch* utility shall begin to search for the place
> to apply the patch at the line number at the beginning of the hunk,
> plus or minus any offset used in applying the previous hunk. If lines
> matching the hunk context are not found, *patch* shall scan both
> forwards and backwards at least 1000 bytes for a set of lines that
> match the hunk context.
>
> If no such place is found and it is a context difference, then another
> scan shall take place, ignoring the first and last line of context. If
> that fails, the first two and last two lines of context shall be
> ignored and another scan shall be made. Implementations may search
> more extensively for installation locations.
>
> If no location can be found, the *patch* utility shall append the hunk
> to the reject file. A rejected hunk that is a copied context
> difference, an [*ed*](../utilities/ed.html) difference, or a normal
> difference shall be written in copied-context-difference format
> regardless of the format of the patch file. It is
> implementation-defined whether a rejected hunk that is a unified
> context difference is written in copied-context-difference format or
> in unified-context-difference format. If the input was a normal or
> [*ed*](../utilities/ed.html)-style difference, the reject file may
> contain differences with zero lines of context. The line numbers on
> the hunks in the reject file may be different from the line numbers in
> the patch file since they shall reflect the approximate locations for
> the failed hunks in the new file rather than the old one.
>
> If the type of patch is an [*ed*](../utilities/ed.html) diff, the
> implementation may accomplish the patching by invoking the
> [*ed*](../utilities/ed.html) utility.

#### []{#tag_20_90_14}EXIT STATUS {#exit-status .mansect}

> The following exit values shall be returned:
>
>  0
> :   Successful completion.
>
>  1
> :   One or more lines were written to a reject file.
>
> \>1
> :   An error occurred.

#### []{#tag_20_90_15}CONSEQUENCES OF ERRORS {#consequences-of-errors .mansect}

> Patches that cannot be correctly placed in the file shall be written
> to a reject file.

------------------------------------------------------------------------

::: box
*The following sections are informative.*
:::

#### []{#tag_20_90_16}APPLICATION USAGE {#application-usage .mansect}

> The **-R** option does not work with [*ed*](../utilities/ed.html)
> scripts because there is too little information to reconstruct the
> reverse operation.
>
> The **-p** option makes it possible to customize a patch file to local
> user directory structures without manually editing the patch file. For
> example, if the filename in the patch file was:
>
>
>     /curds/whey/src/blurfl/blurfl.c
>
> Setting **-p 0** gives the entire pathname unmodified; **-p 1** gives:
>
>
>     curds/whey/src/blurfl/blurfl.c
>
> without the leading \<slash\>, **-p 4** gives:
>
>
>     blurfl/blurfl.c
>
> and not specifying **-p** at all gives:
>
>
>     blurfl.c .

#### []{#tag_20_90_17}EXAMPLES {#examples .mansect}

> None.

#### []{#tag_20_90_18}RATIONALE {#rationale .mansect}

> Some of the functionality in historical *patch* implementations was
> not specified. The following documents those features present in
> historical implementations that have not been specified.
>
> A deleted piece of functionality was the `'+'` pseudo-option allowing
> an additional set of options and a patch file operand to be given.
> This was seen as being insufficiently useful to standardize.
>
> In historical implementations, if the string `"Prereq:"` appeared in
> the header, the *patch* utility would search for the corresponding
> version information (the string specified in the header, delimited by
> \<blank\> characters or the beginning or end of a line or the file)
> anywhere in the original file. This was deleted as too simplistic and
> insufficiently trustworthy a mechanism to standardize. For example,
> if:
>
>
>     Prereq: 1.2
>
> were in the header, the presence of a delimited 1.2 anywhere in the
> file would satisfy the prerequisite.
>
> The following options were dropped from historical implementations of
> *patch* as insufficiently useful to standardize:
>
> **-b**
> :   The **-b** option historically provided a method for changing the
>     name extension of the backup file from the default **.orig**. This
>     option has been modified and retained in this volume of
>     POSIX.1-2017.
>
> **-F**
> :   The **-F** option specified the number of lines of a context diff
>     to ignore when searching for a place to install a patch.
>
> **-f**
> :   The **-f** option historically caused *patch* not to request
>     additional information from the user.
>
> **-r**
> :   The **-r** option historically provided a method of overriding the
>     extension of the reject file from the default **.rej**.
>
> **-s**
> :   The **-s** option historically caused *patch* to work silently
>     unless an error occurred.
>
> **-x**
> :   The **-x** option historically set internal debugging flags.
>
> In some file system implementations, the saving of a **.orig** file
> may produce unwanted results. In the case of 12, 13, or 14-character
> filenames (on file systems supporting 14-character maximum filenames),
> the **.orig** file overwrites the new file. The reject file may also
> exceed this filename limit. It was suggested, due to some historical
> practice, that a \<tilde\> ( `'˜'` ) suffix be used instead of
> **.orig** and some other character instead of the **.rej** suffix.
> This was rejected because it is not obvious to the user which file is
> which. The suffixes **.orig** and **.rej** are clearer and more
> understandable.
>
> The **-b** option has the opposite sense in some historical
> implementations-do not save the **.orig** file. The default case here
> is not to save the files, making *patch* behave more consistently with
> the other standard utilities.
>
> The **-w** option in early proposals was changed to **-l** to match
> historical practice.
>
> The **-N** option was included because without it, a non-interactive
> application cannot reject previously applied patches. For example, if
> a user is piping the output of [*diff*](../utilities/diff.html) into
> the *patch* utility, and the user only wants to patch a file to a
> newer version non-interactively, the **-N** option is required.
>
> Changes to the **-l** option description were proposed to allow
> matching across \<newline\> characters in addition to just \<blank\>
> characters. Since this is not historical practice, and since some
> ambiguities could result, it is suggested that future developments in
> this area utilize another option letter, such as **-L**.
>
> The **-u** option of GNU *patch* has been added, along with support
> for unified context formats.

#### []{#tag_20_90_19}FUTURE DIRECTIONS {#future-directions .mansect}

> None.

#### []{#tag_20_90_20}SEE ALSO {#see-also .mansect}

> [*diff*](../utilities/diff.html#), [*ed*](../utilities/ed.html#)
>
> XBD [*Environment Variables*](../basedefs/V1_chap08.html#tag_08),
> [*Utility Syntax Guidelines*](../basedefs/V1_chap12.html#tag_12_02)

#### []{#tag_20_90_21}CHANGE HISTORY {#change-history .mansect}

> First released in Issue 4.

#### []{#tag_20_90_22}Issue 5 {#issue-5 .mansect}

> The FUTURE DIRECTIONS section is added.

#### []{#tag_20_90_23}Issue 6 {#issue-6 .mansect}

> This utility is marked as part of the User Portability Utilities
> option.
>
> The description of the **-D** option and the steps in [Filename
> Determination](#tag_20_90_13_02) are changed to match historical
> practice as defined in the IEEE P1003.2b draft standard.
>
> The normative text is reworded to avoid use of the term \"must\" for
> application requirements.
>
> IEEE Std 1003.1-2001/Cor 1-2002, item XCU/TC1/D6/34 is applied,
> clarifying the way that the *patch* utility performs **ifdef**
> selection for the **-D** option.

#### []{#tag_20_90_24}Issue 7 {#issue-7 .mansect}

> The *patch* utility is moved from the User Portability Utilities
> option to the Base. User Portability Utilities is now an option for
> interactive utilities.
>
> SD5-XCU-ERN-97 is applied, updating the SYNOPSIS.
>
> SD5-XCU-ERN-103 and SD5-XCU-ERN-120 are applied, adding the **-u**
> option.
>
> Austin Group Interpretation 1003.1-2001 #126 is applied, changing the
> description of the *LC_MESSAGES* and *LC_CTYPE* environment variables
> and adding the *LC_COLLATE* environment variable.

