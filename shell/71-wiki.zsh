# WikiPedik project-memory helpers

_wiki_vault_root() {
  print -r -- "$HOME/Desktop/WikiPedik"
}

_wiki_memory_root() {
  print -r -- "$HOME/Desktop/WikiPedik/dev"
}

_wiki_scope_path() {
  local scope=$1
  local root="$(_wiki_memory_root)"
  local co prod repo
  local -a parts

  if [ -z "$scope" ]; then
    return 1
  fi

  parts=("${(@s:/:)scope}")
  co="${parts[1]:-}"
  prod="${parts[2]:-}"
  repo="${parts[3]:-}"

  case ${#parts[@]} in
    1) print -r -- "$root/20-projects/$co" ;;
    2) print -r -- "$root/20-projects/$co/$prod" ;;
    3) print -r -- "$root/20-projects/$co/$prod/repos/$repo" ;;
    *) return 1 ;;
  esac
}

_wiki_scope_company() {
  local scope=$1
  local -a parts

  parts=("${(@s:/:)scope}")
  print -r -- "${parts[1]:-}"
}

_wiki_company_config_value() {
  local co=$1
  local key=$2
  local cfg="$HOME/Desktop/Prokectfiles/$co/.company-config"

  [ -f "$cfg" ] || return 0
  awk -F':[[:space:]]*' -v key="$key" '$1 == key { print $2; exit }' "$cfg"
}

