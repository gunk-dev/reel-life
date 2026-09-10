# reel-life NixOS module
#
# Systemd hardening overview:
#   This module runs reel-life as a sandboxed systemd service with DynamicUser,
#   strict filesystem protection, capability dropping, syscall filtering, and
#   optional network allowlisting.
#
# Testing the sandbox:
#   systemd-analyze security reel-life
#   Expected score: ~2.0 (on a 0–10 scale where lower is more secure).
#
# Network allowlisting:
#   The `restrictNetwork` option enables systemd's IPAddressAllow/IPAddressDeny
#   for localhost (Sonarr) access. Since IPAddressAllow works with IPs, not DNS
#   names, external APIs (api.anthropic.com, chat.googleapis.com) require either:
#     1. Adding their IP ranges to `allowedHosts` (IPs change over time), or
#     2. Using nftables for DNS-aware filtering (recommended for production):
#
#   networking.nftables = {
#     enable = true;
#     tables.reel-life-egress = {
#       family = "inet";
#       content = ''
#         chain output {
#           type filter hook output priority 0; policy drop;
#
#           # Allow loopback traffic
#           oifname "lo" accept;
#
#           # Allow established/related connections
#           ct state { established, related } accept;
#
#           # Allow DNS queries from the service user
#           meta skuid "reel-life" udp dport 53 accept;
#           meta skuid "reel-life" tcp dport 53 accept;
#
#           # Allow access to required services
#           meta skuid "reel-life" tcp dport { 80, 443 } ip daddr {
#             127.0.0.1,           # Sonarr
#             api.anthropic.com,   # Anthropic (resolved at rule load time)
#             chat.googleapis.com  # Google Chat (resolved at rule load time)
#           } accept;
#         }
#       '';
#     };
#   };
#
#   Note: nftables rules can reference DNS names resolved at rule-load time.
#   For dynamic IPs, consider a systemd timer that periodically re-resolves
#   and reloads the nftables ruleset.

{ config, lib, pkgs, ... }:
let
  cfg = config.services.reel-life;
