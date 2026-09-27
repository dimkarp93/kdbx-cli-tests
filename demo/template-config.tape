# Demo: --template renders a config file, replacing {{Title}} placeholders in its content with real secrets
Output "/demo/template-config.gif"

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

Type "echo 'Template file on disk (no real secret inside):'"
Enter
Type "cat /tmp/demo-settings.json.tmpl"
Enter
Sleep 2s

Type "kdbx-cli --key-store /tmp/demo-store.kdbx --template=cfg:/tmp/demo-settings.json.tmpl -- cat {{cfg}}"
Enter
Sleep 4s

Type "echo 'The template file itself was never modified:'"
Enter
Type "cat /tmp/demo-settings.json.tmpl"
Enter
Sleep 15s
