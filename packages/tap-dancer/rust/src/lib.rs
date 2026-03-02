use std::io::{self, Write};

pub struct TestResult {
    pub number: usize,
    pub name: String,
    pub ok: bool,
    pub directive: Option<String>,
    pub error_message: Option<String>,
    pub exit_code: Option<i32>,
    pub output: Option<String>,
    pub suppress_yaml: bool,
}

pub struct TapWriter<'a> {
    w: &'a mut dyn Write,
    counter: usize,
    failed: bool,
    plan_emitted: bool,
}

impl<'a> TapWriter<'a> {
    pub fn new(w: &'a mut dyn Write) -> io::Result<Self> {
        writeln!(w, "TAP version 14")?;
        Ok(Self {
            w,
            counter: 0,
            failed: false,
            plan_emitted: false,
        })
    }

    fn child(w: &'a mut dyn Write) -> Self {
        Self {
            w,
            counter: 0,
            failed: false,
            plan_emitted: false,
        }
    }

    pub fn count(&self) -> usize {
        self.counter
    }

    pub fn has_failures(&self) -> bool {
        self.failed
    }

    pub fn ok(&mut self, desc: &str) -> io::Result<usize> {
        self.counter += 1;
        writeln!(self.w, "ok {} - {}", self.counter, desc)?;
        Ok(self.counter)
    }

    pub fn not_ok(&mut self, desc: &str) -> io::Result<usize> {
        self.counter += 1;
        self.failed = true;
        writeln!(self.w, "not ok {} - {}", self.counter, desc)?;
        Ok(self.counter)
    }

    pub fn not_ok_diag(
        &mut self,
        desc: &str,
        diagnostics: &[(&str, &str)],
    ) -> io::Result<usize> {
        self.counter += 1;
        self.failed = true;
        writeln!(self.w, "not ok {} - {}", self.counter, desc)?;
        write_diagnostics_block(self.w, diagnostics)?;
        Ok(self.counter)
    }

    pub fn skip(&mut self, desc: &str, reason: &str) -> io::Result<usize> {
        self.counter += 1;
        writeln!(self.w, "ok {} - {} # SKIP {}", self.counter, desc, reason)?;
        Ok(self.counter)
    }

    pub fn todo(&mut self, desc: &str, reason: &str) -> io::Result<usize> {
        self.counter += 1;
        writeln!(
            self.w,
            "not ok {} - {} # TODO {}",
            self.counter, desc, reason
        )?;
        Ok(self.counter)
    }

    pub fn bail_out(&mut self, reason: &str) -> io::Result<()> {
        writeln!(self.w, "Bail out! {}", reason)
    }

    pub fn comment(&mut self, text: &str) -> io::Result<()> {
        writeln!(self.w, "# {}", text)
    }

    pub fn pragma(&mut self, key: &str, enabled: bool) -> io::Result<()> {
        let sign = if enabled { "+" } else { "-" };
        writeln!(self.w, "pragma {}{}", sign, key)
    }

    pub fn plan(&mut self) -> io::Result<()> {
        if self.plan_emitted {
            return Ok(());
        }
        self.plan_emitted = true;
        writeln!(self.w, "1..{}", self.counter)
    }

    pub fn plan_ahead(&mut self, n: usize) -> io::Result<()> {
        self.plan_emitted = true;
        writeln!(self.w, "1..{}", n)
    }

    pub fn plan_skip(&mut self, reason: &str) -> io::Result<()> {
        self.plan_emitted = true;
        writeln!(self.w, "1..0 # SKIP {}", reason)
    }

    pub fn subtest(
        &mut self,
        name: &str,
        f: impl FnOnce(&mut TapWriter) -> io::Result<()>,
    ) -> io::Result<()> {
        writeln!(self.w, "    # Subtest: {}", name)?;
        let mut indent = IndentWriter { w: &mut *self.w };
        let mut child = TapWriter::child(&mut indent);
        f(&mut child)
    }
}

struct IndentWriter<'a> {
    w: &'a mut dyn Write,
}

