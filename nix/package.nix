{ lib, buildGoModule }:

buildGoModule {
  pname = "reel-life";
  version = "0.1.0";
  src = ./..;

  vendorHash = "sha256-AAgs5s5IDr79qlfymZTOPMmclJGdVi0hfrEIPdEEEEI=";

  subPackages = [ "cmd/reel-life" "cmd/reel-life-report" ];

  meta = {
    description = "AI-powered chatops agent for media curation";
    mainProgram = "reel-life";
  };
}
