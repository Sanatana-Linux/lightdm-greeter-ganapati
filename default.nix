{ lib
, buildGoModule
, pkg-config
, libX11
, libXcursor
, libXrandr
, libXinerama
, libXi
, libXxf86vm
, wayland
, wayland-protocols
, libxkbcommon
, libGL
, vulkan-loader
, vulkan-headers
, libxcb
, dbus
, buildHeadless ? false
}:

buildGoModule {
  pname = "lightdm-greeter-ganapati";
  version = "1.0.0";

  src = ./.;

  # vendorHash is used by buildGoModule to lock Go module dependencies.
  # If dependencies change, update this hash (or run with lib.fakeHash to obtain new).
  vendorHash = "sha256-beIm25whY72wrzIyiHd8YzcsRQd+NDYx14aK+ODhjFg=";

  subPackages = [ "cmd/lightdm-greeter-ganapati" ];

  nativeBuildInputs = [
    pkg-config
  ];

  buildInputs = [
    dbus
  ] ++ lib.optionals (!buildHeadless) [
    # Graphics libraries (only needed for GUI build)
    libX11
    libXcursor
    libXrandr
    libXinerama
    libXi
    libXxf86vm
    libxcb
    wayland
    wayland-protocols
    libxkbcommon
    libGL
    vulkan-loader
    vulkan-headers
  ];

  tags = lib.optionals buildHeadless [ "headless" ];

  postInstall = ''
    mkdir -p $out/share/xgreeters
    cat > $out/share/xgreeters/lightdm-greeter-ganapati.desktop <<EOF
[Desktop Entry]
Name=LightDM Greeter Ganapati
Comment=Premium Go-native LightDM Greeter (Remover of Obstacles)
Exec=$out/bin/lightdm-greeter-ganapati
Type=Application
EOF
  '';

  meta = with lib; {
    description = "A premium, minimalist display manager/greeter rewritten in Go";
    homepage = "https://github.com/max-moser/lightdm-greeter-ganapati";
    license = licenses.mit;
    platforms = platforms.linux;
    maintainers = [ ];
  };
}