impl IndentWriter<'_> {
    fn indent_lines(&mut self, s: &str) -> io::Result<()> {
        let lines: Vec<&str> = s.split('\n').collect();
        for (i, line) in lines.iter().enumerate() {
            if i == lines.len() - 1 && line.is_empty() {
                break;
            }
            let indented = format!("    {}\n", line);
            self.w.write_all(indented.as_bytes())?;
        }
        Ok(())
    }
}

impl Write for IndentWriter<'_> {
    fn write(&mut self, buf: &[u8]) -> io::Result<usize> {
        let s = std::str::from_utf8(buf)
            .map_err(|e| io::Error::new(io::ErrorKind::InvalidData, e))?;
        self.indent_lines(s)?;
        Ok(buf.len())
    }

    fn write_fmt(&mut self, fmt: std::fmt::Arguments<'_>) -> io::Result<()> {
        let s = fmt.to_string();
        self.indent_lines(&s)
    }

    fn flush(&mut self) -> io::Result<()> {
        self.w.flush()
    }
}

fn write_diagnostics_block(w: &mut dyn Write, diagnostics: &[(&str, &str)]) -> io::Result<()> {
    if diagnostics.is_empty() {
        return Ok(());
    }
    writeln!(w, "  ---")?;
    for (key, value) in diagnostics {
        write_yaml_field(w, key, value)?;
    }
    writeln!(w, "  ...")
}

fn strip_ansi(s: &str) -> String {
    let mut result = String::with_capacity(s.len());
    let mut chars = s.chars();
    while let Some(c) = chars.next() {
        if c == '\x1b' {
            if let Some(next) = chars.next() {
                if next == '[' {
                    // Consume CSI sequence: parameters and final byte
                    for c in chars.by_ref() {
                        if c.is_ascii_alphabetic() {
                            break;
                        }
                    }
                }
                // Non-CSI escape sequence: skip the two chars
            }
        } else {
            result.push(c);
        }
    }
    result
}

fn normalize_line_endings(s: &str) -> String {
    s.replace("\r\n", "\n").replace('\r', "\n")
}

fn sanitize_yaml_value(value: &str) -> String {
    let value = normalize_line_endings(value);
    strip_ansi(&value)
}

fn write_yaml_field(w: &mut (impl Write + ?Sized), key: &str, value: &str) -> io::Result<()> {
    let value = sanitize_yaml_value(value);
    if value.contains('\n') {
        writeln!(w, "  {key}: |")?;
        for line in value.lines() {
            writeln!(w, "    {line}")?;
        }
    } else {
        writeln!(w, "  {key}: \"{value}\"")?;
    }
    Ok(())
}

fn has_yaml_block(result: &TestResult) -> bool {
    !result.ok || result.output.is_some()
}

// --- Free functions (original API, unchanged) ---

pub fn write_version(w: &mut impl Write) -> io::Result<()> {
    writeln!(w, "TAP version 14")
}

pub fn write_plan(w: &mut impl Write, count: usize) -> io::Result<()> {
    writeln!(w, "1..{count}")
}

pub fn write_test_point(w: &mut impl Write, result: &TestResult) -> io::Result<()> {
    let status = if result.ok { "ok" } else { "not ok" };
    if let Some(ref directive) = result.directive {
        writeln!(w, "{status} {} - {} # {directive}", result.number, result.name)?;
    } else {
        writeln!(w, "{status} {} - {}", result.number, result.name)?;
    }

    if !result.suppress_yaml && has_yaml_block(result) {
        writeln!(w, "  ---")?;
        if let Some(ref message) = result.error_message {
            write_yaml_field(w, "message", message)?;
        }
        if !result.ok {
            writeln!(w, "  severity: fail")?;
        }
        if let Some(code) = result.exit_code {
            writeln!(w, "  exitcode: {code}")?;
        }
        if let Some(ref output) = result.output {
            write_yaml_field(w, "output", output)?;
        }
        writeln!(w, "  ...")?;
    }

    Ok(())
}

pub fn write_bail_out(w: &mut impl Write, reason: &str) -> io::Result<()> {
    writeln!(w, "Bail out! {reason}")
}