_wiki_scope_product_log() {
  local scope=$1
  local root="$(_wiki_memory_root)"
  local -a parts

  parts=("${(@s:/:)scope}")
  if [ ${#parts[@]} -ge 2 ]; then
    print -r -- "$root/20-projects/${parts[1]}/${parts[2]}/log.md"
  fi
}

_wiki_status_count() {
  local dir=$1 wanted=$2
  if [ ! -d "$dir" ]; then
    print -r -- "0"
    return 0
  fi
  find "$dir" -name _inbox.md -type f -print0 \
    | xargs -0 rg -c "^Status: $wanted$" 2>/dev/null \
    | awk -F: '{sum += $NF} END {print sum + 0}'
}

_wiki_hot_stamp() {
  local dir=$1
  if [ ! -d "$dir" ]; then
    return 0
  fi
  find "$dir" -name hot.md -type f -print0 \
    | xargs -0 rg -n "<!-- last refreshed:" 2>/dev/null \
    | sed -n '1,5p'
}

_wiki_git_dirty() {
  local root="$(_wiki_vault_root)"
  git -C "$root" status --porcelain 2>/dev/null
}

_wiki_dirty_outside_scope() {
  local scope=$1
  local git_root="$(_wiki_vault_root)"
  local memory_root="$(_wiki_memory_root)"
  local dir rel product_log product_log_rel root_index_rel line path

  dir="$(_wiki_scope_path "$scope")" || {
    _wiki_git_dirty
    return 0
  }
  rel="${dir#$git_root/}"
  product_log="$(_wiki_scope_product_log "$scope")"
  product_log_rel="${product_log#$git_root/}"
  root_index_rel="${memory_root#$git_root/}/20-projects/index.md"

  while IFS= read -r line; do
    [ -z "$line" ] && continue
    path="${line[4,-1]}"
    case "$line" in
      R*|C*) path="${path##* -> }" ;;
    esac

    case "$path" in
      "$rel"|"$rel"/*) ;;
      "$product_log_rel") ;;
      "$root_index_rel") ;;
      *) print -r -- "$line" ;;
    esac
  done <<< "$(_wiki_git_dirty)"
}

_wiki_preflight() {
  local scope=$1
  local dir
  dir="$(_wiki_scope_path "$scope")" || {
    echo "Usage: wiki sync [--commit] [--push] <company[/product[/repo]]>"
    return 1
  }

  if [ ! -d "$dir" ]; then
    echo "⚠ Wiki scope not found: $scope"
    echo "  Expected: $dir"
    echo "  Run: wiki bootstrap ${scope//\// }"
    return 1
  fi

  echo "─── Wiki sync preflight ───"
  echo "Scope: $scope"
  echo "Path:  ${dir/#$HOME/~}"
  echo "Pending inbox: $(_wiki_status_count "$dir" candidate) candidate"
  echo "Drained inbox:  $(_wiki_status_count "$dir" drained) drained"

  local stamp
  stamp="$(_wiki_hot_stamp "$dir")"
  if [ -n "$stamp" ]; then
    echo "Hot cache:"
    print -r -- "$stamp" | sed 's/^/  /'
  else
    echo "Hot cache: none"
  fi

  local dirty
  dirty="$(_wiki_git_dirty)"
  if [ -n "$dirty" ]; then
    echo "Git status before: dirty"
    print -r -- "$dirty" | sed -n '1,20p' | sed 's/^/  /'
  else
    echo "Git status before: clean"
  fi
  echo ""
}

_wiki_postflight() {
  local scope=$1
  local dir
  dir="$(_wiki_scope_path "$scope")" || return 1

  echo ""
  echo "─── Wiki sync summary ───"
  echo "Scope: $scope"
  echo "Pending inbox: $(_wiki_status_count "$dir" candidate) candidate"
  echo "Drained inbox:  $(_wiki_status_count "$dir" drained) drained"
  echo "Rejected inbox: $(_wiki_status_count "$dir" rejected) rejected"

  local root="$(_wiki_vault_root)"
  local changed
  changed="$(git -C "$root" status --short 2>/dev/null)"
  if [ -n "$changed" ]; then
    echo ""
    echo "Changed files:"
    print -r -- "$changed" | sed -n '1,80p' | sed 's/^/  /'
  else
    echo ""
    echo "Changed files: none"
  fi
}

_wiki_commit() {
  local scope=$1 push=$2
  local root="$(_wiki_vault_root)"
  local memory_root="$(_wiki_memory_root)"
  local dir rel product_log product_log_rel dirty dirty_outside
  local co git_email

  dir="$(_wiki_scope_path "$scope")" || {
    echo "Usage: wiki-commit <company[/product[/repo]]>"
    return 1
  }

  dirty="$(_wiki_git_dirty)"
  if [ -z "$dirty" ]; then
    echo "Wiki commit: nothing to commit"
    return 0
  fi

  dirty_outside="$(_wiki_dirty_outside_scope "$scope")"
  if [ -n "$dirty_outside" ]; then
    echo "Refusing wiki commit: dirty files exist outside scope '$scope'."
    print -r -- "$dirty_outside" | sed 's/^/  /'
    return 1
  fi

  rel="${dir#$root/}"
  git -C "$root" add -A -- "$rel"

  # Root project index is auto-managed by bootstrap and is safe to commit with
  # any project-memory scope. It contains only company links, not repo details.
  if [ -f "$memory_root/20-projects/index.md" ]; then
    git -C "$root" add -A -- "${memory_root#$root/}/20-projects/index.md"
  fi

  product_log="$(_wiki_scope_product_log "$scope")"
  if [ -n "$product_log" ]; then
    product_log_rel="${product_log#$root/}"
    git -C "$root" add -A -- "$product_log_rel"
  fi

  co="$(_wiki_scope_company "$scope")"
  git_email="$(_wiki_company_config_value "$co" git_email)"

  if [ -n "$git_email" ]; then
    GIT_AUTHOR_EMAIL="$git_email" \
    GIT_COMMITTER_EMAIL="$git_email" \
      git -C "$root" commit -m "docs(wiki): синхронизировать память $scope" || return $?
  else
    git -C "$root" commit -m "docs(wiki): синхронизировать память $scope" || return $?
  fi

  if [ "$push" = "1" ]; then
    git -C "$root" push
  else
    echo "Wiki commit created. Push manually with:"
    echo "  cd ~/Desktop/WikiPedik && git push"
  fi
}

wiki-bootstrap-product() {
  zsh "$HOME/dotfiles/skills-stash/wiki/scripts/wiki-bootstrap-product.sh" "$@"
}

_wiki_curator() {
  local skill=$1
  local scope="${2:-}"
  local instruction="${3:-}"
  local mode="${4:-interactive}"

  if [ -z "$scope" ]; then
    echo "Usage: wiki {sync|status|synthesize} <company[/product[/repo]]>"
    return 1
  fi

  (
    cd "$HOME/Desktop/WikiPedik/dev" || return 1
    if [ -t 0 ]; then
      CODEX_HOME="$HOME/.codex-wiki" codex --no-alt-screen "Use skill $skill. Scope: $scope. $instruction"
    elif [ "$mode" = "readonly" ]; then
      CODEX_HOME="$HOME/.codex-wiki" codex exec "Use skill $skill. Scope: $scope. $instruction"
    else
      echo "⚠ wiki $skill needs an interactive terminal for confirmation."
      echo "  Re-run from your shell, not from a non-interactive command."
      return 1
    fi
  )
}

wiki-sync() {
  local do_commit=0 do_push=0
  while [ $# -gt 0 ]; do
    case "$1" in
      --commit) do_commit=1; shift ;;
      --push) do_commit=1; do_push=1; shift ;;
      --help|-h)
        echo "Usage: wiki sync [--commit] [--push] <company[/product[/repo]]>"
        return 0
        ;;
      *) break ;;
    esac
  done

  local scope="${1:-}"
  if [ -z "$scope" ]; then
    echo "Usage: wiki sync [--commit] [--push] <company[/product[/repo]]>"
    return 1
  fi

  local dirty_before dirty_outside_before
  dirty_before="$(_wiki_git_dirty)"
  dirty_outside_before="$(_wiki_dirty_outside_scope "$scope")"
  _wiki_preflight "$scope" || return 1

  _wiki_curator "inbox-drain" "$scope" \
    "Drain worker captures into curated WikiPedik pages. Preserve company boundaries. For each candidate, propose the target and ask before applying. Update repo index, product log, and hot.md when accepted."
  local rc=$?

  _wiki_postflight "$scope"

  if [ $rc -ne 0 ]; then
    echo "wiki sync failed with exit code $rc"
    return $rc
  fi

  if [ $do_commit -eq 1 ]; then
    if [ -n "$dirty_outside_before" ]; then
      echo ""
      echo "Refusing auto-commit: WikiPedik had dirty files outside this scope before sync."
      echo "Review manually:"
      echo "  cd ~/Desktop/WikiPedik && git status"
      return 1
    fi
    if [ -n "$dirty_before" ]; then
      echo ""
      echo "Auto-commit note: pre-existing dirty files were inside scope and are treated as inbox input."
    fi
    _wiki_commit "$scope" "$do_push"
  fi
}

wiki-status() {
  local scope="${1:-}"
  if [ -z "$scope" ]; then
    echo "Usage: wiki status <company[/product[/repo]]>"
    return 1
  fi

  _wiki_preflight "$scope" || return 1
  _wiki_curator "wiki-status" "$scope" \
    "Report inbox counts, curated pages, synthesis candidates, stale health files, recent log entries, and recommended next actions. Do not modify files." \
    "readonly"
  local rc=$?
  _wiki_postflight "$scope"
  return $rc
}

wiki-synthesize() {
  _wiki_curator "wiki-synthesize" "${1:-}" \
    "Review synthesis candidates and repeated repo lessons/gotchas. Propose cross-repo product patterns before writing product shared pages."
}

wiki-git() {
  local root="$(_wiki_vault_root)"
  (cd "$root" && git "$@")
}

wiki-commit() {
  local scope="${1:-}"
  if [ -z "$scope" ]; then
    echo "Usage: wiki-commit <company[/product[/repo]]>"
    return 1
  fi
  _wiki_commit "$scope" "0"
}
