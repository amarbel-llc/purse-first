{
  description = "Claude Plugin Marketplace: MCP servers and tool routing for Claude Code";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/23d72dabcb3b12469f57b37170fcbc1789bd7457";
    nixpkgs-master.url = "github:NixOS/nixpkgs/b28c4999ed71543e71552ccfd0d7e68c581ba7e9";
    utils.url = "https://flakehub.com/f/numtide/flake-utils/0.1.102";

    go.url = "github:friedenberg/eng?dir=devenvs/go";
    shell.url = "github:friedenberg/eng?dir=devenvs/shell";

    grit = {
      url = "github:amarbel-llc/grit";
      inputs.nixpkgs.follows = "nixpkgs";
      inputs.nixpkgs-master.follows = "nixpkgs-master";
    };
    get-hubbed = {
      url = "github:amarbel-llc/get-hubbed";
      inputs.nixpkgs.follows = "nixpkgs";
      inputs.nixpkgs-master.follows = "nixpkgs-master";
    };
    lux = {
      url = "github:friedenberg/lux";
      inputs.nixpkgs.follows = "nixpkgs";
      inputs.nixpkgs-master.follows = "nixpkgs-master";
    };
    nix-mcp-server = {
      url = "github:friedenberg/nix-mcp-server";
      inputs.nixpkgs.follows = "nixpkgs";
      inputs.nixpkgs-master.follows = "nixpkgs-master";
    };
  };

  outputs =
    {
      self,
      nixpkgs,
      nixpkgs-master,
      utils,
      go,
      shell,
      grit,
      get-hubbed,
      lux,
      nix-mcp-server,
    }:
    utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = import nixpkgs {
          inherit system;
          overlays = [
            go.overlays.default
          ];
        };

        version = "0.1.0";

        # Upstream packages
        grit-pkg = grit.packages.${system}.default;
        lux-pkg = lux.packages.${system}.default;
        nix-mcp-server-pkg = nix-mcp-server.packages.${system}.default;

        # get-hubbed wrapped with gh on PATH
        get-hubbed-pkg =
          pkgs.runCommand "get-hubbed"
            {
              nativeBuildInputs = [ pkgs.makeWrapper ];
            }
            ''
              mkdir -p $out/bin
              makeWrapper ${get-hubbed.packages.${system}.default}/bin/get-hubbed $out/bin/get-hubbed \
                --prefix PATH : ${pkgs.lib.makeBinPath [ pkgs.gh ]}
            '';

        # purse-first with static build flags
        purse-first-pkg = pkgs.buildGoApplication {
          pname = "purse-first";
          inherit version;
          src = ./.;
          modules = ./gomod2nix.toml;
          subPackages = [ "cmd/purse-first" ];
          CGO_ENABLED = "0";
          ldflags = [
            "-s"
            "-w"
          ];

          meta = with pkgs.lib; {
            description = "MCP-first tool routing for Claude Code";
            homepage = "https://github.com/friedenberg/purse-first";
            license = licenses.mit;
          };
        };

        # Aggregated packages
        mcp-all = pkgs.symlinkJoin {
          name = "mcp-all";
          paths = [
            grit-pkg
            get-hubbed-pkg
            lux-pkg
            nix-mcp-server-pkg
          ];
        };

        marketplace = pkgs.symlinkJoin {
          name = "claude-plugin-marketplace";
          paths = [
            grit-pkg
            get-hubbed-pkg
            lux-pkg
            nix-mcp-server-pkg
            purse-first-pkg
          ];
        };
      in
      {
        packages = {
          default = marketplace;
          inherit mcp-all;
          grit = grit-pkg;
          get-hubbed = get-hubbed-pkg;
          lux = lux-pkg;
          nix-mcp-server = nix-mcp-server-pkg;
          purse-first = purse-first-pkg;
        };

        devShells.default = pkgs.mkShell {
          packages = with pkgs; [
            just
          ];

          inputsFrom = [
            go.devShells.${system}.default
            shell.devShells.${system}.default
          ];

          shellHook = ''
            echo "purse-first - dev environment"
          '';
        };

        apps.default = {
          type = "app";
          program = "${purse-first-pkg}/bin/purse-first";
        };

        apps.install = {
          type = "app";
          program = toString (
            pkgs.writeShellScript "install-marketplace" ''
              set -euo pipefail

              CLAUDE_CONFIG_DIR="''${HOME}/.claude"
              MCP_CONFIG_FILE="''${CLAUDE_CONFIG_DIR}/mcp.json"

              log() {
                ${pkgs.gum}/bin/gum style --foreground 212 "$1"
              }

              log_success() {
                ${pkgs.gum}/bin/gum style --foreground 82 "$1"
              }

              log_error() {
                ${pkgs.gum}/bin/gum style --foreground 196 "$1"
              }

              if [[ ! -d "$CLAUDE_CONFIG_DIR" ]]; then
                log "Creating $CLAUDE_CONFIG_DIR..."
                mkdir -p "$CLAUDE_CONFIG_DIR"
              fi

              LUX_PORT="''${LUX_PORT:-19419}"

              # Build MCP server entries using absolute nix store paths
              NEW_SERVERS=$(${pkgs.jq}/bin/jq -n \
                --arg grit_cmd "${grit-pkg}/bin/grit" \
                --arg get_hubbed_cmd "${get-hubbed-pkg}/bin/get-hubbed" \
                --arg lux_port "$LUX_PORT" \
                --arg nix_mcp_cmd "${nix-mcp-server-pkg}/bin/nix-mcp-server" \
                '{
                  grit: {type: "stdio", command: $grit_cmd, args: []},
                  "get-hubbed": {type: "stdio", command: $get_hubbed_cmd, args: []},
                  lux: {type: "sse", url: ("http://localhost:" + $lux_port + "/sse")},
                  nix: {type: "stdio", command: $nix_mcp_cmd, args: []}
                }')

              if [[ -f "$MCP_CONFIG_FILE" ]]; then
                log "Found existing MCP config at $MCP_CONFIG_FILE"

                # Check for any existing marketplace servers
                EXISTING=$(${pkgs.jq}/bin/jq -r '[.mcpServers // {} | keys[] | select(. == "grit" or . == "get-hubbed" or . == "lux" or . == "nix")] | length' "$MCP_CONFIG_FILE")

                if [[ "$EXISTING" -gt 0 ]]; then
                  if ${pkgs.gum}/bin/gum confirm "Found $EXISTING existing marketplace server(s). Overwrite?"; then
                    UPDATED=$(${pkgs.jq}/bin/jq --argjson servers "$NEW_SERVERS" '.mcpServers = (.mcpServers // {} | . * $servers)' "$MCP_CONFIG_FILE")
                    echo "$UPDATED" > "$MCP_CONFIG_FILE"
                    log_success "Updated marketplace MCP server configurations"
                  else
                    log "Skipping MCP config update"
                  fi
                else
                  UPDATED=$(${pkgs.jq}/bin/jq --argjson servers "$NEW_SERVERS" '.mcpServers = (.mcpServers // {} | . * $servers)' "$MCP_CONFIG_FILE")
                  echo "$UPDATED" > "$MCP_CONFIG_FILE"
                  log_success "Added marketplace MCP servers to existing configuration"
                fi
              else
                log "Creating new MCP config at $MCP_CONFIG_FILE"
                ${pkgs.jq}/bin/jq -n --argjson servers "$NEW_SERVERS" '{mcpServers: $servers}' > "$MCP_CONFIG_FILE"
                log_success "Created MCP configuration"
              fi

              log ""

              # Install purse-first hooks
              log "Installing purse-first hooks..."
              ${purse-first-pkg}/bin/purse-first install
              log_success "Installed purse-first hooks (PreToolUse, PostToolUse, Stop)"

              log ""

              # Start lux SSE server
              LUX_STATE_DIR="''${HOME}/.local/state/purse-first"
              mkdir -p "$LUX_STATE_DIR"
              LUX_PID_FILE="''${LUX_STATE_DIR}/lux.pid"

              if [[ -f "$LUX_PID_FILE" ]] && kill -0 "$(cat "$LUX_PID_FILE")" 2>/dev/null; then
                log "Lux SSE server already running (PID $(cat "$LUX_PID_FILE"))"
              else
                log "Starting lux SSE server on port $LUX_PORT..."
                ${lux-pkg}/bin/lux mcp sse --addr ":$LUX_PORT" &
                echo $! > "$LUX_PID_FILE"
                log_success "Started lux SSE server (PID $!, port $LUX_PORT)"
              fi

              log ""
              log "Installation complete! All MCP servers and purse-first hooks are configured."
              log "Configuration written to: $MCP_CONFIG_FILE"
              log ""
              log "To verify, run: cat $MCP_CONFIG_FILE"
            ''
          );
        };

        apps.start-lux = {
          type = "app";
          program = toString (
            pkgs.writeShellScript "start-lux" ''
              set -euo pipefail

              LUX_PORT="''${LUX_PORT:-19419}"
              LUX_STATE_DIR="''${HOME}/.local/state/purse-first"
              mkdir -p "$LUX_STATE_DIR"
              LUX_PID_FILE="''${LUX_STATE_DIR}/lux.pid"

              if [[ -f "$LUX_PID_FILE" ]] && kill -0 "$(cat "$LUX_PID_FILE")" 2>/dev/null; then
                echo "Lux SSE server already running (PID $(cat "$LUX_PID_FILE"))"
                exit 0
              fi

              ${lux-pkg}/bin/lux mcp sse --addr ":$LUX_PORT" &
              echo $! > "$LUX_PID_FILE"
              echo "Started lux SSE server (PID $!, port $LUX_PORT)"
            ''
          );
        };

        apps.stop-lux = {
          type = "app";
          program = toString (
            pkgs.writeShellScript "stop-lux" ''
              set -euo pipefail

              LUX_STATE_DIR="''${HOME}/.local/state/purse-first"
              LUX_PID_FILE="''${LUX_STATE_DIR}/lux.pid"

              if [[ -f "$LUX_PID_FILE" ]]; then
                PID=$(cat "$LUX_PID_FILE")
                if kill -0 "$PID" 2>/dev/null; then
                  kill "$PID"
                  echo "Stopped lux SSE server (PID $PID)"
                else
                  echo "Lux SSE server not running (stale PID file)"
                fi
                rm -f "$LUX_PID_FILE"
              else
                echo "No lux PID file found"
              fi
            ''
          );
        };
      }
    );
}