pub fn write_comment(w: &mut impl Write, text: &str) -> io::Result<()> {
    writeln!(w, "# {text}")
}

pub fn write_skip(w: &mut impl Write, num: usize, desc: &str, reason: &str) -> io::Result<()> {
    writeln!(w, "ok {num} - {desc} # SKIP {reason}")
}

pub fn write_todo(w: &mut impl Write, num: usize, desc: &str, reason: &str) -> io::Result<()> {
    writeln!(w, "not ok {num} - {desc} # TODO {reason}")
}

// --- New free functions ---

pub fn write_pragma(w: &mut impl Write, key: &str, enabled: bool) -> io::Result<()> {
    let sign = if enabled { "+" } else { "-" };
    writeln!(w, "pragma {sign}{key}")
}

pub fn write_plan_skip(w: &mut impl Write, reason: &str) -> io::Result<()> {
    writeln!(w, "1..0 # SKIP {reason}")
}

#[cfg(test)]
mod tests {
    use super::*;

    // --- Free function tests (existing, unchanged) ---

    #[test]
    fn version_line() {
        let mut buf = Vec::new();
        write_version(&mut buf).unwrap();
        assert_eq!(String::from_utf8(buf).unwrap(), "TAP version 14\n");
    }

    #[test]
    fn plan_line() {
        let mut buf = Vec::new();
        write_plan(&mut buf, 3).unwrap();
        assert_eq!(String::from_utf8(buf).unwrap(), "1..3\n");
    }

    #[test]
    fn plan_zero() {
        let mut buf = Vec::new();
        write_plan(&mut buf, 0).unwrap();
        assert_eq!(String::from_utf8(buf).unwrap(), "1..0\n");
    }

    #[test]
    fn passing_test_point() {
        let mut buf = Vec::new();
        let result = TestResult {
            number: 1,
            name: "build".into(),
            ok: true,
            directive: None,
            error_message: None,
            exit_code: None,
            output: None,
            suppress_yaml: false,
        };
        write_test_point(&mut buf, &result).unwrap();
        assert_eq!(String::from_utf8(buf).unwrap(), "ok 1 - build\n");
    }

    #[test]
    fn passing_test_point_with_output() {
        let mut buf = Vec::new();
        let result = TestResult {
            number: 1,
            name: "build".into(),
            ok: true,
            directive: None,
            error_message: None,
            exit_code: None,
            output: Some("building\n".into()),
            suppress_yaml: false,
        };
        write_test_point(&mut buf, &result).unwrap();
        let out = String::from_utf8(buf).unwrap();
        assert!(out.contains("ok 1 - build\n"));
        assert!(out.contains("  ---\n"));
        assert!(out.contains("  output: |\n"));
        assert!(out.contains("    building\n"));
        assert!(out.contains("  ...\n"));
    }

    #[test]
    fn failing_test_point() {
        let mut buf = Vec::new();
        let result = TestResult {
            number: 2,
            name: "test".into(),
            ok: false,
            directive: None,
            error_message: Some("something failed".into()),
            exit_code: Some(1),
            output: None,
            suppress_yaml: false,
        };
        write_test_point(&mut buf, &result).unwrap();
        let out = String::from_utf8(buf).unwrap();
        assert!(out.contains("not ok 2 - test\n"));
        assert!(out.contains("  ---\n"));
        assert!(out.contains("  message: \"something failed\"\n"));
        assert!(out.contains("  severity: fail\n"));
        assert!(out.contains("  exitcode: 1\n"));
        assert!(out.contains("  ...\n"));
    }

    #[test]
    fn failing_test_point_with_multiline_output() {
        let mut buf = Vec::new();
        let result = TestResult {
            number: 1,
            name: "multi".into(),
            ok: false,
            directive: None,
            error_message: None,
            exit_code: None,
            output: Some("line one\nline two".into()),
            suppress_yaml: false,
        };
        write_test_point(&mut buf, &result).unwrap();
        let out = String::from_utf8(buf).unwrap();
        assert!(out.contains("  output: |\n"));
        assert!(out.contains("    line one\n"));
        assert!(out.contains("    line two\n"));
    }

