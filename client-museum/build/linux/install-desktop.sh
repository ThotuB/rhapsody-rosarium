#!/usr/bin/env bash
# Installs a desktop entry for this checkout's bin/museum, for the current user only, so GNOME's dock
# and app grid show the museum with its rose icon (build/appicon.png) instead of a generic one. It's
# named after the app id in main.go: on Wayland that's how a window is matched to its entry.
#
#   build/linux/install-desktop.sh            # install, or update after moving the checkout
#   build/linux/install-desktop.sh --remove   # remove it again
set -euo pipefail

id=ro.rhapsodyrosarium.Museum
root="$(cd "$(dirname "$0")/../.." && pwd)" # client-museum
entry="${XDG_DATA_HOME:-$HOME/.local/share}/applications/$id.desktop"

if [ "${1:-}" = "--remove" ]; then
    rm -f "$entry"
    echo "removed $entry"
    exit 0
fi

mkdir -p "$(dirname "$entry")"
# Path: the app reads config/ and media/ relative to the folder it runs from
cat >"$entry" <<EOF
[Desktop Entry]
Type=Application
Name=Rhapsody Rosarium museum
Comment=Plays the city's pianos on the installation's speakers
Exec=$root/bin/museum
Path=$root
Icon=$root/build/appicon.png
StartupWMClass=$id
Categories=AudioVideo;Audio;
Terminal=false
EOF
echo "installed $entry"
