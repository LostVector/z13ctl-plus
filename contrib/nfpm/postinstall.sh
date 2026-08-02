#!/bin/sh
udevadm control --reload-rules
udevadm trigger
systemctl enable --now z13ctl-plus-perms.service || true
echo "z13ctl-plus user daemon installed disabled; enable only after stopping other Z13 daemons:"
echo "  systemctl --user enable --now z13ctl-plus.socket z13ctl-plus.service"
