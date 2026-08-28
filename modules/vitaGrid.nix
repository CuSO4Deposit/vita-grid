{
  lib,
  config,
  pkgs,
  ...
}:

let
  cfg = config.services.vitaGrid;
in
{
  options.services.vitaGrid = {
    package = lib.mkOption {
      type = lib.types.package;
      default = pkgs.vita-grid;
    };

    statusd = {
      enable = lib.mkEnableOption "vita-grid statusd";
      configFile = lib.mkOption { type = lib.types.path; };
    };

    aggregator = {
      enable = lib.mkEnableOption "vita-grid aggregator";
      configFile = lib.mkOption { type = lib.types.path; };
      proxy = lib.mkOption {
        type = lib.types.nullOr lib.types.str;
        default = null;
        description = "HTTP proxy URL for the aggregator (needed when outbound internet requires a proxy).";
      };
    };
  };

  config = lib.mkMerge [
    (lib.mkIf cfg.statusd.enable {
      systemd.services.vita-grid-statusd = {
        description = "vita-grid status daemon";
        wantedBy = [ "multi-user.target" ];
        after = [ "network.target" ];
        serviceConfig = {
          ExecStart = "${cfg.package}/bin/statusd -config ${cfg.statusd.configFile}";
          Restart = "on-failure";
        };
      };
    })

    (lib.mkIf cfg.aggregator.enable {
      systemd.services.vita-grid-aggregator = {
        description = "vita-grid aggregator";
        wantedBy = [ "multi-user.target" ];
        after = [ "network.target" ];
        environment = lib.optionalAttrs (cfg.aggregator.proxy != null) {
          http_proxy = cfg.aggregator.proxy;
          https_proxy = cfg.aggregator.proxy;
          HTTP_PROXY = cfg.aggregator.proxy;
          HTTPS_PROXY = cfg.aggregator.proxy;
        };
        serviceConfig = {
          ExecStart = "${cfg.package}/bin/aggregator -config ${cfg.aggregator.configFile}";
          Restart = "on-failure";
        };
      };
    })
  ];
}
