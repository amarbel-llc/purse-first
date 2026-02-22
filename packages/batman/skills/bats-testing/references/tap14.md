# TAP14 - The Test Anything Protocol v14

> Source: <https://testanything.org/tap-version-14-specification.html>
>
> Copyright 2015-2022 by Isaac Z. Schlueter and Contributors.
> Copyright 2003-2007 by Michael G Schwern, Andy Lester, and Andy Armstrong.
> Released under the Artistic License 2.0.

## Goals

- Document the observed behavior of widely used TAP implementations.
- Add no features that are not already in wide usage across multiple implementations.
- Explicitly allow what is already allowed, deny what is already denied.
- Provide updated and clearer guidance for new TAP implementations.

## Synopsis

The Test Anything Protocol represents a straightforward text-based
communication layer between testing modules and test harnesses. Originally
developed as part of Perl's test infrastructure, TAP now has implementations
across numerous languages including C/C++, Python, PHP, JavaScript, and others.

The specification uses RFC 2119 key words including "must," "must not,"
"required," "shall," "shall not," "should," "should not," "recommended,"
"may," and "optional" with their defined meanings.

## Changes From TAP13 Format

TAP14 maintains backward compatibility with TAP13, allowing TAP14 output to
be parsed by TAP13 harnesses and vice versa. Key modifications include:

- Updated version line to "14"
- Support for child tests as 4-space indented TAP streams with trailing test
  points and leading comment lines
- Formalized conventions for YAML diagnostic indentation (2 spaces), plan
  numbering, character escaping, and description formatting
- Enhanced clarity on parsing rules based on existing implementations

## TAP14 Format

### General Grammar

```
TAPDocument := Version Plan Body | Version Body Plan
Version     := "TAP version 14\n"
Plan        := "1.." (Number) (" # " Reason)? "\n"
Body        := (TestPoint | BailOut | Pragma | Comment | Anything | Empty | Subtest)*
TestPoint   := ("not ")? "ok" (" " Number)? ((" -")? (" " Description) )? (" " Directive)? "\n" (YAMLBlock)?
Directive   := " # " ("todo" | "skip") (" " Reason)?
YAMLBlock   := "  ---\n" (YAMLLine)* "  ...\n"
YAMLLine    := "  " (YAML)* "\n"
BailOut     := "Bail out!" (" " Reason)? "\n"
Reason      := [^\n]+
Pragma      := "pragma " [+-] PragmaKey "\n"
PragmaKey   := ([a-zA-Z0-9_-])+
Subtest     := ("# Subtest" (": " SubtestName)?)? "\n" SubtestDocument TestPoint
Comment     := ^ (" ")* "#" [^\n]* "\n"
Empty       := [\s\t]* "\n"
Anything    := [^\n]+ "\n"
```

### Example Output

```tap
TAP version 14
1..4
ok 1 - Input file opened
not ok 2 - First line of the input valid
  ---
  message: 'First line invalid'
  severity: fail
  data:
    got: 'Flirble'
    expect: 'Fnible'
  ...
ok 3 - Read the rest of the file
not ok 4 - Summarized correctly # TODO Not written yet
  ---
  message: "Can't make summary yet"
  severity: todo
  ...
```

## Harness Behavior

A harness represents any program analyzing TAP output, typically a test
framework runner, programmatic parser, parent test object, or result reporter.

### Reading and Interpretation

Harnesses should read TAP from a test program's standard output rather than
standard error. Line endings should be normalized by replacing `\r\n` or `\r`
instances with `\n`.

### Failure Determination

A harness should treat a test program as failed if:

- TAP output lines indicate test failure
- TAP output is invalid in an unrecoverable manner
- The test program's exit code is non-zero (including fatal signal termination)

When one or more programs fail, the harness should communicate this through
appropriate means -- command-line tools via exit codes, web systems via status
indicators, etc.

## Document Structure

### Encoding

TAP14 producers must encode output using UTF-8. Harnesses should interpret TAP
streams using UTF-8, with optional support for alternative encodings for
backward compatibility.

### Version Line

The initial line must be:

```
TAP version 14
```

Harnesses may interpret TAP13 streams as TAP14 due to compatibility, and may
treat streams lacking a version line as failed tests.

### Plan

The Plan specifies the expected test count and must appear exactly once --
either before all test points or after all of them.

