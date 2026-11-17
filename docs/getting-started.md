# getting started

wraith runs on linux. build it with go 1.27 or later. you need one configured model backend; ollama is the default. neovim and zsh are defaults, not requirements. `wl-copy` enables system clipboard yanks on wayland.

## install from source

```sh
git clone https://github.com/erdajt/wraith.git
cd wraith
make install
export PATH="$HOME/.local/bin:$PATH"
```

add that path to your shell configuration if it is not already present. `make build` writes `bin/wraith` without installing. updating an existing checkout uses the same build and install steps. restart running instances to use the new binary.

## choose a backend

for ollama, pull the model you want before starting wraith:

```sh
ollama pull hf.co/filvyb/Qwen3.5-9B-heretic-v2-GGUF:Q5_K_M
wraith
```

this is the configured default, not a hardware recommendation. `/models` lists installed models. choosing one loads it immediately. wraith starts `ollama serve` only when the configured plain-http loopback connection is refused. an existing server is reused and stays running when wraith exits.

for another installed model:

```sh
wraith --model your-installed-model
```

for an api, edit `~/.config/wraith/config.yaml` and set a profile's model and credentials. for an installed cli, configure its command and authenticate it separately. [providers](providers.md) covers each transport, model selection, reasoning and profile options.

```sh
wraith --provider codex
wraith --provider claude
wraith --provider openai --model your-model-id
```

## first conversation

1. launch wraith in the directory where the work belongs.
2. type a prompt. `ctrl+s` sends it; enter inserts a newline.
3. press escape for normal mode. `i` returns to insert mode; `:w` sends.
4. press tab to focus the conversation. use `j/k`, counts, search and visual selection.
5. type `/settings` in insert mode to configure the app. enter selects a completion without a separate send.
6. quit with normal-mode `q` or global `ctrl+q`.

```sh
wraith review the certificate configuration in this project
```

positional text becomes a draft. it is not sent automatically. flags must precede that text.

## set your editor and shell

```text
/set editor ["nvim"]
/set shell ["bash", "-i"]
/set clipboard ["wl-copy"]
```

commands are executable-and-argument arrays. `ctrl+e` opens the focused draft or message in your editor. save and exit to apply it. saving a prompt does not send it. an approved command's editor has different semantics: save and exit authorizes that edited command to run once.

## run a command

agent mode is the default. the model can propose a concrete command with a reason. review it in the approval panel and choose run, hide output, edit, modify, or reject.

approved agent commands run in the background. `/jobs` shows live status and output; `/history` keeps saved results. the agent receives a job id and bounded evidence unless you hide the output. it can search and page that output by id. [execution](execution.md) explains approval, cancellation and sharing.

ordinary code blocks are suggestions. use `/blocks` or enter on a command in the conversation to edit or run one. a command appearing in text does not execute it.

## keep context

```text
/remember include verification steps and state uncertainty
/pin scope: the isolated local lab only
/set environment.scope directory
/env set TARGET_DOMAIN=lab.example # isolated lab service
```

instructions apply to every chat. pins belong to the current conversation. private variables give the model a name and description while resolving their values locally. [context](context.md) and [private variables](private-variables.md) explain persistence and limits.

## return later

```sh
wraith --resume
wraith --resume SESSION_ID
```

the picker lists conversations from the current directory. `/resume` does the same inside the app. `/clear` starts fresh while retaining the previous conversation. [sessions](sessions.md) covers resume, forks, export and recovery.

## verify the setup

```sh
wraith doctor
wraith config
wraith models
```

`doctor` checks configured tools, model discovery and thinking support. `config` prints the configuration path. for ollama gpu placement, run `ollama ps` during generation. wraith does not force gpu offload.

`wraith --demo` opens a read-only sample without a model request. it does not test your connection, run shells or save changes. use [troubleshooting](troubleshooting.md) when real startup, copying or execution fails.
