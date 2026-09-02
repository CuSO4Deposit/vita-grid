#include <Arduino.h>
#include <ArduinoJson.h>
#include <BH1750.h>
#include <FastLED.h>
#include <HTTPClient.h>
#include <Preferences.h>
#include <WiFi.h>
#include <Wire.h>

#include "config_data.h"

namespace {

CRGB leds[MAX_LEDS];

struct LedMap {
  String name;
  uint8_t index;
};

LedMap mapping[MAX_LEDS];
uint8_t mappingCount = 0;

uint16_t ledCount = MAX_LEDS;
bool serpentine = false;

bool bhEnabled = false;
uint8_t bhSda = 21;
uint8_t bhScl = 22;
float bhOffLux = 20;
uint8_t brightLevel = 50;
uint32_t pollIntervalSec = 30;
uint32_t blinkMs = 500;

CRGB colorOk = CRGB::Green;
CRGB colorWarn = CRGB::Yellow;
CRGB colorError = CRGB::Red;
CRGB colorAbsent = CRGB::Red;

bool configApplied = false;

Preferences prefs;

String statusUrl;
String configUrl;

unsigned long lastPollMs = 0;
unsigned long lastBrightMs = 0;
unsigned long lastConfigMs = 0;

constexpr uint32_t kConfigRefreshMs = 10UL * 60UL * 1000UL;
constexpr char kPrefsNs[] = "vtgrid";
constexpr char kPrefsKey[] = "config";

CRGB colorFromArray(JsonArray arr, CRGB def) {
  if (arr.size() >= 3) {
    return CRGB(arr[0].as<uint8_t>(), arr[1].as<uint8_t>(), arr[2].as<uint8_t>());
  }
  return def;
}

bool applyConfig(JsonDocument& doc) {
  mappingCount = 0;

  ledCount = doc["led"]["count"] | MAX_LEDS;
  if (ledCount < 1 || ledCount > MAX_LEDS) {
    ledCount = MAX_LEDS;
  }
  serpentine = doc["led"]["serpentine"] | false;

  bhEnabled = doc["bh1750"]["enabled"] | false;
  bhSda = doc["bh1750"]["sda"] | 21;
  bhScl = doc["bh1750"]["scl"] | 22;
  bhOffLux = doc["bh1750"]["offLux"] | 20.0f;

  brightLevel = doc["brightness"]["level"] | 50;
  FastLED.setBrightness(brightLevel);

  JsonObject colors = doc["colors"];
  colorOk = colorFromArray(colors["ok"].as<JsonArray>(), CRGB::Green);
  colorWarn = colorFromArray(colors["warn"].as<JsonArray>(), CRGB::Yellow);
  colorError = colorFromArray(colors["error"].as<JsonArray>(), CRGB::Red);
  colorAbsent = colorFromArray(colors["absent"].as<JsonArray>(), CRGB::Red);

  blinkMs = doc["blink"]["intervalMs"] | 500;
  pollIntervalSec = doc["poll"]["interval"] | 30;

  for (JsonObject m : doc["mapping"].as<JsonArray>()) {
    if (mappingCount >= MAX_LEDS) break;
    String name = m["name"] | "";
    int idx = m["index"] | -1;
    if (name.isEmpty() || idx < 0 || idx >= (int)ledCount) {
      Serial.printf("config: dropping mapping entry %s@%d\n", name.c_str(), idx);
      continue;
    }
    mapping[mappingCount].name = name;
    mapping[mappingCount].index = idx;
    mappingCount++;
  }

  configApplied = true;
  return true;
}

bool saveConfigToNVS(JsonDocument& doc) {
  String json;
  serializeJson(doc, json);
  return prefs.putString(kPrefsKey, json);
}

bool loadConfigFromNVS() {
  String json = prefs.getString(kPrefsKey, "");
  if (json.isEmpty()) {
    return false;
  }
  JsonDocument doc;
  DeserializationError err = deserializeJson(doc, json);
  if (err) {
    return false;
  }
  return applyConfig(doc);
}

bool fetchConfig() {
  HTTPClient http;
  http.setTimeout(10000);
  if (!http.begin(configUrl)) {
    Serial.println("config: http.begin failed");
    return false;
  }
  int code = http.GET();
  if (code != HTTP_CODE_OK) {
    Serial.printf("config: http %d\n", code);
    http.end();
    return false;
  }
  String body = http.getString();
  http.end();

  JsonDocument doc;
  DeserializationError err = deserializeJson(doc, body);
  if (err) {
    Serial.printf("config: parse error %s\n", err.c_str());
    return false;
  }
  if (!applyConfig(doc)) {
    return false;
  }
  saveConfigToNVS(doc);
  return true;
}

CRGB colorForState(const char* state, bool busy, unsigned long now) {
  CRGB c;
  if (strcmp(state, "error") == 0) {
    c = colorError;
  } else if (strcmp(state, "warn") == 0) {
    c = colorWarn;
  } else {
    c = colorOk;
  }
  if (busy && (now / blinkMs) % 2 == 0) {
    c = CRGB::Black;
  }
  return c;
}

uint16_t physicalIndex(uint16_t logical) {
  if (!serpentine) return logical;
  constexpr uint16_t cols = 8;
  uint16_t row = logical / cols;
  uint16_t col = logical % cols;
  if (row % 2 == 1) col = cols - 1 - col;
  return row * cols + col;
}

void setMappedLeds(JsonArray arr, unsigned long now) {
  FastLED.clear();
  for (uint8_t i = 0; i < mappingCount; i++) {
    CRGB c = colorAbsent;
    for (JsonObject sig : arr) {
      const char* name = sig["name"];
      if (name && mapping[i].name == name) {
        const char* state = sig["state"] | "warn";
        bool busy = sig["busy"] | false;
        c = colorForState(state, busy, now);
        break;
      }
    }
    leds[physicalIndex(mapping[i].index)] = c;
  }
  FastLED.show();
}

void blinkAll(CRGB c, unsigned long now) {
  FastLED.clear();
  if ((now / blinkMs) % 2 == 0) {
    fill_solid(leds, MAX_LEDS, c);
  }
  FastLED.show();
}

bool connectWifi() {
  if (WiFi.status() == WL_CONNECTED) return true;
  WiFi.mode(WIFI_STA);
  WiFi.begin(WIFI_SSID, WIFI_PASS);
  Serial.printf("connecting %s", WIFI_SSID);
  unsigned long start = millis();
  while (WiFi.status() != WL_CONNECTED && millis() - start < 20000) {
    delay(200);
    Serial.print(".");
  }
  Serial.println();
  if (WiFi.status() == WL_CONNECTED) {
    Serial.printf("ip: %s\n", WiFi.localIP().toString().c_str());
    return true;
  }
  Serial.println("wifi failed");
  return false;
}

void updateBrightness() {
  if (!bhEnabled) return;
  BH1750 sensor;
  if (!sensor.begin(BH1750::CONTINUOUS_HIGH_RES_MODE, 0x23, &Wire)) {
    Serial.println("bh1750 not found");
    return;
  }
  uint16_t lux = sensor.readLightLevel();
  Serial.printf("lux: %u\n", lux);
  if (lux <= bhOffLux) {
    FastLED.setBrightness(brightLevel);
    FastLED.clear();
    FastLED.show();
    return;
  }
  FastLED.setBrightness(brightLevel);
  FastLED.show();
}

void pollStatus(unsigned long now) {
  HTTPClient http;
  http.setTimeout(10000);
  if (!http.begin(statusUrl)) {
    Serial.println("status: http.begin failed");
    return;
  }
  int code = http.GET();
  if (code != HTTP_CODE_OK) {
    Serial.printf("status: http %d\n", code);
    http.end();
    return;
  }
  String body = http.getString();
  http.end();

  JsonDocument doc;
  DeserializationError err = deserializeJson(doc, body);
  if (err) {
    Serial.printf("status: parse error %s\n", err.c_str());
    return;
  }
  setMappedLeds(doc.as<JsonArray>(), now);
}

}  // namespace

