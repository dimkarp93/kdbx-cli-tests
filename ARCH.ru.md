# Архитектура kdbx-cli-tests

Документ описывает, как устроены тесты, от чего они зависят и как работает демо-запись (VHS). Как запускать — см. `README.ru.md`.

## 1. Принципы

- **Чёрный ящик.** Тесты не импортируют код `kdbx-cli`. Они запускают выпущенный бинарь и смотрят только на наблюдаемое поведение: stdout, stderr, код выхода, что получила дочерняя программа, что увидел сервис.
- **Тестируется релиз, а не исходники.** Версия задаётся в `versions.txt`, бинарь берётся из GitHub-релиза с тегом `v<версия>`.
- **Реальные программы и сервисы.** Каналы доставки проверяются на настоящих `psql`, `ssh`, `git`, `restic`, `gpg`, `sudo`, `keepassxc-cli`. Сетевые сервисы — только `postgres` и `sshd`, и только там, где без них нельзя доказать инвариант (`tests/docker`, §5); всё, что можно проверить локально, живёт в `tests/basic` без единого docker-сервиса.
- **Детерминизм и изоляция.** Каждый прогон начинается с чистого стека и новых случайных паролей. У каждого теста свой `HOME`, у каждой команды таймаут. Во время тестов сеть наружу не нужна: всё скачивается на этапе сборки образов.
- **Без скрытых пропусков.** Если внутри раннера нет нужной программы, тест падает, а не пропускается. Так образ не деградирует молча.

## 2. Общая схема

```
 хост                               docker compose (сеть e2e, проект kdbx-cli-tests)
 ────                               ──────────────────────────────────────────────────
 just test ──► .env (секреты) ──┬─► postgres   sshd
     │                          │      ▲         ▲
     │                          │      └─────────┘ реальные протоколы
     │                          └─► runner (без TTY, -T)
     │                               go test ./... ──► kdbx-cli (релиз) ──► psql/ssh/git/...
     │                                    │
     └──────── _logs/ ◄───────────────────┘  compose.log, demo/ (bind mount)
```

Раннер — единственный контейнер, где выполняется Go-код тестов. Сервисы поднимаются заранее через `up --wait`; раннер запускается отдельно через `compose run --rm -T runner …`.

## 3. Четыре команды, три непересекающихся группы

- **`just test-basic`** — только `tests/basic`, `compose run --no-deps`: сервисы не поднимаются вообще.
- **`just test-docker`** — только `tests/docker`, с `postgres`+`sshd`.
- **`just test`** — объединение обеих групп одним `go test ./...` внутри поднятого стека: гарантирует, что всё работает, без записи demo.
- **`just test-demo`** — стоит особняком: `DEMO=1 just test`, то есть сначала весь `test`, и только если он прошёл — рендер `demo/*.tape` (см. §8).

Жизненный цикл `just test` (общий для `test`/`test-demo`, `test-docker` — тот же без шага 4/6, `test-basic` — без сервисов вовсе):

1. `down -v` и удаление `.env`: чистый старт, даже если прошлый прогон оборвался.
2. `just env` генерирует `.env` (`0600`): для каждой переменной из `SECRETS` значение `openssl rand -hex 16`.
3. `just up`: `compose up -d --build --wait` поднимает `postgres`+`sshd` и ждёт их healthcheck, затем `compose build runner`.
4. `prepare-demo` (только при `DEMO=1`) пересоздаёт `_logs/demo/` с правами `0777`, чтобы в каталог мог писать пользователь `tester` из контейнера.
5. `compose run --rm -T runner go test -count=1 -tags=e2e ./...` (или `./tests/docker/...` для `test-docker`, `./tests/basic/...` для `test-basic`) с учётом `MASK` и `VERBOSE`.
6. Если тесты прошли и `DEMO=1`: по очереди `compose run --rm -T runner vhs demo/<name>.tape` для каждого `demo/*.tape` — сервисы ещё живы, поэтому сценарии вроде `askpass-ssh`/`psql-terminal` подключаются к настоящим `sshd`/`postgres` (см. §8, «VHS-записи»).
7. Если что-то упало (тесты или запись demo), `compose logs` сохраняется в `_logs/compose.log`.
8. `down -v` и удаление `.env`: контейнеры, тома и пароли исчезают. Код выхода `just` равен коду выхода тестов.