    #[test]
    fn bail_out() {
        let mut buf = Vec::new();
        write_bail_out(&mut buf, "database down").unwrap();
        assert_eq!(String::from_utf8(buf).unwrap(), "Bail out! database down\n");
    }

    #[test]
    fn comment() {
        let mut buf = Vec::new();
        write_comment(&mut buf, "a note").unwrap();
        assert_eq!(String::from_utf8(buf).unwrap(), "# a note\n");
    }

    #[test]
    fn skip_directive() {
        let mut buf = Vec::new();
        write_skip(&mut buf, 3, "optional feature", "not supported").unwrap();
        assert_eq!(
            String::from_utf8(buf).unwrap(),
            "ok 3 - optional feature # SKIP not supported\n"
        );
    }

    #[test]
    fn todo_directive() {
        let mut buf = Vec::new();
        write_todo(&mut buf, 4, "future work", "not implemented").unwrap();
        assert_eq!(
            String::from_utf8(buf).unwrap(),
            "not ok 4 - future work # TODO not implemented\n"
        );
    }

    // --- New free function tests ---

    #[test]
    fn pragma_enable() {
        let mut buf = Vec::new();
        write_pragma(&mut buf, "strict", true).unwrap();
        assert_eq!(String::from_utf8(buf).unwrap(), "pragma +strict\n");
    }

    #[test]
    fn pragma_disable() {
        let mut buf = Vec::new();
        write_pragma(&mut buf, "strict", false).unwrap();
        assert_eq!(String::from_utf8(buf).unwrap(), "pragma -strict\n");
    }

    #[test]
    fn plan_skip_free() {
        let mut buf = Vec::new();
        write_plan_skip(&mut buf, "not supported on this platform").unwrap();
        assert_eq!(
            String::from_utf8(buf).unwrap(),
            "1..0 # SKIP not supported on this platform\n"
        );
    }

    // --- TapWriter method tests ---

    #[test]
    fn writer_emits_version() {
        let mut buf = Vec::new();
        let _tw = TapWriter::new(&mut buf).unwrap();
        assert_eq!(String::from_utf8(buf).unwrap(), "TAP version 14\n");
    }

    #[test]
    fn writer_ok() {
        let mut buf = Vec::new();
        let mut tw = TapWriter::new(&mut buf).unwrap();
        let n = tw.ok("first test").unwrap();
        assert_eq!(n, 1);
        let n = tw.ok("second test").unwrap();
        assert_eq!(n, 2);
        let out = String::from_utf8(buf).unwrap();
        assert!(out.contains("ok 1 - first test\n"));
        assert!(out.contains("ok 2 - second test\n"));
    }

    #[test]
    fn writer_not_ok() {
        let mut buf = Vec::new();
        let mut tw = TapWriter::new(&mut buf).unwrap();
        let n = tw.not_ok("broken").unwrap();
        assert_eq!(n, 1);
        assert!(tw.has_failures());
        let out = String::from_utf8(buf).unwrap();
        assert!(out.contains("not ok 1 - broken\n"));
    }

    #[test]
    fn writer_not_ok_diag() {
        let mut buf = Vec::new();
        let mut tw = TapWriter::new(&mut buf).unwrap();
        tw.not_ok_diag("broken", &[("message", "segfault"), ("severity", "fail")])
            .unwrap();
        let out = String::from_utf8(buf).unwrap();
        assert!(out.contains("not ok 1 - broken\n"));
        assert!(out.contains("  ---\n"));
        assert!(out.contains("  message: \"segfault\"\n"));
        assert!(out.contains("  severity: \"fail\"\n"));
        assert!(out.contains("  ...\n"));
    }

    #[test]
    fn writer_not_ok_diag_multiline() {
        let mut buf = Vec::new();
        let mut tw = TapWriter::new(&mut buf).unwrap();
        tw.not_ok_diag("broken", &[("output", "line one\nline two")])
            .unwrap();
        let out = String::from_utf8(buf).unwrap();
        assert!(out.contains("  output: |\n"));
        assert!(out.contains("    line one\n"));
        assert!(out.contains("    line two\n"));
    }

