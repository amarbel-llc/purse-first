# Every dewey analyzer binary is a singlechecker over x/tools >= v0.50, whose
# unitchecker reads each import's types from its vetx file. godyn only speaks
# that type-bearing protocol to a vetTool declaring passthru.typedVetx; without
# it a consumer's verify-go-analyzer fails "no package vetx file for bufio"
# (igloo#95). Asserts the declaration at eval time; building the check is cheap.
#
# A typedVetx tool must also be built with a Go at least as new as the registry
# Go (igloo's goToolchain.go) the consumer's godyn lane uses, else the lane
# fails "package requires newer Go version go1.27 (application built with
# go1.26)" (igloo#96). Each analyzer exposes the Go it was built with as
# passthru.go.
{
  pkgs,
  analyzers,
  registryGo,
}:
let
  missing = builtins.attrNames (
    pkgs.lib.filterAttrs (_: drv: !(drv.passthru.typedVetx or false)) analyzers
  );
  tooOld = builtins.attrNames (
    pkgs.lib.filterAttrs (
      _: drv: !(pkgs.lib.versionAtLeast (drv.passthru.go.version or "0") registryGo.version)
    ) analyzers
  );
in
pkgs.runCommand "analyzer-typed-vetx"
  {
    inherit missing tooOld;
  }
  (
    if missing == [ ] && tooOld == [ ] then
      "touch $out"
    else
      ''
        echo "analyzers missing passthru.typedVetx = true: ${builtins.concatStringsSep " " missing}" >&2
        echo "analyzers built with Go older than registry Go ${registryGo.version}: ${builtins.concatStringsSep " " tooOld}" >&2
        exit 1
      ''
  )