`*-local` варианты всех четырёх команд (`test-local`, `test-basic-local`, `test-docker-local`, `test-demo-local`) идентичны, но собирают `kdbx-cli` из `../kdbx-cli` вместо релиза (см. §4).

## 4. Версия kdbx-cli

```
versions.txt (0.9.1)
  └► justfile: export KDBX_CLI_VERSION := `tr -d '[:space:]' < versions.txt`
       └► compose.yaml: runner.build.args.KDBX_CLI_VERSION и runner.environment.KDBX_CLI_VERSION (${…:?})
            └► runner/Dockerfile: github_install.sh -s https://github.com dimkarp93/kdbx-cli kdbx-cli "$KDBX_CLI_VERSION"
                 (скачивает kdbx-cli-linux-<arch>.tar.gz и SHA256SUMS, сверяет сумму, ставит в /usr/local/bin,
                  затем test "$(kdbx-cli --version)" = "$KDBX_CLI_VERSION")
                      └► TestMain → harness.CheckBinary(): ещё раз сверяет --version с KDBX_CLI_VERSION
```

Версия проверяется дважды: при сборке образа и при старте тестов. Это защищает от закэшированного образа со старым бинарём.

### `LOCAL=1`: сборка из исходников

```
LOCAL=1 just test (или just test-local / test-basic-local / test-demo-local)
  └► just: kdbx_cli_version читает ../kdbx-cli/versions.txt вместо versions.txt этого репозитория
       (нет ../kdbx-cli или в нём нет versions.txt → kdbx_cli_version = NOTFOUND)
  └► stage-local-src: NOTFOUND или нет ../kdbx-cli → понятная ошибка и exit 1
       иначе → rsync ../kdbx-cli (без .git и бинаря kdbx-cli) в runner/kdbx-cli-src/ (в .gitignore)
  └► compose.yaml: runner.build.args.KDBX_CLI_LOCAL=1
       └► runner/Dockerfile: COPY runner/kdbx-cli-src /opt/kdbx-cli-src
            RUN cd /opt/kdbx-cli-src && GOFLAGS=-mod=vendor go build -o /usr/local/bin/kdbx-cli ./cmd/kdbx-cli
            (та же -ldflags-схема, что в Makefile самого kdbx-cli, channel=local)
```

Без `LOCAL=1` `stage-local-src` просто очищает `runner/kdbx-cli-src/` до пустого `.gitkeep` — Dockerfile берёт ветку `github_install.sh` как раньше, слой `COPY` при этом не инвалидируется (содержимое каталога побайтово одинаково между обычными прогонами).

## 5. Сервисы

Сервисов ровно два — только то, что нельзя доказать без реальной сети (§9 объясняет, почему остальное намеренно не заведено):

| Сервис | Образ | Что проверяем | Готовность | Кто использует |
|---|---|---|---|---|
| `postgres` | `postgres:17.6-alpine` | пароль роли `app_stdin` (SCRAM); роль создаёт `services/postgres/init.sh` из `E2E_PG_STDIN_PW` | `pg_isready -h 127.0.0.1` | STDIN-PG-3 (демо `psql-terminal`) |
| `sshd` | свой, `alpine:3.22` + openssh | пароль `alice`, ключи из `/seed/ssh/authorized_keys` (`StrictModes no`) | `nc -z 127.0.0.1 22` | ASK-SSH-3 (демо `askpass-ssh`), FILE-SSH-1/2 |
| `runner` | свой, `golang:1.26-trixie` | среда выполнения тестов | — (профиль `runner`) | все тесты |

Демо-сценарий `psql-terminal` заводит свою собственную роль `app_demo` (фиксированный пароль, создаётся `demo/setup.sh` через админа postgres) — она не пересекается с `app_stdin`, чтобы не трогать случайный пароль реального теста.

