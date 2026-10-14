Output "../demo.gif"

Set FontSize 18
Set Width 1000
Set Height 560
Set Theme "Dracula"
Set Shell bash
Set TypingSpeed 60ms

Hide
Type "bash --rcfile /tmp/runbook-demo-gen/rcfile.bash -i"
Enter
Sleep 500ms
Show

Type "runbook run db/migrate"
Enter
Sleep 3500ms

Type "runbook start infra/up api/run web/run"
Enter
Sleep 1500ms

Type "runbook ps"
Enter
Sleep 2000ms

Type "runbook logs"
Enter
Sleep 4000ms
Ctrl+C
Sleep 500ms

Type "runbook stop web/run api/run infra/up"
Enter
Sleep 2000ms
