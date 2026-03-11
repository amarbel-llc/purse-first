{
  description = "My Claude Plugin Marketplace";

  inputs = {
    purse-first.url = "github:amarbel-llc/purse-first";

    # Inherit nixpkgs from purse-first for consistency.
    nixpkgs.follows = "purse-first/nixpkgs";
    nixpkgs-master.follows = "purse-first/nixpkgs-master";
    utils.follows = "purse-first/utils";

    # --- Add your plugin inputs below ---
    # Each plugin should follow nixpkgs for consistent builds:
    #
    # my-plugin = {
    #   url = "github:my-org/my-plugin";
    #   inputs.nixpkgs.follows = "nixpkgs";
    #   inputs.nixpkgs-master.follows = "nixpkgs-master";
    # };
  };

  outputs =
    {
      purse-first,
      nixpkgs,
      nixpkgs-master,
      utils,
      ...
    }@inputs:
    purse-first.lib.mkMarketplace {
      inherit nixpkgs nixpkgs-master utils;

      # Provide the purse-first CLI from the flake input.
      purse-first-cli = purse-first;

      # --- Configure your marketplace ---
      name = "my-marketplace";
      owner = {
        name = "my-org";
        email = "team@my-org.com";
      };

      # List your plugins per system.
      plugins = system: [
        # inputs.my-plugin.packages.${system}.default
      ];

      # Optional: bundle skills (Markdown + YAML frontmatter).
      # skills = ./skills;

      # Optional: per-plugin metadata for the marketplace.
      # pluginConfig = builtins.fromJSON (builtins.readFile ./marketplace-config.json);
    };
}