### Раннер (`runner/`)

- База `golang:1.26-trixie`: Go нужен, чтобы запускать `go test` внутри контейнера.
- Клиенты из apt: `keepassxc-minimal` (даёт `keepassxc-cli`), `postgresql-client`, `openssh-client`, `git`, `restic`, `gnupg`, `sudo`, `curl`, `jq`, `procps`, `util-linux`, `netcat-openbsd`, `openssl`, плюс `ffmpeg`/`chromium` и статический `ttyd` для demo-записей (§8).
- Установщик `github_install.sh` скачивается с коммита `INSTALL_REF` в `/opt/install/` — нужен и для установки самого `kdbx-cli` из релиза, и для теста `EnvGH3` (dry-run, без сети).
- Пользователь `tester` с sudo по паролю и `Defaults timestamp_timeout=0`: кэш sudo не маскирует тесты.
- Исходники тестов копируются в `/src`. `GOFLAGS=-mod=vendor`, `GOCACHE=/tmp/gocache`, `KDBX_CLI_BIN=/usr/local/bin/kdbx-cli`, `KDBX_CLI_E2E_ROOT=/work`, `E2E_IN_DOCKER=1`.
- `entrypoint.sh` запускается от root: задаёт пароль `tester` из `E2E_SUDO_PW`, отдаёт `/seed` пользователю `tester` и понижает привилегии через `setpriv`.

Общий том `seed` монтируется в `runner` и `sshd` — единственное, что там лежит теперь, это `/seed/ssh/authorized_keys` (создаётся самим `sshd`-контейнером при старте), туда FILE-SSH/ASK-SSH-тесты дописывают свои ключи.

## 6. Структура кода

```
internal/harness/            общий код, без build-тега
  sandbox.go                 Sandbox, NewSandbox, SafeName, MakeStore, WriteConfig, ConfigPath, StoreTitles,
                             BaseEnv, Exec, RunEnv, Run, RunStdin, RunWithPassword, RunNoPassword, Result
  kdbx.go                    Config/Section (JSON-схема конфига kdbx-cli), LoadConfig, KeepassRun, ExportTitles
  binary.go                  CheckBinary, KDBX_CLI_BIN / KDBX_CLI_VERSION
tests/basic/                 быстрый набор (тег e2e), без сервисов — большинство сценариев
  basic_test.go              TestMain (без ожидания сервисов), newSandbox, E2E-*
  helpers_test.go            secret, requireTool, expectOK/expectFail, storeWith, resticRepo, runtimeSandbox
  pty_test.go                ptySession (creack/pty), ответы на запросы терминала, TTY-1..3
  env_test.go, file_test.go, ssh_test.go, stdin_test.go, negative_test.go, security_test.go
tests/docker/                только то, что нельзя проверить без postgres/sshd (тег e2e)
  harness_test.go            TestMain (ожидание postgres+sshd), newSandbox, secret, requireTool, expectOK/expectFail
  pty_test.go                startPTY, STDIN-PG-3, ASK-SSH-3
  ssh_test.go                sshPasswordArgs/sshKeyArgs/authorizedKey, FILE-SSH-1/2
services/                    postgres/init.sh (роль app_stdin), sshd/ (Dockerfile, entrypoint.sh, sshd_config)
demo/                        setup.sh (демо-хранилище + роль app_demo в postgres) + *.tape (сценарии vhs)
```

### Песочница теста

`harness.NewSandbox(t, bin)` создаёт `/work/<SafeName(t.Name())>/` (для хоста — `KDBX_CLI_E2E_ROOT`):

- `home/` — `HOME` для всех команд теста, внутри `home/.config/kdbx-cli/default` (конфиг) и `home/store.kdbx`;
- `run/` (`0700`) — `XDG_RUNTIME_DIR`, туда kdbx-cli кладёт каталог askpass;
- каталог удаляется в `t.Cleanup`, если не задан `KDBX_CLI_E2E_KEEP=1`.

