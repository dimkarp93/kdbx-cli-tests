# Demo: psql ignores the --stdin secret, asks for a password at the terminal instead, and connects once it's typed there
Output "/demo/psql-terminal.gif"

Set Shell "bash"
Set FontSize 18
Set Width 1400
Set Height 700
Set Padding 20
Set TypingSpeed 40ms

Hide
Type "export KDBX_CLI_PASSWORD=demo-pass-123 && bash demo/setup.sh && clear" Enter
Sleep 1s
Show

Type "echo 'Secrets in the demo store /tmp/demo-store.kdbx:'"
Enter
Type "keepassxc-cli ls /tmp/demo-store.kdbx"
Enter
Sleep 500ms
Type "demo-pass-123"
Enter
Sleep 1s
Type "keepassxc-cli show -a Password /tmp/demo-store.kdbx pg-stdin"
Enter
Sleep 500ms
Type "demo-pass-123"
Enter
Sleep 1s

Type "echo '--stdin=pg-stdin injects a secret, but psql still prompts on the terminal:'"
Enter
Sleep 1s
Type "kdbx-cli --key-store /tmp/demo-store.kdbx --stdin=pg-stdin -- psql -h postgres -U app_demo -d postgres -W -Atc 'select 1'"
Enter
Sleep 2s
Type "demo-pg-pass-123"
Enter
Sleep 1s
Type "echo 'Confirms: --stdin was ignored, the same password had to be typed manually to connect'"
Enter
Sleep 15s
