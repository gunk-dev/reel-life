{ pkgs }:
pkgs.testers.runNixOSTest {
  name = "reel-life-outcome-report";
  nodes.machine = { lib, ... }: {
    imports = [ ../module.nix ];
    users.users.operator.isNormalUser = true;
    users.users.operator.extraGroups = [ "wheel" ];
    users.users.outsider.isNormalUser = true;
    services.reel-life = {
      enable = true;
      package = pkgs.reel-life;
      evidencePath = "/var/lib/reel-life/events.jsonl";
      outcomeReportUsers = [ "operator" ];
    };
    systemd.services.reel-life.serviceConfig = {
      StateDirectory = "reel-life";
      ExecStart = lib.mkForce (pkgs.writeShellScript "seed-evidence" ''
        umask 077
        echo '{"type":"tool.result","operation":"PRIVATE_CANARY","outcome":"failed","attributes":{"secret":"PRIVATE_CANARY"}}' > /var/lib/reel-life/events.jsonl
        exec ${pkgs.coreutils}/bin/sleep infinity
      '');
    };
  };
  testScript = ''
    import json
    machine.wait_for_unit("reel-life.service")
    machine.wait_until_succeeds("test -s /var/lib/reel-life/events.jsonl")
    before = machine.succeed("sha256sum /var/lib/reel-life/events.jsonl")
    machine.fail("su - operator -c 'cat /var/lib/reel-life/events.jsonl'")
    print(machine.succeed("sudo -l -U operator"))
    result = machine.succeed("su - operator -c 'sudo -n /run/current-system/sw/bin/reel-life-outcomes'")
    assert "PRIVATE_CANARY" not in result, result
    report = json.loads(result)
    assert report["tool_failed"] == 1 and report["events"] == 1, report
    machine.fail("su - operator -c 'sudo -n /run/current-system/sw/bin/reel-life-outcomes -events /etc/shadow'")
    machine.fail("su - operator -c 'sudo -n /run/current-system/sw/bin/reel-life-outcomes --help'")
    machine.fail("su - operator -c 'sudo -n -E /run/current-system/sw/bin/reel-life-outcomes'")
    machine.fail("su - operator -c 'sudo -n BASH_ENV=/tmp/untrusted-shell-env /run/current-system/sw/bin/reel-life-outcomes'")
    machine.fail("su - operator -c 'sudo -n cat /var/lib/reel-life/events.jsonl'")
    machine.fail("su - outsider -c 'sudo -n /run/current-system/sw/bin/reel-life-outcomes'")
    assert before == machine.succeed("sha256sum /var/lib/reel-life/events.jsonl")
  '';
}
