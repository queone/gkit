#!/usr/bin/env bash
# bashrc_user.sh 1.3.0
# Generic interactive bash settings for a user account on macOS

[[ $- == *i* ]] || return # skip all of this for non-interactive shells

# Command history stays in memory: nothing is written to disk, so no ~/.bash_history
export HISTFILE=
# Terminal.app creates ~/.bash_sessions before this file runs and hooks the exit trap
# to save into it; drop both so the directory never comes back
trap - EXIT
rmdir ~/.bash_sessions 2>/dev/null

# XDG base directories; tools that honor them keep their files out of the home root
export XDG_CONFIG_HOME="$HOME/.config"
export XDG_DATA_HOME="$HOME/.local/share"
export XDG_STATE_HOME="$HOME/.local/state"
export XDG_CACHE_HOME="$HOME/.cache"
# Tools that need their own variable to stay out of the home root
export RUSTUP_HOME="$XDG_DATA_HOME/rustup"
export CARGO_HOME="$XDG_DATA_HOME/cargo"
export CODEX_HOME="$XDG_DATA_HOME/codex"
export LIMA_HOME="$XDG_DATA_HOME/lima"
export COLIMA_HOME="$XDG_DATA_HOME/colima"
export CLAUDE_CONFIG_DIR="$XDG_CONFIG_HOME/claude"
export AZURE_CONFIG_DIR="$XDG_CONFIG_HOME/azure"
export DOCKER_CONFIG="$XDG_CONFIG_HOME/docker"
export MPLCONFIGDIR="$XDG_CONFIG_HOME/matplotlib"
export npm_config_cache="$XDG_CACHE_HOME/npm"
export LESSHISTFILE="$XDG_STATE_HOME/lesshst"
export GOPATH=~/.go
export PATH="$PATH:/usr/local/bin:$GOPATH/bin:$HOME/.local/bin:$CARGO_HOME/bin"

Red='\[\e[1;31m\]' Blu='\[\e[1;34m\]' Mag='\[\e[0;35m\]'
Grn='\[\e[1;32m\]' Yel='\[\e[1;33m\]' Rst='\[\e[0m\]'
if [[ -f "$XDG_CONFIG_HOME/bash/gitbranch.sh" ]]; then
    source "$XDG_CONFIG_HOME/bash/gitbranch.sh"
    PS1="${Grn}\h \W${Rst} \$(git_branch '${Yel}%s${Rst} ')$ "
else
    PS1="${Grn}\h \W${Rst}$ "
fi

export BASH_SILENCE_DEPRECATION_WARNING=1
export HOMEBREW_NO_ANALYTICS=1 # Disable homebrew Google Analytics collection
export HISTCONTROL=ignoreboth
export HISTIGNORE='ls:cd:ll:h'
export EDITOR=vi
alias grep='grep --color=auto'
alias ls='gls -N --color --group-directories-first'
alias ll='ls -la'
alias h='history'
alias vi='vim'
alias vs='/Applications/VSCodium.app/Contents/Resources/app/bin/codium -r'
alias ipa="ifconfig | grep 'inet ' | grep -v 127.0.0.1 | cut -d' ' -f2"
alias d='date +"%Y-$(date +%b | tr A-Z a-z)-%d %a %I:%M$(date +%p | tr A-Z a-z | cut -c1)" | tee >(pbcopy)'
alias vm='limactl'
alias tagv='git tag --sort=-v:refname | head -1'

# Private settings (tokens, tenant IDs, account aliases) live outside this file
[[ -f ~/.bashrc.local ]] && source ~/.bashrc.local
