# Demo: --secret-file writes the secret to a temp file and hands gpg its path via --passphrase-file
Output "/demo/gpg-decrypt.gif"

Set Shell "bash"
Set FontSize 18
Set Width 1400
Set Height 700
Set Padding 20
Set TypingSpeed 40ms

Hide
Type "export GNUPGHOME=/tmp/demo-gnupg KDBX_CLI_PASSWORD=demo-pass-123 && bash demo/setup.sh && clear" Enter
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
Type "keepassxc-cli show -a Password /tmp/demo-store.kdbx gpg-pass"
Enter
Sleep 500ms
Type "demo-pass-123"
Enter
Sleep 1s

Type "kdbx-cli --key-store /tmp/demo-store.kdbx --secret-file=gpg-pass -- gpg --batch --no-symkey-cache --pinentry-mode loopback --passphrase-file {{gpg-pass}} -d /tmp/demo-secret.txt.gpg"
Enter
Sleep 15s
