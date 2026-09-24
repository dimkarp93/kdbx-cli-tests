# kdbx-cli-tests

Интеграционные тесты для [kdbx-cli](https://github.com/dimkarp93/kdbx-cli), вынесенные из самой утилиты, чтобы она оставалась маленькой и без лишних зависимостей. Тесты работают с `kdbx-cli` как с чёрным ящиком: запускают выпущенный бинарь против реальных программ и реальных сервисов в Docker и проверяют все каналы доставки секретов (`secrets`, `stdin`, `files`, `askpass`), а также инварианты безопасности.

## Какая версия тестируется

В `versions.txt` лежит версия `kdbx-cli` (`X.Y.Z`, тот же формат, что в kdbx-cli). При сборке образа раннера бинарь ставится из GitHub-релиза `v<версия>` через `github_install.sh` (SHA256 проверяется), а тесты не запустятся, если `kdbx-cli --version` сообщает другую версию. Чтобы проверить новый релиз, поменяйте `versions.txt`.

## Требования

Docker с compose v2, `make`, `openssl`. Go на хосте нужен только для `make vet` и `make vendor`.

## Запуск

```sh
make test                        # всё: сервисы + оба набора (~1 мин)
make test MASK=TestAskSSH        # только тесты, чьё имя совпадает с регуляркой
make test-basic                  # быстрый набор без сервисов (только контейнер раннера)
make test TRANSCRIPT=1           # записать, что делал каждый тест (см. ниже)
make test VERBOSE=1              # go test -v
make up                          # поднять сервисы и засеять их (для отладки)
make shell                       # шелл в контейнере раннера, с настоящим TTY
make logs                        # логи сервисов
make down                        # остановить и удалить всё
```

`make test` завершается с кодом тестов. При падении логи сервисов сохраняются в `_logs/compose.log`. Упавший тест печатает код выхода, stdout и stderr команды.

## Запись: `TRANSCRIPT=1`

Всё складывается в `_logs/transcripts/`:

- `SUMMARY.txt` — тестируемая версия `kdbx-cli`, затем все тесты с PASS/FAIL, длительностью и файлом протокола;
- `<Test>.log` — читаемый протокол: каждая команда с аргументами, добавленным окружением, stdin, stdout, stderr, кодом выхода и временем, плюс заметки теста (запросы, пришедшие в мок GitHub, что проверили сканеры утечек) и итог;
- `<Test>.cast` — запись терминала для PTY-сценариев (промпт пароля, `psql`, `ssh`) в формате asciinema: `asciinema play -s 0.2 _logs/transcripts/TestTTY1_MasterPasswordFromTerminal.cast`, либо в GIF через `agg`. Нажатия клавиш при воспроизведении не видны (пароль вводится без эха), их список есть в `.log`.

Секреты везде замаскированы: запись из хранилища выглядит как `<secret:Title>`, мастер-пароль — `<master-password>`, пароли сервисов — именем своей переменной. Поэтому `<secret:REG_TOKEN>|<secret:REG_TOKEN>` в stdout означает, что дочерний процесс действительно дважды получил этот секрет. Файлами можно делиться; CI прикладывает их к каждому прогону как artifact `e2e-logs`.

## Что покрыто

| Набор | Сценарии |
|---|---|
| `tests/basic` | env, слияние секций, флаги, dry-run, `check`/`config`, все каналы на `sh -c`, коды выхода, неверный пароль |
| env | `psql` + `PGPASSWORD`, `github_install.sh` против мока GitHub API, `gitea_install.sh` против Gitea, выбор секции, dry-run без сети |
| stdin | `psql -W` без TTY, `--stdin-keep-open`, `skopeo login --password-stdin`, `keepassxc-cli db-create`, `sudo -S` |
| files | `restic --password-file`, `gpg --passphrase-file`, `mariadb --defaults-extra-file`, `PGPASSFILE`, ключ ssh через `ssh-add`, `ssh -i` (фиксирует, что ssh закрывает дескриптор) |
| askpass | пароль ssh и passphrase ключа, `git clone` по HTTP, `sudo -A`, `RESTIC_PASSWORD_COMMAND` |
| PTY | промпт мастер-пароля (ввод, неверный пароль, Ctrl+D/Ctrl+C), `psql` предпочитает терминал, `SSH_ASKPASS_REQUIRE=force` |
| безопасность | секрета нет в argv ни одного процесса и на диске (для всех каналов), каталог askpass — `0700` и удаляется, секрет есть в environ дочернего процесса, но не kdbx-cli |
| негативные | многострочный секрет в stdin, нет плейсхолдера, нет записи, проброс кода выхода программы, неверный токен GitHub |

## Известные падения на 0.9.1

`TestFileMySQL1_MultilineOptionFile`, `TestFilePG1_PGPassFile` и `TestFileSSH2_PrivateKeyThroughSSHAdd` падают на 0.9.1: файл в памяти создаётся с правами `0777`, и `mariadb`, libpq и `ssh-add` его отвергают. Исправление (права `0600`) уже есть в kdbx-cli, но ещё не выпущено; тесты станут зелёными, когда `versions.txt` будет указывать на релиз с ним.

## Структура

- `compose.yaml` — PostgreSQL, MariaDB, sshd, Gitea, registry, мок GitHub API и контейнер `runner`;
- `runner/` — образ раннера: клиентские программы, `keepassxc-cli`, установщики `dimkarp93/install`, `kdbx-cli` из релиза;
- `services/` — мок GitHub (`mockgithub`, только stdlib), sshd, инициализация PostgreSQL;
- `seed/` — наполнение Gitea (пользователь, токен, приватный репозиторий, релиз);
- `internal/harness/` — песочницы, схема конфига, работа с `keepassxc-cli`, протоколы;
- `tests/basic/`, `tests/docker/` — тесты (build-тег `e2e`).

На GitHub набор гоняется в `.github/workflows/e2e.yml` (на push, ежедневно и вручную).
