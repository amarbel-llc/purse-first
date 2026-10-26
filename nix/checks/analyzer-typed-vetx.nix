# Every dewey analyzer binary is a singlechecker over x/tools >= v0.50, whose
# unitchecker reads each import's types from its vetx file. godyn only speaks
# that type-bearing protocol to a vetTool declaring passthru.typedVetx; without
# it a consumer's verify-go-analyzer fails "no package vetx file for bufio"
# (igloo#95). Asserts the declaration at eval time; building the check is cheap.
{
  pkgs,
  analyzers,
}:
let
  missing = builtins.attrNames (
    pkgs.lib.filterAttrs (_: drv: !(drv.passthru.typedVetx or false)) analyzers
  );
in
pkgs.runCommand "analyzer-typed-vetx"
  {
    inherit missing;
  }
  (
    if missing == [ ] then
      "touch $out"
    else
      ''
        echo "analyzers missing passthru.typedVetx = true: ${builtins.concatStringsSep " " missing}" >&2
        exit 1
      ''
  )