void setup() {
  Serial.begin(115200);
  delay(500);

  prefs.begin(kPrefsNs, false);
  if (!loadConfigFromNVS()) {
    Serial.println("no config in NVS, will fetch");
  }

  FastLED.addLeds<WS2812B, LED_PIN, LED_ORDER>(leds, MAX_LEDS);
  FastLED.setBrightness(brightLevel);
  FastLED.clear();
  FastLED.show();

  statusUrl = String(AGGREGATOR_URL) + "/status";
  configUrl = String(AGGREGATOR_URL) + "/config/" BOARD_ID ".json";

  if (bhEnabled) {
    Wire.begin(bhSda, bhScl);
  }

  Serial.printf("board: %s, mapping: %u entries\n", BOARD_ID, mappingCount);
}

void loop() {
  unsigned long now = millis();

  if (WiFi.status() != WL_CONNECTED) {
    connectWifi();
    delay(1000);
    return;
  }

  if (!configApplied) {
    blinkAll(CRGB::Red, now);
    if (now - lastConfigMs >= 5000) {
      lastConfigMs = now;
      if (fetchConfig()) {
        Serial.printf("config fetched: mapping %u entries, poll %us\n", mappingCount, pollIntervalSec);
        lastPollMs = 0;
      }
    }
    delay(100);
    return;
  }

  if (now - lastConfigMs >= kConfigRefreshMs) {
    lastConfigMs = now;
    if (!fetchConfig()) {
      Serial.println("config refresh failed, keeping NVS copy");
    }
  }

  if (now - lastPollMs >= pollIntervalSec * 1000UL) {
    lastPollMs = now;
    pollStatus(now);
  }

  if (bhEnabled && now - lastBrightMs >= 10000) {
    lastBrightMs = now;
    updateBrightness();
  }
}