```
Plan := "1.." Number ("" | "# " Reason)
```

A plan of `1..0` indicates complete test skipping. TAP14 producers must output
plans beginning with `1`. Harnesses may allow other starting numbers but must
treat out-of-range test IDs as failures.

Examples:

```tap
1..10
1..0 # WWW::Mechanize not installed
```

### Test Points

Test points form TAP's core, with each containing:

- **Test Status**: `ok` or `not ok` (required, case-sensitive)
- **Test Number**: Sequential integer (recommended)
- **Description**: Text prefixed with `" - "` (recommended)
- **Directive**: Special markers for special conditions (optional)

#### Test Status

Lines beginning with `/^not ok/` represent failures; `/^ok/` represents
successes. This is the only mandatory element.

#### Test Point ID

Test point IDs should follow the status. If omitted, the harness maintains an
internal counter. Both numbered and unnumbered tests are acceptable:

```tap
1..5
not ok
ok
not ok
ok
ok
```

This is equivalent to:

```tap
1..5
not ok 1
ok 2
not ok 3
ok 4
ok 5
```

Test points may appear in any order, but all IDs must fall within the plan
range. Unique IDs within a document are recommended; duplicate IDs may warrant
warnings or failure treatment but must not be treated as invalid TAP.

#### Description

Text after the test number but before any `#` character serves as the
description:

```tap
ok 42 - this is the description of the test
```

Descriptions should be separated by `" - "` to avoid confusion with numeric
IDs. Harnesses must treat descriptions with and without this separator
identically. The leading `" - "` should not be presented to users as part of
the description itself.

#### Directive

Directives are special notes following the first unescaped `#` with
surrounding whitespace. Currently defined directives are `TODO` and `SKIP`.
They are case-insensitive.

Harnesses may support additional platform-specific directives. Unrecognized
directives must not cause test failures and should be included in description
text.

##### Whitespace Around Directive Delimiter

For maximum compatibility:

1. Producers must output directives with at least one space before and after
   the `#`
2. Harnesses must not treat escaped `#` characters as delimiter marks
3. Harnesses may accept non-conformant delimiters but should warn about them

Examples:

```tap
# MUST be treated as SKIP
ok 1 - must be skipped test # SKIP

# MUST NOT be treated as SKIP
ok 2 - must not be skipped test \# SKIP

# MAY be treated as SKIP, but SHOULD warn
ok 3 - may skip, but should warn# skip
ok 4 - may skip, but should warn #skip
ok 5 - may skip, but should warn#skip
```

##### TODO Tests

Beginning with `# TODO`, tests represent features to implement or bugs to fix:

```tap
not ok 14 # TODO bend space and time
```

Successful TODO tests may be reported by the harness as promoted. Harnesses
must not treat failing TODO tests as overall failures. TODO tests should be
reported as items needing work when appropriate.

##### SKIP Tests

Beginning with `# SKIP`, tests represent unrun or temporarily ignored tests:

```tap
ok 14 - mung the gums # SKIP leave gums unmunged for now
```

Harnesses must not treat failing SKIP tests as failures. SKIP tests should be
reported as untested items when appropriate.

### YAML Diagnostics

Test points may be followed by a 2-space indented block between `---` and
`...` markers. This YAML diagnostic provides structured information about the
test point:

```tap
not ok 3 - Resolve address
  ---
  message: "Failed with error 'hostname peebles.example.com not found'"
  severity: fail
  found:
    hostname: 'peebles.example.com'
    address: ~
  wanted:
    hostname: 'peebles.example.com'
    address: '85.193.201.85'
  at:
    file: test/dns-resolve.c
    line: 142
  ...
```

TAP14 harnesses must allow any data structures supported by their YAML parser.

### Comments

Lines starting with `#` (preceded by optional whitespace) outside YAML blocks
are comments. Harnesses may present, ignore, or assign meaning to comments but
must not treat them as test failures.

### Pragmas

Pragmas are boolean switches controlling harness behavior:

```tap
pragma +bail
pragma -strict
```

Structure: `pragma [+-]key` where keys use alphanumeric characters, underscores,
and hyphens.

Pragmas' meanings are implementation-specific. Harnesses may respond or ignore
pragmas but must not treat unrecognized pragma keys as failures.

### Blank Lines

