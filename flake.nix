{
  description = "LightDM Elephant Greeter - Premium Go-native display manager greeter";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
  };

  outputs = { self, nixpkgs }:
    let
      supportedSystems = [ "x86_64-linux" "aarch64-linux" ];
      forEachSystem = f: nixpkgs.lib.genAttrs supportedSystems (system: f (import nixpkgs {
        inherit system;
        config = { allowUnfree = true; };
      }));
    in
    {
      packages = forEachSystem (pkgs: {
        default = pkgs.callPackage ./default.nix { };
        headless = pkgs.callPackage ./default.nix { buildHeadless = true; };
      });

      devShells = forEachSystem (pkgs: {
        default = pkgs.mkShell {
          name = "lightdm-elephant-greeter-dev";

          nativeBuildInputs = with pkgs; [
            go
            gopls
            pkg-config
            golangci-lint
          ];

          buildInputs = with pkgs; [
            # X11 development libraries
            xorg.libX11
            xorg.libXcursor
            xorg.libXrandr
            xorg.libXinerama
            xorg.libXi
            xorg.libXxf86vm
            
            # Wayland development libraries
            wayland
            wayland-protocols
            libxkbcommon
            
            # Graphics drivers and libraries
            libGL
            vulkan-loader
            vulkan-headers
            
            # System services integration
            dbus
          ];

          # Set up runtime library search path so Vulkan and OpenGL renderers find drivers
          LD_LIBRARY_PATH = pkgs.lib.makeLibraryPath (with pkgs; [
            libGL
            vulkan-loader
            xorg.libX11
            xorg.libXcursor
            xorg.libXrandr
            xorg.libXinerama
            xorg.libXi
            libxkbcommon
            wayland
          ]);

          shellHook = ''
            echo "=================================================="
            echo "   LightDM Elephant Greeter Dev Environment       "
            echo "=================================================="
            echo "  System dependencies loaded successfully."
            echo "  - To build standard UI: go build ./..."
            echo "  - To build headless CLI: go build -tags headless ./..."
            echo "=================================================="
          '';
        };
      });
    };
}