    #[test]
    fn writer_not_ok_diag_empty() {
        let mut buf = Vec::new();
        let mut tw = TapWriter::new(&mut buf).unwrap();
        tw.not_ok_diag("broken", &[]).unwrap();
        let out = String::from_utf8(buf).unwrap();
        assert!(out.contains("not ok 1 - broken\n"));
        assert!(!out.contains("---"));
    }

    #[test]
    fn writer_skip() {
        let mut buf = Vec::new();
        let mut tw = TapWriter::new(&mut buf).unwrap();
        let n = tw.skip("optional", "not supported").unwrap();
        assert_eq!(n, 1);
        assert!(!tw.has_failures());
        let out = String::from_utf8(buf).unwrap();
        assert!(out.contains("ok 1 - optional # SKIP not supported\n"));
    }

    #[test]
    fn writer_todo() {
        let mut buf = Vec::new();
        let mut tw = TapWriter::new(&mut buf).unwrap();
        let n = tw.todo("future", "not done").unwrap();
        assert_eq!(n, 1);
        let out = String::from_utf8(buf).unwrap();
        assert!(out.contains("not ok 1 - future # TODO not done\n"));
    }

    #[test]
    fn writer_bail_out() {
        let mut buf = Vec::new();
        let mut tw = TapWriter::new(&mut buf).unwrap();
        tw.bail_out("on fire").unwrap();
        let out = String::from_utf8(buf).unwrap();
        assert!(out.contains("Bail out! on fire\n"));
    }

    #[test]
    fn writer_comment() {
        let mut buf = Vec::new();
        let mut tw = TapWriter::new(&mut buf).unwrap();
        tw.comment("a note").unwrap();
        let out = String::from_utf8(buf).unwrap();
        assert!(out.contains("# a note\n"));
    }

    #[test]
    fn writer_pragma() {
        let mut buf = Vec::new();
        let mut tw = TapWriter::new(&mut buf).unwrap();
        tw.pragma("strict", true).unwrap();
        tw.pragma("strict", false).unwrap();
        let out = String::from_utf8(buf).unwrap();
        assert!(out.contains("pragma +strict\n"));
        assert!(out.contains("pragma -strict\n"));
    }

    #[test]
    fn writer_trailing_plan() {
        let mut buf = Vec::new();
        let mut tw = TapWriter::new(&mut buf).unwrap();
        tw.ok("one").unwrap();
        tw.ok("two").unwrap();
        tw.plan().unwrap();
        let out = String::from_utf8(buf).unwrap();
        assert!(out.ends_with("1..2\n"));
    }

    #[test]
    fn writer_plan_idempotent() {
        let mut buf = Vec::new();
        let mut tw = TapWriter::new(&mut buf).unwrap();
        tw.ok("one").unwrap();
        tw.plan().unwrap();
        tw.plan().unwrap();
        let out = String::from_utf8(buf).unwrap();
        assert_eq!(out.matches("1..1").count(), 1);
    }

    #[test]
    fn writer_plan_ahead() {
        let mut buf = Vec::new();
        let mut tw = TapWriter::new(&mut buf).unwrap();
        tw.plan_ahead(5).unwrap();
        let out = String::from_utf8(buf).unwrap();
        assert!(out.contains("1..5\n"));
    }

    #[test]
    fn writer_plan_skip() {
        let mut buf = Vec::new();
        let mut tw = TapWriter::new(&mut buf).unwrap();
        tw.plan_skip("missing dependency").unwrap();
        let out = String::from_utf8(buf).unwrap();
        assert!(out.contains("1..0 # SKIP missing dependency\n"));
    }

    #[test]
    fn writer_counter() {
        let mut buf = Vec::new();
        let mut tw = TapWriter::new(&mut buf).unwrap();
        assert_eq!(tw.count(), 0);
        tw.ok("a").unwrap();
        assert_eq!(tw.count(), 1);
        tw.ok("b").unwrap();
        assert_eq!(tw.count(), 2);
    }