Blank lines (containing only whitespace) outside YAML blocks must be ignored.
Blank lines within YAML blocks must be preserved as they have semantic meaning
in YAML.

### Bail out!

Emergency measure to halt testing immediately:

```tap
Bail out!
Bail out! MySQL is not running.
```

Words are case-insensitive. Any message after the magic words becomes the
stopping reason, which should be presented to users. Characters may be escaped:

```tap
Bail out! \# and \\ are not supported
```

### Anything Else

Invalid TAP lines (not matching version, plan, test point, YAML diagnostic,
pragma, blank, or bail out) may be silently ignored, passed to stderr/stdout,
or reported otherwise. They should not cause test failures by default.

## Escaping

Users may include `#` or `\` characters in test descriptions, plan comments,
bailout reasons, or TODO/SKIP reasons using escaping:

- `\#` represents a literal `#`
- `\\` represents a literal `\`
- No other characters may be escaped

Examples:

```tap
ok 1 - hello \# world
not ok 2 - path: C:\\Users\\name
```

### Parsing Rules

Harnesses must:

1. Treat `\\` as literal `\` but ignore for escaping purposes
2. Treat `\#` as literal `#` (not a delimiter) if the `\` is not itself escaped
3. Treat unescaped `#` as literal in descriptions if doing so wouldn't create
   a delimiter

### Escaping Examples

```tap
TAP version 14

# description: hello
# todo: true
ok 1 - hello # todo

# description: hello # todo
# todo: false
ok 2 - hello \# todo

# description: hello
# todo: true
# todo reason: hash # character
ok 3 - hello # todo hash \# character

# description: hello \
# todo: true
# todo reason: hash # character
ok 5 - hello \\# todo hash \# character

# description: hello # description # todo
# todo: false
ok 7 - hello # description # todo

# multiple escaped \ in a row
# description: hello \\\# todo
# todo: false
ok 8 - hello \\\\\\\# todo

1..8
```

## Subtests

Subtests nest TAP14 streams within parent streams, useful for:

1. Harnesses outputting TAP format themselves
2. Test frameworks providing nested assertion grouping APIs

### Example: Harness-Produced TAP

```tap
TAP version 14
1..2

# Subtest: foo.tap
    1..2
    ok 1
    ok 2 - this passed
ok 1 - foo.tap

# Subtest: bar.tap
    ok 1 - object should be a Bar
    not ok 2 - object.isBar should return true
      ---
      found: false
      wanted: true
      at:
        file: test/bar.ts
        line: 43
        column: 8
      ...
    ok 3 - object can bar bears # TODO
    1..3
not ok 2 - bar.tap
  ---
  fail: 1
  todo: 1
  ...
```

### Design Philosophy

Subtests gracefully degrade for TAP13 harnesses. Since TAP13 ignores indented
non-TAP output, most TAP13 harnesses interpret only the terminating test point,
still reporting overall results correctly despite lacking subtest details.

TAP13 parsers often treat repeated Version declarations as errors, so subtests
should omit Version lines when TAP13 compatibility is desired.

### Bare Subtests

Simplest form: 4-space indented valid TAP lines:

```tap
TAP version 14
    ok 1 - subtest test point
    1..1
ok 1 - subtest passing
1..1
```

Multiple nesting levels use multiples of 4-space indentation:

```tap
TAP version 14
        ok 1 - nested twice
        1..1
    ok 1 - nested parent
    1..1
ok 1 - double nest passing
1..1
```

The first test point at the parent level terminates the subtest and serves as
its correlated test point.

### Commented Subtests

Subtests may be introduced with a parent-level comment defining expected
termination point naming:

```
# Subtest: <name>
```

or

```
# Subtest
```

A commented subtest with a name must be terminated by a matching description.
Without a name, termination must have no description.

Example:

```tap
TAP version 14

ok 1 - in the parent

# Subtest: nested
    1..1
    ok 1 - in the subtest
ok 2 - nested

# Subtest: empty
    1..0
ok 3 - empty

# Subtest
    ok 1 - name is optional
    1..1
ok 4

1..4
```

### Subtest Bailouts

Bailouts in nested subtests must halt the entire test run. For TAP13 backward
compatibility, producers should emit a `Bail out!` line at root indentation
when subtests bail out.

### Subtest Pragmas

Pragmas set in subtests affect only that subtest's parsing. Harnesses must not
allow subtest pragmas to affect parent document parsing.

