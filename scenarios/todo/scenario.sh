#!/usr/bin/env bash
cd "$(dirname "$0")"
rm -rf build/
mkdir build/
cd build
git init
claude "generate a new todo list app using next.js, sqlite, and lockplane"

# pretend this is my level of knowledge:
# https://www.youtube.com/watch?v=S5ULIoThOIM
# https://www.youtube.com/watch?v=5ELwn5ZFz2c
# https://www.youtube.com/watch?v=A-0qjpViZFQ

# validation:
# - lockplane latest is installed
# - claude skill is installed
# - playwright skill is installed
# - schema/ exists and only has *.lp.sql files
# - no migration or plan files exist
# - local sqlite db is up to date with *.lp.sql files
# - basic functionality of the app works (add, remove, complete todo items)
# - app uses lockplane-js for db access
# - playwright is used manually to verify the app functionality
