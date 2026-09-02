import json
import os

if "Import" in globals():
    Import("env")
    project_dir = env["PROJECT_DIR"]
else:
    project_dir = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))

config_path = os.path.join(project_dir, "config.json")
out_path = os.path.join(project_dir, "src", "config_data.h")

with open(config_path, encoding="utf-8") as f:
    cfg = json.load(f)

wifi = cfg.get("wifi", {})
ssid = wifi.get("ssid", "")
passwd = wifi.get("pass", "")
if not ssid or not passwd:
    raise SystemExit("config.json: wifi.ssid / wifi.pass required")

agg = cfg.get("aggregator", {})
base_url = agg.get("url", "")
board = agg.get("board", "")
if not base_url or not board:
    raise SystemExit("config.json: aggregator.url / aggregator.board required")

led = cfg.get("led", {})
order = led.get("order", "GRB")
if order not in ("RGB", "GRB", "RBG", "BRG", "GBR", "RGBW"):
    raise SystemExit(f"config.json: bad led.order {order}")

header = (
    "// generated from config.json — do not edit\n"
    "#pragma once\n"
    "#define MAX_LEDS 64\n"
    f"#define LED_ORDER {order}\n"
    f'#define WIFI_SSID "{ssid}"\n'
    f'#define WIFI_PASS "{passwd}"\n'
    f'#define AGGREGATOR_URL "{base_url}"\n'
    f'#define BOARD_ID "{board}"\n'
)
with open(out_path, "w", encoding="utf-8") as f:
    f.write(header)
print(f"wrote {out_path}: board={board}, base={base_url}, order={order}")
