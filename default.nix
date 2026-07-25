{ lib
, buildGoModule
, pkg-config
, xorg
, wayland
, wayland-protocols
, libxkbcommon
, libGL
, vulkan-loader
, dbus
, buildHeadless ? false
}:

buildGoModule {
  pname = "lightdm-elephant-greeter";
  version = "1.0.0";

  src = ./.;

  # vendorHash is used by buildGoModule to lock Go module dependencies.
  # If dependencies change, update this hash (or run with lib.fakeHash to obtain new).
  vendorHash = "sha256-6N6U7e47UoN61y7v39VInLg7WpOnVbM+9yS/V1g0mC0=";

  subPackages = [ "cmd/lightdm-elephant-greeter" ];

  nativeBuildInputs = [
    pkg-config
  ];

  buildInputs = [
    dbus
  ] ++ lib.optionals (!buildHeadless) [
    # Graphics libraries (only needed for GUI build)
    xorg.libX11
    xorg.libXcursor
    xorg.libXrandr
    xorg.libXinerama
    xorg.libXi
    xorg.libXxf86vm
    wayland
    wayland-protocols
    libxkbcommon
    libGL
    vulkan-loader
  ];

  tags = lib.optionals buildHeadless [ "headless" ];

  postInstall = ''
    mkdir -p $out/share/xgreeters
    cat > $out/share/xgreeters/lightdm-elephant-greeter.desktop <<EOF
[Desktop Entry]
Name=LightDM Elephant Greeter
Comment=Premium Go-native LightDM Greeter
Exec=$out/bin/lightdm-elephant-greeter
Type=Application
EOF
  '';

  meta = with lib; {
    description = "A premium, minimalist display manager/greeter rewritten in Go";
    homepage = "https://github.com/max-moser/lightdm-elephant-greeter";
    license = licenses.mit;
    platforms = platforms.linux;
    maintainers = [ ];
  };
}
