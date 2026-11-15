#!/usr/bin/env bash
# Flashes firmware/lights onto the Magma Splash over USB, with the dfu-util copy in firmware/lights.
#
# First, in the Arduino IDE: Sketch -> Export Compiled Binary (board settings in the README).
# Then unplug the strips' 12 V supply (with it on, dfu mode doesn't connect over usb) and put the
# Magma in dfu mode: hold BOOT0, tap NRST, release BOOT0.
#
#   ./flash-lights.sh            # the only Magma in dfu mode
#   ./flash-lights.sh <serial>   # one of several, serials from: sudo firmware/lights/dfu-util -l
set -euo pipefail

dir="$(cd "$(dirname "$0")/firmware/lights" && pwd)"
dfu="$dir/dfu-util"
bin="$dir/build/STMicroelectronics.stm32.GenF4/lights.ino.bin"
# only STM32s in dfu mode: other devices show up in dfu mode too (a Framework laptop's camera does),
# and flashing one of those could brick it
device=0483:df11

if [ ! -f "$bin" ]; then
    echo "no binary at $bin: export it in the Arduino IDE first (Sketch -> Export Compiled Binary)" >&2
    exit 1
fi
if [ "$bin" -ot "$dir/lights.ino" ]; then
    echo "the binary is older than lights.ino: export it again in the Arduino IDE first" >&2
    exit 1
fi

serial="${1:-}"
if [ -z "$serial" ]; then
    serials=$(sudo "$dfu" -l 2>/dev/null |
        sed -n 's/^Found DFU: \[0483:df11\].* alt=0,.* serial="\([^"]*\)".*/\1/p' | sort -u)
    case $(printf '%s' "$serials" | grep -c .) in
    0)
        echo "no Magma in dfu mode: unplug the strips' 12 V supply (it keeps dfu mode from connecting)," >&2
        echo "hold BOOT0, tap NRST, release BOOT0, then run this again" >&2
        exit 1
        ;;
    1) serial=$serials ;;
    *)
        echo "several STM32s in dfu mode, run again with the serial of the one to flash:" >&2
        printf '  %s\n' $serials >&2
        exit 1
        ;;
    esac
fi

echo "flashing $(basename "$bin") onto $device, serial $serial"
log=$(mktemp)
trap 'rm -f "$log"' EXIT
# dfu-util sometimes reports an error after a finished download, as the board leaves dfu mode
# (":leave"), so success is read from its output instead of its exit status
sudo "$dfu" -d "$device" -S "$serial" -a 0 -s 0x08000000:leave -D "$bin" 2>&1 | tee "$log" || true
if ! grep -q "File downloaded successfully" "$log"; then
    echo "flashing failed, see above" >&2
    exit 1
fi

# the board restarts into the sketch and comes back as a usb serial port
for _ in $(seq 20); do
    if ls /dev/ttyACM* >/dev/null 2>&1; then
        echo "done: running the sketch, on $(ls /dev/ttyACM* | head -1)"
        exit 0
    fi
    sleep 0.25
done
echo "flashed, but no /dev/ttyACM* showed up yet: unplug and replug the Magma" >&2
