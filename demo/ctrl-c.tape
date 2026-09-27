# Demo: Ctrl+D is ignored at the password prompt, Ctrl+C aborts it
Output "/demo/ctrl-c.gif"

Set Shell "bash"
Set FontSize 20
Set Width 1200
Set Height 700
Set Padding 20
Set TypingSpeed 60ms

Hide
Type "bash demo/setup.sh && clear" Enter
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
Type "keepassxc-cli show -a Password /tmp/demo-store.kdbx MARK"
Enter
Sleep 500ms
Type "demo-pass-123"
Enter
Sleep 1s

Type "kdbx-cli --key-store /tmp/demo-store.kdbx --secrets=MARK:MARK -- sh -c 'echo ran:$MARK'"
Enter
Sleep 1s
Ctrl+D
Sleep 1s
Ctrl+C
Sleep 15s
