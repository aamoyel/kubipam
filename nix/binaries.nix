{ pkgs, lib, }:
let inherit (lib) cleanSource cleanSourceWith;
in pkgs.buildGoModule {
  pname = "kubeipam";
  version = "0.1.2";

  src = cleanSourceWith {
    filter = name: _:
      !((baseNameOf name) == "Dockerfile" || (baseNameOf name) == "Makefile"
        || (baseNameOf name) == "README.md" || (baseNameOf name) == "PROJECT"
        || (baseNameOf name) == "config" || (baseNameOf name) == "conf"
        || (baseNameOf name) == "nix");
    src = cleanSource ../.;
  };

  CGO_ENABLED = 0;

  subPackages = [ "cmd" ];

  vendorHash = "sha256-UuybaBHvd4t2eVI9QFqGKSmi4gBagYbC5WR4W0YOcrU=";

  postInstall = "mv $out/bin/cmd $out/bin/$pname";

  doCheck = true;

  meta = with lib; {
    description = "$pname; version: $version";
    homepage = "http://github.com/aamoyel/$pname";
    license = licenses.asl20;
    platforms = platforms.linux;
    mainProgram = "$pname";
  };
}
