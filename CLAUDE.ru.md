# CLAUDE.md

## Стиль кода

**Не пиши комментарии в коде.** Код должен быть самодокументируемым через имена. Единственное исключение — короткая строка «почему», если причина иначе не видна (скрытый инвариант, обход бага).

## Язык

Весь код, скрипты, сообщения тестов и CI-конфиги — **только на английском**. Документация ведётся парами файлов: `<name>.ru.md` (источник истины), `<name>.en.md` и `<name>.md` — симлинк на `<name>.en.md`. Изменение документа — правка обеих версий.

## Назначение

Интеграционные тесты `kdbx-cli` по принципу чёрного ящика. Тестируемая версия — `versions.txt` (`X.Y.Z`); образ раннера ставит этот GitHub-релиз (`dimkarp93/kdbx-cli`, тег `v<версия>`) через `github_install.sh`, а `harness.CheckBinary` не даёт запуститься, если `kdbx-cli --version` отличается. Код kdbx-cli тесты не импортируют: схема конфига намеренно продублирована в `internal/harness/kdbx.go` как контракт.

## Структура

- `compose.yaml` — сервисы `postgres` и `sshd` (только то, что нужно демо и `tests/docker`) и `runner` (профиль `runner`, запуск с `-T`, без TTY). `KDBX_CLI_VERSION` приходит из justfile.
- `runner/` — `golang:1.26-trixie` + клиентские программы + `keepassxc-minimal` + `vhs`/`ttyd`/`ffmpeg`/`chromium` для демо-записей (`VHS_NO_SANDBOX=1`); entrypoint задаёт sudo-пароль `tester` и понижает привилегии.
- `demo/` — `setup.sh` поднимает демо-хранилище из секретов `.env`, `*.tape` — сценарии `vhs` (реальная интерактивная сессия в терминале, не выдержка из go-test-протокола); рендерятся в `_logs/demo/*.gif` через `just test-demo`.
- `services/` — инициализация PostgreSQL (`postgres/init.sh`, роль `app_stdin`), sshd.
- `internal/harness` — `Sandbox` (свой `HOME` на тест, `MakeStore`, `WriteConfig`, `StoreTitles`, `Run*`/`Exec` с таймаутом 30 с), `Demo`/`Cast` (протоколы и asciinema-записи по тестам, секреты замаскированы), `CheckBinary`, `WriteSummaryHeader`, схема конфига и работа с `keepassxc-cli`.
- `tests/basic` — быстрый набор (без сервисов, большинство сценариев), `tests/docker` — только то, что нельзя проверить без postgres/sshd (`TestMain` ждёт эти два сервиса). Не пересекаются. Build-тег `e2e`.

## Команды

`[MASK=…] [LOCAL=1] [VERBOSE=1] just test` (basic+docker, без демо, гарантирует работоспособность), `just test-basic`, `just test-docker` (только сервис-зависимые сценарии), `just test-demo` (отдельно: прогон + запись demo), `*-local` варианты всех четырёх (kdbx-cli из `../kdbx-cli`, не из `versions.txt`), `just list-tests` (что входит в basic/docker/demo, в 3 колонки), `just list-demos`, `just demo <name>`, `just make-screens`, `just screen <name>`, `just up/shell/logs/down`, `just vet`, `just vendor`/`vendor-check` (`GOFLAGS=-mod=vendor`). Секреты сервисов генерируются в `.env` на каждый прогон; в `_logs/` — логи compose, протоколы и demo-записи (`.gif`).
