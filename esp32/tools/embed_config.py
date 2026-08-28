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
if "ssid" not in wifi or "pass" not in wifi:
    raise SystemExit("config.json: wifi.ssid / wifi.pass required")

poll = cfg.get("poll", {})
if "url" not in poll:
    raise SystemExit("config.json: poll.url required")

led = cfg.get("led", {})
pin = int(led.get("pin", 4))
count = int(led.get("count", 64))
order = led.get("order", "GRB")
if order not in ("RGB", "GRB", "RBG", "BRG", "GBR", "RGBW"):
    raise SystemExit(f"config.json: bad led.order {order}")

mapping = cfg.get("mapping", [])
if not mapping:
    raise SystemExit("config.json: mapping must not be empty")
seen = set()
for m in mapping:
    name = m.get("name", "")
    idx = int(m.get("index", -1))
    if not name or not 0 <= idx < count:
        raise SystemExit(f"config.json: bad mapping entry {m}")
    if idx in seen:
        raise SystemExit(f"config.json: duplicate mapping index {idx}")
    seen.add(idx)

header = (
    "// generated from config.json — do not edit\n"
    "#pragma once\n"
    f"#define LED_PIN {pin}\n"
    f"#define LED_COUNT {count}\n"
    f"#define LED_ORDER {order}\n"
    f'static const char CONFIG_JSON[] PROGMEM = R"raw({json.dumps(cfg, ensure_ascii=False, separators=(",", ":"))})raw";\n'
)
with open(out_path, "w", encoding="utf-8") as f:
    f.write(header)
print(f"wrote {out_path}: pin={pin}, count={count}, mapping={len(mapping)}")
