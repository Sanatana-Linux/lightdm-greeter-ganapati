{
  description = "LightDM Elephant Greeter - Premium Go-native display manager greeter";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    sharabha-gtk.url = "github:Sanatana-Linux/sharabha-gtk-theme";
  };

  outputs = { self, nixpkgs, sharabha-gtk }:
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

      nixosModules = {
        default = { config, lib, pkgs, ... }:
          let
            cfg = config.services.xserver.displayManager.lightdm.greeters.ganapati;
          in
          {
            options.services.xserver.displayManager.lightdm.greeters.ganapati = {
              enable = lib.mkEnableOption "LightDM Greeter Ganapati";
              
              wallpaper = lib.mkOption {
                type = lib.types.nullOr lib.types.path;
                default =
                  config.stylix.image
                  or config.services.xserver.displayManager.lightdm.background
                  or null;
                defaultText = lib.literalMD "config.stylix.image, else services.xserver.displayManager.lightdm.background (if Stylix enabled)";
                description = "Background wallpaper image (automatically inherits from Stylix, or the LightDM background, if set).";
              };

              themeName = lib.mkOption {
                type = lib.types.str;
                default = config.gtk.theme.name or "Adwaita-dark";
                defaultText = lib.literalMD "config.gtk.theme.name (if Stylix/GTK enabled)";
                description = "GTK theme name to apply (automatically inherits from Stylix).";
              };

              darkTheme = lib.mkOption {
                type = lib.types.bool;
                default = if config ? stylix && config.stylix ? polarity then config.stylix.polarity == "dark" else true;
                defaultText = lib.literalMD "config.stylix.polarity == 'dark' (if Stylix enabled)";
                description = "Whether to prefer dark theme mode.";
              };
            };

            config = lib.mkIf cfg.enable {
              services.xserver.displayManager.lightdm = {
                enable = true;

                # Select Ganapati as the active greeter. This overrides the
                # nixpkgs default (lightdm-gtk-greeter, enabled by default via
                # greeters.gtk.enable = true) — setting `greeter` directly beats
                # the gtk module's mkDefault, so lightdm.conf's greeter-session
                # resolves to us instead of the gtk-greeter.
                greeter = {
                  name = "lightdm-greeter-ganapati";
                  package = self.packages.${pkgs.system}.default.xgreeters;
                };
                greeters.gtk.enable = lib.mkForce false;
              };

              environment.systemPackages = [
                self.packages.${pkgs.system}.default
              ];

              environment.etc."lightdm/lightdm-greeter-ganapati.conf".text = ''
                [GTK]
                gtk-theme-name=${cfg.themeName}
                gtk-application-prefer-dark-theme=${if cfg.darkTheme then "true" else "false"}

                [Greeter]
                ${lib.optionalString (cfg.wallpaper != null) "background=${cfg.wallpaper}"}
              '';
            };
          };
      };

      nixosConfigurations = {
        vm = nixpkgs.lib.nixosSystem {
          system = "x86_64-linux";
          modules = [
            ({ pkgs, ... }: {
              imports = [
                "${nixpkgs}/nixos/modules/virtualisation/qemu-vm.nix"
                self.nixosModules.default
              ];
              
              system.stateVersion = "24.11";
              networking.hostName = "nixos-greeter-test";
              
              # Enable XServer and LightDM with our Greeter Ganapati package.
              # The module sets greeter.name/package, which overrides the
              # nixpkgs default gtk-greeter; no extraConfig greeter-session is
              # needed (and it would lose to the template's own line anyway).
              services.xserver = {
                enable = true;
                videoDrivers = [ "modesetting" ]; # Explicit virtualized graphics driver
                desktopManager.xterm.enable = true;
                displayManager.lightdm.enable = true;
              };

              # Disable QEMU VM default root autologin so LightDM can claim tty1 for the graphical greeter
              services.getty.autologinUser = pkgs.lib.mkForce null;
              
              # Increase LightDM boot timeout
              systemd.services.display-manager.serviceConfig.TimeoutStartSec = 120;



              # Install our greeter package on the system so LightDM finds the .desktop file
              environment.systemPackages = [
                self.packages.x86_64-linux.default
                # Install the host's GTK theme so the greeter can resolve and apply it
                sharabha-gtk.packages.${pkgs.stdenv.hostPlatform.system}.default
              ];

              # Match the host system's GTK theme and wallpaper so the greeter
              # renders with the same styling as the real desktop.
              services.xserver.displayManager.lightdm.greeters.ganapati = {
                enable = true;
                themeName = "sharabha-gtk-theme";
                wallpaper = ./.assets/wallpaper.png;
              };

              # Test user accounts
              users.users.testuser = {
                isNormalUser = true;
                password = "testpassword";
                description = "Standard Test User";
                extraGroups = [ "wheel" ];
              };

              # QEMU graphics options with Spice server for virt-manager/virt-viewer
              virtualisation.vmVariant = {
                virtualisation.graphics = true;
                virtualisation.qemu.options = [
                  "-display gtk,gl=on"
                  "-spice port=5900,addr=127.0.0.1,disable-ticketing=on"
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
