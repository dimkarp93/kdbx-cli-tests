# CLAUDE.md

## Стиль кода

**Не пиши комментарии в коде.** Код должен быть самодокументируемым через имена. Единственное исключение — короткая строка «почему», если причина иначе не видна (скрытый инвариант, обход бага).

## Язык

Весь код, скрипты, сообщения тестов и CI-конфиги — **только на английском**. Документация ведётся парами файлов: `<name>.ru.md` (источник истины), `<name>.en.md` и `<name>.md` — симлинк на `<name>.en.md`. Изменение документа — правка обеих версий.

## Назначение

Интеграционные тесты `kdbx-cli` по принципу чёрного ящика. Тестируемая версия — `versions.txt` (`X.Y.Z`); образ раннера ставит этот GitHub-релиз (`dimkarp93/kdbx-cli`, тег `v<версия>`) через `github_install.sh`, а `harness.CheckBinary` не даёт запуститься, если `kdbx-cli --version` отличается. Код kdbx-cli тесты не импортируют: схема конфига намеренно продублирована в `internal/harness/kdbx.go` как контракт.

## Структура

- `compose.yaml` — сервисы (postgres, mariadb, sshd, gitea, registry + registry-auth, mockgithub) и `runner` (профиль `runner`, запуск с `-T`, без TTY). `KDBX_CLI_VERSION` приходит из Makefile.
- `runner/` — `golang:1.26-trixie` + клиентские программы + `keepassxc-minimal` + установщики, закреплённые `INSTALL_REF`; entrypoint задаёт sudo-пароль `tester` и понижает привилегии.
- `services/mockgithub` — мок GitHub releases API (pretty JSON, `url` ассета раньше `name`, 404 без токена, `/__requests`, `/__reset`, `/__echo`); `main_test.go` сверяет его с разбором в установщике.
- `seed/` — `seed.sh` (хост: админ Gitea) → `runner-seed.sh` (токен, приватный репозиторий, релиз, `/seed/gitea.env`, `/seed/ready`).
- `internal/harness` — `Sandbox` (свой `HOME` на тест, `MakeStore`, `WriteConfig`, `StoreTitles`, `Run*`/`Exec` с таймаутом 30 с), `Transcript`/`Cast` (протоколы и asciinema-записи по тестам, секреты замаскированы), `CheckBinary`, `WriteSummaryHeader`, схема конфига и работа с `keepassxc-cli`.
- `tests/basic` — быстрый набор (без сервисов), `tests/docker` — полный набор (`TestMain` ждёт сервисы и `/seed/ready`). Build-тег `e2e`.

## Команды

`make test [MASK=…] [TRANSCRIPT=1] [VERBOSE=1]`, `make test-basic`, `make up/shell/logs/down`, `make vet`, `make vendor`/`vendor-check` (`GOFLAGS=-mod=vendor`). Секреты сервисов генерируются в `.env` на каждый прогон; в `_logs/` — логи compose и протоколы.