in
{
  options.services.reel-life = {
    enable = lib.mkEnableOption "reel-life media chatops agent";

    package = lib.mkOption {
      type = lib.types.package;
      default = pkgs.reel-life;
      description = "The reel-life package to use";
    };

    sonarrUrl = lib.mkOption {
      type = lib.types.str;
      default = "http://localhost:8989";
      description = "Sonarr base URL";
    };

    chatBackend = lib.mkOption {
      type = lib.types.str;
      default = "googlechat";
      description = "Chat backend to use";
    };

    agentModel = lib.mkOption {
      type = lib.types.str;
      default = "claude-sonnet-4-5-20250929";
      description = "Claude model to use for the agent";
    };

    agentMaxTokens = lib.mkOption {
      type = lib.types.int;
      default = 4096;
      description = "Maximum tokens for agent responses";
    };

    agentHistorySize = lib.mkOption {
      type = lib.types.int;
      default = 40;
      description = "Maximum conversation turns per chat (0 to disable history)";
    };

    agentHistoryPath = lib.mkOption {
      type = lib.types.str;
      default = "";
      description = "Path to persist conversation history (empty = in-memory only)";
    };

    monitorEnabled = lib.mkOption {
      type = lib.types.bool;
      default = true;
      description = "Whether to enable the health monitor";
    };

    monitorInterval = lib.mkOption {
      type = lib.types.str;
      default = "5m";
      description = "Polling interval for the health monitor";
    };

    logLevel = lib.mkOption {
      type = lib.types.str;
      default = "info";
      description = "Log level (debug, info, warn, error)";
    };

    logFormat = lib.mkOption {
      type = lib.types.str;
      default = "text";
      description = "Log format (text, json)";
    };

    listenPort = lib.mkOption {
      type = lib.types.port;
      default = 9090;
      description = "HTTP port for health checks and webhooks";
    };

    environmentFiles = lib.mkOption {
      type = lib.types.listOf lib.types.path;
      default = [ ];
      description = "Files containing environment variables (for secrets like API keys)";
    };

    chatTelegramChatID = lib.mkOption {
      type = lib.types.int;
      default = 0;
      description = "Telegram chat ID for proactive alerts, 0 for auto-capture";
    };

    chatTelegramAdminChatID = lib.mkOption {
      type = lib.types.int;
      default = 0;
      description = "Telegram chat ID for admin/health notifications (0 = use main chat ID)";
    };

    chatTelegramAllowedUsers = lib.mkOption {
      type = lib.types.listOf lib.types.int;
      default = [ ];
      description = "Telegram user IDs allowed to interact with the bot";
    };

    radarrUrl = lib.mkOption {
      type = lib.types.str;
      default = "";
      description = "Radarr base URL (leave empty to disable)";
    };

    prowlarrUrl = lib.mkOption {
      type = lib.types.str;
      default = "";
      description = "Prowlarr base URL (leave empty to disable)";
    };

    overseerrUrl = lib.mkOption {
      type = lib.types.str;
      default = "";
      description = "Overseerr base URL (leave empty to disable)";
    };

    notebookEnabled = lib.mkOption {
      type = lib.types.bool;
      default = false;
      description = "Whether to enable the persistent notebook";
    };

    notebookPath = lib.mkOption {
      type = lib.types.str;
      default = "";
      description = "Path for persistent notebook storage";
    };

    evidencePath = lib.mkOption {
      type = lib.types.str;
      default = "";
      description = "Path for the append-only, redacted product evidence ledger";
    };

    remediationEnabled = lib.mkOption {
      type = lib.types.bool;
      default = false;
      description = "Enable deterministic, allowlisted automatic remediation policies";
    };

    remediationMaxAttempts = lib.mkOption {
      type = lib.types.ints.positive;
      default = 1;
      description = "Maximum automatic attempts per incident";
    };

    remediationCooldown = lib.mkOption {
      type = lib.types.str;
      default = "1h";
      description = "Cooldown between automatic remediation attempts";
    };

    locationName = lib.mkOption {
      type = lib.types.str;
      default = "";
      description = "Display name for the location (e.g. Montreal, QC)";
    };

    locationLatitude = lib.mkOption {
      type = lib.types.float;
      default = 0.0;
      description = "Latitude for weather lookups";
    };

    locationLongitude = lib.mkOption {
      type = lib.types.float;
      default = 0.0;
      description = "Longitude for weather lookups";
    };

    restrictNetwork = lib.mkOption {
      type = lib.types.bool;
      default = false;
      description = ''
        When enabled, restricts network access to allowedHosts only via
        systemd's IPAddressAllow/IPAddressDeny. Note: external API access
        (Anthropic, Google Chat) requires adding their IP ranges to
        allowedHosts, or using nftables for DNS-aware filtering.
      '';
    };

    allowedHosts = lib.mkOption {
      type = lib.types.listOf lib.types.str;
      default = [ "127.0.0.1" "::1" ];
      description = ''
        IP addresses the service is allowed to connect to when
        restrictNetwork is enabled. Sonarr (localhost) is always allowed
        via the defaults. Add resolved IPs for api.anthropic.com and
        chat.googleapis.com as needed.
      '';
    };
  };

  config = lib.mkIf cfg.enable {
    assertions = [
      {
        assertion = !cfg.remediationEnabled || cfg.monitorEnabled;
        message = "services.reel-life.monitorEnabled must be true when remediationEnabled is true";
      }
      {
        assertion = !cfg.remediationEnabled || cfg.evidencePath != "";
        message = "services.reel-life.evidencePath is required when remediationEnabled is true";
      }
    ];

    environment.etc."reel-life/config.yaml".text = builtins.toJSON ({
      sonarr = { base_url = cfg.sonarrUrl; };
      chat = {
        backend = cfg.chatBackend;
      } // lib.optionalAttrs (cfg.chatBackend == "telegram") {
        telegram_chat_id = cfg.chatTelegramChatID;
        telegram_admin_chat_id = cfg.chatTelegramAdminChatID;
        telegram_allowed_users = cfg.chatTelegramAllowedUsers;
      };
      agent = {
        model = cfg.agentModel;
        max_tokens = cfg.agentMaxTokens;
        history_size = cfg.agentHistorySize;
      } // lib.optionalAttrs (cfg.agentHistoryPath != "") {
        history_path = cfg.agentHistoryPath;
      };
      monitor = {
        enabled = cfg.monitorEnabled;
        interval = cfg.monitorInterval;
      };
      remediation = {
        enabled = cfg.remediationEnabled;
        max_attempts = cfg.remediationMaxAttempts;
        cooldown = cfg.remediationCooldown;
      };
      log = {
        level = cfg.logLevel;
        format = cfg.logFormat;
      };
      server = {
        port = cfg.listenPort;
      };
    } // lib.optionalAttrs (cfg.radarrUrl != "") {
      radarr = { base_url = cfg.radarrUrl; };
    } // lib.optionalAttrs (cfg.prowlarrUrl != "") {
      prowlarr = { base_url = cfg.prowlarrUrl; };
    } // lib.optionalAttrs (cfg.overseerrUrl != "") {
      overseerr = { base_url = cfg.overseerrUrl; };
    } // lib.optionalAttrs cfg.notebookEnabled {
      notebook = {
        enabled = true;
      } // lib.optionalAttrs (cfg.notebookPath != "") {
        path = cfg.notebookPath;
      };
    } // lib.optionalAttrs (cfg.evidencePath != "") {
      evidence = { path = cfg.evidencePath; };
    } // lib.optionalAttrs (cfg.locationName != "") {
      location = {
        name = cfg.locationName;
        latitude = cfg.locationLatitude;
        longitude = cfg.locationLongitude;
      };
    });

    systemd.services.reel-life = {
      description = "reel-life media chatops agent";
      after = [ "network-online.target" ];
      wants = [ "network-online.target" ];
      wantedBy = [ "multi-user.target" ];

      serviceConfig = {
        Type = "simple";
        ExecStart = "${cfg.package}/bin/reel-life -config /etc/reel-life/config.yaml";
        Restart = "on-failure";
        RestartSec = 10;

        # Run as an ephemeral system user
        DynamicUser = true;

        # Filesystem: strict read-only root, no home, private /tmp and /dev
        ProtectSystem = "strict";
        ProtectHome = true;
        PrivateTmp = true;
        PrivateDevices = true;
        ReadWritePaths =
          lib.optional (cfg.notebookPath != "") (builtins.dirOf cfg.notebookPath)
          ++ lib.optional (cfg.agentHistoryPath != "") (builtins.dirOf cfg.agentHistoryPath)
          ++ lib.optional (cfg.evidencePath != "") (builtins.dirOf cfg.evidencePath);

        # Kernel: block access to tunables, modules, logs, cgroups, clock, hostname
        ProtectKernelTunables = true;
        ProtectKernelModules = true;
        ProtectKernelLogs = true;
        ProtectControlGroups = true;
        ProtectClock = true;
        ProtectHostname = true;

        # Capabilities: drop everything
        CapabilityBoundingSet = "";
        AmbientCapabilities = "";
        NoNewPrivileges = true;

        # Syscall filtering: allow standard service calls, deny dangerous ones
        SystemCallFilter = [
          "@system-service"
          "~@mount"
          "~@module"
          "~@reboot"
          "~@swap"
          "~@raw-io"
          "~@clock"
          "~@debug"
          "~@obsolete"
        ];
        SystemCallArchitectures = "native";

        # Security: lock down personality, namespaces, realtime, SUID/SGID
        LockPersonality = true;
        RestrictRealtime = true;
        RestrictSUIDSGID = true;
        RestrictNamespaces = true;
        RestrictAddressFamilies = [ "AF_INET" "AF_INET6" "AF_UNIX" ];

        # Resource limits
        MemoryMax = "256M";
        CPUQuota = "50%";
        TasksMax = 32;

        # Secrets via environment files (e.g. agenix paths)
        EnvironmentFile = cfg.environmentFiles;
      } // lib.optionalAttrs cfg.restrictNetwork {
        # Network allowlist: only permit traffic to specified IPs
        IPAddressAllow = cfg.allowedHosts;
        IPAddressDeny = "any";
      };
    };
  };
}