    #[test]
    fn writer_has_failures_tracks_not_ok() {
        let mut buf = Vec::new();
        let mut tw = TapWriter::new(&mut buf).unwrap();
        assert!(!tw.has_failures());
        tw.ok("pass").unwrap();
        assert!(!tw.has_failures());
        tw.not_ok("fail").unwrap();
        assert!(tw.has_failures());
    }

    #[test]
    fn writer_subtest() {
        let mut buf = Vec::new();
        let mut tw = TapWriter::new(&mut buf).unwrap();
        tw.subtest("group", |sub| {
            sub.ok("nested one")?;
            sub.ok("nested two")?;
            sub.plan()
        })
        .unwrap();
        tw.ok("group").unwrap();
        tw.plan().unwrap();
        let out = String::from_utf8(buf).unwrap();
        assert!(out.contains("    # Subtest: group\n"));
        assert!(out.contains("    ok 1 - nested one\n"));
        assert!(out.contains("    ok 2 - nested two\n"));
        assert!(out.contains("    1..2\n"));
        assert!(out.contains("ok 1 - group\n"));
        assert!(out.ends_with("1..1\n"));
    }

    #[test]
    fn writer_nested_subtest() {
        let mut buf = Vec::new();
        let mut tw = TapWriter::new(&mut buf).unwrap();
        tw.subtest("outer", |sub| {
            sub.ok("before")?;
            sub.subtest("inner", |inner| {
                inner.ok("deep")?;
                inner.plan()
            })?;
            sub.ok("inner")?;
            sub.plan()
        })
        .unwrap();
        tw.ok("outer").unwrap();
        tw.plan().unwrap();
        let out = String::from_utf8(buf).unwrap();
        assert!(out.contains("    # Subtest: outer\n"));
        assert!(out.contains("    ok 1 - before\n"));
        assert!(out.contains("        # Subtest: inner\n"));
        assert!(out.contains("        ok 1 - deep\n"));
        assert!(out.contains("        1..1\n"));
        assert!(out.contains("    ok 2 - inner\n"));
        assert!(out.contains("    1..2\n"));
    }

    #[test]
    fn writer_subtest_with_skip() {
        let mut buf = Vec::new();
        let mut tw = TapWriter::new(&mut buf).unwrap();
        tw.subtest("optional", |sub| {
            sub.skip("feature x", "not available")?;
            sub.plan()
        })
        .unwrap();
        tw.ok("optional").unwrap();
        let out = String::from_utf8(buf).unwrap();
        assert!(out.contains("    ok 1 - feature x # SKIP not available\n"));
    }

    #[test]
    fn writer_subtest_with_pragma() {
        let mut buf = Vec::new();
        let mut tw = TapWriter::new(&mut buf).unwrap();
        tw.subtest("streaming", |sub| {
            sub.pragma("streamed-output", true)?;
            sub.ok("step one")?;
            sub.plan()
        })
        .unwrap();
        tw.ok("streaming").unwrap();
        let out = String::from_utf8(buf).unwrap();
        assert!(out.contains("    pragma +streamed-output\n"));
    }

    // --- Directive/comment on TestResult ---

    #[test]
    fn test_point_with_directive() {
        let mut buf = Vec::new();
        let result = TestResult {
            number: 1,
            name: "optional feature".into(),
            ok: true,
            directive: Some("SKIP not supported".into()),
            error_message: None,
            exit_code: None,
            output: None,
            suppress_yaml: false,
        };
        write_test_point(&mut buf, &result).unwrap();
        assert_eq!(
            String::from_utf8(buf).unwrap(),
            "ok 1 - optional feature # SKIP not supported\n"
        );
    }

    #[test]
    fn test_point_without_directive() {
        let mut buf = Vec::new();
        let result = TestResult {
            number: 1,
            name: "plain".into(),
            ok: true,
            directive: None,
            error_message: None,
            exit_code: None,
            output: None,
            suppress_yaml: false,
        };
        write_test_point(&mut buf, &result).unwrap();
        assert_eq!(String::from_utf8(buf).unwrap(), "ok 1 - plain\n");
    }

