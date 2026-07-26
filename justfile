# Justfile - LightDM Elephant Greeter Go Rewrite

# Default target listing all commands
default:
	@just --list

# Run unit and state-machine tests
test:
	go test -v -tags headless ./...

# Build standard graphical binary (requires graphics pkg-config headers)
build:
	mkdir -p build
	go build -v -o build/lightdm-greeter-ganapati ./cmd/lightdm-greeter-ganapati

# Build portable headless CLI/TUI binary (zero cgo requirements)
build-headless:
	mkdir -p build
	go build -tags headless -v -o build/lightdm-greeter-ganapati-headless ./cmd/lightdm-greeter-ganapati

# Build the NixOS QEMU VM for testing display integrations
vm-build:
	nix build --option substituters "https://cache.nixos.org" .#nixosConfigurations.vm.config.system.build.vm

# Run the compiled NixOS VM in QEMU to test the greeter interface live
vm-run:
	./result/bin/run-nixos-greeter-test-vm
