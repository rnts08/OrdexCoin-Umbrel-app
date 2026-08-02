#!/bin/bash

# OrdexCoin umbrelOS app — exports.
#
# umbrelOS sources this file before rendering docker-compose.yml, so the
# variables below are available to the app's compose file as
# ${APP_ORDEXCOIN_P2P_PORT} etc.

# P2P port published on the host for inbound peers. The container always
# listens on 25174 internally (see docker-compose.yml); this is the host-side
# port. Override it per-device if 25174 is already in use.
export APP_ORDEXCOIN_P2P_PORT="${APP_ORDEXCOIN_P2P_PORT:-25174}"
