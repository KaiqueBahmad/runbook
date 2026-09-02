package cli

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
    local cur prev file i
    local -a seen opt
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
    file=""
    for ((i = 1; i < COMP_CWORD; i++)); do
        case "${COMP_WORDS[i]}" in
            -f|--file) i=$((i + 1)); file="${COMP_WORDS[i]}" ;;
            -*) ;;
            *) seen+=("${COMP_WORDS[i]}") ;;
        esac
    done

    case "${#seen[@]}" in
        0)
            COMPREPLY=($(compgen -W 'list run start stop status logs completion iamllm' -- "$cur"))
            ;;
        1)
            case "${seen[0]}" in
                completion)
                    COMPREPLY=($(compgen -W 'bash zsh fish' -- "$cur"))
                    ;;
                run|start|stop|logs)
                    # The names come from the runbook.yml being completed for,
                    # asked of the very runbook being typed. bash has nowhere to
                    # show the description behind the tab, so it is cut off. A
                    # newline IFS keeps names with spaces in one piece.
                    [[ -n "$file" ]] && opt=(-f "$file")
                    local IFS=$'\n'
                    COMPREPLY=($(compgen -W "$("${COMP_WORDS[0]}" list "${opt[@]}" 2>/dev/null | cut -f1)" -- "$cur"))
                    ;;
            esac
            ;;
    esac
}
complete -F _runbook runbook
`

const zshCompletion = `# runbook completion for zsh
_runbook() {
    local state prog=$words[1]
    local -a commands
    commands=(
        'list:print the name of every command in the runbook.yml'
        'run:run one command in this terminal'
        'start:run one command in the background'
        'stop:end a command that was started'
        'status:show which commands are running'
        'logs:listen to what a started command writes'
        'completion:print a completion script for bash, zsh or fish'
        'iamllm:print what a language model needs to know about Runbook'
    )

    _arguments -C \
        '(-h --help)'{-h,--help}'[print this help and exit]' \
        '(-f --file)'{-f,--file}'[runbook.yml to work on]:file:_files' \
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
                run|start|stop|logs)
                    # _describe wants name:description, list gives name<tab>
                    # description, so the first tab of each line becomes a colon.
                    local -a names
                    names=(${${(f)"$($prog list 2>/dev/null)"}/$'\t'/:})
                    (( CURRENT == 2 )) && _describe 'command' names
                    ;;
            esac
            ;;
    esac
}
compdef _runbook runbook
`

const fishCompletion = `# runbook completion for fish

# The words typed so far, flags and their values left out, so a completion can
# tell "runbook run <TAB>" from "runbook run api <TAB>".
function __runbook_seen
    set -l skip 0
    for word in (commandline -opc)[2..-1]
        if test $skip -eq 1
            set skip 0
        else if contains -- $word -f --file
            set skip 1
        else if not string match -q -- '-*' $word
            echo $word
        end
    end
end

# True while the argument of one of the given commands is being typed.
function __runbook_argument_of
    set -l seen (__runbook_seen)
    test (count $seen) -eq 1
    and contains -- "$seen[1]" $argv
end

# The command names, asked of the very runbook being typed. Each line is a name
# and, behind a tab, the description fish shows beside it.
function __runbook_names
    set -l prog (commandline -opc)[1]
    $prog list 2>/dev/null
end

complete -c runbook -f
complete -c runbook -s h -l help -d 'print this help and exit'
complete -c runbook -s f -l file -r -F -d 'runbook.yml to work on'
complete -c runbook -n 'test (count (__runbook_seen)) -eq 0' -a list \
    -d 'print the name of every command in the runbook.yml'
complete -c runbook -n 'test (count (__runbook_seen)) -eq 0' -a run \
    -d 'run one command in this terminal'
complete -c runbook -n 'test (count (__runbook_seen)) -eq 0' -a start \
    -d 'run one command in the background'
complete -c runbook -n 'test (count (__runbook_seen)) -eq 0' -a stop \
    -d 'end a command that was started'
complete -c runbook -n 'test (count (__runbook_seen)) -eq 0' -a status \
    -d 'show which commands are running'
complete -c runbook -n 'test (count (__runbook_seen)) -eq 0' -a logs \
    -d 'listen to what a started command writes'
complete -c runbook -n 'test (count (__runbook_seen)) -eq 0' -a completion \
    -d 'print a completion script for bash, zsh or fish'
complete -c runbook -n 'test (count (__runbook_seen)) -eq 0' -a iamllm \
    -d 'print what a language model needs to know about Runbook'
complete -c runbook -n '__runbook_argument_of completion' \
    -a 'bash zsh fish' -d shell
complete -c runbook -n '__runbook_argument_of run start stop logs' \
    -a '(__runbook_names)' -d command
`
