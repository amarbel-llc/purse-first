#! /usr/bin/env bats

setup() {
  load "$(dirname "$BATS_TEST_FILE")/common.bash"
  export output
}

function simple_echo_preserves_output { # @test
  run sandcastle echo hello
  assert_success
  assert_output "hello"
}

function multi_word_echo_preserves_all_words { # @test
  run sandcastle echo hello world
  assert_success
  assert_output "hello world"
}

function bash_c_preserves_quoted_command { # @test
  run sandcastle bash -c 'echo hello_from_bash_c'
  assert_success
  assert_output "hello_from_bash_c"
}

function bash_c_preserves_stdout_and_stderr { # @test
  run sandcastle bash -c 'echo on_stdout; echo on_stderr >&2'
  assert_success
  assert_line "on_stdout"
  assert_line "on_stderr"
}

function pipe_inside_bash_c_produces_output { # @test
  run sandcastle bash -c 'echo piped | cat'
  assert_success
  assert_output "piped"
}

function multiline_output_preserved { # @test
  run sandcastle bash -c 'printf "line1\nline2\nline3\n"'
  assert_success
  assert_line --index 0 "line1"
  assert_line --index 1 "line2"
  assert_line --index 2 "line3"
}

function seq_produces_all_lines { # @test
  run sandcastle seq 1 5
  assert_success
  assert_output - <<-EOM
	1
	2
	3
	4
	5
	EOM
}

function shell_flag_preserves_output { # @test
  run sandcastle --shell bash echo hello_shell
  assert_success
  assert_output "hello_shell"
}

function exit_code_propagated_on_failure { # @test
  run sandcastle bash -c 'exit 42'
  assert_failure 42
}

function special_characters_in_arguments { # @test
  run sandcastle echo 'hello   world'
  assert_success
  assert_output "hello   world"
}
