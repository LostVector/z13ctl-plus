#!/bin/sh
systemctl --global disable z13ctl-plus.socket z13ctl-plus.service || true
systemctl disable --now z13ctl-plus-perms.service || true
