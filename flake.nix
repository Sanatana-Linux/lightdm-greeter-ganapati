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
          name = "lightdm-greeter-ganapati-dev";

          nativeBuildInputs = with pkgs; [
            go
            gopls
            pkg-config
            golangci-lint
          ];

          buildInputs = with pkgs; [
            # X11 development libraries
            libX11
            libXcursor
            libXrandr
            libXinerama
            libXi
            libXxf86vm
            libxcb
            
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
            libX11
            libXcursor
            libXrandr
            libXinerama
            libXi
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

      apps = forEachSystem (pkgs: {
        vm = {
          type = "app";
          program = "${self.nixosConfigurations.vm.config.system.build.vm}/bin/run-nixos-vm";
        };
      });

      nixosConfigurations = {
        vm = nixpkgs.lib.nixosSystem {
          system = "x86_64-linux";
          modules = [
            ({ pkgs, ... }: {
              imports = [
                "${nixpkgs}/nixos/modules/virtualisation/qemu-vm.nix"
              ];
              
              system.stateVersion = "24.11";
              networking.hostName = "nixos-greeter-test";
              
              # Enable XServer and LightDM with our Elephant Greeter Go rewrite
              services.xserver = {
                enable = true;
                videoDrivers = [ "modesetting" ]; # Explicit virtualized graphics driver
                desktopManager.xterm.enable = true;
                displayManager.lightdm = {
                  enable = true;
                  greeter = {
                    name = "lightdm-greeter-ganapati";
                    package = self.packages.x86_64-linux.default;
                  };
                };
              };
              
              # Increase LightDM boot timeout
              systemd.services.display-manager.serviceConfig.TimeoutStartSec = 120;

              # Install our greeter package on the system so LightDM finds the .desktop file
              environment.systemPackages = [
                self.packages.x86_64-linux.default
              ];

              # Test user accounts
              users.users.testuser = {
                isNormalUser = true;
                password = "testpassword";
                description = "Standard Test User";
                extraGroups = [ "wheel" ];
              };

              # QEMU graphics options with graphical window and serial log file
              virtualisation.vmVariant = {
                virtualisation.graphics = true;
                virtualisation.qemu.options = [
                  "-display gtk,gl=on"
                  "-serial file:qemu-serial.log"
                ];
                virtualisation.resolution = { x = 1024; y = 768; };
              };
            })
          ];
        };
      };
    };
}