`MakeStore` собирает KeePass XML, импортирует его через `keepassxc-cli import` с мастер-паролем `TestPassword` и сразу удаляет XML, чтобы открытый текст не остался на диске (это важно для SEC-DISK). Мастер-пароль передаётся kdbx-cli через `KDBX_CLI_PASSWORD`, кроме PTY-тестов, где он вводится с терминала.

`Exec` — единая точка запуска команд: `exec.CommandContext` с таймаутом 30 с, `WaitDelay` 2 с, stdout и stderr собираются, при таймауте тест падает с частичным выводом. Все `Run*` и `sh` — обёртки над ним.

### PTY

`ptySession` запускает процесс под псевдотерминалом 200×40 (`pty.StartWithSize`), в фоне читает вывод и умеет `expect` (подстрока с таймаутом), `send`, `wait`, `mustExit`, `kill`. Под терминалом kdbx-cli (termenv/lipgloss) запрашивает цвет фона (`OSC 11`) и позицию курсора (`CSI 6n`). Хелпер отвечает на эти запросы, как настоящий терминал, иначе каждый запуск тратил бы 5 с на ожидание.

## 7. Зависимости

### Go

| Модуль | Где | Зачем |
|---|---|---|
| stdlib | везде | весь harness, тесты |
| `github.com/creack/pty` | `tests/basic/pty_test.go`, `tests/docker/pty_test.go` | псевдотерминал для TTY-сценариев |
| `golang.org/x/sys/unix` | `tests/basic/pty_test.go` | чтение termios (эхо после Ctrl+C) |

Зависимости лежат в `vendor/` (`GOFLAGS=-mod=vendor`), поэтому `go test` в раннере не ходит в сеть. `just vendor-check` проверяет, что `vendor/` соответствует `go.mod`.

Граф пакетов: `tests/basic` → `internal/harness`, `creack/pty`, `x/sys`; `tests/docker` → `internal/harness`, `creack/pty`. Код `kdbx-cli` не импортируется. Схема конфига продублирована в `kdbx.go` намеренно: если формат в утилите поменяется, это заметят тесты.

### Что скачивается при сборке

| Что | Откуда | Закреплено |
|---|---|---|
| `kdbx-cli` | GitHub release `dimkarp93/kdbx-cli` | `versions.txt` + SHA256SUMS |
| установщик `github_install.sh` | raw.githubusercontent.com `dimkarp93/install` | `INSTALL_REF` (коммит) |
| клиентские программы | Debian trixie apt | версия дистрибутива |
| `vhs` | `go install` | без версии (`@latest`) |
| `ttyd` | GitHub release `tsl0922/ttyd` | `TTYD_VERSION` |
| образы сервисов | Docker Hub | теги с точной версией |

### Тесты и сервисы

| Группа | postgres | sshd |
|---|---|---|
| `tests/basic` (всё остальное) | | |
| `tests/docker`: STDIN-PG-3 | ✓ | |
| `tests/docker`: ASK-SSH-3, FILE-SSH-1/2 | | ✓ |

`TestMain` в `tests/docker` ждёт TCP-порты `postgres:5432` и `sshd:22` из `serviceAddrs`. `tests/basic` не ждёт ничего — сервисов у неё нет.

### Секреты

```
justfile (openssl rand) ─► .env ─► env раннера ─► secret(t, "E2E_…") ─► MakeStore(.kdbx) ─► kdbx-cli ─► программа ─► сервис
```

Захардкоженных паролей нет. Тест кладёт в `.kdbx` то же значение, с которым настроен сервис (или которое сам создал, как `demo/setup.sh` — роль `app_demo`), поэтому успешный вход однозначно доказывает, что секрет доставлен. `E2E_REG_PW` (историческое имя) больше не привязан ни к какому сервису — несколько тестов в `tests/basic` (restic, gpg, keepassxc) используют его просто как источник случайной строки.

### Общее состояние и порядок

Тесты не используют `t.Parallel()` и идут последовательно. Общее состояние у них такое:

- `/seed/ssh/authorized_keys` (тесты дописывают свои ключи);
- файлы в `/etc` от sudo-тестов (уникальные имена).

