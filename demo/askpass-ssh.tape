# Demo: --askpass feeds the password into ssh without a terminal prompt
Output "/demo/askpass-ssh.gif"

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
Type "keepassxc-cli show -a Password /tmp/demo-store.kdbx ssh-pw"
Enter
Sleep 500ms
Type "demo-pass-123"
Enter
Sleep 1s

Type "kdbx-cli --key-store /tmp/demo-store.kdbx --askpass=ssh-pw -- ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o LogLevel=ERROR -o ConnectTimeout=10 -o PubkeyAuthentication=no -o PreferredAuthentications=password -o NumberOfPasswordPrompts=1 alice@sshd echo ok"
Enter
Sleep 15s
