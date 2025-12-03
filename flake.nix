{
  description = "Flowctl SDK - Go SDK for building flowctl components";

  inputs = {
    nixpkgs.url     = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs { inherit system; };
      in {
        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            go                    # Go 1.25.4
            protobuf              # protoc (for proto integration)
            protoc-gen-go
            protoc-gen-go-grpc
            git
            gnumake
            golangci-lint         # linting
            gotools               # godoc, goimports, etc.
          ];

          shellHook = ''
            # Set custom prompt
            export PS1="\[\033[1;32m\][nix:flowctl-sdk]\[\033[0m\] \[\033[1;34m\]\w\[\033[0m\] \[\033[1;36m\]\$\[\033[0m\] "

            echo "Flowctl SDK development environment ready!"
            echo "Go      : $(go version)"
            echo "protoc  : $(protoc --version)"
            echo ""
            echo "Available commands:"
            echo "  go build ./...           - Build all packages"
            echo "  go test ./...            - Run tests"
            echo "  golangci-lint run        - Run linters"
            echo "  go mod tidy              - Update dependencies"
          '';
        };

        # Build packages for examples
        packages = {
          contract-events-processor = pkgs.buildGoModule {
            pname = "contract-events-processor";
            version = "1.0.0";
            src = ./.;
            subPackages = [ "examples/contract-events-processor" ];
            vendorHash = "sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="; # Will be updated by nix
          };

          postgresql-consumer = pkgs.buildGoModule {
            pname = "postgresql-consumer";
            version = "1.0.0";
            src = ./.;
            subPackages = [ "examples/postgresql-consumer" ];
            vendorHash = "sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="; # Will be updated by nix
          };

          default = pkgs.buildGoModule {
            pname = "flowctl-sdk";
            version = "0.1.0";
            src = ./.;
            vendorHash = null;  # SDK is a library, no main package
            buildPhase = ''
              echo "flowctl-sdk is a library package - no binaries to build"
            '';
            installPhase = ''
              mkdir -p $out
              echo "flowctl-sdk is a library package - imported via go get" > $out/README
            '';
          };
        };
      });
}