## 8. Демо-запись (VHS)

Запись включается только при `DEMO=1` (`just test-demo`), уже после того, как `go test` прошёл, и никак не связана с самим Go-кодом тестов: тесты только проверяют поведение, ничего из происходящего в них не записывается. Вся запись — это сценарии `demo/*.tape`, прогоняемые через [`vhs`](https://github.com/charmbracelet/vhs):

```
DEMO=1 just test (уже после прохождения go test)
  └► prepare-demo: _logs/demo/ (0777)
  └► for tape in demo/*.tape:
       compose run -v _logs/demo:/demo runner vhs "$tape"
         └► demo/setup.sh (Hide-блок) создаёт /tmp/demo-store.kdbx: ssh-pw из E2E_SSH_PW, pg-stdin — фиксированный пароль
            новой роли app_demo (создаётся тут же через админа postgres, отдельно от случайного app_stdin теста)
         └► реальная интерактивная сессия в headless-терминале (ttyd) записывается кадрами через
            headless Chromium (xterm.js) и собирается в GIF через ffmpeg
```

`runner/Dockerfile` ставит `vhs` (`go install`), статический `ttyd` (apt-пакета нет на trixie), `ffmpeg`, `chromium`; `VHS_NO_SANDBOX=1` обязателен — Chromium без него отказывается стартовать в контейнере. `Output` в `.tape`-файлах — обязательно в кавычках (`Output "/demo/name.gif"`): нераскавыченный абсолютный путь не парсится этой версией vhs; то же самое верно для двойных кавычек внутри `Type "…"` — вложенное экранирование `\"…\"` не парсится, для строк с кавычками (SQL-литералы и т.п.) используйте другой символ кавычек внутри.

Каждый `.tape` заканчивается `Sleep 15s` на финальном кадре — время, чтобы зритель прочитал результат перед тем, как GIF зациклится.

`just list-demos` — список алиасов (имя `.tape` без расширения) с пометкой, отрендерен ли уже `.gif`; `just demo <name>` открывает готовый `_logs/demo/<name>.gif` через `open`/`xdg-open`.

Проверок здесь нет: `vhs` рендерит то, что напечатал сценарий, и не сверяет вывод `kdbx-cli` ни с чем. Если реальное поведение изменится, `.tape` всё равно «успешно» отрендерит гифку — просто, возможно, неправильную. Единственные настоящие проверки — в Go-тестах (§6); demo-сценарии вручную дублируют те же шаги и не синхронизируются с тестами автоматически.

## 9. Как расширять

- **Новый сценарий без сервисов.** Файл в `tests/basic/` с тегом `e2e`. `newSandbox` уже есть в `basic_test.go`; `secret`/`requireTool`/`expectOK`/`expectFail`/`storeWith`/`resticRepo` — в `helpers_test.go`. Это правило по умолчанию: пишите сюда, если сценарий вообще можно доказать без сети.
- **Новый сценарий на `postgres`/`sshd`.** Файл в `tests/docker/` с тегом `e2e`, только если без реального сервиса инвариант не доказать (см. §5) — иначе это дубликат уже проверенного канала, и ему место в `tests/basic`. `requireTool`, `newSandbox`, `sb.Run*`; для PTY — `startPTY`.
- **Новый сервис.** Прежде чем заводить — проверьте, нельзя ли то же самое доказать локально (см. предыдущий пункт про дубликаты). Если правда нужен: добавить в `compose.yaml` с healthcheck, в `depends_on` раннера и в `serviceAddrs` (`tests/docker/harness_test.go`). Если нужен пароль — новая переменная в `SECRETS` justfile.
- **Новая версия kdbx-cli.** Поменять `versions.txt`. Если поменялось поведение, обновить тесты в том же коммите.
- **Новое VHS-демо.** Файл `demo/<name>.tape`; setup — через `Hide`-блок (`bash demo/setup.sh`), не показывая его зрителю; `Output` — обязательно в кавычках. `just test-demo` подхватывает файл автоматически, `just demo <name>` открывает результат, `just list-tests` покажет его в колонке DEMO.
