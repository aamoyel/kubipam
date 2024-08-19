{ pkgs }:
let
  binaries = pkgs.callPackage ./binaries.nix { };
  # trick to override the tag by processing the generated hash + making it dry
  makeDummyImage = {
    fakeRootCommands =
      "\n      ln -s var/run run\n      ln -s bin/${binaries.pname} manager\n    ";
    name = binaries.pname;
    contents = [
      binaries
      (pkgs.dockerTools.fakeNss.override {
        extraPasswdLines = [
          "nixbld:x:${toString 1001}:${
            toString 0
          }:Build user:/home/${binaries.pname}:/noshell"
        ];
        extraGroupLines = [ "nixbld:!:${toString 1001}:" ];
      })
    ];

    config = {
      User = "1001:0";
      Entrypoint = [ "/manager" ];
    };
  };
  imageDummy = pkgs.dockerTools.streamLayeredImage {
    fakeRootCommands = makeDummyImage.fakeRootCommands;
    name = makeDummyImage.name;
    contents = makeDummyImage.contents;
    config = makeDummyImage.config;
  };
in pkgs.dockerTools.streamLayeredImage {
  fakeRootCommands = makeDummyImage.fakeRootCommands;
  tag = binaries.version + "-" + imageDummy.imageTag;
  name = makeDummyImage.name;
  contents = makeDummyImage.contents;
  config = makeDummyImage.config;
}