    // --- Carriage return stripping ---

    #[test]
    fn yaml_field_strips_cr_lf() {
        let mut buf = Vec::new();
        write_yaml_field(&mut buf, "output", "line one\r\nline two\r\n").unwrap();
        let out = String::from_utf8(buf).unwrap();
        assert!(!out.contains('\r'));
        assert!(out.contains("  output: |\n"));
        assert!(out.contains("    line one\n"));
        assert!(out.contains("    line two\n"));
    }

    #[test]
    fn yaml_field_strips_bare_cr() {
        let mut buf = Vec::new();
        write_yaml_field(&mut buf, "message", "hello\rworld").unwrap();
        let out = String::from_utf8(buf).unwrap();
        assert!(!out.contains('\r'));
        assert!(out.contains("  message: |\n"));
        assert!(out.contains("    hello\n"));
        assert!(out.contains("    world\n"));
    }

    // --- ANSI escape code stripping ---

    #[test]
    fn yaml_field_strips_ansi_sgr() {
        let mut buf = Vec::new();
        write_yaml_field(&mut buf, "message", "\x1b[31merror\x1b[0m happened").unwrap();
        let out = String::from_utf8(buf).unwrap();
        assert_eq!(out, "  message: \"error happened\"\n");
    }

    #[test]
    fn yaml_field_strips_ansi_csi_non_sgr() {
        let mut buf = Vec::new();
        write_yaml_field(&mut buf, "output", "\x1b[2Jcleared screen").unwrap();
        let out = String::from_utf8(buf).unwrap();
        assert_eq!(out, "  output: \"cleared screen\"\n");
    }

    #[test]
    fn yaml_field_preserves_plain_text() {
        let mut buf = Vec::new();
        write_yaml_field(&mut buf, "message", "no escapes here").unwrap();
        let out = String::from_utf8(buf).unwrap();
        assert_eq!(out, "  message: \"no escapes here\"\n");
    }

    #[test]
    fn strip_ansi_function() {
        assert_eq!(strip_ansi("\x1b[32mok\x1b[0m"), "ok");
        assert_eq!(strip_ansi("\x1b[31mnot ok\x1b[0m"), "not ok");
        assert_eq!(strip_ansi("\x1b[2Jafter clear"), "after clear");
        assert_eq!(strip_ansi("no escapes"), "no escapes");
    }

    // --- Suppress YAML block mode ---

    #[test]
    fn test_point_suppress_yaml() {
        let mut buf = Vec::new();
        let result = TestResult {
            number: 1,
            name: "failing".into(),
            ok: false,
            directive: None,
            error_message: Some("bad stuff".into()),
            exit_code: Some(1),
            output: Some("verbose output".into()),
            suppress_yaml: true,
        };
        write_test_point(&mut buf, &result).unwrap();
        let out = String::from_utf8(buf).unwrap();
        assert_eq!(out, "not ok 1 - failing\n");
    }

    #[test]
    fn test_point_no_suppress_yaml() {
        let mut buf = Vec::new();
        let result = TestResult {
            number: 1,
            name: "failing".into(),
            ok: false,
            directive: None,
            error_message: Some("bad".into()),
            exit_code: None,
            output: None,
            suppress_yaml: false,
        };
        write_test_point(&mut buf, &result).unwrap();
        let out = String::from_utf8(buf).unwrap();
        assert!(out.contains("  ---\n"));
        assert!(out.contains("  message: \"bad\"\n"));
    }

    // --- normalize_line_endings ---

    #[test]
    fn normalize_crlf() {
        assert_eq!(normalize_line_endings("a\r\nb\r\n"), "a\nb\n");
    }

    #[test]
    fn normalize_bare_cr() {
        assert_eq!(normalize_line_endings("a\rb"), "a\nb");
    }

    #[test]
    fn normalize_lf_unchanged() {
        assert_eq!(normalize_line_endings("a\nb"), "a\nb");
    }
}
