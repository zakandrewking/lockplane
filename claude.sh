#!/bin/bash

if ! security find-generic-password -s "Claude Code-credentials" -g 2>/dev/null | grep -q acct; then
    echo "No Claude creds in Keychain - unlocking keychain..."
    security unlock-keychain ~/Library/Keychains/login.keychain-db
fi

claude -c --dangerously-skip-permissions
