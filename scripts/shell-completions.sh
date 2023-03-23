#!/bin/sh

BINARY=avalanche

# the first argument is an optional custom installation location,
# if not set, try via which (mostly applies if called standalone)
if [ -z $1 ]; then
  FULLBIN=$(which $BINARY)
  if [ ! -z $FULLBIN ]; then
   # which found something, so we need to extract its path directory
   BINDIR=$(dirname $FULLBIN)
  fi

  if [ -z $BINDIR ]; then
    echo "No avalanche binary found, did you add the avalanche binary to your PATH variable?"
    echo "Nothing changed, exiting"
    exit 1
  fi
else
  # the script has been called with the binary location
  # (normal workflow if called via the install script)
  BINDIR=$1
fi

sed_in_place() {
  expr=$1
  file=$2
  if [ $(uname) = Darwin ]
  then
    sed -i "" "$expr" "$file"
  else
    sed -i "$expr" "$file"
  fi
}

completions() {
  echo "Installing shell completion scripts"
  BASH_COMPLETION_MAIN=~/.bash_completion
  BASH_COMPLETION_SCRIPTS_DIR=~/.local/share/bash-completion/completions
  BASH_COMPLETION_SCRIPT_PATH=$BASH_COMPLETION_SCRIPTS_DIR/avalanche.sh
  mkdir -p $BASH_COMPLETION_SCRIPTS_DIR
  $BINDIR/$BINARY completion bash > $BASH_COMPLETION_SCRIPT_PATH
  touch $BASH_COMPLETION_MAIN
  sed_in_place "/.*# avalanche completion/d" $BASH_COMPLETION_MAIN
  echo "source $BASH_COMPLETION_SCRIPT_PATH # avalanche completion" >> $BASH_COMPLETION_MAIN
  if [ $(uname) = Darwin ]
  then
      BREW_INSTALLED=false
      which brew >/dev/null 2>&1 && BREW_INSTALLED=true
      if [ $BREW_INSTALLED = true ]
      then
          BASHRC=~/.bashrc
          touch $BASHRC
          sed_in_place "/.*# avalanche completion/d" $BASHRC
          echo "source $(brew --prefix)/etc/bash_completion # avalanche completion" >> $BASHRC
      else 
          echo "warning: brew not found on macos. bash avalanche command completion not installed"
      fi
  fi

  ZSH_COMPLETION_MAIN=~/.zshrc
  ZSH_COMPLETION_SCRIPTS_DIR=~/.local/share/zsh-completion/completions
  ZSH_COMPLETION_SCRIPT_PATH=$ZSH_COMPLETION_SCRIPTS_DIR/_avalanche
  mkdir -p $ZSH_COMPLETION_SCRIPTS_DIR
  $BINDIR/$BINARY completion zsh > $ZSH_COMPLETION_SCRIPT_PATH
  touch $ZSH_COMPLETION_MAIN
  sed_in_place "/.*# avalanche completion/d" $ZSH_COMPLETION_MAIN
  echo "fpath=($ZSH_COMPLETION_SCRIPTS_DIR \$fpath) # avalanche completion" >> $ZSH_COMPLETION_MAIN
  echo "rm -f ~/.zcompdump; compinit # avalanche completion" >> $ZSH_COMPLETION_MAIN
  echo "Done"
}

completions
