// Museum lights: up to 6 single colour 12 V led strips, dimmed by PWM through L298N boards (or
// MOSFETs), set by the museum controller over USB serial. The fades and pulses run here, so the
// controller only sends a line when something changes.
//
// Commands, one per line at 115200 baud, readable and typeable by hand in a serial monitor:
//   L <strip> <level 0-255> <ms>    fade the strip to level over ms (0 = at once)
//   P <strip> <pulses per second>   pulse between off and its level, 0 = steady at its level
//   ?                               answers READY <strips>
// Strips count from 0. Malformed lines are ignored. Prints READY <strips> once it has started.
//
// Runs on a GroundStudio Magma Splash (STM32F411, the one used; how to flash it is in the README), a
// GroundStudio Carbon V3 (ESP32) or an Arduino Uno / Nano; the pins are picked by the board it's
// compiled for, below.
//
// Wiring an L298N for 4 independent strips: PWM on IN1-IN4, the ENA / ENB jumpers left on,
// strip + to OUT1-OUT4, strip - to the supply's GND, 12 V into its 12 V terminal with the
// 5V-EN jumper on, and its GND to the board's GND. See hardware.md.

#include <math.h>
#include <stdlib.h>
#include <string.h>

// PWM pins, in strip order: IN1-IN4 of the first L298N, then IN1-IN2 of a second one
#if defined(ARDUINO_ARCH_ESP32)
// Carbon V3: none of the boot strapping pins (GPIO 0, 2, 5, 12, 15), the input only ones (34-39),
// the RGB led's (2, 4), the I2C plug's (21, 22) or GPIO 14, which pulses while booting
const int PINS[] = {32, 33, 25, 26, 13, 19};
#elif defined(ARDUINO_ARCH_STM32)
// Magma Splash: all on its left header, clear of USB (PA11, PA12), the debug port (PA13, PA14),
// its led (PC13), BOOT1 (PB2) and the I2C plug (PB6, PB7)
const int PINS[] = {PA6, PA7, PB0, PB1, PA0, PA1};
#else
// Uno / Nano: all 6 of their PWM pins
const int PINS[] = {3, 5, 6, 9, 10, 11};
#endif

const uint8_t STRIPS = sizeof(PINS) / sizeof(PINS[0]);
const unsigned long TICK_MS = 10;

// PWM steps. Brightness is squared for the eye (see gammaCorrect), which leaves few steps at the
// dim end: with 8 bits a pulse up to a quarter brightness gets only 16 and looks jerky, so boards
// that can (the Magma Splash, the Carbon V3) use 12 bits. The Uno / Nano only have 8
#if defined(ARDUINO_ARCH_AVR)
const int PWM_BITS = 8;
#else
const int PWM_BITS = 12;
#endif
const long PWM_MAX = (1L << PWM_BITS) - 1;

struct Strip {
  float level;              // brightness now, 0-255, before the pulse
  float from, to;           // fading from -> to
  unsigned long fadeStart;  // millis() when the fade began
  unsigned long fadeMs;     // 0 when not fading
  float rate;               // pulses per second, 0 = steady
  float nextRate;           // a stop waits for the pulse's brightest point, so it doesn't jump
  float phase;              // 0-1 through a pulse, carries on when the rate changes
};

Strip strips[STRIPS];
char line[40];
uint8_t lineLength = 0;
bool lineTooLong = false;
unsigned long lastTick = 0;

void setup() {
#if !defined(ARDUINO_ARCH_AVR)
  analogWriteResolution(PWM_BITS);
#endif
  for (uint8_t i = 0; i < STRIPS; i++) {
    pinMode(PINS[i], OUTPUT);
    analogWrite(PINS[i], 0);
    strips[i] = Strip{0, 0, 0, 0, 0, 0, 0, 0.5};
  }
  Serial.begin(115200);
  ready();
}

void loop() {
  readCommands();

  unsigned long now = millis();
  if (now - lastTick < TICK_MS) return;
  float dt = (now - lastTick) / 1000.0;
  lastTick = now;
  for (uint8_t i = 0; i < STRIPS; i++) update(strips[i], PINS[i], now, dt);
}

void ready() {
  Serial.print("READY ");
  Serial.println(STRIPS);
}

void update(Strip &s, int pin, unsigned long now, float dt) {
  if (s.fadeMs > 0) {
    unsigned long t = now - s.fadeStart;
    if (t >= s.fadeMs) {
      s.level = s.to;
      s.fadeMs = 0;
    } else {
      s.level = s.from + (s.to - s.from) * t / s.fadeMs;
    }
  }

  float brightness = s.level;
  if (s.rate > 0) {
    float before = s.phase;
    // stopping: a slow pulse hurries to its brightest point, within half a second, so a strip that's
    // asked to stop pulsing (e.g. to follow the sound) doesn't keep pulsing for seconds
    float rate = s.nextRate == 0 ? max(s.rate, 2.0f) : s.rate;
    s.phase += rate * dt;
    s.phase -= floor(s.phase);
    // stopping: once the pulse passes its brightest point it stays there, steady at its level
    if (s.nextRate == 0 && before < 0.5 && s.phase >= 0.5) {
      s.rate = 0;
      s.phase = 0.5;
    }
    // smoothly off -> on -> off once per pulse, the same as the piano nodes' led
    brightness = s.level * (1 - cos(2 * M_PI * s.phase)) / 2;
  }
  analogWrite(pin, gammaCorrect(brightness));
}

// eyes see brightness roughly as the square of the PWM duty: without this, the low end of a
// fade looks like steps and the top half hardly changes. level is 0-255, the result 0-PWM_MAX
long gammaCorrect(float level) {
  float x = constrain(level, 0, 255) / 255.0;
  return (long)(x * x * PWM_MAX + 0.5);
}

void readCommands() {
  while (Serial.available() > 0) {
    char c = Serial.read();
    if (c == '\n' || c == '\r') {
      line[lineLength] = '\0';
      if (lineLength > 0 && !lineTooLong) run(line);
      lineLength = 0;
      lineTooLong = false;
    } else if (lineLength < sizeof(line) - 1) {
      line[lineLength++] = c;
    } else {
      lineTooLong = true;  // dropped once it ends
    }
  }
}

// whitespace separated arguments of a command, 0 if there aren't enough
uint8_t split(char *text, char *args[], uint8_t want) {
  uint8_t n = 0;
  for (char *arg = strtok(text, " \t"); arg != NULL && n < want; arg = strtok(NULL, " \t")) {
    args[n++] = arg;
  }
  return n == want ? n : 0;
}

void run(char *text) {
  char *args[4];
  if (strcmp(text, "?") == 0) {
    ready();
  } else if (text[0] == 'L' && split(text, args, 4)) {
    int i = atoi(args[1]);
    if (i < 0 || i >= STRIPS) return;
    Strip &s = strips[i];
    s.from = s.level;
    s.to = constrain(atoi(args[2]), 0, 255);
    s.fadeStart = millis();
    long ms = atol(args[3]);
    s.fadeMs = ms > 0 ? ms : 0;
    if (s.fadeMs == 0) s.level = s.to;
  } else if (text[0] == 'P' && split(text, args, 3)) {
    int i = atoi(args[1]);
    if (i < 0 || i >= STRIPS) return;
    Strip &s = strips[i];
    float rate = max(0.0f, (float)atof(args[2]));
    s.nextRate = rate;
    if (rate > 0) {
      if (s.rate == 0) s.phase = 0.5;  // steady is a pulse held at its brightest point
      s.rate = rate;
    }
  }
}