### Subtest Parsing/Generating Rules

1. Subtest TAP documents are indented 4 spaces
2. Subtests must be valid TAP documents (non-empty):
   - Minimum `1..0` line
   - Subtests nest by adding 4 characters per level
   - YAML diagnostics indent 2 spaces relative to test points (6 spaces in
     nested subtests, 10 in doubly-nested)
3. Subtests should omit Version lines for TAP13 compatibility
4. Subtests terminate with a single parent-level test point (correlated test
   point) reflecting subtest pass/fail status:
   - Producers must communicate intended status via correlated test point
     semantics
   - Harnesses should base overall subtest assessment on correlated test point
     but may treat nested document failures as subtest failures
5. Otherwise-valid TAP indented non-multiples of 4 spaces should be treated as
   non-TAP
6. Producers should omit Version lines in subtest documents
7. Producers should emit Subtest Comments when subtest names are known initially
8. If Subtest Comment provided: Harnesses should continue until matching test
   point (same name or both nameless) at parent level, treating intervening
   un-indented lines as non-TAP, and failing if no match found
9. Without Subtest Comment: Harnesses must treat next parent-level test point
   as subtest end, treating intervening non-matching indented lines as non-TAP
10. Harnesses should treat unterminated subtests as non-TAP
11. Subtest bailouts must abort the entire process
12. Parent-level lines between subtest introduction and valid termination must
    be treated as non-TAP output

## Examples

### Common With Explanation

```tap
TAP version 14
1..6
#
# Create a new Board and Tile, then place
# the Tile onto the board.
#
ok 1 - The object isa Board
ok 2 - Board size is zero
ok 3 - The object isa Tile
ok 4 - Get possible places to put the Tile
ok 5 - Placing the tile produces no error
ok 6 - Board size is 1
```

### Unknown Amount and Failures

```tap
TAP version 14
ok 1 - retrieving servers from the database
# need to ping 6 servers
ok 2 - pinged diamond
ok 3 - pinged ruby
not ok 4 - pinged saphire
  ---
  message: 'hostname "saphire" unknown'
  severity: fail
  ...
ok 5 - pinged onyx
not ok 6 - pinged quartz
  ---
  message: 'timeout'
  severity: fail
  ...
ok 7 - pinged gold
1..7
```

### Giving Up

```tap
TAP version 14
1..573
not ok 1 - database handle
Bail out! Couldn't connect to database.
```

### Skipping a Few

```tap
TAP version 14
1..5
ok 1 - approved operating system
# $^0 is solaris
ok 2 - # SKIP no /sys directory
ok 3 - # SKIP no /sys directory
ok 4 - # SKIP no /sys directory
ok 5 - # SKIP no /sys directory
```

### Skipping Everything

```tap
TAP version 14
1..0 # skip because English-to-French translator isn't installed
```

### TODO Tests

```tap
TAP version 14
1..4
ok 1 - Creating test program
ok 2 - Test program runs, no error
not ok 3 - infinite loop # TODO halting problem unsolved
not ok 4 - infinite loop 2 # TODO halting problem unsolved
```

### Creative Liberties

```tap
TAP version 14
ok - created Board
ok
ok
ok
ok
ok
ok
ok
  ---
  message: "Board layout"
  severity: comment
  dump:
     board:
       - '      16G         05C        '
       - '      G N C       C C G      '
       - '        G           C  +     '
       - '10C   01G         03C        '
       - 'R N G G A G       C C C      '
       - '  R     G           C  +     '
       - '      01G   17C   00C        '
       - '      G A G G N R R N R      '
       - '        G     R     G        '
  ...
ok - board has 7 tiles + starter tile
1..9
```

## Authors

The TAP 14 Specification was authored by Isaac Z. Schlueter with considerable
input from Matt Layman, Leon Timmermans, Bruno P. Kinoshita, and Chad Granum.

The TAP13 Specification was written by Andy Armstrong with contributions from
Pete Krawczyk, Paul Johnson, Ian Langworth, and Nik Clayton, based on original
documentation by Andy Lester and `Test::Harness` documentation by Michael
Schwern.

The TAP format originated in Perl 1's test script (created by Larry Wall) and
was further developed by Tim Bunce and Andreas Koenig through their
`Test::Harness` modifications.
