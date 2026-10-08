#!/bin/bash
# Compact view of MICAPP diagnostics: crashes, editor-open timings in ms,
# input-queue lag, last capture pipelines. Run after using the app.
cd "$(dirname "$0")"

echo "=== crash.log (критические события) ==="
if [ -f crash.log ]; then tail -40 crash.log; else echo "(пока пусто)"; fi
echo

echo "=== Сессия ==="
[ -f .micapp.session ] && cat .micapp.session
echo

echo "=== Открытия редактора (последние 60 строк) ==="
grep -hE "PERF \[editor #[0-9]+\] (open|WINDOW VISIBLE|first paint)|⚠ SLOW" app.log | tail -60
echo

echo "=== Задержка событий в очереди (backlog) ==="
grep -hE "PERF \[events\] backlog" app.log | tail -20 || true
echo

echo "=== Последние захваты (пайплайн capture→editor→post) ==="
grep -hE "PERF \[(capture|editor|post) #[0-9]+\]" app.log | tail -50
