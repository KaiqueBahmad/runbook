package main

// shells are the shells completion can print a script for.
var shells = []string{"bash", "zsh", "fish"}

// completionScript returns the completion script for shell, which parseArgs
// has already checked is one of shells.
func completionScript(shell string) string {
	switch shell {
	case "bash":
		return bashCompletion
	case "zsh":
		return zshCompletion
	case "fish":
		return fishCompletion
	}
	return ""
}

// The scripts complete the commands runbook takes, the shells completion
// prints a script for, and a path after -f. They call no other program, so
// they cost nothing on a shell that never runs runbook.

const bashCompletion = `# runbook completion for bash
_runbook() {
    local cur prev i
    local -a seen
    cur="${COMP_WORDS[COMP_CWORD]}"
    prev="${COMP_WORDS[COMP_CWORD-1]}"

    case "$prev" in
        -f|--file)
            COMPREPLY=($(compgen -f -- "$cur"))
            return
            ;;
    esac

    if [[ "$cur" == -* ]]; then
        COMPREPLY=($(compgen -W '-f --file -h --help' -- "$cur"))
        return
    fi

    # What has been typed already, flags and their values left out.
    for ((i = 1; i < COMP_CWORD; i++)); do
        case "${COMP_WORDS[i]}" in
            -f|--file) i=$((i + 1)) ;;
            -*) ;;
            *) seen+=("${COMP_WORDS[i]}") ;;
        esac
    done

    case "${#seen[@]}" in
        0)
            COMPREPLY=($(compgen -W 'list completion' -- "$cur"))
            ;;
        1)
            if [[ "${seen[0]}" == completion ]]; then
                COMPREPLY=($(compgen -W 'bash zsh fish' -- "$cur"))
            fi
            ;;
    esac
}
complete -F _runbook runbook
`

const zshCompletion = `# runbook completion for zsh
_runbook() {
    local state
    local -a commands
    commands=(
        'list:print the name of every command in the Runbookfile'
        'completion:print a completion script for bash, zsh or fish'
    )

    _arguments -C \
        '(-h --help)'{-h,--help}'[print this help and exit]' \
        '(-f --file)'{-f,--file}'[Runbookfile to work on]:runbookfile:_files' \
        '1: :->command' \
        '*:: :->argument'

    case $state in
        command)
            _describe 'command' commands
            ;;
        argument)
            # words and CURRENT are the command's own arguments here, so
            # CURRENT is 2 only while the first one is being typed.
            case $words[1] in
                completion)
                    (( CURRENT == 2 )) && _values 'shell' bash zsh fish
                    ;;
            esac
            ;;
    esac
}
compdef _runbook runbook
`

const fishCompletion = `# runbook completion for fish
complete -c runbook -f
complete -c runbook -s h -l help -d 'print this help and exit'
complete -c runbook -s f -l file -r -F -d 'Runbookfile to work on'
complete -c runbook -n __fish_use_subcommand -a list \
    -d 'print the name of every command in the Runbookfile'
complete -c runbook -n __fish_use_subcommand -a completion \
    -d 'print a completion script for bash, zsh or fish'
complete -c runbook \
    -n '__fish_seen_subcommand_from completion; and not __fish_seen_subcommand_from bash zsh fish' \
    -a 'bash zsh fish' -d shell
`
