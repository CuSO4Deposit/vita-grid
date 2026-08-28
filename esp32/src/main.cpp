#include <Arduino.h>
#include <ArduinoJson.h>
#include <BH1750.h>
#include <FastLED.h>
#include <HTTPClient.h>
#include <WiFi.h>
#include <Wire.h>

#include "config_data.h"

namespace {

CRGB leds[LED_COUNT];

struct LedMap {
  String name;
  uint8_t index;
};

LedMap mapping[LED_COUNT];
uint8_t mappingCount = 0;

String ssid;
String pass;
String statusUrl;
uint32_t pollIntervalSec = 30;

bool bhEnabled = false;
uint8_t bhSda = 21;
uint8_t bhScl = 22;
float bhOffLux = 20;
uint8_t brightLevel = 50;

unsigned long lastPollMs = 0;
unsigned long lastBrightMs = 0;

constexpr uint32_t kBlinkMs = 500;

CRGB colorForState(const char* state, bool busy, unsigned long now) {
  CRGB c;
  if (strcmp(state, "error") == 0) {
    c = CRGB::Red;
  } else if (strcmp(state, "warn") == 0) {
    c = CRGB::Yellow;
  } else {
    c = CRGB::Green;
  }
  if (busy && (now / kBlinkMs) % 2 == 0) {
    c = CRGB::Black;
  }
  return c;
}

void setMappedLeds(JsonArray arr, unsigned long now) {
  FastLED.clear();
  for (uint8_t i = 0; i < mappingCount; i++) {
    CRGB c = CRGB::Red;
    for (JsonObject sig : arr) {
      const char* name = sig["name"];
      if (name && mapping[i].name == name) {
        const char* state = sig["state"] | "warn";
        bool busy = sig["busy"] | false;
        c = colorForState(state, busy, now);
        break;
      }
    }
    leds[mapping[i].index] = c;
  }
  FastLED.show();
}

void blinkAll(CRGB c, unsigned long now) {
  FastLED.clear();
  if ((now / kBlinkMs) % 2 == 0) {
    fill_solid(leds, LED_COUNT, c);
  }
  FastLED.show();
}

bool parseConfig() {
  JsonDocument doc;
  DeserializationError err = deserializeJson(doc, CONFIG_JSON);
  if (err) {
    Serial.printf("config parse error: %s\n", err.c_str());
    return false;
  }

  ssid = doc["wifi"]["ssid"] | "";
  pass = doc["wifi"]["pass"] | "";
  statusUrl = doc["poll"]["url"] | "";
  pollIntervalSec = doc["poll"]["interval"] | 30;

  bhEnabled = doc["bh1750"]["enabled"] | false;
  bhSda = doc["bh1750"]["sda"] | 21;
  bhScl = doc["bh1750"]["scl"] | 22;
  bhOffLux = doc["bh1750"]["offLux"] | 20.0f;
  brightLevel = doc["brightness"]["level"] | 50;

  mappingCount = 0;
  for (JsonObject m : doc["mapping"].as<JsonArray>()) {
    if (mappingCount >= LED_COUNT) break;
    mapping[mappingCount].name = m["name"] | "";
    mapping[mappingCount].index = m["index"] | 0;
    mappingCount++;
  }
  return mappingCount > 0 && !ssid.isEmpty() && !statusUrl.isEmpty();
}

bool connectWifi() {
  if (WiFi.status() == WL_CONNECTED) return true;
  WiFi.mode(WIFI_STA);
  WiFi.begin(ssid.c_str(), pass.c_str());
  Serial.printf("connecting %s", ssid.c_str());
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
    Serial.println("http.begin failed");
    return;
  }
  int code = http.GET();
  if (code != HTTP_CODE_OK) {
    Serial.printf("http status: %d\n", code);
    http.end();
    return;
  }
  String body = http.getString();
  http.end();

  JsonDocument doc;
  DeserializationError err = deserializeJson(doc, body);
  if (err) {
    Serial.printf("status parse error: %s\n", err.c_str());
    return;
  }
  setMappedLeds(doc.as<JsonArray>(), now);
}

}  // namespace

void setup() {
  Serial.begin(115200);
  delay(500);

  if (!parseConfig()) {
    while (true) {
      blinkAll(CRGB::Red, millis());
      delay(100);
    }
  }

  FastLED.addLeds<WS2812B, LED_PIN, LED_ORDER>(leds, LED_COUNT);
  FastLED.setBrightness(brightLevel);
  FastLED.clear();
  FastLED.show();

  if (bhEnabled) {
    Wire.begin(bhSda, bhScl);
  }

  Serial.printf("mapping: %u entries\n", mappingCount);
}

void loop() {
  unsigned long now = millis();

  if (WiFi.status() != WL_CONNECTED) {
    connectWifi();
    delay(1000);
    return;
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
